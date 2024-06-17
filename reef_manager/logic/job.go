package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

//
// Job manager.
//

type JobManagerT struct {
	JobQueue JobQueue
}

var JobManager JobManagerT

//
// End job manager.
//

type JobSubmission struct {
	// TODO: add other required fields (referenced code + dataset).
	Name string `json:"name"`
}

type queuedJob struct {
	Job database.Job
}

func NewQueuedJob(job database.Job) queuedJob {
	return queuedJob{
		Job: job,
	}
}

// Implements Prioritizable.
func (j queuedJob) submittedAt() time.Time {
	return j.Job.Submitted
}

func (j queuedJob) IsHigherThan(other prioritizable) bool {
	otherJob := other.(queuedJob)
	return j.submittedAt().Before(otherJob.submittedAt())
}

func (m *JobManagerT) SubmitJob(submission JobSubmission) (newID string, err error) {
	now := time.Now().Local()

	// Create a hash for the ID.
	// TODO: use correct hash input.
	idBinary := sha256.Sum256(append([]byte(now.String()), []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}...))
	newID = hex.EncodeToString(idBinary[0:])

	job := database.Job{
		ID:        newID,
		Name:      submission.Name,
		Submitted: now,
		Status:    database.StatusQueued,
	}

	if err = database.AddJob(job); err != nil {
		return "", err
	}

	m.JobQueue.Push(job)

	return newID, nil
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
	m.JobQueue.Push(job)

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
		m.JobQueue.Push(job)
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

func newJobManager() JobManagerT {
	return JobManagerT{
		JobQueue: NewJobQueue(),
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

	notStarted := make([]database.Job, 0)

	for !m.JobQueue.IsEmpty() {
		job, found := m.JobQueue.Pop()
		if !found {
			panic("Impossible, this is a bug")
		}

		log.Debugf("Attempting to start job `%s`...", job.ID)

		// TODO: here is the place where we can give the node a previous state to resume from.
		couldStart, err := NodeManager.StartJobOnFreeNode(job.ID, nil)
		if err != nil {
			log.Errorf("HARD ERROR: Could not start job `%s`: %s", job.ID, err.Error())
			return err
		}

		if !couldStart {
			log.Debugf("Could not start job `%s`", job.ID)
			notStarted = append(notStarted, job)
			continue
		}

		// TODO: modify in database

		log.Infof("Job `%s` started", job.ID)
	}

	for _, job := range notStarted {
		m.JobQueue.Push(job)
	}

	return nil
}

const jobDaemonFreq = time.Second * 5

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
