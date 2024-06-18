package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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
	Conn *websocket.Conn
	Lock sync.Mutex
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

type NodeMap struct {
	Map  map[NodeID]Node
	Lock sync.RWMutex
}

type NodeManagerT struct {
	Nodes NodeMap
}

type NodeWeb struct {
	Info        NodeInfo   `json:"info"`
	LastPing    *time.Time `json:"lastPing"`
	ID          string     `json:"id"`
	WorkerState []*JobID   `json:"workerState"`
}

var NodeManager NodeManagerT

func (m *NodeManagerT) ListNodes() []NodeWeb {
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

func (m *NodeManagerT) GetNode(id NodeID) (node Node, found bool) {
	m.Nodes.Lock.RLock()
	defer m.Nodes.Lock.RUnlock()

	nodeRaw, found := m.Nodes.Map[id]
	return nodeRaw, found
}

func (m *NodeManagerT) ConnectNode(node NodeInfo, conn *WSConn) (nodeObj Node) {
	newID := sha256.Sum256(append([]byte(node.EndpointIP), []byte(node.Name)...))
	newIDString := hex.EncodeToString(newID[0:])

	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	if _, alreadyExists := m.Nodes.Map[newID]; alreadyExists {
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

	m.Nodes.Map[newID] = nodeObj

	log.Infof(
		"[node] Handshake success: connected to new node `%s` ip=`%s` name=`%s` with %d workers",
		newIDString,
		node.EndpointIP,
		node.Name,
		node.NumWorkers,
	)

	return nodeObj
}

func (m *NodeManagerT) DropNode(id NodeID) bool {
	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	node, exists := m.Nodes.Map[id]
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

	delete(m.Nodes.Map, id)

	log.Debugf("[node] Dropped node with ID `%s`", IDToString(id))

	return true
}

func (m *NodeManagerT) RegisterPing(id NodeID) bool {
	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	if _, found := m.Nodes.Map[id]; !found {
		return false
	}

	now := time.Now().Local()
	*m.Nodes.Map[id].LastPing = now

	log.Debugf("[node] Received ping for node with ID `%s`", IDToString(id))

	return true
}

//
// TODO: write code that looks at the queued jobs and dispatches it to a free node.
//

func newNodeManager() NodeManagerT {
	return NodeManagerT{
		Nodes: NodeMap{
			Map:  make(map[NodeID]Node),
			Lock: sync.RWMutex{},
		},
	}
}
