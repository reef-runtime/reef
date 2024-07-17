package logic

import (
	"fmt"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

//
// Fetches all queued jobs and tries to start them.
//

func (m *JobManagerT) QueuedJobs() (first LockedValue[Job], irstExists bool) {
	m.NonFinishedJobs.Lock.RLock()

	var earliest LockedValue[Job]

	for _, job := range m.NonFinishedJobs.Map {
		job.Lock.RLock()
		status := job.Data.Status
		submitted := job.Data.Data.Submitted
		job.Lock.RUnlock()

		if status != StatusQueued {
			continue
		}

		if earliest.Data == nil {
			earliest = job
			continue
		}

		earliest.Lock.RLock()
		earliestSubmitted := earliest.Data.Data.Submitted
		earliest.Lock.RUnlock()

		if submitted.Before(earliestSubmitted) {
			earliest = job
		}
	}

	m.NonFinishedJobs.Lock.RUnlock()

	return earliest, earliest.Data != nil
}

// Does critical housekeeping and management on the job manager.
// Starts queued jobs, manages maximum allowed job runtime.
// Error is a critical error, like a database fault.
func (m *JobManagerT) JobManagerMainLoopIteration() error {
	// Try to start all queued jobs.
	if err := m.tryToStartQueuedJobs(); err != nil {
		return err
	}

	// Check maximum allowed runtime of all running jobs.
	if err := m.checkAllowedRuntime(); err != nil {
		return err
	}

	return nil
}

// Error is critical as well.
func (m *JobManagerT) tryToStartQueuedJobs() error {
	for {
		first, firstExists := m.QueuedJobs()
		if !firstExists {
			log.Trace("Job queue is empty")
			break
		}

		first.Lock.RLock()
		id := first.Data.Data.Id
		first.Lock.RUnlock()

		log.Debugf("Attempting to start job `%s`...", id)

		couldStart, err := m.StartJobOnFreeNode(first)
		if err != nil {
			log.Errorf("HARD ERROR: Could not start job `%s`: %s", id, err.Error())
			return err
		}

		if !couldStart {
			log.Debugf("Could not start job `%s`", id)
			break
		}

		log.Infof("Job `%s` started", id)
	}

	return nil
}

// Error is critical as well.
// nolint:funlen
func (m *JobManagerT) checkAllowedRuntime() error {
	// All jobs in this slice are over their allowed maximum runtime.
	jobsToBeKilled := make([]LockedValue[Job], 0)

	// Increment runtime counter of all running jobs.
	m.NonFinishedJobs.Lock.RLock()
	for _, job := range m.NonFinishedJobs.Map {
		job.Lock.RLock()
		status := job.Data.Status
		job.Lock.RUnlock()

		// Job is not running, cannot increment runtime counter.
		if status != StatusRunning {
			continue
		}

		job.Lock.Lock()
		secondsToAdd := time.Since(job.Data.LastRuntimeIncrement).Seconds()

		job.Data.RuntimeSeconds += uint64(secondsToAdd)
		log.Tracef("Updated runtime of job `%s` to %d seconds", job.Data.Data.Id, job.Data.RuntimeSeconds)

		// This jobs exceeds the maximum allowed runtime.
		if job.Data.RuntimeSeconds > m.MaxJobRuntimeSecs {
			if !job.Data.IsBeingAborted {
				jobsToBeKilled = append(jobsToBeKilled, job)
				job.Data.IsBeingAborted = true
			} else {
				log.Debugf(
					"Job `%s` has exceeded maximum allowed runtime and is already being aborted, doing nothing...",
					job.Data.Data.Id,
				)
			}
		}

		job.Data.LastRuntimeIncrement = time.Now()
		job.Lock.Unlock()
	}
	m.NonFinishedJobs.Lock.RUnlock()

	// Kill all jobs which took too long.
	for _, jobToBeKilled := range jobsToBeKilled {
		jobToBeKilled.Lock.RLock()
		jobID := jobToBeKilled.Data.Data.Id
		jobToBeKilled.Lock.RUnlock()

		log.Debugf("Aborting job `%s`: exceeded maximum runtime of %d seconds...", jobID, m.MaxJobRuntimeSecs)

		const abortMsg = "Maximum allowed runtime of %d seconds was exceeded, this job will be terminated."

		if err := database.AddLog(database.JobLog{
			Kind:    database.LogKindSystem,
			Created: time.Now(),
			Content: fmt.Sprintf(abortMsg, m.MaxJobRuntimeSecs),
			JobId:   jobID,
		}); err != nil {
			return err
		}

		found, err := m.AbortJob(jobID)
		if err != nil {
			log.Errorf("Could not abort job `%s` (which exceeded maximum runtime): %s", jobID, err.Error())
			continue
		}

		if !found {
			log.Debugf("Could not abort job `%s` (which exceeded maximum runtime): job not found anymore", jobID)
			continue
		}
	}

	return nil
}

func (m *JobManagerT) JobManagerDaemon() {
	log.Info("Job queue daemon is running...")

	for {
		time.Sleep(jobDaemonFreq)
		log.Trace("Trying to start all queued jobs...")
		if err := m.JobManagerMainLoopIteration(); err != nil {
			panic(err.Error())
		}
	}
}
