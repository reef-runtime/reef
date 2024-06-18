package database

import (
	"time"
)

const LogTableName = "log"

type LogKind uint16

const (
	// Normal logging.
	LogKindProgram LogKind = iota
	// Logs that the node may produce.
	LogKindNode
	// Logs that the manager may produce.
	LogKindSystem
)

func IsValidLogKind(from uint16) bool {
	switch from {
	case uint16(LogKindProgram), uint16(LogKindNode),
		uint16(LogKindSystem):
		return true
	default:
		return false
	}
}

type JobLog struct {
	Kind    LogKind   `json:"kind"`
	Created time.Time `json:"created"`
	Content string    `json:"content"`
	JobID   string    `json:"jobId"`
}

func AddLog(joblog JobLog) error {
	_, err := db.builder.Insert(JobTableName).Values(
		joblog.Kind,
		joblog.Created,
		joblog.Content,
		joblog.JobID,
	).Exec()

	if err != nil {
		log.Errorf("Could not add log to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func DeleteLogs(jobID string) (found bool, err error) {
	// nolint:goconst
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

func GetLastLogs(limit *uint64, jobID string) ([]JobLog, error) {
	query := db.builder.
		Select("*").
		From(LogTableName).
		Where("log.job_id=?", jobID).
		OrderBy("created ASC")

	if limit != nil {
		query = query.Limit(*limit)
	}

	res, err := query.Query()

	if err != nil {
		// nolint:goconst
		log.Errorf("Could not list logs: executing query failed: %s", err.Error())
		return nil, err
	}

	if res.Err() != nil {
		log.Errorf("Could not list logs: executing query failed: %s", res.Err())
		return nil, err
	}
	defer res.Close()

	logs := make([]JobLog, 0)

	for res.Next() {
		var joblog JobLog
		if err := res.Scan(
			&joblog.Kind,
			&joblog.Created,
			&joblog.Content,
			&joblog.JobID,
		); err != nil {
			log.Errorf("Could not list logs: scanning results failed: %s", err.Error())
			return nil, err
		}

		logs = append(logs, joblog)
	}

	return logs, nil
}
