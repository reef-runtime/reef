package logic

import (
	"encoding/binary"
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
	job Job,
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
	if err := nestedBody.SetJobId(job.Data.ID); err != nil {
		return nil, err
	}

	//
	// Program Byte Code.
	//
	if err := nestedBody.SetProgramByteCode(programByteCode); err != nil {
		return nil, err
	}

	//
	// Progress.
	//
	nestedBody.SetProgress(job.Progress)

	//
	// Interpreter State.
	//
	if err := nestedBody.SetInterpreterState(job.InterpreterState); err != nil {
		return nil, err
	}

	if err := toNodeMsg.Body().SetStartJob(nestedBody); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

func (m *JobManagerT) StartJobOnNode(
	nodeData Node,
	job Job,
	workerIdx uint16,
	programByteCode []byte,
) error {
	msg, err := toNodeJobInitializationMessage(
		uint32(workerIdx),
		job,
		programByteCode,
	)
	if err != nil {
		return err
	}

	err = nodeData.Conn.WriteMessage(
		websocket.BinaryMessage,
		msg,
	)

	if err != nil {
		return err
	}

	// TODO: what??

	m.Nodes.Lock.Lock()
	m.Nodes.Map[nodeData.ID].WorkerState[workerIdx] = &job.Data.ID
	m.Nodes.Lock.Unlock()

	log.Debugf(
		"[node] Job `%s` starting on node `%s`",
		job.Data.ID,
		IDToString(nodeData.ID),
	)

	// TODO: wait for job to start
	// Now we wait until the job responds with `CODE_STARTED_JOB`.
	// Then, we invoke another function to handle this.

	return nil
}

// TODO: use this function
func (m *JobManagerT) jobStartedJobCallbackInternal(nodeID NodeID, message []byte) (jobID JobID, err error) {
	const leadingBytes = 1 // Due to the leading signal byte.
	workerIdxByteSize := binary.Size(uint16(0))
	jobIDBytes := 64

	expectedLen := leadingBytes + workerIdxByteSize + jobIDBytes
	msgLen := len(message)

	if msgLen != expectedLen {
		return "", fmt.Errorf("expected message length %d, got %d", expectedLen, msgLen)
	}

	workerIndex := binary.BigEndian.Uint16(message[leadingBytes : leadingBytes+workerIdxByteSize])
	jobID = string(message[leadingBytes+workerIdxByteSize:])

	m.Nodes.Lock.RLock()
	numWorkersForThisNode := m.Nodes.Map[nodeID].Info.NumWorkers
	m.Nodes.Lock.RUnlock()

	if workerIndex >= numWorkersForThisNode {
		return "", fmt.Errorf("worker index returned from node is %d (>= %d)", workerIndex, numWorkersForThisNode)
	}

	m.Nodes.Lock.RLock()
	currWorkerJobId := m.Nodes.Map[nodeID].WorkerState[workerIndex]
	m.Nodes.Lock.RUnlock()

	if currWorkerJobId == nil {
		return "", fmt.Errorf("worker index returned from node (%d) is not busy with job ID `nil` -> node sent bogus value", workerIndex)
	}

	// TODO: sanity check between received and real job ID

	if *currWorkerJobId != jobID {
		return "", fmt.Errorf("job ID returned from node (%s) != expected `%s` -> node sent bogus value", jobID, *currWorkerJobId)
	}

	log.Debugf("[node] Received callback from node, started job `%s` on node `%s`", jobID, FormatBinarySliceAsHex(nodeID[:]))

	return jobID, nil
}

// TODO: actually call this function
func (m *JobManagerT) JobStartedJobCallback(nodeID NodeID, message []byte) error {
	jobID, err := m.jobStartedJobCallbackInternal(nodeID, message)
	if err != nil {
		if !m.DropNode(nodeID) {
			panic("Impossible: node which should be dropped does not exist")
		}
		return err
	}

	fmt.Printf("=====================> `%s`\n", jobID)

	found, err := m.setJobStatus(jobID, database.StatusRunning)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("failed to set job to running: job `%s` not found in database", jobID)
	}

	return nil
}

