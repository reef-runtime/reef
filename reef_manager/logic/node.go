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
	Name       string `json:"string"`
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
	LastPing *time.Time
	Conn     *WSConn
	ID       NodeID
	// Length of the slice is the number of workers of that node.
	// Therefore maps every worker to a possible jobID.
	// If the mapped jobID is `nil`, the worker is free and can start a job.
	WorkerState []*JobID
}

type NodeWeb struct {
	Info        NodeInfo   `json:"info"`
	LastPing    *time.Time `json:"lastPing"`
	ID          string     `json:"id"`
	WorkerState []*JobID   `json:"workerState"`
}

func (m *JobManagerT) ListNodes() []NodeWeb {
	m.Nodes.Lock.RLock()
	defer m.Nodes.Lock.RUnlock()

	nodes := make([]NodeWeb, len(m.Nodes.Map))

	cnt := 0
	for nodeID, node := range m.Nodes.Map {
		nodes[cnt] = NodeWeb{
			Info:        node.Info,
			LastPing:    node.LastPing,
			ID:          IDToString(nodeID),
			WorkerState: node.WorkerState,
		}
		cnt++
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

func (m *JobManagerT) StateSync(nodeID NodeID, state StateSync) error {
	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	node, found := m.Nodes.Map[nodeID]
	if !found {
		return fmt.Errorf("state sync: node `%s` was not found", IDToString(nodeID))
	}

	if state.WorkerIndex >= node.Info.NumWorkers {
		return fmt.Errorf("state sync: worker %d is illegal", state.WorkerIndex)
	}

	jobID := node.WorkerState[state.WorkerIndex]
	if jobID == nil {
		return fmt.Errorf("state sync: worker %d on node `%s` is idle", state.WorkerIndex, IDToString(nodeID))
	}

	m.NonFinishedJobs.Lock.Lock()
	defer m.NonFinishedJobs.Lock.Unlock()

	job, found := m.NonFinishedJobs.Map[*jobID]
	if !found {
		log.Debugf(
			"state sync: job `%s` not found on node `%s` worker %d",
			*jobID,
			IDToString(nodeID),
			state.WorkerIndex,
		)
		return nil
	}

	newJob := job
	newJob.Logs = append(newJob.Logs, state.Logs...)
	newJob.Progress = state.Progress
	newJob.InterpreterState = state.InterpreterState

	m.NonFinishedJobs.Map[*jobID] = newJob

	log.Debugf("State sync job `%s` worker %d, progress %3f%%", *jobID, state.WorkerIndex, state.Progress)
	return nil
}

func (m *JobManagerT) GetNode(id NodeID) (node Node, found bool) {
	node, found = m.Nodes.Get(id)
	return node, found
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
		LastPing:    &now,
		Conn:        conn,
		ID:          newID,
		WorkerState: make([]*string, node.NumWorkers),
	}

	m.Nodes.Insert(newID, nodeObj)

	log.Infof(
		"[node] Handshake success: connected to new node `%s` ip=`%s` name=`%s` with %d workers",
		newIDString,
		node.EndpointIP,
		node.Name,
		node.NumWorkers,
	)

	return nodeObj
}

func (m *JobManagerT) DropNode(id NodeID) bool {
	m.Nodes.Lock.Lock()
	node, exists := m.Nodes.Map[id]
	m.Nodes.Lock.Unlock()

	if !exists {
		return false
	}

	//
	// Put every job which was running on the node back into <queued>
	//

	for _, potentialJob := range node.WorkerState {
		if potentialJob == nil {
			continue
		}

		jobID := *potentialJob

		log.Infof(
			"[node] Job `%s` has lost its node (%s)",
			jobID,
			IDToString(node.ID),
		)

		if err := JobManager.ParkJob(jobID); err != nil {
			log.Errorf("Could not park job: %s", err.Error())
		}
	}

	m.Nodes.Delete(id)

	log.Debugf("[node] Dropped node with ID `%s`", IDToString(id))

	return true
}

func (m *JobManagerT) RegisterPing(id NodeID) bool {
	if _, found := m.Nodes.Get(id); !found {
		return false
	}

	now := time.Now()
	m.Nodes.Lock.Lock()
	*m.Nodes.Map[id].LastPing = now
	m.Nodes.Lock.Unlock()

	log.Debugf("[node] Received ping for node with ID `%s`", IDToString(id))

	return true
}

func newManager(compiler *CompilerManager) JobManagerT {
	return JobManagerT{
		Compiler:        compiler,
		Nodes:           newLockedMap[NodeID, Node](),
		NonFinishedJobs: newLockedMap[JobID, Job](),
	}
}
