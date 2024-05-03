package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

type JobSubmission struct {
	// TODO: add other required fields (referenced code + dataset)
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

func SubmitJob(submission JobSubmission) (newID string, err error) {
	now := time.Now().Local()

	// Create a hash for the ID.
	// TODO: use correct hash input.
	idBinary := sha256.Sum256(append([]byte(now.String()), []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}...))
	newID = hex.EncodeToString(idBinary[0:len(idBinary)])

	if err = database.AddJob(database.Job{
		ID:        newID,
		Name:      submission.Name,
		Submitted: now,
		Status:    database.StatusQueued,
	}); err != nil {
		return "", err
	}

	return newID, nil
}
