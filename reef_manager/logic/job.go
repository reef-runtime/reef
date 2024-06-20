package logic

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/reef-runtime/reef/reef_manager/database"
)

const jobDaemonFreq = time.Second * 5

//
// Job manager.
//

type JobManagerT struct {
	JobQueue JobQueue
	Compiler *CompilerManager

	Nodes NodeMap
}

var JobManager JobManagerT

//
// End job manager.
//

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

type JobSubmission struct {
	Name string `json:"name"`
	// Attaching a dataset to a job submission is optimal.
	DatasetID  *string                `json:"datasetId"`
	SourceCode string                 `json:"sourceCode"`
	Language   JobProgrammingLanguage `json:"language"`
}

type QueuedJob struct {
	Job          database.Job
	WasmArtifact []byte
}

// func newQueuedJob(job database.Job) queuedJob {
// 	return queuedJob{
// 		Job: job,
// 	}
// }

// Implements Prioritizable.
func (j QueuedJob) submittedAt() time.Time {
	return j.Job.Submitted
}

func (j QueuedJob) IsHigherThan(other prioritizable) bool {
	otherJob := other.(QueuedJob)
	return j.submittedAt().Before(otherJob.submittedAt())
}

type Job struct {
	Data database.Job `json:"data"`
	// TODO: Other fields can follow.
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

func (m *JobManagerT) SubmitJob(submission JobSubmission) (newID string, compilerErr *string, backendErr error) {
	now := time.Now()

	// Create a hash for the ID.
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(submission); err != nil {
		return "", nil, err
	}

	idBinary := sha256.Sum256(append(
		[]byte(now.String()),
		buffer.Bytes()...,
	))

	newID = hex.EncodeToString(idBinary[0:])

	artifact, compileErr, err := m.Compiler.Compile(submission.Language, submission.SourceCode)
	if err != nil {
		return "", nil, err
	}
	if compileErr != nil {
		return "", compileErr, nil
	}

	fmt.Println(spew.Sdump(artifact.Wasm)[0:1000])

	job := database.Job{
		ID:        newID,
		Name:      submission.Name,
		Submitted: now,
		Status:    database.StatusQueued,
		WasmID:    artifact.Hash,
	}

	if backendErr = database.AddJob(job); backendErr != nil {
		return "", nil, backendErr
	}

	m.JobQueue.Push(job, artifact.Wasm)

	return newID, nil, nil
}

// Can only be used while the job is queued.
func (m *JobManagerT) AbortJob(jobID string) (found bool, err error) {
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
	if !m.JobQueue.Delete(jobID) {
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
	job, found, err := database.GetJob(jobID)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("could not park job `%s`: job not found", jobID)
	}

	if job.Status == database.StatusQueued {
		log.Tracef("Found job `%s` but it is already in <queued> state", jobID)
		return nil
	}

	found, err = database.ModifyJobStatus(jobID, database.StatusQueued)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("could not park job `%s`: job not found during modification", jobID)
	}

	// Put the job back into the JPQ.
	job.Status = database.StatusQueued

	// Load the artifact from storage again.
	artifact, err := m.Compiler.getCached(job.WasmID)
	if err != nil {
		log.Errorf("failed to park job `%s`: could not load job's Wasm from cache", err.Error())
		return err
	}

	m.JobQueue.Push(job, artifact)

	log.Debugf("[job] Parked ID `%s`", jobID)

	return nil
}

// func (m *JobManagerT) SetJobState(jobID JobID, newState database.JobStatus) (found bool) {
// 	database.ModifyJobStatus(jobID, newState)
// 	return true
// }

func (m *JobManagerT) init() error {
	// Initialize priority queue
	queuedJobs, err := database.ListJobsFiltered(database.StatusQueued)
	if err != nil {
		return err
	}

	for _, job := range queuedJobs {
		log.Tracef("Loaded saved queued job from DB: `%s`", job.ID)

		// Load artifact bytes from storage.
		artifact, err := m.Compiler.getCached(job.WasmID)
		if err != nil {
			log.Errorf("Could not restore saved jobs: load cached Wasm: %s", err.Error())
			return err
		}

		m.JobQueue.Push(job, artifact)
	}

	return nil
}

func SaveResult(jobID string, content []byte, contentType database.ContentType) error {
	now := time.Now().Local()

	result := database.Result{
		JobID:       jobID,
		Content:     content,
		ContentType: contentType,
		Created:     now,
	}

	if err := database.SaveResult(result); err != nil {
		return err
	}

	return nil
}

func newJobManager(compiler *CompilerManager) JobManagerT {
	return JobManagerT{
		JobQueue: NewJobQueue(),
		Compiler: compiler,
	}
}

//
// When called, fetches all queued jobs and tries to start them.
//

// Error is a critical error, like a database fault.
func (m *JobManagerT) TryToStartQueuedJobs() error {
	if m.JobQueue.IsEmpty() {
		log.Debugf("Job queue is empty, not starting any jobs.")
		return nil
	}

	notStarted := make([]QueuedJob, 0)

	for !m.JobQueue.IsEmpty() {
		job, found := m.JobQueue.Pop()
		if !found {
			panic("Impossible, this is a bug")
		}

		log.Debugf("Attempting to start job `%s`...", job.Job.ID)

		// TODO: here is the place where we can give the node a previous state to resume from.
		couldStart, err := JobManager.StartJobOnFreeNode(job, nil)
		if err != nil {
			log.Errorf("HARD ERROR: Could not start job `%s`: %s", job.Job.ID, err.Error())
			return err
		}

		if !couldStart {
			log.Debugf("Could not start job `%s`", job.Job.ID)
			notStarted = append(notStarted, job)
			continue
		}

		// TODO: modify in database

		log.Infof("Job `%s` started", job.Job.ID)
	}

	for _, job := range notStarted {
		m.JobQueue.Push(job.Job, job.WasmArtifact)
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
