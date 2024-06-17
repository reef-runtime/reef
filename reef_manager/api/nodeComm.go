package api

import (
	"fmt"
	"sync"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/logic"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func MessageToNodeEmptyMessage(kind node.MessageToNodeKind) ([]byte, error) {
	// msg, err := logic.MessageToNode()
	// if err != nil {
	// 	return nil, err
	// }

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

//
// Node Handshake.
//

func MessageToNodeAssignID(nodeID logic.NodeID) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNodeMsg, err := node.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	toNodeMsg.SetKind(node.MessageToNodeKind_assignID)

	//
	// Nested.
	//

	assignIDMsg, err := node.NewAssignIDMessage(seg)
	if err != nil {
		return nil, err
	}

	if err := assignIDMsg.SetNodeID(nodeID[:]); err != nil {
		return nil, err
	}

	if err := toNodeMsg.Body().SetAssignID(assignIDMsg); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

func performHandshake(conn *logic.WSConn) (logic.Node, error) {
	initMsg, err := MessageToNodeEmptyMessage(node.MessageToNodeKind_initHandShake)
	if err != nil {
		return logic.Node{}, err
	}

	conn.Lock.Lock()

	endpointIP := conn.Conn.RemoteAddr()
	err = conn.Conn.WriteMessage(
		websocket.BinaryMessage,
		initMsg,
	)

	conn.Lock.Unlock()

	if err != nil {
		return logic.Node{}, err
	}

	conn.Lock.Lock()
	typ, message, err := conn.Conn.ReadMessage()
	conn.Lock.Unlock()

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

	// spew.Dump(unmarshaledRaw)

	handshakeResponse, err := node.ReadRootHandshakeRespondMessage(unmarshaledRaw)
	if err != nil {
		return logic.Node{}, fmt.Errorf("could not read handshake response: %s", err.Error())
	}

	numWorkers := handshakeResponse.NumWorkers()
	nodeName, err := handshakeResponse.NodeName()
	if err != nil {
		return logic.Node{}, fmt.Errorf("could not read node name: %s", err.Error())
	}

	//
	// Adding the node.
	//

	newNode := logic.NodeManager.ConnectNode(logic.NodeInfo{
		EndpointIP: endpointIP.String(),
		Name:       nodeName,
		NumWorkers: numWorkers,
	}, conn)

	assignIDMsg, err := MessageToNodeAssignID(newNode.ID)
	if err != nil {
		return logic.Node{}, err
	}

	conn.Lock.Lock()
	err = conn.Conn.WriteMessage(websocket.BinaryMessage, assignIDMsg)
	conn.Lock.Unlock()

	if err != nil {
		log.Warnf(
			"[node] handshake with `%s` failed: could not deliver ID to node: %s",
			newNode.Info.EndpointIP,
			err.Error(),
		)

		return logic.Node{}, err
	}

	return newNode, nil
}

//
// Dropping Nodes.
//

func dropNode(conn *logic.WSConn, closeCode int, nodeID logic.NodeID) {
	nodeIDString := logic.IDToString(nodeID)
	log.Debugf("[node] Dropping node `%s`...", nodeIDString)

	const closeMessageTimeout = time.Second * 5
	message := websocket.FormatCloseMessage(closeCode, "")

	conn.Lock.Lock()
	err := conn.Conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(closeMessageTimeout))
	conn.Lock.Unlock()

	if err != nil {
		log.Warnf("[node] did not respond to close message in time")
	}

	conn.Lock.Lock()
	err = conn.Conn.Close()
	conn.Lock.Unlock()

	if err != nil {
		log.Warnf("[node] could not close TCP connection")
	}

	if !logic.NodeManager.DropNode(nodeID) {
		log.Errorf("[node] dropping unknown node `%s`, this is a bug", nodeIDString)
	}
}

//
// Ping & Heartbeating.
//

func pingOrPongMessage(isPing bool) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNode, err := node.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	kind := node.MessageToNodeKind_pong
	if isPing {
		kind = node.MessageToNodeKind_ping
	}

	toNode.SetKind(kind)
	toNode.Body().SetEmpty()

	return msg.Marshal()
}

