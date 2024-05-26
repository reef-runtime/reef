package database

import (
	"time"
)

const LogTableName = "log"

type LogKind uint16

const (
	// Normal logging
	LogKindLevelTrace LogKind = iota
	LogKindLevelDebug
	LogKindLevelInfo
	LogKindLevelWarn
	LogKindLevelError
	LogKindLevelFatal
	// Progress reporting
	LogKindProgress
)

func IsValidLogKind(from uint16) bool {
	switch from {
	case uint16(LogKindLevelTrace), uint16(LogKindLevelDebug), uint16(LogKindLevelInfo), uint16(LogKindLevelWarn), uint16(LogKindLevelError), uint16(LogKindLevelFatal):
		return true
	case uint16(LogKindProgress):
		return true
	default:
		return false
	}
}

type JobLog struct {
	// ID      int       `json:"id"`
	Kind    LogKind   `json:"kind"`
	Created time.Time `json:"created"`
	Content string    `json:"content"`
	Job_id  string    `json:"job_id"`
}

func AddLog(joblog JobLog) error {
	_, err := db.builder.Insert(JobTableName).Values(
		// joblog.ID,
		joblog.Kind,
		joblog.Created,
		joblog.Content,
		joblog.Job_id,
	).Exec()

	if err != nil {
		log.Errorf("Could not add log to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func DeleteLogs(jobID string) (found bool, err error) {
	res, err := db.builder.Delete(JobTableName).Where("log.job_id=?", jobID).Exec()
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func GetLastLogs(num uint64, jobID string) ([]JobLog, error) {
	res, err := db.builder.Select("*").From(LogTableName).Where("log.job_id=?", jobID).OrderBy("id DESC").Limit(num).Query()

	if err != nil {
		log.Errorf("Could not list logs: executing query failed: %s", err.Error())
		return nil, err
	}

	logs := make([]JobLog, 0)

	for res.Next() {
		var joblog JobLog
		if err := res.Scan(
			// &joblog.ID,
			&joblog.Kind,
			&joblog.Created,
			&joblog.Content,
			&joblog.Job_id,
		); err != nil {
			log.Errorf("Could not list logs: scanning results failed: %s", err.Error())
			return nil, err
		}

		logs = append(logs, joblog)
	}

	return logs, nil
}