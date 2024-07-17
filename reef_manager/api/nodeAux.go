package api

import (
	"net/http"

	"capnproto.org/go/capnp/v3"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/logic"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

//
// Single normal REST endpoint.
//

func GetNodes(ctx *gin.Context) {
	nodes := logic.JobManager.ListNodes()
	ctx.JSON(http.StatusOK, nodes)
}

//
// Dropping Nodes.
//

func dropNode(conn *logic.WSConn, closeCode int, nodeID logic.NodeId) {
	nodeIDString := logic.IdToString(nodeID)
	log.Debugf("[node] Dropping node `%s`...", nodeIDString)

	message := websocket.FormatCloseMessage(closeCode, "")
	if err := conn.WriteControl(websocket.CloseMessage, message); err != nil {
		log.Warnf("[node] did not respond to close message in time")
	}

	if err := conn.Close(); err != nil {
		log.Warnf("[node] could not close TCP connection")
	}

	if !logic.JobManager.DropNode(nodeID) {
		log.Errorf("[node] dropping unknown node `%s`, this is a bug", nodeIDString)
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

func messagePingHandler(conn *logic.WSConn, nodeID logic.NodeId) func(string) error {
	return func(_ string) error {
		msg, err := pingMessage()
		if err != nil {
			return err
		}

		log.Tracef("[node] received ping from `%s`", logic.IdToString(nodeID))

		err = conn.WriteMessage(websocket.PongMessage, msg)

		if err != nil {
			log.Tracef("[node] sending pong failed: %s", err.Error())
			return err
		}

		setLastPing(nodeID)

		return nil
	}
}

func setLastPing(nodeID logic.NodeId) {
	if !logic.JobManager.RegisterPing(nodeID) {
		log.Errorf(
			"[node] could not register ping, node `%s` does not exist, this is a bug",
			logic.IdToString(nodeID),
		)
	}
}
