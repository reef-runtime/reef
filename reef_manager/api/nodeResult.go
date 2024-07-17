package api

import (
	"fmt"

	"github.com/reef-runtime/reef/reef_manager/logic"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

func processJobResultFromNode(nodeID logic.NodeId, message node.MessageFromNode) error {
	result, err := parseJobResultFromNode(nodeID, message)
	if err != nil {
		return fmt.Errorf("parse job result: %s", err.Error())
	}

	log.Debugf("Job result: %s", result.String())

	if err := logic.JobManager.ProcessResult(nodeID, result); err != nil {
		return err
	}

	return nil
}

func parseJobResultFromNode(nodeID logic.NodeId, message node.MessageFromNode) (logic.JobResult, error) {
	if message.Body().Which() != node.MessageFromNode_body_Which_jobResult {
		panic("assertion failed: expected body type is job result, got something different")
	}

	result, err := message.Body().JobResult()
	if err != nil {
		return logic.JobResult{}, fmt.Errorf("could not parse job result: %s", err.Error())
	}

	nodeInfo, found := logic.JobManager.Nodes.Get(nodeID)
	if !found {
		return logic.JobResult{}, fmt.Errorf(
			"illegal node: node Id `%s` references non-existent node",
			logic.IdToString(nodeID),
		)
	}

	workerIndex := result.WorkerIndex()

	nodeInfo.Lock.RLock()
	nodeNumWorkers := nodeInfo.Data.Info.NumWorkers
	nodeInfo.Lock.RUnlock()

	if workerIndex >= nodeNumWorkers {
		return logic.JobResult{}, fmt.Errorf(
			"illegal worker index: node returned illegal worker index (%d) when num workers is %d",
			workerIndex,
			nodeNumWorkers,
		)
	}

	nodeInfo.Lock.RLock()
	jobID := nodeInfo.Data.WorkerState[workerIndex]
	nodeInfo.Lock.RUnlock()

	if jobID == nil {
		return logic.JobResult{}, fmt.Errorf(
			"wrong worker index: node returned worker index (%d), this worker is idle",
			workerIndex,
		)
	}

	contents, err := result.Contents()
	if err != nil {
		return logic.JobResult{}, fmt.Errorf("could not decode result content bytes: %s", err.Error())
	}

	return logic.JobResult{
		JobID:       *jobID,
		WorkerIndex: workerIndex,
		Success:     result.Success(),
		ContentType: result.ContentType(),
		Contents:    contents,
	}, nil
}
