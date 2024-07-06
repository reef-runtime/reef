package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const uiManagerIdleSleep = time.Millisecond * 100
const uiDataCacheInvalidation = time.Second * 10

var UIManager UISubscriptionsManager

//
// UI Update message.
//

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

type DataCollectionMsg struct {
	Topic WebSocketTopic `json:"topic"`
	Data  any            `json:"data"`
}

// func (n UIUpdateNewData) Kind() UIUpdateKind { return UIUPdateKindNewData }

//
// Subscriptions.
//

type WebSocketTopic struct {
	Kind WebSocketTopicKind `json:"kind"`
	// Is populated when topic requires additional information, such as single job.
	Additional string `json:"additional"`
}

type WebSocketTopicKind string

const (
	WSTopicAllJobs   WebSocketTopicKind = "all_jobs"
	WSTopicSingleJob WebSocketTopicKind = "single_job"
	WSTopicNodes     WebSocketTopicKind = "nodes"
)

func (t WebSocketTopic) Validate() error {
	switch t.Kind {
	case WSTopicAllJobs, WSTopicNodes:
		if t.Additional != "" {
			return errors.New("the additional field must be empty for this topic kind")
		}
		return nil
	case WSTopicSingleJob:
		if t.Additional == "" {
			return errors.New("the additional field cannot be empty for single jobs")
		}
		return nil
	default:
		return fmt.Errorf("illegal topic kind: `%s`", t.Kind)
	}
}

type WebSocketSubscribeMessage struct {
	Topics []WebSocketTopic `json:"topics"`
}

//
// UI subscriptions manager.
//

type UIConn[MessageT any] struct {
	WS   *WSConn
	Chan chan MessageT
	// This will prevent a flooding of the frontend with new data.
	TopicsWithLastUpdate LockedMap[WebSocketTopic, time.Time]
}

