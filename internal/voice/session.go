package voice

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// SessionState represents the current state of a voice session
type SessionState string

const (
	StateIdle         SessionState = "idle"
	StateListening    SessionState = "listening"
	StateTransmitting SessionState = "transmitting"
	StateRxBusy       SessionState = "rx_busy"
)

// Broadcaster defines the interface for broadcasting events
type Broadcaster interface {
	BroadcastJSON(v interface{})
}

// Hearing represents a voice activity event (matches store.Hearing)
type Hearing struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	My       string `json:"my" gorm:"index"`
	Ur       string `json:"ur"`
	Rpt1     string `json:"rpt1"`
	Rpt2     string `json:"rpt2"`
	Module   string `json:"module" gorm:"index"`
	Protocol string `json:"protocol"`

	Duration  float64 `json:"duration"`
	AudioFile string  `json:"audio_file,omitempty"`
}

// TableName sets the table name for GORM
func (Hearing) TableName() string {
	return "hearings"
}

// Session represents a single client's voice session
type Session struct {
	ID            string
	Callsign      string
	Module        string
	State         SessionState
	Authenticated bool
	Conn          *websocket.Conn
	SharedClient  *SharedVoiceClient // Changed from VoiceClient to SharedVoiceClient
	ClientPool    *VoiceClientPool   // Reference to the pool
	Config        *SessionConfig
	DB            *gorm.DB
	Hub           Broadcaster

	mu             sync.RWMutex
	lastActivity   time.Time
	txStartTime    time.Time
	activeTransmit bool
	hearingID      uint // Database ID for current transmission
}

// SessionConfig holds configuration for voice sessions
type SessionConfig struct {
	RequirePassword  bool
	TransmitPassword string
	MaxTxDuration    time.Duration
	OpusBitrate      int
	ReflectorAddr    string
}

// WSMessage represents a WebSocket message for voice control
type WSMessage struct {
	Type          string `json:"type"`
	Module        string `json:"module,omitempty"`
	Callsign      string `json:"callsign,omitempty"`
	Password      string `json:"password,omitempty"`
	Opus          string `json:"opus,omitempty"` // base64 encoded
	State         string `json:"state,omitempty"`
	From          string `json:"from,omitempty"`
	Reason        string `json:"reason,omitempty"`
	ActiveTalker  string `json:"active_talker,omitempty"`
	MaxTxDuration int    `json:"max_tx_duration,omitempty"` // seconds
}

// NewSession creates a new voice session using the shared client pool
func NewSession(id string, conn *websocket.Conn, config *SessionConfig, db *gorm.DB, hub Broadcaster, pool *VoiceClientPool) (*Session, error) {
	session := &Session{
		ID:           id,
		State:        StateIdle,
		Conn:         conn,
		ClientPool:   pool,
		Config:       config,
		DB:           db,
		Hub:          hub,
		lastActivity: time.Now(),
	}

	return session, nil
}

// Start begins the voice session
func (s *Session) Start() error {
	// Session doesn't connect immediately - waits for voice_start message
	// to know which module to join
	log.Printf("Session %s: Started (waiting for voice_start)", s.ID)
	return nil
}

// Stop ends the voice session and cleans up resources
func (s *Session) Stop() error {
	s.mu.Lock()
	module := s.Module
	callsign := s.Callsign
	state := s.State
	sharedClient := s.SharedClient
	s.mu.Unlock()

	// If transmitting, send PTT stop
	if state == StateTransmitting && sharedClient != nil {
		sharedClient.SendPTTStop(module, callsign)
	}

	// Unregister from shared client
	if sharedClient != nil {
		sharedClient.UnregisterSession(s.ID)
	}

	// Release the shared client (decrements ref count, closes if zero)
	if module != "" && s.ClientPool != nil {
		s.ClientPool.ReleaseClient(module, s.ID)
	}

	s.mu.Lock()
	s.State = StateIdle
	s.SharedClient = nil
	s.mu.Unlock()

	log.Printf("Session %s: Stopped", s.ID)
	return nil
}

