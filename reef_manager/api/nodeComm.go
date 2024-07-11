package api

import (
	"fmt"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

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

func MessageToNodeAssignId(nodeId logic.NodeId) ([]byte, error) {
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

	assignIdMsg, err := node.NewAssignIdMessage(seg)
	if err != nil {
		return nil, err
	}

	if err := assignIdMsg.SetNodeId(nodeId[:]); err != nil {
		return nil, err
	}

	if err := toNodeMsg.Body().SetAssignId(assignIdMsg); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

func performHandshake(conn *logic.WSConn) (logic.Node, error) {
	initMsg, err := MessageToNodeEmptyMessage(node.MessageToNodeKind_initHandShake)
	if err != nil {
		return logic.Node{}, err
	}

	endpointIP := conn.RemoteAddr()
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
		return logic.Node{}, fmt.Errorf("could not read handshake response: %s", err.Error())
	}

	kind := decodedEnclosingMsg.Kind()

	switch kind {
	case node.MessageFromNodeKind_handshakeResponse:
		log.Tracef("Received handshakeResponse")
	default:
		return logic.Node{}, fmt.Errorf("received illegal/unexpected message kind from node during handshake: %d", kind)
	}

	if decodedEnclosingMsg.Body().Which() != node.MessageFromNode_body_Which_handshakeResponse {
		return logic.Node{}, fmt.Errorf("received illegal body kind from node during handshake: %d", decodedEnclosingMsg.Body().Which())
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
		EndpointIP: endpointIP.String(),
		Name:       nodeName,
		NumWorkers: numWorkers,
	}, conn)

	assignIdMsg, err := MessageToNodeAssignId(newNode.Id)
	if err != nil {
		return logic.Node{}, err
	}

	err = conn.WriteMessage(websocket.BinaryMessage, assignIdMsg)

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

//
// Dropping Nodes.
//

func dropNode(conn *logic.WSConn, closeCode int, nodeId logic.NodeId) {
	nodeIdString := logic.IdToString(nodeId)
	log.Debugf("[node] Dropping node `%s`...", nodeIdString)

	message := websocket.FormatCloseMessage(closeCode, "")
	if err := conn.WriteControl(websocket.CloseMessage, message); err != nil {
		log.Warnf("[node] did not respond to close message in time")
	}

	if err := conn.Close(); err != nil {
		log.Warnf("[node] could not close TCP connection")
	}

	if !logic.JobManager.DropNode(nodeId) {
		log.Errorf("[node] dropping unknown node `%s`, this is a bug", nodeIdString)
	}
}

//
// Ping & Heartbeating.
//

func pingMessage() ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNode, err := node.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	kind := node.MessageToNodeKind_ping

	toNode.SetKind(kind)
	toNode.Body().SetEmpty()

	return msg.Marshal()
}

func messagePingHandler(conn *logic.WSConn, nodeId logic.NodeId) func(string) error {
	return func(_ string) error {
		msg, err := pingMessage()
		if err != nil {
			return err
		}

		log.Tracef("[node] received ping from `%s`", logic.IdToString(nodeId))

		err = conn.WriteMessage(websocket.PongMessage, msg)

		if err != nil {
			log.Tracef("[node] sending pong failed: %s", err.Error())
			return err
		}

		setLastPing(nodeId)

		return nil
	}
}

func setLastPing(nodeId logic.NodeId) {
	if !logic.JobManager.RegisterPing(nodeId) {
		log.Errorf(
			"[node] could not register ping, node `%s` does not exist, this is a bug",
			logic.IdToString(nodeId),
		)
	}
}

