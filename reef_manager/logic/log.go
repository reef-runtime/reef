package logic

import (
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

type LogEntry struct {
	Kind    database.LogLevel `json:"kind"`
	Content string            `josn:"content"`
	Job_id  string            `json:"job_id"`
}

func SubmitLog(entry LogEntry) error {
	now := time.Now().Local()

	joblog := database.JobLog{
		// ID:      0,
		Kind:    entry.Kind,
		Created: now,
		Content: entry.Content,
		Job_id:  entry.Job_id,
	}

	err := database.AddLog(joblog)
	if err != nil {
		return err
	}

	return nil
}
