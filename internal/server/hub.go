package server

import (
	"encoding/json"
	"hash/fnv"
	"log"
	"net/http"
	"sync"

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

	// Voice session management
	activeTransmitters map[string]string // module -> callsign
	mu                 sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:          make(chan []byte),
		Register:           make(chan *Client),
		Unregister:         make(chan *Client),
		Clients:            make(map[*Client]bool),
		activeTransmitters: make(map[string]string),
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

// Voice session management methods

// hashSessionID converts a session ID string to a numeric ID
func hashSessionID(sessionID string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(sessionID))
	return h.Sum32()
}

// HasActiveTransmitter checks if a module has an active transmitter
func (h *Hub) HasActiveTransmitter(module string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.activeTransmitters[module]
	return exists
}

// GetActiveTransmitter returns the callsign of the active transmitter for a module
func (h *Hub) GetActiveTransmitter(module string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	callsign, exists := h.activeTransmitters[module]
	return callsign, exists
}

// SetActiveTransmitter sets the active transmitter for a module
func (h *Hub) SetActiveTransmitter(module, callsign, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activeTransmitters[module] = callsign
	log.Printf("Active transmitter on module %s: %s (session %s)", module, callsign, sessionID)

	// Note: Hearing events are broadcast by the voice session handler in session.go
	// We don't broadcast here to avoid duplicate entries
}

// ClearActiveTransmitter removes the active transmitter for a module
func (h *Hub) ClearActiveTransmitter(module, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.activeTransmitters, module)
	log.Printf("Cleared active transmitter on module %s (session %s)", module, sessionID)

	// Note: Closing events are broadcast by the voice session handler in session.go
	// We don't broadcast here to avoid duplicate entries
}
