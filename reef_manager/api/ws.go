package api

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

////
//// UI Update message.
////
// type UIUpdateMessage interface {
// 	Kind() UIUpdateKind
// }
//
// type UIUpdateKind uint8
//
// const (
// 	// TODO: is this message kind really required?
// 	UIUpdateKindClose UIUpdateKind = iota
// 	UIUPdateKindNewData
// )

type UIUpdateNewData struct {
	Topic WebSocketTopic
	Data  any
}

//
// Subscriptions.
//

type WebSocketTopic struct {
	Kind WebSocketTopicKind `json:"kind"`
	// Is not null when topic requires additional information, such as single job.
	Additional *string `json:"additional"`
}

type WebSocketTopicKind string

const (
	WSTopicAllJobs WebSocketTopicKind = "jobs"
	WSTopicNodes
	WSTopicSingleJob
)

type WebSocketSubscribeMessage struct {
	Topics []WebSocketTopic `json:"topics"`
}

//
// UI subscriptions manager.
//

type UIConn[MessageT any] struct {
	WS   *logic.WSConn
	Chan chan MessageT
}

type UISubscriptionsManager struct {
	Connections logic.LockedMap[net.Addr, UIConn[UIUpdateNewData]]
}

func (m *UISubscriptionsManager) InitConn(ctx *gin.Context) {
	conn, err := logic.Upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsConn := logic.NewWSConn(conn)

	conn.SetPongHandler(func(appData string) error {
		log.Tracef("RECEIVED PONG!")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		// TODO
		return nil
	})

	uiConn := UIConn[UIUpdateNewData]{
		WS:   wsConn,
		Chan: make(chan UIUpdateNewData),
	}

	m.connMainLoop(uiConn)
}

func (m *UISubscriptionsManager) connMainLoop(conn UIConn[UIUpdateNewData]) {
	m.addConn(conn)

	// Spawn concurrent sender goroutine.
	go func() {
		for update := range conn.Chan {
			buf, err := json.Marshal(update)
			if err != nil {
				log.Errorf("[UI] Could not marshal new data message: %s", err.Error())
				return
			}

			if err := conn.WS.WriteMessage(websocket.TextMessage, buf); err != nil {
				log.Warnf("[UI] client disconnected without closing message, cannot send status: %s", err.Error())
			}
		}
	}()

	// Listen to incoming messages.
	for {
		msgType, message, err := conn.WS.ReadMessageWithTimeout(time.Time{})
		if err != nil {
			log.Debugf("[UI] error while reading message: %s", err.Error())
			break
		}

		switch msgType {
		case websocket.TextMessage:
			fmt.Printf("text: %s\n", string(message))

			var subscribeMessage WebSocketSubscribeMessage
			if err := json.Unmarshal(message, &subscribeMessage); err != nil {
				log.Debugf("[UI] Client sent illegal subscribe message: %s", err.Error())
				return
			}

		case websocket.CloseMessage:
			m.dropConn(conn.WS.RemoteAddr())
			return
		}
	}
}

func (m *UISubscriptionsManager) NotifyOfEvent(event UIUpdateNewData) {
	log.Tracef("[UI] Notifying all listening clients of event")
	m.Connections.Lock.RLock()
	for addr, conn := range m.Connections.Map {
		conn.Chan <- event
		log.Tracef("Notified client `%s` of event", addr)
	}
	m.Connections.Lock.RUnlock()
	log.Tracef("[UI] Notified all listening clients of event")
}

func (m *UISubscriptionsManager) addConn(conn UIConn[UIUpdateNewData]) {
	addr := conn.WS.RemoteAddr()
	m.Connections.Insert(addr, conn)
	log.Debugf("[UI] client `%s` connected", addr)
}

func (m *UISubscriptionsManager) dropConn(addr net.Addr) {
	conn, found := m.Connections.Delete(addr)
	if !found {
		log.Tracef("[UI] could not drop client `%s`: it does not exist", addr)
		return
	}
	close(conn.Chan)
	log.Debugf("[UI] client `%s` was dropped", addr)
}
