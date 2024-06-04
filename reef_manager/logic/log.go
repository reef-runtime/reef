package logic

import (
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

type LogEntry struct {
	Kind    database.LogKind `json:"kind"`
	Content string           `json:"content"`
	JobID   string           `json:"jobId"`
}

func SubmitLog(entry LogEntry) error {
	now := time.Now()

	joblog := database.JobLog{
		Kind:    entry.Kind,
		Created: now,
		Content: entry.Content,
		JobID:   entry.JobID,
	}

	err := database.AddLog(joblog)
	if err != nil {
		return err
	}

	return nil
}
