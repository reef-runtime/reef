package logic

import (
	"encoding/json"

	"github.com/reef-runtime/reef/reef_manager/database"
)

type APIJob struct {
	Job      database.JobWithResult
	Progress float32
	Status   JobStatus
	Logs     []database.JobLog
}

//
// This flattens the outputted JSON so that we don't have a lot of nested nonsense.
//

func (j *APIJob) MarshalJSON() ([]byte, error) {
	var result *database.Result

	if j.Job.Result != nil {
		result = &database.Result{
			Success:     j.Job.Result.Success,
			JobID:       j.Job.Job.Id,
			Content:     nil,
			ContentType: j.Job.Result.ContentType,
			Created:     j.Job.Result.Created,
		}
	}

	intermediate := map[string]interface{}{
		"id":        j.Job.Job.Id,
		"name":      j.Job.Job.Name,
		"submitted": j.Job.Job.Submitted,
		"wasmId":    j.Job.Job.WasmId,
		"datasetId": j.Job.Job.DatasetId,
		"owner":     j.Job.Job.Owner,
		"progress":  j.Progress,
		"status":    j.Status,
		"logs":      j.Logs,
		"result":    result,
	}
	return json.Marshal(intermediate)
}

func (m *JobManagerT) ListJobs() ([]APIJob, error) {
	// Do not exclude finished jobs.
	dbJobs, err := database.ListJobs(nil, nil)
	if err != nil {
		return nil, err
	}

	jobs := make([]APIJob, len(dbJobs))
	for idx, data := range dbJobs {
		// Do not include logs.
		enriched, err := m.enrichJob(data, false)
		if err != nil {
			return nil, err
		}
		jobs[idx] = enriched
	}

	return jobs, nil
}

func (m *JobManagerT) GetJob(id JobId, withLogs bool) (job APIJob, found bool, err error) {
	raw, found, err := database.GetJob(id, nil)
	if err != nil || !found {
		return job, found, err
	}

	enriched, err := m.enrichJob(raw, withLogs)

	return enriched, true, err
}

func (m *JobManagerT) enrichJob(job database.JobWithResult, withLogs bool) (APIJob, error) {
	var progress float32 = 1.0
	var logs []database.JobLog
	status := StatusDone

	runningJob, found := m.NonFinishedJobs.Get(job.Job.Id)
	//nolint:nestif
	if found {
		// If the job in the DB is running, add runtime information to the output.
		runningJob.Lock.RLock()

		progress = runningJob.Data.Progress
		status = runningJob.Data.Status

		if withLogs {
			logs = runningJob.Data.Logs
		}

		runningJob.Lock.RUnlock()
	} else if withLogs {
		// Load logs from database.
		logsDB, err := database.GetLastLogs(nil, job.Job.Id)
		if err != nil {
			return APIJob{}, err
		}
		logs = logsDB
	}

	return APIJob{
		Job:      job,
		Progress: progress,
		Status:   status,
		Logs:     logs,
	}, nil
}
