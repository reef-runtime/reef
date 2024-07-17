package logic

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

const jobDaemonFreq = time.Second * 2
const minUIUpdateDelay = time.Second

type JobManagerT struct {
	// Predefined job definitions which can be tweaked / submitted by the user.
	Templates       []Template
	Compiler        *CompilerManager
	Nodes           LockedMap[NodeId, LockedValue[Node]]
	NonFinishedJobs LockedMap[JobId, LockedValue[Job]]
	// Job manager will send data to this channel once something changed.
	SendUIUpdatesTo chan DataCollectionMsg
	// UI manager will send data into this channel once it receives some updates.
	RequestToRefreshData chan WebSocketTopic
	// If a job's active execution time is high that this value, the job is killed.
	MaxJobRuntimeSecs uint64
	// Any nodes with a name contained in that slice will not receive any jobs.
	NodesBlackList []string
}

var JobManager JobManagerT

type JobProgrammingLanguage string

const (
	RustLanguage = "rust"
	CLanguage    = "c"
)

func (l JobProgrammingLanguage) Validate() error {
	switch l {
	case RustLanguage, CLanguage:
		return nil
	default:
		return fmt.Errorf("invalid programming language `%s`", l)
	}
}

type JobStatus uint16

const (
	// Job not started yet, waiting for worker to pick this job.
	StatusQueued JobStatus = iota
	// Worker was selected, worker is initializing (e.g dataset transfer).
	StatusStarting
	// Node has sent that the job is running, waiting for completion.
	StatusRunning
	// Job has exited either successfully or failed with an error.
	// Actual error information is contained in the job result.
	StatusDone
)

type Job struct {
	Data             database.JobTableData
	WorkerNodeID     *NodeId
	Progress         float32
	Status           JobStatus `json:"status"`
	Logs             []database.JobLog
	InterpreterState []byte
	//
	// `RuntimeSeconds += since(LastRuntimeIncrement)`
	// This is executed periodically to keep the runtime of the job up to date..
	//
	LastRuntimeIncrement time.Time
	RuntimeSeconds       uint64
	IsBeingAborted       bool
}

func (m *JobManagerT) SubmitJob(
	language JobProgrammingLanguage,
	sourceCode string,
	name string,
	datasetID string,
	ownerID string,
) (idString string, compilerErr *string, backendErr error) {
	now := time.Now()

	// Create a hash for the Id.
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(struct {
		Language   JobProgrammingLanguage
		SourceCode string
		Name       string
		DatasetID  string
		OwnerID    string
	}{
		Language:   language,
		SourceCode: sourceCode,
		Name:       name,
		DatasetID:  datasetID,
		OwnerID:    ownerID,
	}); err != nil {
		return "", nil, err
	}

	idBinary := sha256.Sum256(append(
		[]byte(now.String()),
		buffer.Bytes()...,
	))

	idString = hex.EncodeToString(idBinary[0:])

	// Check if there is already a result for this job.
	_, found, err := database.GetResult(idString)
	if err != nil {
		return "", nil, fmt.Errorf("get result: %s", err.Error())
	}

	if found {
		log.Infof("Not submitting job internally: job `%s` was already executed on the system", idString)
		return idString, nil, nil
	}

	artifact, compileErr, err := m.Compiler.Compile(language, sourceCode)
	if err != nil {
		return "", nil, fmt.Errorf("compile error: %s", err.Error())
	}
	if compileErr != nil {
		return "", compileErr, nil
	}

	jobTableData := database.JobTableData{
		Id:        idString,
		Name:      name,
		Submitted: now,
		WasmId:    artifact.Hash,
		DatasetId: datasetID,
		Owner:     ownerID,
	}

	if backendErr = database.AddJob(jobTableData); backendErr != nil {
		return "", nil, backendErr
	}

	job := Job{
		Data:                 jobTableData,
		WorkerNodeID:         nil,
		Progress:             0,
		Status:               StatusQueued,
		Logs:                 make([]database.JobLog, 0),
		InterpreterState:     nil,
		LastRuntimeIncrement: time.Now(),
		RuntimeSeconds:       0,
		IsBeingAborted:       false,
	}

	m.NonFinishedJobs.Insert(idString, NewLockedValue(job))

	// Notify UI about state change.
	m.updateAllJobStates()

	return idString, nil, nil
}

// Places a job back into queued.
// Would be called if a node disconnects while a job runs on this very node.
func (m *JobManagerT) ParkJob(jobId string) error {
	job, found := m.NonFinishedJobs.Get(jobId)
	if !found {
		return fmt.Errorf("could not park job `%s`: job not found", jobId)
	}

	job.Lock.RLock()
	jobStatus := job.Data.Status
	job.Lock.RUnlock()

	if jobStatus == StatusQueued {
		log.Tracef("Park: found job `%s` but it is already in <queued> status", jobId)
		return nil
	}

	// Put the job back into the queued status.
	m.setJobStatus(job, StatusQueued)

	if !found {
		return fmt.Errorf("park: job `%s` not found", jobId)
	}

	// Set worker node Id to `nil`.

	job.Lock.Lock()
	job.Data.LastRuntimeIncrement = time.Now()
	job.Data.WorkerNodeID = nil
	job.Lock.Unlock()

	log.Debugf("[job] Parked Id `%s`", jobId)

	// Notify UI about state change.
	m.updateSingleJobState(jobId)

	return nil
}

func (m *JobManagerT) setJobStatus(job LockedValue[Job], status JobStatus) {
	job.Lock.Lock()
	job.Data.Status = status
	job.Lock.Unlock()
}

// This waits for refresh requests and provides the UI manager with new data if it needs it.
func (m *JobManagerT) ListenToRefreshRequests() {
	for msg := range m.RequestToRefreshData {
		switch msg.Kind {
		case WSTopicAllJobs:
			m.updateAllJobStates()
		case WSTopicSingleJob:
			m.updateSingleJobState(msg.Additional)
		case WSTopicNodes:
			m.updateNodeState()
		default:
			panic("A new topic kind was introduced without updating this code")
		}
	}
}

func (m *JobManagerT) ListTemplates() []Template {
	return m.Templates
}

func (m *JobManagerT) Init() error {
	// Initialize 'priority queue', include all jobs at first, also the ones that have already finished.
	// TODO: maybe optimize this so that finished jobs are not included.
	queuedJobs, err := database.ListJobs(nil, nil)
	if err != nil {
		return err
	}

	for _, dbJob := range queuedJobs {
		// Job is already finished, do not run it again.
		if dbJob.Result != nil {
			continue
		}

		log.Tracef("Loaded saved job from DB as queued: `%s`", dbJob.Job.Id)

		// Put job back in queued state.
		job := Job{
			Data: database.JobTableData{
				Id:        dbJob.Job.Id,
				Name:      dbJob.Job.Name,
				WasmId:    dbJob.Job.WasmId,
				DatasetId: dbJob.Job.DatasetId,
				Submitted: dbJob.Job.Submitted,
				Owner:     dbJob.Job.Owner,
			},

			// Set job to queued.
			Progress:             0,
			Status:               StatusQueued,
			Logs:                 make([]database.JobLog, 0),
			InterpreterState:     nil,
			WorkerNodeID:         nil,
			LastRuntimeIncrement: time.Now(),
			RuntimeSeconds:       0,
			IsBeingAborted:       false,
		}

		m.NonFinishedJobs.Insert(dbJob.Job.Id, NewLockedValue(job))
	}

	// Launch job queue daemon.
	go m.JobManagerDaemon()

	// Launch goroutine to listen to data refresh requests.
	go m.ListenToRefreshRequests()

	return nil
}