func HandleNodeConnection(c *gin.Context) {
	// TODO: add timeouts

	conn, err := logic.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsConn := logic.NewWSConn(conn)

	node, err := performHandshake(wsConn)
	if err != nil {
		log.Errorf("[node] handshake with `%s` failed: %s", conn.RemoteAddr(), err.Error())
		return
	}

	// TODO: use wrapper

	// Add node to manager.

	pingHandler := messagePingHandler(wsConn, node.Id)
	conn.SetPingHandler(pingHandler)

	conn.SetPongHandler(func(appData string) error {
		log.Tracef("RECEIVED PONG!")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		dropNode(wsConn, code, node.Id)
		return nil
	})

	// TODO: place the receive loop somewhere else.

	// Blocking receive loop.
	for {
		// Timeout 0 means wait forever.
		msgType, message, err := wsConn.ReadMessageWithTimeout(time.Time{})
		if err != nil {
			log.Debugf("[node] error while reading message: %s", err.Error())
			dropNode(wsConn, websocket.CloseAbnormalClosure, node.Id)
			break
		}

		switch msgType {
		case websocket.TextMessage:
			fmt.Printf("text: %s\n", string(message))
		case websocket.BinaryMessage:
			// log.Tracef("[node] received binary message: %s", logic.FormatBinarySliceAsHex(message))
			setLastPing(node.Id)

			if err := handleGenericIncoming(node, message); err != nil {
				log.Errorf("[node] failed to act upon message: %s", err.Error())
				dropNode(wsConn, websocket.CloseAbnormalClosure, node.Id)
				return
			}
		case websocket.PingMessage:
			if err := pingHandler(string(message[1:])); err != nil {
				log.Warnf("[node] Dropping due to illegal ping: %v", message)
				dropNode(wsConn, websocket.CloseAbnormalClosure, node.Id)
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

func handleGenericIncoming(nodeData logic.Node, message []byte) error {
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
	case node.MessageFromNodeKind_handshakeResponse:
		log.Tracef("Received late handshakeResponse from node `%s` (%s)", logic.IdToString(nodeData.Id), nodeData.Info.EndpointIP)
		return nil
	case node.MessageFromNodeKind_jobStateSync:
		return processStateSyncFromNode(nodeData.Id, decodedEnclosingMsg)
	case node.MessageFromNodeKind_jobResult:
		return processJobResultFromNode(nodeData.Id, decodedEnclosingMsg)
	default:
		return fmt.Errorf("received illegal message kind from node: %d", kind)
	}
}

func processStateSyncFromNode(nodeId logic.NodeId, message node.MessageFromNode) error {
	parsed, err := parseStateSync(nodeId, message)
	if err != nil {
		return err
	}

	if err := logic.JobManager.StateSync(nodeId, parsed); err != nil {
		return err
	}

	return nil
}

func parseStateSync(nodeId logic.NodeId, message node.MessageFromNode) (logic.StateSync, error) {
	if message.Body().Which() != node.MessageFromNode_body_Which_jobStateSync {
		panic("assertion failed: expected body type is job state sync, got something different")
	}

	result, err := message.Body().JobStateSync()
	if err != nil {
		return logic.StateSync{}, fmt.Errorf("could not parse job state sync: %s", err.Error())
	}

	nodeInfo, found := logic.JobManager.Nodes.Get(nodeId)
	if !found {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"illegal node: node Id `%s` references non-existent node",
			logic.IdToString(nodeId),
		)
	}

	workerIndex := result.WorkerIndex()

	nodeInfo.Lock.RLock()
	numWorkers := nodeInfo.Data.Info.NumWorkers
	nodeInfo.Lock.RUnlock()

	if workerIndex >= numWorkers {
		// nolint:goconst
		return logic.StateSync{}, fmt.Errorf(
			"illegal worker index: node returned illegal worker index (%d) when num workers is %d",
			workerIndex,
			numWorkers,
		)
	}

	nodeInfo.Lock.RLock()
	jobId := nodeInfo.Data.WorkerState[workerIndex]
	nodeInfo.Lock.RUnlock()

	if jobId == nil {
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

		content, err := currLog.Content()
		if err != nil {
			return logic.StateSync{}, fmt.Errorf("could not decode log message content: %s", err.Error())
		}

		logsOutput[idx] = database.JobLog{
			Kind:    logKind,
			Created: time.Now(),
			Content: string(content),
			JobId:   *jobId,
		}
	}

	return logic.StateSync{
		WorkerIndex:      workerIndex,
		JobId:            *jobId,
		Progress:         progress,
		Logs:             logsOutput,
		InterpreterState: interpreterState,
	}, nil
}

func processJobResultFromNode(nodeId logic.NodeId, message node.MessageFromNode) error {
	result, err := parseJobResultFromNode(nodeId, message)
	if err != nil {
		return fmt.Errorf("parse job result: %s", err.Error())
	}

	log.Debugf("Job result: %s", result.String())

	if err := logic.JobManager.ProcessResult(nodeId, result); err != nil {
		return err
	}

	return nil
}

func parseJobResultFromNode(nodeId logic.NodeId, message node.MessageFromNode) (logic.JobResult, error) {
	if message.Body().Which() != node.MessageFromNode_body_Which_jobResult {
		panic("assertion failed: expected body type is job result, got something different")
	}

	result, err := message.Body().JobResult()
	if err != nil {
		return logic.JobResult{}, fmt.Errorf("could not parse job result: %s", err.Error())
	}

	nodeInfo, found := logic.JobManager.Nodes.Get(nodeId)
	if !found {
		return logic.JobResult{}, fmt.Errorf(
			"illegal node: node Id `%s` references non-existent node",
			logic.IdToString(nodeId),
		)
	}

	workerIndex := result.WorkerIndex()

	nodeInfo.Lock.RLock()
	nodeNumWorkers := nodeInfo.Data.Info.NumWorkers
	nodeInfo.Lock.RUnlock()

	if workerIndex >= nodeNumWorkers {
		return logic.JobResult{}, fmt.Errorf(
			"illegal worker index: node returned illegal worker index (%d) when num workers is %d",
			workerIndex,
			nodeNumWorkers,
		)
	}

	nodeInfo.Lock.RLock()
	jobId := nodeInfo.Data.WorkerState[workerIndex]
	nodeInfo.Lock.RUnlock()

	if jobId == nil {
		return logic.JobResult{}, fmt.Errorf(
			"wrong worker index: node returned worker index (%d), this worker is idle",
			workerIndex,
		)
	}

	contents, err := result.Contents()
	if err != nil {
		return logic.JobResult{}, fmt.Errorf("could not decode result content bytes: %s", err.Error())
	}

	return logic.JobResult{
		JobId:       *jobId,
		WorkerIndex: workerIndex,
		Success:     result.Success(),
		ContentType: result.ContentType(),
		Contents:    contents,
	}, nil
}
