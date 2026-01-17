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
)

// SessionState represents the current state of a voice session
type SessionState string

const (
	StateIdle         SessionState = "idle"
	StateListening    SessionState = "listening"
	StateTransmitting SessionState = "transmitting"
	StateRxBusy       SessionState = "rx_busy"
)

// Session represents a single client's voice session
type Session struct {
	ID            string
	Callsign      string
	Module        string
	State         SessionState
	Authenticated bool
	Conn          *websocket.Conn
	VoiceClient   *VoiceClient
	Config        *SessionConfig

	mu             sync.RWMutex
	lastActivity   time.Time
	txStartTime    time.Time
	activeTransmit bool
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
	Type         string `json:"type"`
	Module       string `json:"module,omitempty"`
	Callsign     string `json:"callsign,omitempty"`
	Password     string `json:"password,omitempty"`
	Opus         string `json:"opus,omitempty"` // base64 encoded
	State        string `json:"state,omitempty"`
	From         string `json:"from,omitempty"`
	Reason       string `json:"reason,omitempty"`
	ActiveTalker string `json:"active_talker,omitempty"`
}

// NewSession creates a new voice session
func NewSession(id string, conn *websocket.Conn, config *SessionConfig) (*Session, error) {
	// Create NNG voice client
	voiceClient, err := NewVoiceClient(config.ReflectorAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create voice client: %w", err)
	}

	session := &Session{
		ID:           id,
		State:        StateIdle,
		Conn:         conn,
		VoiceClient:  voiceClient,
		Config:       config,
		lastActivity: time.Now(),
	}

	// Register handlers for incoming voice messages from reflector
	voiceClient.OnMessage("audio_data", session.handleAudioFromReflector)
	voiceClient.OnMessage("state", session.handleStateChange)

	return session, nil
}

// Start begins the voice session
func (s *Session) Start() error {
	// Connect to reflector voice endpoint
	if err := s.VoiceClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to reflector: %w", err)
	}

	// Start listening for messages from reflector
	go func() {
		if err := s.VoiceClient.Listen(); err != nil {
			log.Printf("Session %s: Voice client listen error: %v", s.ID, err)
		}
	}()

	log.Printf("Session %s: Started", s.ID)
	return nil
}

// Stop ends the voice session and cleans up resources
func (s *Session) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If transmitting, send PTT stop
	if s.State == StateTransmitting {
		s.VoiceClient.SendPTTStop(s.Module, s.Callsign)
	}

	// Disconnect from reflector
	if err := s.VoiceClient.Disconnect(); err != nil {
		log.Printf("Session %s: Failed to disconnect voice client: %v", s.ID, err)
	}

	s.State = StateIdle
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

	s.Module = msg.Module
	s.Callsign = msg.Callsign
	s.State = StateListening

	log.Printf("Session %s: %s started listening to module %s", s.ID, s.Callsign, s.Module)

	return s.sendState(StateListening)
}

// handleVoiceStop stops the voice session
func (s *Session) handleVoiceStop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State == StateTransmitting {
		s.VoiceClient.SendPTTStop(s.Module, s.Callsign)
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

	// Send PTT start to reflector
	if err := s.VoiceClient.SendPTTStart(s.Module, s.Callsign); err != nil {
		return fmt.Errorf("failed to send PTT start: %w", err)
	}

	s.State = StateTransmitting
	s.activeTransmit = true
	s.txStartTime = time.Now()

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
	if err := s.VoiceClient.SendPTTStop(s.Module, s.Callsign); err != nil {
		return fmt.Errorf("failed to send PTT stop: %w", err)
	}

	duration := time.Since(s.txStartTime)
	s.State = StateListening
	s.activeTransmit = false

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

	// Send audio data to reflector
	if err := s.VoiceClient.SendAudioData(s.Module, s.Callsign, opusData); err != nil {
		return fmt.Errorf("failed to send audio data: %w", err)
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
