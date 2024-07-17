package logic

import (
	"errors"
	"fmt"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/database"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

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

	nodeID, workerIndex, nodeFound := m.findSuitableNode()
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
// Finds a node that has at least one free worker to execute a job.
// Additionally, this function has the job of balancing the job distribution across the nodes.
// For this, each node is assigned a "suitability score".
// The node with the highest score is elected to run the job.
//

func (m *JobManagerT) findSuitableNode() (nodeID NodeId, workerIdx uint16, found bool) {
	m.Nodes.Lock.RLock()
	defer m.Nodes.Lock.RUnlock()

	maxScore := uint8(0)
	var maxNode LockedValue[Node]
	foundANode := false

	for _, node := range m.Nodes.Map {
		node.Lock.RLock()
		score, isPossible := node.Data.calculateNodeSuitabilityScore()
		node.Lock.RUnlock()

		if isPossible {
			foundANode = true
		}

		if score > maxScore {
			maxScore = score
			maxNode = node
		}
	}

	// No free nodes available.
	if !foundANode {
		log.Debug("Found no suitable nodes: no free nodes")
		return nodeID, 0, false
	}

	log.Debugf("Found most suitable node for job: node-ID: `%s`, score: %d", IdToString(maxNode.Data.Id), maxScore)

	// Return one free worker of the elected node.
	maxNode.Lock.RLock()
	for workerIdx, jobID := range maxNode.Data.WorkerState {
		if jobID == nil {
			maxNode.Lock.RUnlock()
			return maxNode.Data.Id, uint16(workerIdx), true
		}
	}
	maxNode.Lock.RUnlock()

	return nodeID, 0, false
}

func (n Node) calculateNodeSuitabilityScore() (score uint8, isPossible bool) {
	const oneHundred = 100

	amountFreeWorkers := 0
	for _, worker := range n.WorkerState {
		if worker == nil {
			amountFreeWorkers++
		}
	}

	if amountFreeWorkers == 0 {
		return 0, false
	}

	percentFree := float32(amountFreeWorkers) / float32(n.Info.NumWorkers)
	return uint8(percentFree * oneHundred), true
}