// HandleMessage processes incoming WebSocket messages from the browser
func (s *Session) HandleMessage(data []byte) error {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()

	switch msg.Type {
	case "voice_start":
		return s.handleVoiceStart(msg)
	case "voice_stop":
		return s.handleVoiceStop()
	case "ptt_press":
		return s.handlePTTPress(msg)
	case "ptt_release":
		return s.handlePTTRelease()
	case "audio_data":
		return s.handleAudioData(msg)
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleVoiceStart starts listening to a module
func (s *Session) handleVoiceStart(msg WSMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.Module == "" || msg.Callsign == "" {
		return s.sendError("Missing module or callsign")
	}

	// Get or create shared client for this module
	sharedClient, err := s.ClientPool.GetClient(msg.Module)
	if err != nil {
		return fmt.Errorf("failed to get voice client: %w", err)
	}

	s.Module = msg.Module
	s.Callsign = msg.Callsign
	s.SharedClient = sharedClient
	s.State = StateListening

	// Register this session with the shared client
	sharedClient.RegisterSession(s)

	log.Printf("Session %s: %s started listening to module %s", s.ID, s.Callsign, s.Module)

	// Send config along with initial state
	return s.sendMessage(WSMessage{
		Type:          "voice_config",
		State:         string(StateListening),
		MaxTxDuration: int(s.Config.MaxTxDuration.Seconds()),
	})
}

// handleVoiceStop stops the voice session
func (s *Session) handleVoiceStop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State == StateTransmitting && s.SharedClient != nil {
		s.SharedClient.SendPTTStop(s.Module, s.Callsign)
		s.activeTransmit = false
	}

	s.State = StateIdle
	log.Printf("Session %s: %s stopped voice session", s.ID, s.Callsign)

	return s.sendState(StateIdle)
}

// handlePTTPress handles PTT button press
func (s *Session) handlePTTPress(msg WSMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check authentication if password is required
	if s.Config.RequirePassword && !s.Authenticated {
		if msg.Password == "" {
			return s.sendMessage(WSMessage{
				Type: "auth_required",
			})
		}

		if msg.Password != s.Config.TransmitPassword {
			return s.sendMessage(WSMessage{
				Type:   "auth_failed",
				Reason: "invalid_password",
			})
		}

		s.Authenticated = true
	}

	// Check if we're in a state that allows transmission
	if s.State == StateIdle {
		return s.sendError("Not connected to a module")
	}

	if s.State == StateRxBusy {
		return s.sendMessage(WSMessage{
			Type:   "ptt_denied",
			Reason: "rx_busy",
		})
	}

	if s.State == StateTransmitting {
		// Already transmitting, ignore
		return nil
	}

	// NEW: Request PTT from SharedClient (half-duplex enforcement)
	if s.SharedClient == nil {
		return fmt.Errorf("not connected to voice client")
	}

	if err := s.SharedClient.RequestPTT(s.ID, s.Callsign); err != nil {
		// PTT denied - another user is transmitting
		log.Printf("Session %s: PTT denied for %s: %v", s.ID, s.Callsign, err)
		return s.sendMessage(WSMessage{
			Type:   "ptt_denied",
			Reason: err.Error(),
		})
	}

	// PTT granted, send PTT start to reflector
	if err := s.SharedClient.SendPTTStart(s.Module, s.Callsign); err != nil {
		// Failed to send to urfd, but still grant PTT for peer-to-peer
		log.Printf("Session %s: Warning: Failed to send PTT start to urfd: %v", s.ID, err)
		// Don't return error - peer audio will still work
	}

	s.State = StateTransmitting
	s.activeTransmit = true
	s.txStartTime = time.Now().UTC()

	// Create database record for this transmission
	if s.DB != nil {
		hearing := Hearing{
			My:        s.Callsign,
			Ur:        "CQCQCQ",
			Rpt1:      s.Module,
			Rpt2:      "WEB " + s.Module,
			Module:    s.Module,
			Protocol:  "VOICE",
			CreatedAt: s.txStartTime,
		}
		if err := s.DB.Create(&hearing).Error; err != nil {
			log.Printf("Session %s: Failed to create hearing record: %v", s.ID, err)
		} else {
			s.hearingID = hearing.ID
			log.Printf("Session %s: Created hearing record ID %d", s.ID, hearing.ID)

			// Broadcast hearing event to all connected WebSocket clients
			if s.Hub != nil {
				s.Hub.BroadcastJSON(map[string]interface{}{
					"type":       "hearing",
					"status":     "active",
					"id":         hearing.ID,
					"my":         s.Callsign,
					"ur":         "CQCQCQ",
					"rpt1":       s.Module,
					"rpt2":       "WEB " + s.Module,
					"module":     s.Module,
					"protocol":   "VOICE",
					"created_at": s.txStartTime,
				})
			}
		}
	}

	log.Printf("Session %s: %s started transmitting on module %s", s.ID, s.Callsign, s.Module)

	return s.sendState(StateTransmitting)
}

// handlePTTRelease handles PTT button release
func (s *Session) handlePTTRelease() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != StateTransmitting {
		// Not transmitting, ignore
		return nil
	}

	// Send PTT stop to reflector
	if s.SharedClient == nil {
		return fmt.Errorf("not connected to voice client")
	}
	if err := s.SharedClient.SendPTTStop(s.Module, s.Callsign); err != nil {
		// Log warning but don't fail - peer audio already stopped
		log.Printf("Session %s: Warning: Failed to send PTT stop to urfd: %v", s.ID, err)
	}

	// NEW: Release PTT in SharedClient
	s.SharedClient.ReleasePTT(s.ID, s.Callsign)

	duration := time.Since(s.txStartTime)
	durationSecs := duration.Seconds()
	s.State = StateListening
	s.activeTransmit = false

	// Update database record with duration
	if s.DB != nil && s.hearingID > 0 {
		if err := s.DB.Model(&Hearing{}).Where("id = ?", s.hearingID).Update("duration", durationSecs).Error; err != nil {
			log.Printf("Session %s: Failed to update hearing duration: %v", s.ID, err)
		} else {
			log.Printf("Session %s: Updated hearing record ID %d with duration %.2fs", s.ID, s.hearingID, durationSecs)

			// Broadcast closing event to all connected WebSocket clients
			if s.Hub != nil {
				s.Hub.BroadcastJSON(map[string]interface{}{
					"type":       "hearing",
					"status":     "ended",
					"id":         s.hearingID,
					"my":         s.Callsign,
					"ur":         "CQCQCQ",
					"rpt1":       s.Module,
					"rpt2":       "WEB " + s.Module,
					"module":     s.Module,
					"protocol":   "VOICE",
					"created_at": s.txStartTime,
					"duration":   durationSecs,
				})
			}
		}
		s.hearingID = 0
	}

	log.Printf("Session %s: %s stopped transmitting on module %s (duration: %v)",
		s.ID, s.Callsign, s.Module, duration)

	return s.sendState(StateListening)
}

