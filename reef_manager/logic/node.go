package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/database"
)

type JobID = string
type NodeID = [32]byte

type NodeInfo struct {
	EndpointIP string `json:"endpointIP"`
	Name       string `json:"name"`
	// TODO: maybe worker descriptions
	// TODO: maybe the current state of the node?
	NumWorkers uint16 `json:"numWorkers"`
}

type WSConn struct {
	conn  *websocket.Conn
	rLock sync.Mutex
	wLock sync.Mutex
}

func NewWSConn(conn *websocket.Conn) *WSConn {
	c := WSConn{
		conn:  conn,
		rLock: sync.Mutex{},
		wLock: sync.Mutex{},
	}

	return &c
}

const WSTimeout = time.Second * 5

func (s *WSConn) Close() error {
	err := s.conn.Close()
	return err
}

func (s *WSConn) RemoteAddr() net.Addr {
	addr := s.conn.RemoteAddr()
	return addr
}

func (s *WSConn) ReadMessage() (int, []byte, error) {
	now := time.Now()
	then := now.Add(WSTimeout)
	return s.ReadMessageWithTimeout(then)
}

func (s *WSConn) ReadMessageWithTimeout(timeout time.Time) (int, []byte, error) {
	s.rLock.Lock()

	if err := s.conn.SetReadDeadline(timeout); err != nil {
		s.rLock.Unlock()
		return 0, nil, err
	}

	kind, content, err := s.conn.ReadMessage()
	s.rLock.Unlock()
	return kind, content, err
}

func (s *WSConn) WriteMessage(messageType int, data []byte) error {
	s.wLock.Lock()
	now := time.Now()
	then := now.Add(WSTimeout)

	if err := s.conn.SetWriteDeadline(then); err != nil {
		s.rLock.Unlock()
		return err
	}

	err := s.conn.WriteMessage(messageType, data)
	s.wLock.Unlock()
	return err
}

func (s *WSConn) WriteControl(messageType int, data []byte) error {
	s.wLock.Lock()
	now := time.Now()
	then := now.Add(WSTimeout)
	err := s.conn.WriteControl(messageType, data, then)
	s.wLock.Unlock()
	return err
}

type Node struct {
	Info     NodeInfo
	LastPing time.Time
	Conn     *WSConn
	ID       NodeID
	// Length of the slice is the number of workers of that node.
	// Therefore maps every worker to a possible jobID.
	// If the mapped jobID is `nil`, the worker is free and can start a job.
	WorkerState []*JobID
}

type NodeWeb struct {
	Info        NodeInfo  `json:"info"`
	LastPing    time.Time `json:"lastPing"`
	ID          string    `json:"id"`
	WorkerState []*JobID  `json:"workerState"`
}

func (m *JobManagerT) ListNodes() []NodeWeb {
	m.Nodes.Lock.RLock()
	defer m.Nodes.Lock.RUnlock()

	nodes := make([]NodeWeb, len(m.Nodes.Map))

	cnt := 0
	for nodeID, node := range m.Nodes.Map {
		node.Lock.RLock()

		workerState := make([]*JobID, len(node.Data.WorkerState))
		copy(workerState, node.Data.WorkerState)

		nodes[cnt] = NodeWeb{
			Info:        node.Data.Info,
			LastPing:    node.Data.LastPing,
			ID:          IDToString(nodeID),
			WorkerState: workerState,
		}
		cnt++

		node.Lock.RUnlock()
	}

	return nodes
}

type StateSync struct {
	WorkerIndex      uint16
	JobID            string
	Progress         float32
	Logs             []database.JobLog
	InterpreterState []byte
}

//
// Processes a job state sync from a node.
// If this is the first state sync from that job on that node, put the job into the `running state`
// since it was previously in `starting`.
//

func (m *JobManagerT) StateSync(nodeID NodeID, state StateSync) error {
	jobID, err := m.StateSyncWithLockingOps(nodeID, state)
	if err != nil {
		return err
	}

	m.updateSingleJobState(jobID)

	return nil
}

func (m *JobManagerT) StateSyncWithLockingOps(nodeID NodeID, state StateSync) (JobID, error) {
	node, found := m.Nodes.Get(nodeID)
	if !found {
		return "", fmt.Errorf("state sync: node `%s` was not found", IDToString(nodeID))
	}

	node.Lock.RLock()
	numWorkers := node.Data.Info.NumWorkers
	node.Lock.RUnlock()

	if state.WorkerIndex >= numWorkers {
		return "", fmt.Errorf("state sync: worker %d is illegal", state.WorkerIndex)
	}

	node.Lock.RLock()
	jobID := node.Data.WorkerState[state.WorkerIndex]
	node.Lock.RUnlock()
	if jobID == nil {
		return "", fmt.Errorf("state sync: worker %d on node `%s` is idle", state.WorkerIndex, IDToString(nodeID))
	}

	m.NonFinishedJobs.Lock.Lock()
	defer m.NonFinishedJobs.Lock.Unlock()

	job, found := m.NonFinishedJobs.Map[*jobID]
	if !found {
		log.Debugf(
			"State sync: job `%s` not found on node `%s` worker %d",
			*jobID,
			IDToString(nodeID),
			state.WorkerIndex,
		)
		return "", nil
	}

	//
	// Lock job.
	//
	job.Lock.Lock()

	job.Data.Logs = append(job.Data.Logs, state.Logs...)
	job.Data.Progress = state.Progress

	job.Data.InterpreterState = state.InterpreterState

	jobStatus := job.Data.Status

	// If this is the job is in the `starting` state, put it into `running`.
	if jobStatus == StatusStarting {
		job.Data.Status = StatusRunning
		log.Debugf("Set status of job `%s` to `running", *jobID)
	}

	//
	// Unlock job.
	//
	job.Lock.Unlock()

	log.Debugf("State sync job `%s` worker %d, progress %3f%%", *jobID, state.WorkerIndex, state.Progress)

	return *jobID, nil
}