func (m *JobManagerT) findFreeNode() (nodeID NodeID, workerIdx uint16, found bool) {
	m.Nodes.Lock.RLock()
	defer m.Nodes.Lock.RUnlock()

	for nodeID, node := range m.Nodes.Map {
		for workerIdx, jobID := range node.WorkerState {
			if jobID == nil {
				return nodeID, uint16(workerIdx), true
			}
		}
	}

	return nodeID, 0, false
}

func (m *JobManagerT) StartJobOnFreeNode(job Job) (couldStart bool, err error) {
	// Load the WasmCode from storage again.
	wasmCode, err := m.Compiler.getCached(job.Data.WasmID)
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

	log.Debugf("[node] Found free worker index %d on node `%s`", workerIndex, IDToString(nodeID))

	if err := m.StartJobOnNode(node, job, workerIndex, wasmCode); err != nil {
		log.Errorf(
			"[node] Could not start job `%s` on node `%s`: %s",
			job.Data.ID,
			IDToString(nodeID),
			err.Error(),
		)
		return false, nil
	}

	found, err := m.setJobStatusLOCK(job.Data.ID, database.StatusStarting, true)
	if err != nil {
		return false, err
	}

	if !found {
		return false, fmt.Errorf(
			"could not modify job status: job `%s` not found in DB",
			job.Data.ID,
		)
	}

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

func (m *JobManagerT) NodeLogCallBack(nodeID NodeID, message []byte) (err error) {
	parsed, err := JobManager.nodeLogCallBackInternal(nodeID, message)
	if err != nil {
		if !m.DropNode(nodeID) {
			panic("Impossible: node which should be dropped does not exist")
		}
		return err
	}

	// TODO: what to do here?
	fmt.Printf("parsed kind from log: %d\n", parsed.LogKind)

	return nil
}

func (m *JobManagerT) nodeLogCallBackInternal(nodeID NodeID, message []byte) (parsed NodeLogMessage, err error) {
	// TODO: implement this!
	panic("TODO")

	// if !database.IsValidLogKind(logKind) {
	// 	return parsed, fmt.Errorf("node `%s` sent invalid log kind `%d`", IDToString(nodeID), logKind)
	// }
	//
	// m.Nodes.Lock.RLock()
	// numWorkersForThisNode := m.Nodes.Map[nodeID].Info.NumWorkers
	// m.Nodes.Lock.RUnlock()
	//
	// if workerIndex >= numWorkersForThisNode {
	// 	return parsed, fmt.Errorf("worker index returned from node is %d (>= %d)", workerIndex, numWorkersForThisNode)
	// }
	//
	// m.Nodes.Lock.RLock()
	// // spew.Dump(m.Nodes.Map[nodeID])
	// currWorkerJobId := m.Nodes.Map[nodeID].WorkerState[workerIndex]
	// m.Nodes.Lock.RUnlock()
	//
	// if currWorkerJobId == nil {
	// 	return parsed, fmt.Errorf("worker index (%d) returned from node (%s) is free", workerIndex, IDToString(nodeID))
	// }
	//
	// // m.Nodes.Lock.Lock()
	// // jobId := m.Nodes.Map[nodeID].WorkerState[workerIndex]
	// // m.Nodes.Lock.Unlock()
	//
	// log.Debugf("[node] Received log from node `%s` for job `%s` on worker IDX %d", IDToString(nodeID), *currWorkerJobId, workerIndex)
	//
	// return NodeLogMessage{
	// 	LogKind:       database.LogKind(logKind),
	// 	WorkerIndex:   workerIndex,
	// 	ContentLength: contentLength,
	// 	LogContents:   []byte{},
	// }, nil
}
