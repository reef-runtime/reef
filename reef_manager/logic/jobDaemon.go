package logic

import (
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

//
// Fetches all queued jobs and tries to start them.
//

func (m *JobManagerT) QueuedJobs() (j Job, jExists bool) {
	m.NonFinishedJobs.Lock.RLock()

	var earliest Job
	hadEarliest := false

	for _, job := range m.NonFinishedJobs.Map {
		if job.Data.Status != database.StatusQueued {
			continue
		}

		if !hadEarliest || job.Data.Submitted.Before(earliest.Data.Submitted) {
			earliest = job
			hadEarliest = true
		}
	}

	m.NonFinishedJobs.Lock.RUnlock()

	return earliest, hadEarliest
}

// Error is a critical error, like a database fault.
func (m *JobManagerT) TryToStartQueuedJobs() error {
	for {
		first, exists := m.QueuedJobs()
		if !exists {
			log.Trace("Job queue is empty")
			break
		}

		log.Debugf("Attempting to start job `%s`...", first.Data.ID)

		couldStart, err := m.StartJobOnFreeNode(first)
		if err != nil {
			log.Errorf("HARD ERROR: Could not start job `%s`: %s", first.Data.ID, err.Error())
			return err
		}

		if !couldStart {
			log.Debugf("Could not start job `%s`", first.Data.ID)
			break
		}

		log.Infof("Job `%s` started", first.Data.ID)
	}

	return nil
}

func (m *JobManagerT) JobQueueDaemon() {
	log.Info("Job queue daemon is running...")

	for {
		time.Sleep(jobDaemonFreq)
		log.Trace("Trying to start all queued jobs...")
		if err := m.TryToStartQueuedJobs(); err != nil {
			panic(err.Error())
		}
	}
}
