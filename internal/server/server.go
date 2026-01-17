package server

import (
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dbehnke/urfd-nng-dashboard/internal/voice"
	"github.com/gorilla/websocket"
)

type Server struct {
	Hub           *Hub
	Assets        fs.FS
	OnConnect     func(*Client)
	VoiceConfig   *voice.SessionConfig
	VoiceSessions map[string]*voice.Session
}

func NewServer(hub *Hub, assets fs.FS) *Server {
	return &Server{
		Hub:           hub,
		Assets:        assets,
		VoiceSessions: make(map[string]*voice.Session),
	}
}

func (s *Server) Start(addr string) error {
	// Handle WS
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		_, client := UpgradeAndRegister(s.Hub, w, r)
		if client != nil {
			if s.OnConnect != nil {
				s.OnConnect(client)
			}
			go client.WritePump()
		}
	})

	// Handle Voice WS
	if s.VoiceConfig != nil && s.VoiceConfig.ReflectorAddr != "" {
		http.HandleFunc("/ws/voice", func(w http.ResponseWriter, r *http.Request) {
			s.handleVoiceWebSocket(w, r)
		})
		log.Printf("Voice WebSocket endpoint enabled at /ws/voice")
	}

	// Handle Static Files (with SPA routing support)
	fileServer := http.FileServer(http.FS(s.Assets))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If requesting a file that doesn't exist, serve index.html for SPA
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		f, err := s.Assets.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			// Serve index.html for unknown routes (SPA)
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})

	log.Printf("HTTP Server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for now
	},
}

// handleVoiceWebSocket handles voice WebSocket connections
func (s *Server) handleVoiceWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Voice WS Upgrade error: %v", err)
		return
	}

	// Generate session ID
	sessionID := generateSessionID()

	// Create voice session
	session, err := voice.NewSession(sessionID, conn, s.VoiceConfig)
	if err != nil {
		log.Printf("Failed to create voice session: %v", err)
		conn.Close()
		return
	}

	// Start the session
	if err := session.Start(); err != nil {
		log.Printf("Failed to start voice session: %v", err)
		conn.Close()
		return
	}

	// Store session
	s.VoiceSessions[sessionID] = session

	// Handle incoming messages
	defer func() {
		session.Stop()
		delete(s.VoiceSessions, sessionID)
		conn.Close()

		// Clear active transmitter if this session was transmitting
		if session.IsTransmitting() {
			s.Hub.ClearActiveTransmitter(session.GetModule())
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Voice WS error: %v", err)
			}
			break
		}

		// Handle message
		if err := session.HandleMessage(message); err != nil {
			log.Printf("Voice session %s message error: %v", sessionID, err)
		}

		// Update active transmitter tracking
		if session.IsTransmitting() {
			s.Hub.SetActiveTransmitter(session.GetModule(), session.GetCallsign())
		} else if session.GetState() == voice.StateListening {
			// If we just stopped transmitting, clear the active transmitter
			activeCallsign, exists := s.Hub.GetActiveTransmitter(session.GetModule())
			if exists && activeCallsign == session.GetCallsign() {
				s.Hub.ClearActiveTransmitter(session.GetModule())
			}
		}
	}
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of length n
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
