package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type NodeInfo struct {
	EndpointIP string `json:"endpointIP"`
	Name       string `json:"string"`
	// TODO: maybe worker descriptions
	// TODO: maybe the current state of the node?
	NumWorkers uint16 `json:"numWorkers"`
}

type Node struct {
	Info     NodeInfo
	LastPing *time.Time
}

type NodeMap struct {
	Map  map[[32]byte]Node
	Lock sync.Mutex
}

type NodeManagerT struct {
	Nodes NodeMap
}

var NodeManager NodeManagerT

func IdToString(id [32]byte) string {
	return hex.EncodeToString(id[0:])
}

func (m *NodeManagerT) ConnectNode(node NodeInfo) (newID [32]byte) {
	newID = sha256.Sum256(append([]byte(node.EndpointIP), []byte(node.Name)...))
	newIDString := hex.EncodeToString(newID[0:])

	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	if _, alreadyExists := m.Nodes.Map[newID]; alreadyExists {
		panic(fmt.Sprintf("[bug] node with ID %x already exists", newID))
	}

	now := time.Now().Local()

	m.Nodes.Map[newID] = Node{
		Info:     node,
		LastPing: &now,
	}

	log.Infof(
		"[node] Handshake success: connected to new node `%s` ip=`%s` name=`%s` with %d workers",
		newIDString,
		node.EndpointIP,
		node.Name,
		node.NumWorkers,
	)

	return newID
}

func (m *NodeManagerT) DropNode(id [32]byte) bool {
	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	_, exists := m.Nodes.Map[id]
	if !exists {
		return false
	}

	delete(m.Nodes.Map, id)

	log.Debugf("[node] Dropped node with ID `%s`", IdToString(id))

	return true
}

func (m *NodeManagerT) RegisterPing(id [32]byte) bool {
	m.Nodes.Lock.Lock()
	defer m.Nodes.Lock.Unlock()

	if _, found := m.Nodes.Map[id]; !found {
		return false
	}

	now := time.Now().Local()
	*m.Nodes.Map[id].LastPing = now

	log.Debugf("[node] Received ping for node with ID `%s`", IdToString(id))

	return true
}

func newNodeManager() NodeManagerT {
	return NodeManagerT{
		Nodes: NodeMap{
			Map:  make(map[[32]byte]Node),
			Lock: sync.Mutex{},
		},
	}
}
