package logic

import (
	"encoding/binary"
	"fmt"
	"strings"

	"capnproto.org/go/capnp/v3"
	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/database"

	coral "github.com/reef-runtime/reef/reef_protocol"
)

// func MessageToNode() (coral.MessageToNode, error) {
// 	arena := capnp.SingleSegment(nil)
//
// 	_, seg, err := capnp.NewMessage(arena)
// 	if err != nil {
// 		panic(err.Error())
// 	}
//
// 	genericMsg, err := coral.NewMessageToNode(seg)
// 	if err != nil {
// 		return coral.MessageToNode{}, err
// 	}
//
// 	return genericMsg, nil
// }

// TODO: could just use `binary.LittleEndian.PutUint32()`?
// TODO: write test for this function
// func uint32IntoBytes(v uint32) []byte {
// 	return []byte{
// 		byte((v & 0xFF_00_00_00) >> 24),
// 		byte((v & 0x00_FF_00_00) >> 16),
// 		byte((v & 0x00_00_FF_00) >> 8),
// 		byte(v),
// 	}
// }
//
// func uint16IntoBytes(v uint16) []byte {
// 	return []byte{
// 		byte((v & 0xFF_00) >> 8),
// 		byte(v),
// 	}
// }

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
// Message layout for job initialization
// [MANAGER ---> NODE]
//
// |-00-------------|-01-------------04------------|-05-------------06-------------|-07--------------70------------|-71----------------|
// | CODE_START     |         PROGRAM_LENGTH       |        WORKER_INDEX           |             JOB_ID            |     PROGRAM       |
// | JOB            |           <uint32>           |           <uint16>            |            <string>           |     BYTECODE      | (... Other data? ...)
// |----------------|------------------------------|-------------------------------|-------------------------------|-------------------|
// |     1 Byte     |            4 Byte            |                               |             64 Byte           |  <PROGRAM_LENGTH> |
//
//
// Message layout for job started / rejected
// [NODE ---> MANAGER]
//
// |-00-------------|-01-------------04------------|-05-------------68------|
// | CODE_STARTED   |         WORKER_INDEX         |		 JOB_ID         |
// | JOB            |           <uint32>           |        <string>        |
// |----------------|------------------------------|------------------------|
// |     1 Byte     |            4 Byte            |		 64 Byte        |
//
//
//
// const CODE_START_JOB = 0xC0
// const CODE_STARTED_JOB = 0xC1
// const CODE_REJECTED_JOB = 0xC2

func toNodeJobInitializationMessage(
	workerIndex uint32, // TODO: uint16 is enough.
	jobID string,
	programByteCode []byte,
) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNodeMsg, err := coral.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	toNodeMsg.SetKind(coral.MessageToNodeKind_startJob)

	nestedBody, err := coral.NewJobInitializationMessage(seg)
	if err != nil {
		return nil, err
	}

	nestedBody.SetWorkerIndex(workerIndex)

	if err := nestedBody.SetJobID(jobID); err != nil {
		return nil, err
	}

	if err := nestedBody.SetProgramByteCode(programByteCode); err != nil {
		return nil, err
	}

	if err := toNodeMsg.Body().SetStartJob(nestedBody); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

// TODO: migrate into `logic`
func (m *NodeManagerT) StartJobOnNode(node Node, jobID JobID, workerIdx uint16, programByteCode []byte) error {
	msg, err := toNodeJobInitializationMessage(
		uint32(workerIdx),
		jobID,
		programByteCode,
	)
	if err != nil {
		return err
	}

	msg2, _ := capnp.Unmarshal(msg)
	b, _ := coral.ReadRootMessageToNode(msg2)
	spew.Dump(b.Kind())

	node.Conn.Lock.Lock()
	err = node.Conn.Conn.WriteMessage(
		websocket.BinaryMessage,
		msg,
	)
	node.Conn.Lock.Unlock()

	if err != nil {
		return err
	}

	// TODO: what??

	m.Nodes.Lock.Lock()
	m.Nodes.Map[node.ID].WorkerState[workerIdx] = &jobID
	m.Nodes.Lock.Unlock()

	log.Debugf("[node] Started job `%s` on node `%s`", jobID, FormatBinarySliceAsHex(node.ID[:]))

	// Now we wait until the job responds with `CODE_STARTED_JOB`.
	// Then, we invoke another function to handle this.

	return nil
}

func (m *NodeManagerT) jobStartedJobCallbackInternal(nodeID NodeID, message []byte) (jobID JobID, err error) {
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
		return "", fmt.Errorf("worker index returned from node (%d) is not busy with job ID `%s` -> node sent bogus value", workerIndex, *currWorkerJobId)
	}

	// TODO: sanity check between received and real job ID

	if *currWorkerJobId != jobID {
		return "", fmt.Errorf("job ID returned from node (%s) != expected `%s` -> node sent bogus value", jobID, *currWorkerJobId)
	}

	log.Debugf("[node] Received callback from node, started job `%s` on node `%s`", jobID, FormatBinarySliceAsHex(nodeID[:]))

	return jobID, nil
}

func (m *NodeManagerT) JobStartedJobCallback(nodeID NodeID, message []byte) error {
	jobID, err := m.jobStartedJobCallbackInternal(nodeID, message)
	if err != nil {
		if !m.DropNode(nodeID) {
			panic("Impossible: node which should be dropped does not exist")
		}
		return err
	}

	fmt.Printf("=====================> `%s`\n", jobID)

	found, err := database.ModifyJobStatus(jobID, database.StatusRunning)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("Failed to set job to running: job `%s` not found in database", jobID)
	}

	return nil
}

