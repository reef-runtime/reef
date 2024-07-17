package api

import (
	"fmt"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/logic"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

func nodeMainLoop(wsConn *logic.WSConn, node logic.Node, pingHandler func(string) error) {
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
			log.Warnf("[node] received text-kind WS message from `%s`", logic.IdToString(node.Id))
		case websocket.BinaryMessage:
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
			// This is already handled by the pong handler.
		case websocket.CloseMessage:
			// This is already handled by the close handler.
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
		log.Tracef(
			"Received late handshakeResponse from node `%s` (%s)",
			logic.IdToString(nodeData.Id),
			nodeData.Info.EndpointIP,
		)
		return nil
	case node.MessageFromNodeKind_jobStateSync:
		return processStateSyncFromNode(nodeData.Id, decodedEnclosingMsg)
	case node.MessageFromNodeKind_jobResult:
		return processJobResultFromNode(nodeData.Id, decodedEnclosingMsg)
	default:
		return fmt.Errorf("received illegal message kind from node: %d", kind)
	}
}
