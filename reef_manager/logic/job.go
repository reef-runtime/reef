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
	Progress         float32
	Logs             []database.JobLog
	InterpreterState []byte
}

func (m *JobManagerT) ListJobs() ([]Job, error) {
	dbJobs, err := database.ListJobs()
	if err != nil {
		return nil, err
	}

	jobs := make([]Job, len(dbJobs))
	for idx, data := range dbJobs {
		jobs[idx] = Job{
			Data: data,
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

	// fmt.Println(spew.Sdump(artifact.Wasm)[0:1000])

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
	}

	if backendErr = database.AddJob(job.Data); backendErr != nil {
		return "", nil, backendErr
	}

	// fmt.Println("==================")
	m.NonFinishedJobs.Insert(idString, job)
	// fmt.Println("AFTER ==================")

	return idString, nil, nil
}

// Can only be used while the job is queued.
func (m *JobManagerT) AbortJob(jobID string) (found bool, err error) {
	// TODO: also allow cancel

	panic("TODO: CANCEL")

	job, found, err := database.GetJob(jobID)
	if err != nil || !found {
		return found, err
	}

	// Act as there is no queued job with this id.
	if job.Status != database.StatusQueued {
		log.Tracef("Found job `%s` but it is not in <queued> state\n", jobID)
		return false, nil
	}

	// Remove the job from the queue and database.
	if _, found := m.NonFinishedJobs.Delete(jobID); !found {
		log.Errorf("Internal state corruption: job to be aborted was not in job queue, fixing...")
	}

	found, err = database.DeleteJob(jobID)
	if err != nil || !found {
		return found, err
	}

	return true, nil
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

//
// Fetches all queued jobs and tries to start them.
//

func (m *JobManagerT) QueuedJobs() (j Job, jExists bool) {
	m.NonFinishedJobs.Lock.RLock()

	var earliest Job
	hadEarliest := false

	for _, job := range m.NonFinishedJobs.Map {
		if job.Data.Status != database.StatusQueued {
			continue
		}

		if !hadEarliest || job.Data.Submitted.Before(earliest.Data.Submitted) {
			earliest = job
			hadEarliest = true
		}
	}

	m.NonFinishedJobs.Lock.RUnlock()

	return earliest, hadEarliest
}

// Error is a critical error, like a database fault.
func (m *JobManagerT) TryToStartQueuedJobs() error {
	for {
		fmt.Println("find first")
		first, exists := m.QueuedJobs()
		fmt.Println("has first")

		if !exists {
			log.Trace("Job queue is empty")
			break
		}

		log.Debugf("Attempting to start job `%s`...", first.Data.ID)

		couldStart, err := m.StartJobOnFreeNode(first)
		if err != nil {
			log.Errorf("HARD ERROR: Could not start job `%s`: %s", first.Data.ID, err.Error())
			return err
		}

		if !couldStart {
			log.Debugf("Could not start job `%s`", first.Data.ID)
			break
		}

		log.Infof("Job `%s` started", first.Data.ID)
	}

	return nil
}

func (m *JobManagerT) JobQueueDaemon() {
	log.Info("Job queue daemon is running...")

	for {
		time.Sleep(jobDaemonFreq)
		log.Info("Trying to start all queued jobs...")
		if err := m.TryToStartQueuedJobs(); err != nil {
			panic(err.Error())
		}
	}
}
