package websockets

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	//Clients are served from other origins, as the REST API allows with its CORS headers
	CheckOrigin: func(r *http.Request) bool { return true },
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type Websocket struct {
	hub *BroadcastHub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	//Channels to watch
	channels map[int64]struct{}

	//Logged in user who opened the socket
	user User
}

//Events clients can send on their socket
const (
	Typing = "typing"
)

type typingData struct {
	ChannelID int64 `json:"channel_id"`
	Typing    bool  `json:"typing"`
}

//Sent to a channel when someone starts or stops typing in it
type typingEvent struct {
	ChannelID int64 `json:"channel_id"`
	Typing    bool  `json:"typing"`
	User      User  `json:"user"`
}

func (w *Websocket) handleClientEvent(body []byte) {
	var e event
	err := json.Unmarshal(body, &e)
	if err != nil {
		log.Printf("Error decoding client event: %s", err.Error())
		return
	}

	switch e.Type {
	case Typing:
		var data typingData
		err = json.Unmarshal(e.Data, &data)
		if err != nil {
			log.Printf("Error decoding typing event: %s", err.Error())
			return
		}
		//Only to the channels this socket watches
		if !w.IsInterested([]int64{data.ChannelID}) {
			log.Printf("Typing event for unwatched channel %d", data.ChannelID)
			return
		}
		payload, err := json.Marshal(typingEvent{ChannelID: data.ChannelID, Typing: data.Typing, User: w.user})
		if err != nil {
			log.Printf("Error marshaling typing event: %s", err.Error())
			return
		}
		w.hub.Broadcast(UserTyping, payload, []int64{data.ChannelID})
	default:
		log.Printf("Unknown client event: %s", e.Type)
	}
}

func (w *Websocket) IsInterested(channelIDs []int64) bool {
	for _, channel := range(channelIDs) {
		if _, ok := w.channels[channel]; ok {
			return true
		}
	}
	return false
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (w *Websocket) readPump() {
	defer func() {
		w.hub.unregister <- w
		w.conn.Close()
	}()
	w.conn.SetReadLimit(maxMessageSize)
	w.conn.SetReadDeadline(time.Now().Add(pongWait))
	w.conn.SetPongHandler(func(string) error { w.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		log.Printf("Message is: %s", message)
		w.handleClientEvent(message)
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (w *Websocket) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		w.conn.Close()
	}()
	for {
		select {
		case message, ok := <-w.send:
			w.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				log.Printf("hub closed")
				w.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// One event per websocket message, so each one is a single JSON document
			if err := w.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("error on writing: %s", err.Error())
				return
			}
		case <-ticker.C:
			w.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("error on ticker writing: %s", err.Error())
				return
			}
		}
	}
}
