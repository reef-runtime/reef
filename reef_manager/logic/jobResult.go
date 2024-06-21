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

	return fmt.Sprintf("[%v] on %s@%d (%s): %s", r.Success, r.JobID, r.WorkerIndex, contentTypeStr, content)
}

func (m *JobManagerT) ProcessResult(nodeID NodeID, result JobResult) error {
	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	oldNode, found := m.Nodes.Map[nodeID]
	if !found {
		return fmt.Errorf("process result: node ID is illegal: `%s`", IDToString(nodeID))
	}

	if result.WorkerIndex >= oldNode.Info.NumWorkers {
		return fmt.Errorf("process result: worker index is illegal: %d", result.WorkerIndex)
	}

	_, exists, err := database.GetResult(result.JobID)
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("result for job `%s` already exists in database", result.JobID)
	}

	if err := database.SaveResult(database.Result{
		Success:     result.Success,
		JobID:       result.JobID,
		Content:     result.Contents,
		ContentType: database.ContentType(result.ContentType),
		Created:     time.Now(),
	}); err != nil {
		return fmt.Errorf("process result: DB: %s", err.Error())
	}

	jobID := *oldNode.WorkerState[result.WorkerIndex]

	job, found := m.NonFinishedJobs.Delete(jobID)
	if !found {
		return fmt.Errorf("illegal job id in result: `%s`", jobID)
	}

	for _, log := range job.Logs {
		if err := database.AddLog(log); err != nil {
			return fmt.Errorf("save log: %s", err.Error())
		}
	}

	// Finally, delete the job from the worker.
	oldNode.WorkerState[result.WorkerIndex] = nil

	if _, err := database.ModifyJobStatus(jobID, database.StatusDone); err != nil {
		return fmt.Errorf("set status to done: %s", err.Error())
	}

	return nil
}
