package api

import (
	"fmt"
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

const CODE_PING = 0xC0
const CODE_PONG = 0xC1

const CODE_SEND_ID = 0xB1

const CODE_INIT_HANDSHAKE = 0xB0

// Message layout for recv handhake data:
//
// |-00-------------|-01-------------02------------|-03--------------04------------|-05--------------|
// | CODE_RECV_     |         NUM_WORKERS          |          NODE_NAME_LEN        |    NODE_NAME    |
// | HANDSHAKE_DATA |           <uint16>           |            <uint16>           |                 | (... Other data? ...)
// |----------------|------------------------------|-------------------------------|-----------------|
// |     1 Byte     |            2 Byte            |             2 Byte            | <NODE_NAME_LEN> |
//
//

const CODE_RECV_HANDSHAKE_DATA = 0xA0

func initHandshake(conn *websocket.Conn) (logic.NodeInfo, error) {
	if err := conn.WriteMessage(
		websocket.BinaryMessage,
		[]byte{CODE_INIT_HANDSHAKE},
	); err != nil {
		return logic.NodeInfo{}, err
	}

	endpointIP := conn.RemoteAddr()

	typ, message, err := conn.ReadMessage()
	if err != nil {
		return logic.NodeInfo{}, err
	}

	if typ != websocket.BinaryMessage {
		return logic.NodeInfo{}, fmt.Errorf("expected answer to `CODE_INIT_HANDSHAKE` to be binary, got %d", typ)
	}

	// output := make([]string, 0)
	// for _, b := range message {
	// 	output = append(output, fmt.Sprintf("0x%x", b))
	// }

	// fmt.Printf("recv binary: [%s]\n", strings.Join(output, ", "))

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
			"Node returned illegal name length: %d or message with len=%d was too short",
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

func HandleNodeConnection(c *gin.Context) {
	// TODO: add timeouts

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

	// Add node to manager.
	// TODO: use wrapper
	id := logic.NodeManager.ConnectNode(node)

	fmt.Printf("ID======%v\n", id)

	if err := conn.WriteMessage(websocket.BinaryMessage, append([]byte{CODE_SEND_ID}, id[0:]...)); err != nil {
		panic(fmt.Sprintf("[node] Handshake with `%s` failed: %s", node.EndpointIP, err.Error()))
	}

	pingHandler := func(appData string) error {
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte{CODE_PONG}); err != nil {
			return err
		}

		if !logic.NodeManager.RegisterPing(id) {
			panic("node does not exist")
		}

		return nil
	}

	conn.SetPingHandler(pingHandler)

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

				if len(message) == 0 {
					log.Trace("Received empty binary message")
					continue
				}

				if message[0] == CODE_PING {
					if err := pingHandler(string(message[1:])); err != nil {
						panic(err.Error())
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
	}()

	conn.SetPongHandler(func(appData string) error {
		log.Tracef("RECEIVED PONG!")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		message := websocket.FormatCloseMessage(code, "")
		// TODO: do not hardcode the timeout here
		if err := conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
			return err
		}
		return nil
	})

	for {
		time.Sleep(time.Second)
	}

	// for {
	// 	if err := conn.WriteMessage(websocket.TextMessage, []byte("Hello, WebSocket!")); err != nil {
	// 		panic(err.Error())
	// 	}
	// 	time.Sleep(time.Second)
	// }
}
