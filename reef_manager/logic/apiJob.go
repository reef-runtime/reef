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
	intermediate := map[string]interface{}{
		"id":        j.Job.Job.Id,
		"name":      j.Job.Job.Name,
		"submitted": j.Job.Job.Submitted,
		"wasmId":    j.Job.Job.WasmId,
		"datasetId": j.Job.Job.DatasetId,
		"progress":  j.Progress,
		"status":    j.Status,
		"logs":      j.Logs,
		"result":    j.Job.Result,
	}
	return json.Marshal(intermediate)
}

func (m *JobManagerT) ListJobs() ([]APIJob, error) {
	// Do not exclude finished jobs.
	dbJobs, err := database.ListJobs(nil)
	if err != nil {
		return nil, err
	}

	jobs := make([]APIJob, len(dbJobs))
	for idx, data := range dbJobs {
		// Do not include logs.
		jobs[idx] = m.enrichJob(data, false)
	}

	return jobs, nil
}

func (m *JobManagerT) GetJob(id JobId, withLogs bool) (job APIJob, found bool, err error) {
	raw, found, err := database.GetJob(id)
	if err != nil || !found {
		return job, found, err
	}

	return m.enrichJob(raw, withLogs), true, nil
}

func (m *JobManagerT) enrichJob(job database.JobWithResult, withLogs bool) APIJob {
	var progress float32 = 1.0
	var logs []database.JobLog
	status := StatusDone

	runningJob, found := m.NonFinishedJobs.Get(job.Job.Id)
	if found {
		// If the job in the DB is running, add runtime information to the output.
		runningJob.Lock.RLock()

		progress = runningJob.Data.Progress
		status = runningJob.Data.Status

		if withLogs {
			logs = runningJob.Data.Logs
		}

		runningJob.Lock.RUnlock()
	}

	return APIJob{
		Job:      job,
		Progress: progress,
		Status:   status,
		Logs:     logs,
	}
}
