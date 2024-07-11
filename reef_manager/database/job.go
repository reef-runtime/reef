package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const JobTableName = "job"
const ResultTableName = "job_result"

type ContentType uint16

const (
	StringJSON ContentType = iota
	StringPlain
	I32
	Bytes
)

type JobTableData struct {
	// Id generated by SHA265, (32 * 2 = 64) runes long.
	Id string `json:"id"`
	// Friendly name for this job.
	Name      string    `json:"name"`
	Submitted time.Time `json:"submitted"`
	// Hash of the compiled Wasm artifact.
	WasmId string `json:"wasmId"`
	// Dataset Id of the job.
	DatasetId string `json:"datasetId"`
	// Owner session ID of the job.
	Owner string `json:"owner"`
}

type JobWithResult struct {
	// Normal data of this job.
	Job JobTableData `json:"job"`
	// Optional result that is `!= nil` once the job is done.
	Result *Result `json:"result"`
}

type Result struct {
	Success     bool        `json:"success"`
	JobId       string      `json:"jobId"`
	Content     []byte      `json:"content"`
	ContentType ContentType `json:"contentType"`
	// This together with `submitted` on the job can be used to calculate the total time required for a job.
	// However, this also includes time spent in the queue.
	Created time.Time `json:"created"`
}

func AddJob(
	data JobTableData,
) error {
	if _, err := db.builder.Insert(JobTableName).Values(
		data.Id,
		data.Name,
		data.Submitted,
		data.WasmId,
		data.DatasetId,
		data.Owner,
	).Exec(); err != nil {
		log.Errorf("Could not add job to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func DeleteJob(jobId string) (found bool, err error) {
	// Delete all logs and a potential result first.
	if err := DeleteLogs(jobId); err != nil {
		return false, err
	}

	if err := deleteResult(jobId); err != nil {
		return false, err
	}

	res, err := db.builder.Delete(JobTableName).Where("job.id=?", jobId).Exec()
	if err != nil {
		log.Errorf("Could not delete job: job entry: %s", err.Error())
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		log.Errorf("Could not delete job: job entry: rows affected: ", err.Error())
		return false, err
	}

	return affected != 0, nil
}

func ListJobs(idFilter *string) ([]JobWithResult, error) {
	baseQuery := db.builder.
		Select(
			// Job.
			"id",
			"name",
			"submitted",
			"wasm_id",
			"dataset_id",
			"owner",
			// Job result.
			"success",
			// nolint:goconst
			"content",
			"content_type",
			// nolint:goconst
			"created",
		).
		From(JobTableName).
		LeftJoin(ResultTableName).
		JoinClause(
			fmt.Sprintf("ON %s.id = %s.job_id",
				JobTableName,
				ResultTableName,
			),
		).OrderBy("submitted ASC")

	if idFilter != nil {
		baseQuery = baseQuery.Where("id=?", *idFilter)
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

	jobs := make([]JobWithResult, 0)

	for res.Next() {
		var jobWithResult JobWithResult

		var resultSuccess sql.NullBool
		var resultContent []byte
		var resultContentType sql.NullInt16
		var resultCreated sql.NullTime

		if err := res.Scan(
			// Job.
			&jobWithResult.Job.Id,
			&jobWithResult.Job.Name,
			&jobWithResult.Job.Submitted,
			&jobWithResult.Job.WasmId,
			&jobWithResult.Job.DatasetId,
			&jobWithResult.Job.Owner,
			// Result.
			&resultSuccess,
			&resultContent,
			&resultContentType,
			&resultCreated,
		); err != nil {
			log.Errorf("Could not list jobs: scanning results failed: %s", err.Error())
			return nil, err
		}

		if resultContent != nil && resultSuccess.Valid && resultContentType.Valid && resultCreated.Valid {
			jobWithResult.Result = &Result{
				Success:     resultSuccess.Bool,
				JobId:       jobWithResult.Job.Id,
				Content:     resultContent,
				ContentType: ContentType(resultContentType.Int16),
				Created:     resultCreated.Time,
			}
		}

		jobs = append(jobs, jobWithResult)
	}

	return jobs, nil
}

func JobHasOwner(jobID string, owner string) (true bool, err error) {
	db.builder.
		Select("1").
		From(JobTableName).
		Where("job.id=? AND job.owner=?", jobID, owner).
		QueryRow()

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		log.Errorf("job has owner: query failed: %s", err.Error())
		return false, err
	}

	return true, nil
}

func GetJob(jobId string) (job JobWithResult, found bool, err error) {
	jobs, err := ListJobs(&jobId)
	if err != nil {
		return job, false, err
	}

	if len(jobs) == 0 {
		return job, false, nil
	}

	if len(jobs) > 1 {
		panic("Internal state corruption: getJob() returned more than 1 job")
	}

	return jobs[0], true, nil
}

func SaveResult(result Result) error {
	query := db.builder.Insert(ResultTableName).Values(
		result.JobId,
		result.Success,
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

func GetResult(jobId string) (result Result, found bool, err error) {
	res := db.builder.
		Select("*").From(ResultTableName).
		Where(fmt.Sprintf("%s.job_id=?", ResultTableName), jobId).
		QueryRow()
	err = res.Scan(
		&result.JobId,
		&result.Success,
		&result.Content,
		&result.ContentType,
		&result.Created,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return result, false, nil
	}

	if err != nil {
		log.Errorf("Could not get result for job (%s): executing query failed: %s", jobId, err.Error())
		return result, false, err
	}

	return result, true, nil
}

func deleteResult(jobId string) error {
	_, err := db.builder.
		Delete(ResultTableName).
		Where("job_result.job_id=?", jobId).Exec()

	if err != nil {
		log.Errorf("Could not delete job result from database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}
