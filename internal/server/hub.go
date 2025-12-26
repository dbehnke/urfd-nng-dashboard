package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	Send chan []byte
}

type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
		case message := <-h.Broadcast:
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.Clients, client)
				}
			}
		}
	}
}

func (h *Hub) BroadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("JSON Marshal error: %v", err)
		return
	}
	h.Broadcast <- data
}

func UpgradeAndRegister(hub *Hub, w http.ResponseWriter, r *http.Request) (*websocket.Conn, *Client) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS Upgrade error: %v", err)
		return nil, nil
	}
	client := &Client{Hub: hub, Conn: conn, Send: make(chan []byte, 256)}
	client.Hub.Register <- client
	return conn, client
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	_, client := UpgradeAndRegister(hub, w, r)
	if client != nil {
		go client.WritePump()
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.Hub.Unregister <- c
		_ = c.Conn.Close()
	}()
	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
	_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
}
