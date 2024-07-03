package logic

import (
	"time"
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

// Error is a critical error, like a database fault.
func (m *JobManagerT) TryToStartQueuedJobs() error {
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
