package logic

import (
	"fmt"
	"strings"

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
	jobId string,
	datasetId string,
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
	if err := nestedBody.SetJobId(jobId); err != nil {
		return nil, err
	}

	// Program Byte Code.
	if err := nestedBody.SetProgramByteCode(programByteCode); err != nil {
		return nil, err
	}

	// Dataset
	if err := nestedBody.SetDatasetId(datasetId); err != nil {
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
	nodeId := node.Data.Id
	node.Lock.Unlock()

	job.Lock.RLock()
	jobId := job.Data.Data.Id
	job.Lock.RUnlock()

	log.Debugf(
		"[node] Job `%s` starting on node `%s`",
		jobId,
		IdToString(nodeId),
	)

	// NOTE: we don't have to wait for the job to start on the node since the first state sync will be enough to set
	// the job status to running.

	return nil
}

func (m *JobManagerT) findFreeNode() (nodeId NodeId, workerIdx uint16, found bool) {
	m.Nodes.Lock.RLock()
	defer m.Nodes.Lock.RUnlock()

	for nodeId, node := range m.Nodes.Map {
		node.Lock.RLock()

		for workerIdx, jobId := range node.Data.WorkerState {
			if jobId == nil {
				node.Lock.RUnlock()
				return nodeId, uint16(workerIdx), true
			}
		}

		node.Lock.RUnlock()
	}

	return nodeId, 0, false
}

func (m *JobManagerT) StartJobOnFreeNode(job LockedValue[Job]) (couldStart bool, err error) {
	// Load the WasmCode from storage again.
	job.Lock.RLock()
	wasmId := job.Data.Data.WasmId
	job.Lock.RUnlock()

	wasmCode, err := m.Compiler.getCached(wasmId)
	if err != nil {
		log.Errorf("failed to park job `%s`: could not load job's Wasm from cache", err.Error())
		return false, err
	}

	nodeId, workerIndex, nodeFound := m.findFreeNode()
	if !nodeFound {
		return false, nil
	}

	node, nodeFound := m.Nodes.Get(nodeId)
	if !nodeFound {
		return false, nil
	}

	job.Lock.RLock()
	jobId := job.Data.Data.Id
	job.Lock.RUnlock()

	log.Debugf("[node] Found free worker index %d on node `%s`", workerIndex, IdToString(nodeId))

	if err := m.StartJobOnNode(node, job, workerIndex, wasmCode); err != nil {
		log.Errorf(
			"[node] Could not start job `%s` on node `%s`: %s",
			jobId,
			IdToString(nodeId),
			err.Error(),
		)
		return false, nil
	}

	// Set the new status and worker node of this job.
	job.Lock.Lock()
	job.Data.Status = StatusStarting
	job.Data.WorkerNodeID = &nodeId
	job.Lock.Unlock()

	m.updateSingleJobState(jobId)
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
