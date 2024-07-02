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
// Job initialization
//

// workerIndex         @0 :UInt32;
// jobId               @1 :Text;
// programByteCode     @2 :Data;
// # If the job has just been started these will be 0/empty.
// progress            @3 :Float32;
// interpreterState    @4 :Data;

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

	//
	// Worker index.
	//
	nestedBody.SetWorkerIndex(workerIndex)

	//
	// Job ID.
	//
	if err := nestedBody.SetJobId(jobID); err != nil {
		return nil, err
	}

	//
	// Program Byte Code.
	//
	if err := nestedBody.SetProgramByteCode(programByteCode); err != nil {
		return nil, err
	}

	// Dataset ID
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
		job.Data.Data.ID,
		job.Data.Data.DatasetID,
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

	// TODO: what??

	node.Lock.Lock()
	node.Data.WorkerState[workerIdx] = &job.Data.Data.ID
	nodeID := node.Data.ID
	node.Lock.Unlock()

	job.Lock.RLock()
	jobID := job.Data.Data.ID
	job.Lock.RUnlock()

	log.Debugf(
		"[node] Job `%s` starting on node `%s`",
		jobID,
		IDToString(nodeID),
	)

	// TODO: wait for job to start
	// Now we wait until the job responds with `CODE_STARTED_JOB`.
	// Then, we invoke another function to handle this.

	return nil
}

func (m *JobManagerT) findFreeNode() (nodeID NodeID, workerIdx uint16, found bool) {
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

func (m *JobManagerT) StartJobOnFreeNode(job LockedValue[Job]) (couldStart bool, err error) {
	// Load the WasmCode from storage again.
	job.Lock.RLock()
	wasmID := job.Data.Data.WasmID
	job.Lock.RUnlock()

	wasmCode, err := m.Compiler.getCached(wasmID)
	if err != nil {
		log.Errorf("failed to park job `%s`: could not load job's Wasm from cache", err.Error())
		return false, err
	}

	nodeID, workerIndex, nodeFound := m.findFreeNode()
	if !nodeFound {
		return false, nil
	}

	node, nodeFound := m.Nodes.Get(nodeID)
	if !nodeFound {
		return false, nil
	}

	job.Lock.RLock()
	jobID := job.Data.Data.ID
	job.Lock.RUnlock()

	log.Debugf("[node] Found free worker index %d on node `%s`", workerIndex, IDToString(nodeID))

	if err := m.StartJobOnNode(node, job, workerIndex, wasmCode); err != nil {
		log.Errorf(
			"[node] Could not start job `%s` on node `%s`: %s",
			jobID,
			IDToString(nodeID),
			err.Error(),
		)
		return false, nil
	}

	// Set the new status and worker node of this job.
	job.Lock.Lock()
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
