package websockets

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"chat-gobelin-api/internal/users"
)

type BroadcastHub struct {
	// Registered clients.
	clients map[*Websocket]struct{}

	// Inbound messages from the clients.
	broadcast chan message

	// Register requests from the clients.
	register chan *Websocket

	// Unregister requests from clients.
	unregister chan *Websocket
}

// serveWs handles websocket requests from the peer.
func (hub *BroadcastHub) ServeWS(writer http.ResponseWriter, request *http.Request) {
	//One socket can watch several channels: ?channel=1&channel=2
	channelIDStrings := request.URL.Query()["channel"]
	if len(channelIDStrings) == 0 {
		log.Printf("No channel found: %s", request.URL.RawQuery)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	channels := make(map[int64]struct{})
	for _, channelIDString := range channelIDStrings {
		channelID, err := strconv.ParseInt(channelIDString, 10, 64)
		if err != nil {
			log.Printf("Error converting channel ID: %s", err.Error())
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		channels[channelID] = struct{}{}
	}

	//Set by the authentication middleware, events sent by the client are signed with it
	user, ok := request.Context().Value("user").(*users.User)
	if !ok || user == nil {
		log.Printf("No user found")
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Websocket{hub: hub, conn: conn, send: make(chan []byte, 256), channels: channels, user: User{ID: user.ID, Username: user.Username}}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

type message struct{
	Body []byte
	Channel []int64
}

func NewBroadcastHub() *BroadcastHub {
	return &BroadcastHub{
		broadcast:  make(chan message),
		register:   make(chan *Websocket),
		unregister: make(chan *Websocket),
		clients:    make(map[*Websocket]struct{}),
	}
}

func (h *BroadcastHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = struct{}{}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			log.Printf("message: %s", message.Body)
			for client := range h.clients {
				if client.IsInterested(message.Channel) {
					log.Printf("interested client: %v", client)
					select {
					case client.send <- message.Body:
						log.Printf("message sent")
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
		}
	}
}

//Event types, so clients can tell the payloads apart
const (
	MessageCreated = "message.created"
	MessageEdited  = "message.edited"
	ChannelJoined  = "channel.joined"
	ChannelEdited  = "channel.edited"
	UserTyping     = "user.typing"
)

type event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

//Public part of a user, for the events that say who did something
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (h *BroadcastHub) Broadcast(eventType string, data json.RawMessage, channel []int64) {
	body, err := json.Marshal(event{Type: eventType, Data: data})
	if err != nil {
		log.Printf("Error marshaling event: %s", err.Error())
		return
	}
	m := message{
		Body: body,
		Channel: channel,
	}
	h.broadcast <- m
}