func (m *JobManagerT) ConnectNode(node NodeInfo, conn *WSConn) (nodeObj Node) {
	newID := sha256.Sum256(append([]byte(node.EndpointIP), []byte(node.Name)...))
	newIDString := hex.EncodeToString(newID[0:])

	if _, alreadyExists := m.Nodes.Get(nodeObj.ID); alreadyExists {
		panic(fmt.Sprintf("[bug] node with ID %x already exists", newID))
	}

	now := time.Now()
	nodeObj = Node{
		Info:        node,
		LastPing:    now,
		Conn:        conn,
		ID:          newID,
		WorkerState: make([]*string, node.NumWorkers),
	}

	m.Nodes.Insert(newID, NewLockedValue(nodeObj))

	log.Infof(
		"[node] Handshake success: connected to new node `%s` ip=`%s` name=`%s` with %d workers",
		newIDString,
		node.EndpointIP,
		node.Name,
		node.NumWorkers,
	)

	m.updateNodeState()

	return nodeObj
}

func (m *JobManagerT) DropNode(id NodeID) bool {
	node, exists := m.Nodes.Get(id)
	if !exists {
		return false
	}

	node.Lock.RLock()
	nodeID := node.Data.ID
	node.Lock.RUnlock()

	//
	// Put every job which was running on the node back into <queued>.
	//

	node.Lock.RLock()
	for _, potentialJob := range node.Data.WorkerState {
		if potentialJob == nil {
			continue
		}

		jobID := *potentialJob

		log.Infof(
			"[node] Job `%s` has lost its node (%s)",
			jobID,
			IDToString(nodeID),
		)

		if err := JobManager.ParkJob(jobID); err != nil {
			log.Errorf("Could not park job: %s", err.Error())
		}
	}
	node.Lock.RUnlock()

	m.Nodes.Delete(id)

	log.Debugf("[node] Dropped node with ID `%s`", IDToString(id))

	m.updateNodeState()

	return true
}

func (m *JobManagerT) RegisterPing(id NodeID) bool {
	node, found := m.Nodes.Get(id)
	if !found {
		return false
	}

	node.Lock.Lock()
	node.Data.LastPing = time.Now()
	node.Lock.Unlock()

	log.Debugf("[node] Received ping for node with ID `%s`", IDToString(id))

	m.updateNodeState()

	return true
}

func newJobManager(
	compiler *CompilerManager,
	triggerUIUpdates chan DataCollectionMsg,
	refreshData chan WebSocketTopic,
) JobManagerT {
	return JobManagerT{
		Compiler:             compiler,
		Nodes:                newLockedMap[NodeID, LockedValue[Node]](),
		NonFinishedJobs:      newLockedMap[JobID, LockedValue[Job]](),
		SendUIUpdatesTo:      triggerUIUpdates,
		RequestToRefreshData: refreshData,
	}
}

//
// UI state updates.
//

func (m *JobManagerT) updateNodeState() {
	nodes := m.ListNodes()
	m.SendUIUpdatesTo <- DataCollectionMsg{
		Topic: WebSocketTopic{
			Kind:       WSTopicNodes,
			Additional: nil,
		},
		Data: nodes,
	}
}

func (m *JobManagerT) updateAllJobStates() {
	jobs, err := m.ListJobs()

	if err != nil {
		log.Errorf("Could not notify UI about job state change: %s", err.Error())
		return
	}

	m.SendUIUpdatesTo <- DataCollectionMsg{
		Topic: WebSocketTopic{
			Kind:       WSTopicAllJobs,
			Additional: nil,
		},
		Data: jobs,
	}
}

func (m *JobManagerT) updateSingleJobState(jobID string) {
	job, found, err := m.GetJob(jobID, true)

	if err != nil {
		log.Errorf("Could not notify UI about single job state change: job `%s` caused error: %s", jobID, err.Error())
		return
	}

	if !found {
		log.Errorf("Could not notify UI about single job state change: job `%s` not found", jobID)
		return
	}

	m.SendUIUpdatesTo <- DataCollectionMsg{
		Topic: WebSocketTopic{
			Kind:       WSTopicSingleJob,
			Additional: &jobID,
		},
		Data: job,
	}

	m.updateAllJobStates()
}