// handleAudioData handles audio data from the browser
func (s *Session) handleAudioData(msg WSMessage) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State != StateTransmitting {
		return errors.New("not in transmitting state")
	}

	// Check max transmission duration
	if time.Since(s.txStartTime) > s.Config.MaxTxDuration {
		log.Printf("Session %s: Max TX duration exceeded, forcing release", s.ID)
		// Force release PTT
		go s.handlePTTRelease()
		return errors.New("max transmission duration exceeded")
	}

	// Decode base64 Opus data
	opusData, err := base64.StdEncoding.DecodeString(msg.Opus)
	if err != nil {
		return fmt.Errorf("failed to decode opus data: %w", err)
	}

	// NEW: Broadcast peer audio to other sessions on same module (real-time)
	if s.SharedClient != nil {
		s.SharedClient.BroadcastPeerAudio(opusData, s.ID, s.Callsign, s.Module)
	}

	// Send audio data to reflector (for recording)
	if s.SharedClient == nil {
		return fmt.Errorf("not connected to voice client")
	}
	if err := s.SharedClient.SendAudioData(s.Module, s.Callsign, opusData); err != nil {
		// Log warning but don't fail - peer audio already sent
		log.Printf("Session %s: Warning: Failed to send audio to urfd: %v", s.ID, err)
	}

	return nil
}

