package logic

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/database"

	node "github.com/reef-runtime/reef/reef_protocol_node"
)

func FormatBinarySliceAsHex(input []byte) string {
	output := make([]string, 0)
	for _, b := range input {
		output = append(output, fmt.Sprintf("0x%x", b))
	}

	return fmt.Sprintf("[%s]\n", strings.Join(output, ", "))
}

//
// Job initialization.
//

// nolint:funlen
func toNodeJobInitializationMessage(
	workerIndex uint32,
	jobID string,
	datasetID string,
	progress float32,
	interpreterState []byte,
	programByteCode []byte,
) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNodeMsg, err := node.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	toNodeMsg.SetKind(node.MessageToNodeKind_startJob)

	nestedBody, err := node.NewJobStartMessage(seg)
	if err != nil {
		return nil, err
	}

	// Worker index.
	nestedBody.SetWorkerIndex(workerIndex)

	// Job Id.
	if err := nestedBody.SetJobId(jobID); err != nil {
		return nil, err
	}

	// Program Byte Code.
	if err := nestedBody.SetProgramByteCode(programByteCode); err != nil {
		return nil, err
	}

	// Dataset.
	if err := nestedBody.SetDatasetId(datasetID); err != nil {
		return nil, err
	}

	// Progress.
	nestedBody.SetProgress(progress)

	// Interpreter State.
	if err := nestedBody.SetInterpreterState(interpreterState); err != nil {
		return nil, err
	}

	if err := toNodeMsg.Body().SetStartJob(nestedBody); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

func (m *JobManagerT) StartJobOnNode(
	node LockedValue[Node],
	job LockedValue[Job],
	workerIdx uint16,
	programByteCode []byte,
) error {
	msg, err := toNodeJobInitializationMessage(
		uint32(workerIdx),
		job.Data.Data.Id,
		job.Data.Data.DatasetId,
		job.Data.Progress,
		job.Data.InterpreterState,
		programByteCode,
	)
	if err != nil {
		return err
	}

	node.Lock.Lock()
	err = node.Data.Conn.WriteMessage(
		websocket.BinaryMessage,
		msg,
	)
	node.Lock.Unlock()

	if err != nil {
		return err
	}

	node.Lock.Lock()
	node.Data.WorkerState[workerIdx] = &job.Data.Data.Id
	nodeID := node.Data.Id
	node.Lock.Unlock()

	job.Lock.RLock()
	jobID := job.Data.Data.Id
	job.Lock.RUnlock()

	log.Debugf(
		"[node] Job `%s` starting on node `%s`",
		jobID,
		IdToString(nodeID),
	)

	// NOTE: we don't have to wait for the job to start on the node since the first state sync will be enough to set
	// the job status to running.

	return nil
}

func (m *JobManagerT) findFreeNode() (nodeID NodeId, workerIdx uint16, found bool) {
	m.Nodes.Lock.RLock()
	defer m.Nodes.Lock.RUnlock()

	for nodeID, node := range m.Nodes.Map {
		node.Lock.RLock()

		for workerIdx, jobID := range node.Data.WorkerState {
			if jobID == nil {
				node.Lock.RUnlock()
				return nodeID, uint16(workerIdx), true
			}
		}

		node.Lock.RUnlock()
	}

	return nodeID, 0, false
}

// nolint:funlen
func (m *JobManagerT) StartJobOnFreeNode(job LockedValue[Job]) (couldStart bool, err error) {
	// Load the WasmCode from storage again.
	job.Lock.RLock()
	wasmID := job.Data.Data.WasmId
	jobID := job.Data.Data.Id
	job.Lock.RUnlock()

	wasmCode, wasmError := m.Compiler.getCached(wasmID)

	if wasmError == nil && len(wasmCode) == 0 {
		log.Warnf("Will not start job `%s`: cached Wasm code is empty", jobID)
		wasmError = errors.New("empty Wasm code")
	}

	// nolint:nestif
	if wasmError != nil {
		eMsg := fmt.Sprintf(
			"Failed to start job `%s`: could not load job's Wasm from cache: %s",
			jobID,
			wasmError.Error(),
		)
		log.Error(eMsg)

		if err := database.AddLog(database.JobLog{
			Kind:    database.LogKindSystem,
			Created: time.Now(),
			Content: eMsg,
			JobId:   jobID,
		}); err != nil {
			return false, err
		}

		found, err := m.AbortJob(jobID)
		if err != nil {
			log.Errorf("Could not abort job `%s`, which has broken Wasm code: %s", jobID, err.Error())
			return false, nil
		}

		if !found {
			log.Warnf("Could not abort job `%s`, which has broken Wasm code: not found", jobID)
		}

		return false, err
	}

	if len(wasmCode) == 0 {
		panic("This case is excluded")
	}

	nodeID, workerIndex, nodeFound := m.findFreeNode()
	if !nodeFound {
		return false, nil
	}

	node, nodeFound := m.Nodes.Get(nodeID)
	if !nodeFound {
		return false, nil
	}

	log.Debugf("[node] Found free worker index %d on node `%s`", workerIndex, IdToString(nodeID))

	if err := m.StartJobOnNode(node, job, workerIndex, wasmCode); err != nil {
		log.Errorf(
			"[node] Could not start job `%s` on node `%s`: %s",
			jobID,
			IdToString(nodeID),
			err.Error(),
		)
		return false, nil
	}

	// Set the new status and worker node of this job.
	job.Lock.Lock()
	job.Data.LastRuntimeIncrement = time.Now()
	job.Data.Status = StatusStarting
	job.Data.WorkerNodeID = &nodeID
	job.Lock.Unlock()

	m.updateSingleJobState(jobID)
	m.updateNodeState()

	return true, nil
}

//
// Job logging.
//

type NodeLogMessage struct {
	LogKind       database.LogKind
	WorkerIndex   uint16
	ContentLength uint16
	LogContents   []byte
}