type ToUIUpdateMsg struct {
	Topic WebSocketTopic  `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

type CachedUpdate struct {
	Time time.Time
	Data json.RawMessage
}

type UISubscriptionsManager struct {
	FromDatasources chan DataCollectionMsg

	Connections LockedMap[net.Addr, *UIConn[ToUIUpdateMsg]]

	// Is accessed when a new client connects and there is still up-to-date information available.
	UpdateCache LockedMap[WebSocketTopic, CachedUpdate]

	// Is used to tell the underlying data source that it should generate some data.
	// This will trigger an update.
	TriggerDataSourceChan chan WebSocketTopic
}

func NewUIManager() UISubscriptionsManager {
	return UISubscriptionsManager{
		FromDatasources:       make(chan DataCollectionMsg),
		Connections:           newLockedMap[net.Addr, *UIConn[ToUIUpdateMsg]](),
		UpdateCache:           newLockedMap[WebSocketTopic, CachedUpdate](),
		TriggerDataSourceChan: make(chan WebSocketTopic),
	}
}

func (m *UISubscriptionsManager) InitConn(ctx *gin.Context) {
	conn, err := Upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsConn := NewWSConn(conn)

	conn.SetPongHandler(func(appData string) error {
		log.Tracef("RECEIVED PONG!")
		return nil
	})

	conn.SetCloseHandler(func(code int, text string) error {
		// TODO
		return nil
	})

	uiConn := &UIConn[ToUIUpdateMsg]{
		WS:   wsConn,
		Chan: make(chan ToUIUpdateMsg),
		// No topics.
		TopicsWithLastUpdate: newLockedMap[WebSocketTopic, time.Time](),
	}

	m.connMainLoop(uiConn)
}

func (m *UISubscriptionsManager) connMainLoop(conn *UIConn[ToUIUpdateMsg]) {
	log.Debugf("[UI] Client `%s` is connecting...", conn.WS.RemoteAddr())

	m.addConn(conn)

	// Spawn concurrent sender goroutine.
	go func() {
		for update := range conn.Chan {
			marshaled, err := json.Marshal(update)
			if err != nil {
				log.Errorf("[UI] Could not marshal outer JSON: %s", err.Error())
				return
			}

			if err := conn.WS.WriteMessage(websocket.TextMessage, marshaled); err != nil {
				log.Warnf("[UI] client disconnected without closing message, cannot send status: %s", err.Error())
			}
		}
	}()

	if err := conn.WS.WriteMessage(websocket.PingMessage, []byte("ready")); err != nil {
		log.Errorf("[UI] could not send ACK: %s", err.Error())
		return
	}

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
				log.Debugf("[UI] Client sent illegal subscribe message: JSON: %s", err.Error())
				break
			}

			// Validate all topics.
			for _, topic := range subscribeMessage.Topics {
				if err := topic.Validate(); err != nil {
					log.Debugf("[UI] Client sent illegal subscribe message: %s", err.Error())
					break
				}
			}

			log.Debugf("[UI] Client subscribed to `%v`", subscribeMessage.Topics)

			// Subscribe to all topics.
			// Send initial data (cached from previous update)
			conn.TopicsWithLastUpdate.Clear()
			for _, topic := range subscribeMessage.Topics {
				conn.TopicsWithLastUpdate.Insert(topic, time.Time{})

				cached, found := m.UpdateCache.Get(topic)
				if !found || time.Since(cached.Time) > uiDataCacheInvalidation {
					// If there is no cached data available, or if it is too old, request this data to be generated.
					log.Tracef("[UI] No data available for topic `%s`", topic.Kind)
					m.TriggerDataSourceChan <- topic
					log.Tracef("[UI] Sent refresh request for topic `%s`", topic.Kind)
					continue
				}

				// Send initial data to client if it was 'fresh' enough.
				conn.Chan <- ToUIUpdateMsg{
					Topic: topic,
					Data:  cached.Data,
				}
			}
		case websocket.CloseMessage:
			break
		}
	}

	m.dropConn(conn.WS.RemoteAddr())
}

// Waits for incoming update messages and sends them to all listening clients.
func (m *UISubscriptionsManager) WaitAndNotify() {
	buf := newLockedMap[WebSocketTopic, DataCollectionMsg]()

	// Main loop.
	go func() {
		for {
			buf.Lock.RLock()
			for topic, data := range buf.Map {
				m.notifyOfEvent(topic, data)
			}
			buf.Lock.RUnlock()

			buf.Clear()

			time.Sleep(uiManagerIdleSleep)
		}
	}()

	for {
		message := <-m.FromDatasources
		buf.Insert(message.Topic, message)
	}
}

func (m *UISubscriptionsManager) sendUIUpdate(update ToUIUpdateMsg) {
	m.Connections.Lock.RLock()
	for addr, conn := range m.Connections.Map {
		// Skip this client if it is not subscribed on this topic.
		lastSent, subscribedToTopic := conn.TopicsWithLastUpdate.Get(update.Topic)
		if !subscribedToTopic {
			log.Tracef("[UI] Skipping client: not subscribed to topic `%s`", update.Topic.Kind)
			continue
		}

		// No change since last time sending and not too much time has passed:
		last, found := m.UpdateCache.Get(update.Topic)
		if found && time.Since(lastSent) < minUIUpdateDelay && slices.Equal(last.Data, update.Data) {
			log.Tracef("[UI] Skipping client: no change since last time sending")
			continue
		}

		conn.Chan <- update

		conn.TopicsWithLastUpdate.Insert(update.Topic, time.Now())

		log.Tracef("[UI] Notified client `%s` of event", addr)
	}
	m.Connections.Lock.RUnlock()
}

func (m *UISubscriptionsManager) notifyOfEvent(topic WebSocketTopic, event DataCollectionMsg) {
	// Marshal the event body as JSON.
	marshaled, err := json.Marshal(event.Data)
	if err != nil {
		log.Errorf("[UI] Could not marshal JSON: %s", err.Error())
		return
	}

	log.Tracef("[UI] Notifying all listening clients of event")
	m.sendUIUpdate(ToUIUpdateMsg{
		Topic: topic,
		Data:  marshaled,
	})
	log.Tracef("[UI] Notified all listening clients of event `%s`", topic.Kind)

	// Update time of last broadcast.
	m.UpdateCache.Insert(event.Topic, CachedUpdate{
		Time: time.Now(),
		Data: marshaled,
	})
}

func (m *UISubscriptionsManager) addConn(conn *UIConn[ToUIUpdateMsg]) {
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