// handleAudioFromReflector receives audio from the reflector and sends to browser
func (s *Session) handleAudioFromReflector(msg VoiceMessage) {
	s.mu.RLock()

	// Only forward if we're listening to this module
	if s.State == StateIdle || s.Module != msg.Module {
		s.mu.RUnlock()
		return
	}

	// If we receive audio while in listening state, switch to rx_busy
	if s.State == StateListening {
		s.mu.RUnlock()
		s.mu.Lock()
		s.State = StateRxBusy
		s.sendState(StateRxBusy)
		s.mu.Unlock()
		s.mu.RLock()
	}

	s.mu.RUnlock()

	// Encode Opus data as base64 for WebSocket
	opusB64 := base64.StdEncoding.EncodeToString(msg.Opus)

	wsMsg := WSMessage{
		Type: "audio_data",
		Opus: opusB64,
		From: msg.Callsign,
	}

	s.sendMessage(wsMsg)
}

// SendAudioFromPeer sends peer audio (from another browser) to this session
// This is for real-time audio multiplexing between web clients
func (s *Session) SendAudioFromPeer(opusData []byte, fromCallsign string) error {
	s.mu.RLock()

	// Only forward if we're listening (not idle or transmitting)
	if s.State == StateIdle {
		s.mu.RUnlock()
		return nil // Silently ignore if session is idle
	}

	// Don't send if we're transmitting (we're the talker)
	if s.State == StateTransmitting {
		s.mu.RUnlock()
		return nil // Silently ignore if session is transmitting
	}

	// Check if connection exists (for testing scenarios)
	if s.Conn == nil {
		s.mu.RUnlock()
		return nil // Silently ignore if no connection (e.g., in tests)
	}

	s.mu.RUnlock()

	// Encode Opus data as base64 for WebSocket
	opusB64 := base64.StdEncoding.EncodeToString(opusData)

	wsMsg := WSMessage{
		Type: "peer_audio", // Different type to distinguish from reflector audio
		Opus: opusB64,
		From: fromCallsign,
	}

	return s.sendMessage(wsMsg)
}

// handleStateChange handles state changes from the reflector
func (s *Session) handleStateChange(msg VoiceMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we receive a state indicating RX has ended, go back to listening
	if msg.State == "listening" && s.State == StateRxBusy {
		s.State = StateListening
		s.sendState(StateListening)
	}
}

// sendState sends current state to the browser
func (s *Session) sendState(state SessionState) error {
	return s.sendMessage(WSMessage{
		Type:  "voice_state",
		State: string(state),
	})
}

// sendError sends an error message to the browser
func (s *Session) sendError(reason string) error {
	return s.sendMessage(WSMessage{
		Type:   "error",
		Reason: reason,
	})
}

// sendMessage sends a message to the browser over WebSocket
func (s *Session) sendMessage(msg WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := s.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// GetState returns the current session state (thread-safe)
func (s *Session) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// IsTransmitting returns whether the session is actively transmitting
func (s *Session) IsTransmitting() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeTransmit
}

// GetModule returns the current module (thread-safe)
func (s *Session) GetModule() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Module
}

// GetCallsign returns the callsign (thread-safe)
func (s *Session) GetCallsign() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Callsign
}

// GetLastActivity returns the last activity time
func (s *Session) GetLastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActivity
}
