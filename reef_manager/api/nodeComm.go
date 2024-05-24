package api

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const CODE_PING = 0xF0
const CODE_PONG = 0xF1

const CODE_SEND_ID = 0xB1

const CODE_INIT_HANDSHAKE = 0xB0
const CODE_RECV_HANDSHAKE_DATA = 0xA0

// Message layout for recv handhake data:
// [NODE ---> MANAGER]
//
// |-00-------------|-01-------------02------------|-03--------------04------------|-05--------------|
// | CODE_RECV_     |         NUM_WORKERS          |          NODE_NAME_LEN        |    NODE_NAME    |
// | HANDSHAKE_DATA |           <uint16>           |            <uint16>           |                 | (... Other data? ...)
// |----------------|------------------------------|-------------------------------|-----------------|
// |     1 Byte     |            2 Byte            |             2 Byte            | <NODE_NAME_LEN> |
//
//

func initHandshake(conn *logic.WSConn) (logic.NodeInfo, error) {
	conn.Lock.Lock()

	endpointIP := conn.Conn.RemoteAddr()
	err := conn.Conn.WriteMessage(
		websocket.BinaryMessage,
		[]byte{CODE_INIT_HANDSHAKE},
	)

	conn.Lock.Unlock()

	if err != nil {
		return logic.NodeInfo{}, err
	}

	conn.Lock.Lock()
	typ, message, err := conn.Conn.ReadMessage()
	conn.Lock.Unlock()

	if err != nil {
		return logic.NodeInfo{}, err
	}

	if typ != websocket.BinaryMessage {
		return logic.NodeInfo{}, fmt.Errorf("expected answer to `CODE_INIT_HANDSHAKE` to be binary, got %d", typ)
	}

	if len(message) == 0 {
		return logic.NodeInfo{}, errors.New("expected answer to be not empty")
	}

	if message[0] != CODE_RECV_HANDSHAKE_DATA {
		return logic.NodeInfo{}, fmt.Errorf("expected answer byte[0] to be `CODE_RECV_HANDSHAKE_DATA`, got 0x%x", message[0])
	}

	const numWorkersOffsetBytes = 1

	var numWorkers uint16
	numWorkers = uint16(message[numWorkersOffsetBytes]) << 8
	numWorkers |= uint16(message[numWorkersOffsetBytes+1])

	const nameLenOffsetBytes = 3
	const nameContentsOffsetBytes = nameLenOffsetBytes + 2

	var nameLen uint16
	nameLen = uint16(message[nameLenOffsetBytes]) << 8
	nameLen |= uint16(message[nameLenOffsetBytes+1])

	log.Tracef("[node] Handshake: received name length %d", nameLen)

	if nameContentsOffsetBytes > len(message) || int(nameContentsOffsetBytes+nameLen) > len(message) {
		return logic.NodeInfo{}, fmt.Errorf(
			"node returned illegal name length: %d or message with len=%d was too short",
			nameLen,
			len(message),
		)
	}

	nodeName := string(message[nameContentsOffsetBytes : nameLen+nameContentsOffsetBytes])

	return logic.NodeInfo{
		EndpointIP: endpointIP.String(),
		Name:       nodeName,
		NumWorkers: numWorkers,
	}, nil
}

//
// Node management.
//

func dropNode(conn *logic.WSConn, closeCode int, nodeID [32]byte) {
	nodeIDString := logic.IdToString(nodeID)
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

func nodePingHandler(conn *logic.WSConn, nodeID [32]byte) func(string) error {
	return func(appData string) error {
		conn.Lock.Lock()
		err := conn.Conn.WriteMessage(websocket.BinaryMessage, []byte{CODE_PONG})
		conn.Lock.Unlock()

		if err != nil {
			log.Tracef("[node] sending pong failed: %s", err.Error())
			return err
		}

		if !logic.NodeManager.RegisterPing(nodeID) {
			log.Errorf(
				"[node] could not register ping, node `%s` does not exist, this is a bug",
				logic.IdToString(nodeID),
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

	node, err := initHandshake(wsConn)
	if err != nil {
		log.Errorf("[node] handshake with `%s` failed: %s", conn.RemoteAddr(), err.Error())
		return
	}

	// TODO: use wrapper

	// Add node to manager.
	nodeID := logic.NodeManager.ConnectNode(node, conn)

	if err := conn.WriteMessage(websocket.BinaryMessage, append([]byte{CODE_SEND_ID}, nodeID[0:]...)); err != nil {
		log.Warnf(
			"[node] handshake with `%s` failed: could not deliver ID to node: %s",
			node.EndpointIP,
			err.Error(),
		)

		return
	}

	pingHandler := nodePingHandler(wsConn, nodeID)
	conn.SetPingHandler(pingHandler)

	conn.SetPongHandler(func(appData string) error {
		log.Tracef("RECEIVED PONG!")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		dropNode(wsConn, code, nodeID)
		return nil
	})

	// Blocking receive loop.
	for {
		msgType, message, err := conn.ReadMessage()
		if err != nil {
			log.Debugf("[node] error while reading message: %s", err.Error())
			dropNode(wsConn, websocket.CloseAbnormalClosure, nodeID)
			break
		}

		switch msgType {
		case websocket.TextMessage:
			fmt.Printf("text: %s\n", string(message))
		case websocket.BinaryMessage:
			log.Tracef("[node] received binary message: %s", logic.FormatBinarySliceAsHex(message))

			if len(message) == 0 {
				log.Trace("[node] received empty binary message")
				continue
			}

			switch message[0] {
			case CODE_PING:
				if err := pingHandler(string(message[1:])); err != nil {
					dropNode(wsConn, websocket.CloseAbnormalClosure, nodeID)
					return
				}
			case logic.CODE_STARTED_JOB:
				if err := logic.NodeManager.JobStartedJobCallback(nodeID, message); err != nil {
					log.Errorf("job started error: %s", err.Error())
					return
				}
			case logic.CODE_JOB_LOG:
				if err := logic.NodeManager.NodeLogCallBack(nodeID, message); err != nil {
					log.Errorf("job log error: %s", err.Error())
					return
				}
			}
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
