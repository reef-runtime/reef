package database

import (
	"database/sql"
	"errors"
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
	if _, err := db.builder.Insert(JobTableName).Values(
		job.ID,
		job.Name,
		job.Submitted,
		job.Status,
	).Exec(); err != nil {
		log.Errorf("Could not add job to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func DeleteJob(jobID string) (found bool, err error) {
	res, err := db.builder.Delete(JobTableName).Where("job.ID=?", jobID).Exec()
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func ListJobsFiltered(stateFilter JobStatus) ([]Job, error) {
	return listJobsGeneric(&stateFilter)
}

func ListJobs() ([]Job, error) {
	return listJobsGeneric(nil)
}

func listJobsGeneric(stateFilter *JobStatus) ([]Job, error) {
	baseQuery := db.builder.Select("*").From(JobTableName).OrderBy("submitted ASC")

	// Apply optional filter.
	if stateFilter != nil {
		baseQuery = baseQuery.Where("job.status=?", *stateFilter)
	}

	res, err := baseQuery.Query()
	if err != nil {
		log.Errorf("Could not list jobs: executing query failed: %s", err.Error())
		return nil, err
	}

	jobs := make([]Job, 0)

	for res.Next() {
		var job Job
		if err := res.Scan(
			&job.ID,
			&job.Name,
			&job.Submitted,
			&job.Status,
		); err != nil {
			log.Errorf("Could not list jobs: scanning results failed: %s", err.Error())
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func GetJob(jobID string) (job Job, found bool, err error) {
	res := db.builder.Select("*").From(JobTableName).Where("job.Id=?", jobID).QueryRow()
	err = res.Scan(
		&job.ID,
		&job.Name,
		&job.Submitted,
		&job.Status,
	)

	if errors.Is(err, sql.ErrNoRows) {
		log.Tracef("Could not get job (%s): %s", jobID, err.Error())
		return job, false, nil
	}

	if err != nil {
		log.Errorf("Could not get job (%s): executing query failed: %s", jobID, err.Error())
		return job, false, err
	}

	return job, true, nil
}
