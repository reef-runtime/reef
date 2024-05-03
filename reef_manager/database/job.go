package database

import (
	"crypto/sha256"
	"time"
)

const JobTableName = "job"

type JobStatus uint16

const (
	StatusQueued   JobStatus = iota // Job not started yet, waiting for worker to pick this job.
	StatusStarting                  // Worker was selected, worker is initializing (e.g dataset transfer).
	StatusRunning                   // Job is running, waiting for completion.
	StatusFailed                    // Job could not complete due to errors.
	StatusFinished                  // Job has exited successfully or has failed with an error.
)

type Job struct {
	ID        string    `json:"id"`   // ID generated by SHA265.
	Name      string    `json:"name"` // Friendly name for this job.
	Submitted time.Time `json:"submitted"`
	Status    JobStatus `json:"status"`
}

func AddJob(job Job) error {
	if _, err := db.builder.Insert(JobTableName).Values(job.ID[0:], job.Name).Exec(); err != nil {
		log.Errorf("Could not add job to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func DeleteJob(jobID [sha256.Size]byte) (found bool, err error) {
	res, err := db.builder.Delete(JobTableName).Where("job.ID=", jobID).Exec()
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func ListJobs() ([]Job, error) {
	res, err := db.builder.Select("*").From(JobTableName).Query()

	if err != nil {
		log.Errorf("Could not list jobs: executing query failed: %s", err.Error())
		return nil, err
	}

	jobs := make([]Job, 0)

	for res.Next() {
		var job Job
		if err := res.Scan(&job.ID); err != nil {
			return nil, err
		}
	}

	return jobs, nil
}
