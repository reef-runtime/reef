package api

import (
	"fmt"

	"capnproto.org/go/capnp/v3"
	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/logic"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

//
// Node Handshake Messages.
//

func createHandshakeInitMsg() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNode, err := node.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	toNode.SetKind(node.MessageToNodeKind_initHandShake)
	toNode.Body().SetEmpty()

	return msg.Marshal()
}

func createAssignIDMsg(nodeID logic.NodeId) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNodeMsg, err := node.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	toNodeMsg.SetKind(node.MessageToNodeKind_assignId)

	// Nested.
	assignIDMsg, err := node.NewAssignIdMessage(seg)
	if err != nil {
		return nil, err
	}

	if err := assignIDMsg.SetNodeId(nodeID[:]); err != nil {
		return nil, err
	}

	if err := toNodeMsg.Body().SetAssignId(assignIDMsg); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

//
// Node Handshake logic.
//

func HandleNodeConnection(c *gin.Context) {
	// TODO: maybe add timeouts.
	conn, err := logic.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsConn := logic.NewWSConn(conn)

	clientIP := c.ClientIP()
	spew.Dump(c.Request.Header)

	node, err := performHandshake(wsConn, clientIP)
	if err != nil {
		log.Errorf("[node] handshake with `%s` failed: %s", conn.RemoteAddr(), err.Error())
		return
	}

	// Add node to manager.

	pingHandler := messagePingHandler(wsConn, node.Id)
	conn.SetPingHandler(pingHandler)

	conn.SetPongHandler(func(_ string) error {
		log.Tracef("[node] received pong from node `%s`", logic.IdToString(node.Id))
		return nil
	})

	conn.SetCloseHandler(func(code int, _ string) error {
		dropNode(wsConn, code, node.Id)
		return nil
	})

	// Blocking receive loop.
	nodeMainLoop(wsConn, node, pingHandler)
}

//nolint:funlen
func performHandshake(conn *logic.WSConn, endpointIP string) (logic.Node, error) {
	initMsg, err := createHandshakeInitMsg()
	if err != nil {
		return logic.Node{}, err
	}

	err = conn.WriteMessage(
		websocket.BinaryMessage,
		initMsg,
	)
	if err != nil {
		return logic.Node{}, err
	}

	typ, message, err := conn.ReadMessage()
	if err != nil {
		return logic.Node{}, err
	}

	if typ != websocket.BinaryMessage {
		return logic.Node{}, fmt.Errorf("expected answer to handshake initialization to be binary, got %d", typ)
	}

	unmarshaledRaw, err := capnp.Unmarshal(message)
	if err != nil {
		return logic.Node{}, err
	}

	decodedEnclosingMsg, err := node.ReadRootMessageFromNode(unmarshaledRaw)
	if err != nil {
		// nolint:goconst
		return logic.Node{}, fmt.Errorf("could not read handshake response: %s", err.Error())
	}

	kind := decodedEnclosingMsg.Kind()

	switch kind {
	case node.MessageFromNodeKind_handshakeResponse:
		log.Tracef("Received handshakeResponse")
	case node.MessageFromNodeKind_jobStateSync, node.MessageFromNodeKind_jobResult:
		fallthrough
	default:
		return logic.Node{}, fmt.Errorf("received illegal/unexpected message kind from node during handshake: %d", kind)
	}

	if decodedEnclosingMsg.Body().Which() != node.MessageFromNode_body_Which_handshakeResponse {
		return logic.Node{}, fmt.Errorf(
			"received illegal body kind from node during handshake: %d",
			decodedEnclosingMsg.Body().Which(),
		)
	}

	handshakeResponse, err := decodedEnclosingMsg.Body().HandshakeResponse()
	if err != nil {
		return logic.Node{}, fmt.Errorf("could not parse massage body from node during handshake: %s", err.Error())
	}

	numWorkers := handshakeResponse.NumWorkers()
	nodeName, err := handshakeResponse.NodeName()
	if err != nil {
		return logic.Node{}, fmt.Errorf("could not read node name: %s", err.Error())
	}

	//
	// Adding the node.
	//

	newNode := logic.JobManager.ConnectNode(logic.NodeInfo{
		EndpointIP: endpointIP,
		Name:       nodeName,
		NumWorkers: numWorkers,
	}, conn)

	assignIDMsg, err := createAssignIDMsg(newNode.Id)
	if err != nil {
		return logic.Node{}, err
	}

	err = conn.WriteMessage(websocket.BinaryMessage, assignIDMsg)

	if err != nil {
		log.Warnf(
			"[node] handshake with `%s` failed: could not deliver Id to node: %s",
			newNode.Info.EndpointIP,
			err.Error(),
		)

		return logic.Node{}, err
	}

	return newNode, nil
}
