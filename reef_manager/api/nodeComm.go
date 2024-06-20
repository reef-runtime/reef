package api

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/database"
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

	toNodeMsg.SetKind(node.MessageToNodeKind_assignId)

	//
	// Nested.
	//

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

	newNode := logic.JobManager.ConnectNode(logic.NodeInfo{
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

	if !logic.JobManager.DropNode(nodeID) {
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

		if !logic.JobManager.RegisterPing(nodeID) {
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

			if err := handleGenericIncoming(node, message, pingHandler); err != nil {
				log.Errorf("[node] failed to act upon message: %s", err.Error())
				dropNode(wsConn, websocket.CloseAbnormalClosure, node.ID)
				return
			}
		case websocket.PingMessage:
			if err := pingHandler(string(message[1:])); err != nil {
				dropNode(wsConn, websocket.CloseAbnormalClosure, node.ID)
				return
			}
		case websocket.PongMessage:
			fmt.Printf("pong: %x\n", message)
		case websocket.CloseMessage:
			// TODO: handle close
			fmt.Println("closing...")
			return
		}
	}
}

func handleGenericIncoming(nodeData logic.Node, message []byte, pingHandler func(string) error) error {
	unmarshaledRaw, err := capnp.Unmarshal(message)
	if err != nil {
		return err
	}

	decodedEnclosingMsg, err := node.ReadRootMessageFromNode(unmarshaledRaw)
	if err != nil {
		return fmt.Errorf("could not read handshake response: %s", err.Error())
	}

	kind := decodedEnclosingMsg.Kind()

	switch kind {
	case node.MessageFromNodeKind_ping:
		return pingHandler(string(message[1:]))
	case node.MessageFromNodeKind_pong:
		log.Tracef("Received pong from node `%s` (%s)", logic.IDToString(nodeData.ID), nodeData.Info.EndpointIP)
		return nil
	case node.MessageFromNodeKind_jobStateSync:
		return processStateSyncFromNode(nodeData.ID, decodedEnclosingMsg)
	case node.MessageFromNodeKind_jobResult:
		return processJobResultFromNode(nodeData.ID, decodedEnclosingMsg)
	default:
		return fmt.Errorf("received illegal message kind from node: %d", kind)
	}
}

func processStateSyncFromNode(nodeID logic.NodeID, message node.MessageFromNode) error {
	parsed, err := parseStateSync(nodeID, message)
	if err != nil {
		return err
	}

	if err := logic.JobManager.StateSync(nodeID, parsed); err != nil {
		return err
	}

	return nil
}

func parseStateSync(nodeID logic.NodeID, message node.MessageFromNode) (logic.StateSync, error) {
	if message.Body().Which() != node.MessageFromNode_body_Which_jobStateSync {
		panic("assertion failed: expected body type is job state sync, got something different")
	}

	result, err := message.Body().JobStateSync()
	if err != nil {
		return logic.StateSync{}, fmt.Errorf("could not parse job state sync: %s", err.Error())
	}

	nodeInfo, found := logic.JobManager.GetNode(nodeID)
	if !found {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"illegal node: node ID `%s` references non-existent node",
			logic.IDToString(nodeID),
		)
	}

	workerIndex := result.WorkerIndex()
	if workerIndex >= nodeInfo.Info.NumWorkers {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"illegal worker index: node returned illegal worker index (%d) when num workers is %d",
			workerIndex,
			nodeInfo.Info.NumWorkers,
		)
	}

	jobID := nodeInfo.WorkerState[workerIndex]

	if jobID == nil {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"wrong worker index: node returned worker index (%d), this worker is idle",
			workerIndex,
		)
	}

	interpreterState, err := result.Interpreter()
	if err != nil {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf("could not decode interpreter state bytes: %s", err.Error())
	}

	progress := result.Progress()

	logs, err := result.Logs()
	if err != nil {
		return logic.StateSync{}, fmt.Errorf("could not decode interpreter state bytes: %s", err.Error())
	}

	logsLen := logs.Len()
	logsOutput := make([]database.JobLog, logsLen)

	for idx := 0; idx < logsLen; idx++ {
		currLog := logs.At(idx)

		if !database.IsValidLogKind(currLog.LogKind()) {
			return logic.StateSync{}, fmt.Errorf("invalid log kind `%d` received", currLog.LogKind())
		}

		logKind := database.LogKind(currLog.LogKind())
		now := time.Now()

		content, err := currLog.Content()
		if err != nil {
			return logic.StateSync{}, fmt.Errorf("could not decode log message content: %s", err.Error())
		}

		logsOutput[idx] = database.JobLog{
			Kind:    logKind,
			Created: now,
			Content: string(content),
			JobID:   *jobID,
		}
	}

	return logic.StateSync{
		WorkerIndex:      workerIndex,
		JobID:            *jobID,
		Progress:         progress,
		Logs:             logsOutput,
		InterpreterState: interpreterState,
	}, nil
}

func processJobResultFromNode(nodeID logic.NodeID, message node.MessageFromNode) error {
	if message.Body().Which() != node.MessageFromNode_body_Which_jobResult {
		panic("assertion failed: expected body type is job result, got something different")
	}

	result, err := message.Body().JobResult()
	if err != nil {
		return fmt.Errorf("could not parse job result: %s", err.Error())
	}

	nodeInfo, found := logic.JobManager.GetNode(nodeID)
	if !found {
		return fmt.Errorf("illegal node: node ID `%s` references non-existent node", logic.IDToString(nodeID))
	}

	workerIndex := result.WorkerIndex()
	if workerIndex >= nodeInfo.Info.NumWorkers {
		return fmt.Errorf(
			"illegal worker index: node returned illegal worker index (%d) when num workers is %d",
			workerIndex,
			nodeInfo.Info.NumWorkers,
		)
	}

	jobID := nodeInfo.WorkerState[workerIndex]

	if jobID == nil {
		return fmt.Errorf(
			"wrong worker index: node returned worker index (%d), this worker is idle",
			workerIndex,
		)
	}

	contents, err := result.Contents()
	if err != nil {
		return fmt.Errorf("could not decode result content bytes: %s", err.Error())
	}

	switch result.Success() {
	case true:
		log.Infof("Job `%s` has finished with SUCCESS", *jobID)
	case false:
		log.Infof("Job `%s` has finished with ERROR", *jobID)
	}

	contentType := result.ContentType()

	switch contentType {
	case node.ResultContentType_stringJSON, node.ResultContentType_stringPlain:
		strRes := string(contents)
		log.Infof("[STRING] Job result: `%s`", strRes)
	case node.ResultContentType_int64:
		intRes := int64(binary.LittleEndian.Uint64(contents))
		log.Infof("[INT64] Job result: `%d`", intRes)
	case node.ResultContentType_bytes:
		log.Infof("[INT64] Job result: `%s`", hex.EncodeToString(contents))
	default:
		return fmt.Errorf("node returned illegal content type: %d", contentType)
	}

	return nil
}

// func handleJobLog(body node.MessageFromNode_body) error {
// 	jobLog, err := node.ReadRootJobLogMessage(body.Message())
// 	if err != nil {
// 		return err
// 	}
//
// 	panic(jobLog)
// }

// func handleJobProgressReport(body node.MessageFromNode_body) error {
// 	jobProgress, err := node.ReadRootJobProgressReportMessage(body.Message())
// 	if err != nil {
// 		return err
// 	}
//
// 	panic(jobProgress)
// }
