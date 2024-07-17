package api

import (
	"fmt"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

func processStateSyncFromNode(nodeID logic.NodeId, message node.MessageFromNode) error {
	parsed, err := parseStateSync(nodeID, message)
	if err != nil {
		return err
	}

	if err := logic.JobManager.StateSync(nodeID, parsed); err != nil {
		return err
	}

	return nil
}

func parseStateSync(nodeId logic.NodeId, message node.MessageFromNode) (logic.StateSync, error) {
	if message.Body().Which() != node.MessageFromNode_body_Which_jobStateSync {
		panic("assertion failed: expected body type is job state sync, got something different")
	}

	result, err := message.Body().JobStateSync()
	if err != nil {
		return logic.StateSync{}, fmt.Errorf("could not parse job state sync: %s", err.Error())
	}

	nodeInfo, found := logic.JobManager.Nodes.Get(nodeId)
	if !found {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"illegal node: node Id `%s` references non-existent node",
			logic.IdToString(nodeId),
		)
	}

	workerIndex := result.WorkerIndex()

	nodeInfo.Lock.RLock()
	numWorkers := nodeInfo.Data.Info.NumWorkers
	nodeInfo.Lock.RUnlock()

	if workerIndex >= numWorkers {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"illegal worker index: node returned illegal worker index (%d) when num workers is %d",
			workerIndex,
			numWorkers,
		)
	}

	nodeInfo.Lock.RLock()
	jobID := nodeInfo.Data.WorkerState[workerIndex]
	nodeInfo.Lock.RUnlock()

	if jobID == nil {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"wrong worker index: node returned worker index (%d), this worker is idle",
			workerIndex,
		)
	}

	interpreterState, err := result.Interpreter()
	if err != nil {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf("could not decode interpreter state bytes: %s", err.Error())
	}

	progress := result.Progress()

	logs, err := result.Logs()
	if err != nil {
		return logic.StateSync{}, fmt.Errorf("could not decode interpreter state bytes: %s", err.Error())
	}

	logsLen := logs.Len()
	logsOutput := make([]database.JobLog, logsLen)

	for idx := 0; idx < logsLen; idx++ {
		currLog := logs.At(idx)

		if !database.IsValidLogKind(currLog.LogKind()) {
			return logic.StateSync{}, fmt.Errorf("invalid log kind `%d` received", currLog.LogKind())
		}

		logKind := database.LogKind(currLog.LogKind())

		content, err := currLog.Content()
		if err != nil {
			return logic.StateSync{}, fmt.Errorf("could not decode log message content: %s", err.Error())
		}

		logsOutput[idx] = database.JobLog{
			Kind:    logKind,
			Created: time.Now(),
			Content: string(content),
			JobId:   *jobID,
		}
	}

	return logic.StateSync{
		WorkerIndex:      workerIndex,
		JobId:            *jobID,
		Progress:         progress,
		Logs:             logsOutput,
		InterpreterState: interpreterState,
	}, nil
}
