package logic

import (
	"crypto/sha256"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

type JobSubmission struct {
	// TODO: add other required fields.
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

func SubmitJob(submission JobSubmission) (newID [sha256.Size]byte, err error) {
	now := time.Now().Local()

	// Create a hash for the ID.
	// TODO: use correct hash input.
	newID = sha256.Sum256([]byte{0xde, 0xad, 0xbe, 0xef})

	if err = database.AddJob(database.Job{
		ID:        newID,
		Name:      submission.Name,
		Submitted: now,
		Status:    database.StatusQueued,
	}); err != nil {
		return [sha256.Size]byte{}, err
	}

	return newID, nil
}
