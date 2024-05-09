package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type NodeInfo struct {
	EndpointIP string `json:"endpointIP"`
	Name       string `json:"string"`
	// TODO: maybe worker descriptions
	// TODO: maybe the current state of the node?
}

const CODE_INIT_HANDSHAKE = 0xB0

// Message layout for recv handhake data:
//
// |----------------|---------------|-----------------|
// | CODE_RECV_     | NODE_NAME_LEN |    NODE_NAME    |
// | HANDSHAKE_DATA |   <uint16>    |                 | (... Other data? ...)
// |----------------|---------------|-----------------|
// |     1 Byte     |    2 Byte     | <NODE_NAME_LEN> |
//
//

const CODE_RECV_HANDSHAKE_DATA = 0xA0

func initHandshake(conn *websocket.Conn) (NodeInfo, error) {
	if err := conn.WriteMessage(
		websocket.BinaryMessage,
		[]byte{CODE_INIT_HANDSHAKE},
	); err != nil {
		return NodeInfo{}, err
	}

	endpointIP := conn.RemoteAddr()

	typ, message, err := conn.ReadMessage()
	if err != nil {
		return NodeInfo{}, err
	}

	if typ != websocket.BinaryMessage {
		return NodeInfo{}, fmt.Errorf("expected answer to `CODE_INIT_HANDSHAKE` to be binary, got %d", typ)
	}

	if len(message) == 0 {
		return NodeInfo{}, errors.New("expected answer to be not empty")
	}

	if message[0] != CODE_RECV_HANDSHAKE_DATA {
		return NodeInfo{}, fmt.Errorf("expected answer byte[0] to be `CODE_RECV_HANDSHAKE_DATA`, got 0x%x", message[0])
	}

	var nameLen uint16
	nameLen = uint16(message[1]) << 8
	nameLen &= uint16(message[2])

	fmt.Printf("NAME LEN: %d\n", nameLen)

	nodeName := string(message[nameLen:])

	return NodeInfo{
		EndpointIP: endpointIP.String(),
		Name:       nodeName,
	}, nil
}

func HandleNodeConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	node, err := initHandshake(conn)
	if err != nil {
		log.Errorf("[node] Handshake with `%s` failed: %s", conn.RemoteAddr(), err.Error())
		return
	}

	log.Infof("[node] Handshake success: connected to `%s` (%s)", node.EndpointIP, node.Name)

	go func() {
		for {
			typ, message, err := conn.ReadMessage()
			if err != nil {
				panic(err.Error())
			}

			switch typ {
			case websocket.TextMessage:
				fmt.Printf("text: %s\n", string(message))
			case websocket.BinaryMessage:
				fmt.Printf("bin: %x\n", message)
			case websocket.PingMessage:
				fmt.Printf("ping: %x\n", message)
			case websocket.PongMessage:
				fmt.Printf("pong: %x\n", message)
			case websocket.CloseMessage:
				fmt.Println("closing...")
				return
			}
		}
	}()

	for {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket!")); err != nil {
			panic(err.Error())
		}
		time.Sleep(time.Second)
	}
}
