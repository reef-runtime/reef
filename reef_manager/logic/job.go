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

type JobManagerT struct {
	Compiler        *CompilerManager
	Nodes           LockedMap[NodeID, Node]
	NonFinishedJobs LockedMap[JobID, Job]
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

type Job struct {
	Data             database.Job
	WorkerNodeID     *NodeID
	Progress         float32
	Logs             []database.JobLog
	InterpreterState []byte
}

//
// TODO: move into API
//

type ApiJob struct {
	Data     database.Job `json:"data"`
	Progress float32      `json:"progress"`
}

func (m *JobManagerT) ListJobs() ([]ApiJob, error) {
	dbJobs, err := database.ListJobs()
	if err != nil {
		return nil, err
	}

	jobs := make([]ApiJob, len(dbJobs))
	for idx, data := range dbJobs {
		var progress float32 = 1.0

		if data.Status != database.StatusDone {
			runningJob, found := m.NonFinishedJobs.Get(data.ID)

			// If not found: data race: job finished in between function calls.
			// If not, use real progress from job.
			if found {
				progress = runningJob.Progress
			}
		}

		jobs[idx] = ApiJob{
			Data:     data,
			Progress: progress,
		}
	}

	return jobs, nil
}

func (m *JobManagerT) SubmitJob(
	language JobProgrammingLanguage,
	sourceCode string,
	name string,
) (idString string, compilerErr *string, backendErr error) {
	now := time.Now()

	// Create a hash for the ID.
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(struct {
		Language   JobProgrammingLanguage
		SourceCode string
		Name       string
	}{
		Language:   language,
		SourceCode: sourceCode,
		Name:       name,
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
		return "", nil, err
	}
	if compileErr != nil {
		return "", compileErr, nil
	}

	job := Job{
		Data: database.Job{
			ID:        idString,
			Name:      name,
			Submitted: now,
			Status:    database.StatusQueued,
			WasmID:    artifact.Hash,
		},
		Progress:         0,
		Logs:             make([]database.JobLog, 0),
		InterpreterState: nil,
		WorkerNodeID:     nil,
	}

	if backendErr = database.AddJob(job.Data); backendErr != nil {
		return "", nil, backendErr
	}

	m.NonFinishedJobs.Insert(idString, job)

	return idString, nil, nil
}

// Places a job back into queued.
// Would be called if a node disconnects while a job runs on this very node.
func (m *JobManagerT) ParkJob(jobID string) error {
	job, found := m.NonFinishedJobs.Get(jobID)
	if !found {
		return fmt.Errorf("could not park job `%s`: job not found", jobID)
	}

	if job.Data.Status == database.StatusQueued {
		log.Tracef("Park: found job `%s` but it is already in <queued> status", jobID)
		return nil
	}

	// Put the job back into the queued status.
	found, err := m.setJobStatus(jobID, database.StatusQueued)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("park: job `%s` not found", jobID)
	}

	// Set worker node ID to `nil`.
	m.NonFinishedJobs.Lock.Lock()
	oldJob, found := m.NonFinishedJobs.Map[jobID]
	if found {
		newJob := Job{
			Data:             oldJob.Data,
			Progress:         oldJob.Progress,
			Logs:             oldJob.Logs,
			InterpreterState: oldJob.InterpreterState,
			WorkerNodeID:     nil,
		}
		m.NonFinishedJobs.Map[jobID] = newJob
	}

	m.NonFinishedJobs.Lock.Unlock()
	log.Debugf("[job] Parked ID `%s`", jobID)

	return nil
}

func (m *JobManagerT) setJobStatus(jobID string, state database.JobStatus) (bool, error) {
	return m.setJobStatusLOCK(jobID, state, true)
}

func (m *JobManagerT) setJobStatusLOCK(jobID string, state database.JobStatus, lock bool) (bool, error) {
	found, err := database.ModifyJobStatus(jobID, state)
	if err != nil {
		return false, err
	}

	if !found {
		return false, fmt.Errorf("modify job '%s' status: job is not found", jobID)
	}

	if lock {
		m.NonFinishedJobs.Lock.Lock()
	}

	oldJob, found := m.NonFinishedJobs.Map[jobID]

	if lock {
		m.NonFinishedJobs.Lock.Unlock()
	}

	if !found {
		return false, nil
	}
	oldJob.Data.Status = state
	m.NonFinishedJobs.Map[jobID] = oldJob

	return true, nil
}

func (m *JobManagerT) Init() error {
	// Initialize 'priority queue'.
	queuedJobs, err := database.ListJobsFiltered([]database.JobStatus{
		database.StatusQueued,
		database.StatusRunning,
		database.StatusStarting,
	})
	if err != nil {
		return err
	}

	for _, dbJob := range queuedJobs {
		log.Tracef("Loaded saved job from DB as queued: `%s`", dbJob.ID)

		// Put job back in queued state.
		job := Job{
			Data: database.Job{
				ID:        dbJob.ID,
				Name:      dbJob.Name,
				WasmID:    dbJob.WasmID,
				Submitted: dbJob.Submitted,
				Status:    database.StatusQueued,
			},
			Progress:         0,
			Logs:             make([]database.JobLog, 0),
			InterpreterState: nil,
			WorkerNodeID:     nil,
		}

		m.NonFinishedJobs.Insert(dbJob.ID, job)

		found, err := database.ModifyJobStatus(dbJob.ID, database.StatusQueued)
		if err != nil {
			return fmt.Errorf("change job state in restore: %s", err.Error())
		}

		if !found {
			panic("Impossible: job cannot disappear at this point")
		}
	}

	// Launch daemon.
	go m.JobQueueDaemon()

	return nil
}

func (m *JobManagerT) DeleteJob(jobID string) (found bool, err error) {
	_, found, err = database.GetJob(jobID)
	if err != nil || !found {
		return found, err
	}

	// Remove the job from the queue and database.
	m.NonFinishedJobs.Delete(jobID)

	found, err = database.DeleteJob(jobID)
	if err != nil || !found {
		return found, err
	}

	return true, nil
}