func nodePingHandler(conn *logic.WSConn, nodeID logic.NodeID) func(string) error {
	return func(_ string) error {
		msg, err := pingOrPongMessage(false)
		if err != nil {
			return err
		}

		conn.Lock.Lock()
		err = conn.Conn.WriteMessage(websocket.BinaryMessage, msg)
		conn.Lock.Unlock()

		if err != nil {
			log.Tracef("[node] sending pong failed: %s", err.Error())
			return err
		}

		if !logic.NodeManager.RegisterPing(nodeID) {
			log.Errorf(
				"[node] could not register ping, node `%s` does not exist, this is a bug",
				logic.IDToString(nodeID),
			)
		}

		return nil
	}
}

func HandleNodeConnection(c *gin.Context) {
	// TODO: add timeouts

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsConn := &logic.WSConn{
		Conn: conn,
		Lock: sync.Mutex{},
	}

	node, err := performHandshake(wsConn)
	if err != nil {
		log.Errorf("[node] handshake with `%s` failed: %s", conn.RemoteAddr(), err.Error())
		return
	}

	// TODO: use wrapper

	// Add node to manager.

	pingHandler := nodePingHandler(wsConn, node.ID)
	conn.SetPingHandler(pingHandler)

	conn.SetPongHandler(func(appData string) error {
		log.Tracef("RECEIVED PONG!")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		dropNode(wsConn, code, node.ID)
		return nil
	})

	// TODO: place the receive loop somewhere else.

	// Blocking receive loop.
	for {
		msgType, message, err := conn.ReadMessage()
		if err != nil {
			log.Debugf("[node] error while reading message: %s", err.Error())
			dropNode(wsConn, websocket.CloseAbnormalClosure, node.ID)
			break
		}

		switch msgType {
		case websocket.TextMessage:
			fmt.Printf("text: %s\n", string(message))
		case websocket.BinaryMessage:
			log.Tracef("[node] received binary message: %s", logic.FormatBinarySliceAsHex(message))

			if err := handleGenericIncoming(message, pingHandler); err != nil {
				log.Errorf("[node] failed to act upon message: %s", err.Error())
				dropNode(wsConn, websocket.CloseAbnormalClosure, node.ID)
				return
			}

			// switch message[0] {
			// case CODE_PING:
			// 	if err := pingHandler(string(message[1:])); err != nil {
			// 		dropNode(wsConn, websocket.CloseAbnormalClosure, nodeID)
			// 		return
			// 	}
			// case logic.CODE_STARTED_JOB:
			// 	if err := logic.NodeManager.JobStartedJobCallback(nodeID, message); err != nil {
			// 		log.Errorf("job started error: %s", err.Error())
			// 		return
			// 	}
			// case logic.CODE_JOB_LOG:
			// 	if err := logic.NodeManager.NodeLogCallBack(nodeID, message); err != nil {
			// 		log.Errorf("job log error: %s", err.Error())
			// 		return
			// 	}
			// }
		case websocket.PingMessage:
			fmt.Printf("ping: %x\n", message)
		case websocket.PongMessage:
			fmt.Printf("pong: %x\n", message)
		case websocket.CloseMessage:
			fmt.Println("closing...")
			return
		}
	}
}

func handleGenericIncoming(message []byte, pingHandler func(string) error) error {
	unmarshaledRaw, err := capnp.Unmarshal(message)
	if err != nil {
		return err
	}

	decoded, err := node.ReadRootMessageFromNode(unmarshaledRaw)
	if err != nil {
		return fmt.Errorf("could not read handshake response: %s", err.Error())
	}

	switch decoded.Kind() {
	case node.MessageFromNodeKind_ping:
		return pingHandler(string(message[1:]))
	case node.MessageFromNodeKind_pong:
		panic("TODO: implement this.")
	case node.MessageFromNodeKind_jobLog:
		return handleJobLog(decoded.Body())
	case node.MessageFromNodeKind_jobProgressReport:
		return handleJobProgressReport(decoded.Body())
	default:
		// VERY BAD!
		panic("todo: better error handling")
	}
}

func handleJobLog(body node.MessageFromNode_body) error {
	jobLog, err := node.ReadRootJobLogMessage(body.Message())
	if err != nil {
		return err
	}

	panic(jobLog)
}

func handleJobProgressReport(body node.MessageFromNode_body) error {
	jobProgress, err := node.ReadRootJobProgressReportMessage(body.Message())
	if err != nil {
		return err
	}

	panic(jobProgress)
}
