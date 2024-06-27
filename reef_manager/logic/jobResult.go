package logic

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

type JobResult struct {
	JobID       JobID
	WorkerIndex uint16
	Success     bool
	ContentType node.ResultContentType
	Contents    []byte
}

func (r JobResult) String() string {
	var contentTypeStr string
	var content string

	switch r.ContentType {
	case node.ResultContentType_int64:
		contentTypeStr = "int64"
		content = fmt.Sprint(int64(binary.LittleEndian.Uint64(r.Contents)))
	case node.ResultContentType_bytes:
		contentTypeStr = "bytes"
		content = hex.EncodeToString(r.Contents)
	case node.ResultContentType_stringPlain:
		contentTypeStr = "string"
		content = string(r.Contents)
	case node.ResultContentType_stringJSON:
		contentTypeStr = "JSON"
		content = string(r.Contents)
	}

	outcome := "FAILURE"
	if r.Success {
		outcome = "SUCCESS"
	}

	return fmt.Sprintf("[%s] on %s@%d (%s): %s", outcome, r.JobID, r.WorkerIndex, contentTypeStr, content)
}

func (m *JobManagerT) ProcessResult(nodeID NodeID, result JobResult) error {
	jobID, err := m.processResultWithLockingOps(nodeID, result)
	if err != nil {
		return err
	}

	m.updateSingleJobState(jobID)
	m.updateNodeState()

	return nil
}

func (m *JobManagerT) processResultWithLockingOps(nodeID NodeID, result JobResult) (jobID JobID, err error) {
	node, found := m.Nodes.Get(nodeID)

	if !found {
		return "", fmt.Errorf("process result: node ID is illegal: `%s`", IDToString(nodeID))
	}

	node.Lock.RLock()
	numWorkers := node.Data.Info.NumWorkers
	node.Lock.RUnlock()

	if result.WorkerIndex >= numWorkers {
		return "", fmt.Errorf("process result: worker index is illegal: %d", result.WorkerIndex)
	}

	_, exists, err := database.GetResult(result.JobID)
	if err != nil {
		return "", err
	}

	if exists {
		return "", fmt.Errorf("result for job `%s` already exists in database", result.JobID)
	}

	if err := database.SaveResult(database.Result{
		Success:     result.Success,
		JobID:       result.JobID,
		Content:     result.Contents,
		ContentType: database.ContentType(result.ContentType),
		Created:     time.Now(),
	}); err != nil {
		return "", fmt.Errorf("process result: DB: %s", err.Error())
	}

	node.Lock.Lock()
	jobID = *node.Data.WorkerState[result.WorkerIndex]
	// Finally, delete the job from the worker.
	node.Data.WorkerState[result.WorkerIndex] = nil
	node.Lock.Unlock()

	job, found := m.NonFinishedJobs.Delete(jobID)
	if !found {
		return "", fmt.Errorf("illegal job id in result: `%s`", jobID)
	}

	job.Lock.Lock()
	for _, log := range job.Data.Logs {
		if err := database.AddLog(log); err != nil {
			job.Lock.Unlock()
			return "", fmt.Errorf("save log: %s", err.Error())
		}
	}

	// Change progress and status on job.
	// Just in case anyone still borrows the job.
	job.Data.Status = StatusDone
	job.Data.Progress = 1.0
	job.Lock.Unlock()

	return jobID, nil
}