func (m *NodeManagerT) findFreeNode() (nodeID NodeID, workerIdx uint16, found bool) {
	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	for nodeID, node := range m.Nodes.Map {
		for workerIdx, jobID := range node.WorkerState {
			if jobID == nil {
				return nodeID, uint16(workerIdx), true
			}
		}
	}

	return nodeID, 0, false
}

func (m *NodeManagerT) StartJobOnFreeNode(jobID JobID) (couldStart bool, err error) {
	nodeID, workerIndex, nodeFound := m.findFreeNode()
	if !nodeFound {
		return false, nil
	}

	programByteCode := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	m.Nodes.Lock.Lock()
	node, nodeFound := m.Nodes.Map[nodeID]
	m.Nodes.Lock.Unlock()

	if !nodeFound {
		return false, nil
	}

	log.Debugf("[node] Found free worker index %d on node `%s`", workerIndex, IdToString(nodeID))

	if err := m.StartJobOnNode(node, jobID, workerIndex, programByteCode); err != nil {
		log.Errorf("[node] Could not start job `%s` on node `%s`: %s", jobID, IdToString(nodeID), err.Error())
		return false, nil
	}

	found, err := database.ModifyJobStatus(jobID, database.StatusStarting)
	if err != nil {
		return false, err
	}

	if !found {
		return false, fmt.Errorf("Could not modify job status: job `%s` not found in DB", jobID)
	}

	return true, nil
}

//
// Job logging.
//
// Message layout for job (logging / progress) report.
// [NODE ---> MANAGER]
//
// |-00-------------|-01-------------02------------|-03-------------04-------------|-05--------------06------------|-07----------------|
// |  CODE_JOB_LOG  |           LOG_KIND           |        WORKER_INDEX           |          CONTENT_LENGTH       |        LOG        |
// |                |           <uint16>           |           <uint16>            |             <uint16>          |    CONTENT_BYTES  | (... Other data? ...)
// |----------------|------------------------------|-------------------------------|-------------------------------|-------------------|
// |     1 Byte     |            2 Byte            |                               |             2 Byte            |  <CONTENT_LENGTH> |
//
//

const CODE_JOB_LOG = 0xD0

type NodeLogMessage struct {
	LogKind       database.LogKind
	WorkerIndex   uint16
	ContentLength uint16
	LogContents   []byte
}

func (m *NodeManagerT) NodeLogCallBack(nodeID NodeID, message []byte) (err error) {
	parsed, err := NodeManager.nodeLogCallBackInternal(nodeID, message)
	if err != nil {
		if !m.DropNode(nodeID) {
			panic("Impossible: node which should be dropped does not exist")
		}
		return err
	}

	fmt.Printf("parsed kind from log: %d\n", parsed.LogKind)

	return nil
}

func (m *NodeManagerT) nodeLogCallBackInternal(nodeID NodeID, message []byte) (parsed NodeLogMessage, err error) {
	const leadingBytes = 1 // Due to the leading signal byte.
	uint16Size := binary.Size(uint16(0))

	logKindSize := uint16Size
	workerIndexSize := uint16Size
	contentLengthSize := uint16Size

	expectedLen := leadingBytes + logKindSize + workerIndexSize + contentLengthSize
	msgLen := len(message)

	if msgLen < expectedLen {
		return parsed, fmt.Errorf("expected message to be at least of length %d, got %d", expectedLen, msgLen)
	}

	logKind := binary.BigEndian.Uint16(message[leadingBytes : leadingBytes+logKindSize])
	workerIndex := binary.BigEndian.Uint16(message[leadingBytes+logKindSize : leadingBytes+logKindSize+workerIndexSize])
	contentLength := binary.BigEndian.Uint16(message[leadingBytes+logKindSize+workerIndexSize : leadingBytes+logKindSize+workerIndexSize+contentLengthSize])

	msgLen += int(contentLength)
	if msgLen < expectedLen {
		return parsed, fmt.Errorf("expected message with content length to be of length %d, got %d", expectedLen, msgLen)
	}

	if !database.IsValidLogKind(logKind) {
		return parsed, fmt.Errorf("node `%s` sent invalid log kind `%d`", IdToString(nodeID), logKind)
	}

	m.Nodes.Lock.RLock()
	numWorkersForThisNode := m.Nodes.Map[nodeID].Info.NumWorkers
	m.Nodes.Lock.RUnlock()

	if workerIndex >= numWorkersForThisNode {
		return parsed, fmt.Errorf("worker index returned from node is %d (>= %d)", workerIndex, numWorkersForThisNode)
	}

	m.Nodes.Lock.RLock()
	// spew.Dump(m.Nodes.Map[nodeID])
	currWorkerJobId := m.Nodes.Map[nodeID].WorkerState[workerIndex]
	m.Nodes.Lock.RUnlock()

	if currWorkerJobId == nil {
		return parsed, fmt.Errorf("worker index (%d) returned from node (%s) is free", workerIndex, IdToString(nodeID))
	}

	// m.Nodes.Lock.Lock()
	// jobId := m.Nodes.Map[nodeID].WorkerState[workerIndex]
	// m.Nodes.Lock.Unlock()

	log.Debugf("[node] Received log from node `%s` for job `%s` on worker IDX %d", IdToString(nodeID), *currWorkerJobId, workerIndex)

	return NodeLogMessage{
		LogKind:       database.LogKind(logKind),
		WorkerIndex:   workerIndex,
		ContentLength: contentLength,
		LogContents:   []byte{},
	}, nil
}
