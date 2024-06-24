package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const JobTableName = "job"
const ResultTableName = "job_result"

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

type ContentType uint16

const (
	StringJSON ContentType = iota
	StringPlain
	Int64
	Bytes
)

// TODO: include runtime of a job.
type Job struct {
	// ID generated by SHA265, (32 * 2 = 64) runes long.
	ID string `json:"id"`
	// Friendly name for this job.
	Name string `json:"name"`

	// Hash of the compiled Wasm artifact.
	WasmID string `json:"wasmId"`

	// Dataset ID of the Job
	DatasetId string `json:"datasetId"`

	Submitted time.Time `json:"submitted"`
	Status    JobStatus `json:"status"`
}

type Result struct {
	Success     bool        `json:"success"`
	JobID       string      `json:"jobId"`
	Content     []byte      `json:"content"`
	ContentType ContentType `json:"contentType"`
	Created     time.Time   `json:"created"`
}

func AddJob(job Job) error {
	if _, err := db.builder.Insert(JobTableName).Values(
		job.ID,
		job.Name,
		job.Submitted,
		job.Status,
		job.WasmID,
		job.DatasetId,
	).Exec(); err != nil {
		log.Errorf("Could not add job to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func ModifyJobStatus(jobID string, newStatus JobStatus) (found bool, err error) {
	_, found, err = GetJob(jobID)
	if err != nil {
		log.Errorf("Could not modify job status: getting job failed: %s", err.Error())
		return found, err
	}

	if !found {
		return false, nil
	}

	_, err = db.builder.Update(JobTableName).Set("status", newStatus).Where("id=?", jobID).Exec()
	if err != nil {
		log.Errorf("Could not modify job status: failed to execute query: %s", err.Error())
	}

	return true, err
}

func DeleteJob(jobID string) (found bool, err error) {
	// Delete all logs and a potential result first.
	if err := DeleteLogs(jobID); err != nil {
		return false, err
	}

	if err := deleteResult(jobID); err != nil {
		return false, err
	}

	res, err := db.builder.Delete(JobTableName).Where("job.id=?", jobID).Exec()
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func ListJobsFiltered(stateFilter []JobStatus) ([]Job, error) {
	return listJobsGeneric(stateFilter)
}

func ListJobs() ([]Job, error) {
	return listJobsGeneric(nil)
}

func listJobsGeneric(stateFilter []JobStatus) ([]Job, error) {
	baseQuery := db.builder.Select("*").From(JobTableName).OrderBy("submitted ASC")

	// Apply optional filter.
	if len(stateFilter) > 0 {
		completeExpr := ""

		for idx, filter := range stateFilter {
			expr := fmt.Sprintf("status=%d", filter)

			if idx > 0 {
				completeExpr += fmt.Sprintf(" OR %s", expr)
			} else {
				completeExpr = expr
			}
		}

		baseQuery = baseQuery.Where(completeExpr)
	}

	res, err := baseQuery.Query()

	if err != nil {
		// nolint:goconst
		log.Errorf("Could not list jobs: executing query failed: %s", err.Error())
		return nil, err
	}

	if res.Err() != nil {
		// nolint:goconst
		log.Errorf("Could not list jobs: executing query failed: %s", res.Err())
		return nil, err
	}

	defer res.Close()

	jobs := make([]Job, 0)

	for res.Next() {
		var job Job
		if err := res.Scan(
			&job.ID,
			&job.Name,
			&job.Submitted,
			&job.Status,
			&job.WasmID,
			&job.DatasetId,
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
		&job.WasmID,
		&job.DatasetId,
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

func SaveResult(result Result) error {
	query := db.builder.Insert(ResultTableName).Values(
		result.JobID,
		result.Content,
		result.ContentType,
		result.Created,
	)

	if _, err := query.Exec(); err != nil {
		log.Errorf("Could not add result to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func GetResult(jobID string) (result Result, found bool, err error) {
	res := db.builder.
		Select("*").From(ResultTableName).
		Where("job_result.job_id=?", jobID).
		QueryRow()
	err = res.Scan(
		&result.JobID,
		&result.Content,
		&result.ContentType,
		&result.Created,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}

	if err != nil {
		log.Errorf("Could not get result for job (%s): executing query failed: %s", jobID, err.Error())
		return result, false, err
	}

	return result, true, nil
}

func deleteResult(jobID string) error {
	_, err := db.builder.
		Delete(ResultTableName).
		Where("job_result.job_id=?", jobID).Exec()

	if err != nil {
		log.Errorf("Could not delete job result from database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}
