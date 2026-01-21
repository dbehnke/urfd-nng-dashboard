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

	// RxBusyTimeout is the duration after which rx_busy state is automatically cleared
	// if no audio is received. This matches the frontend timeout of 500ms.
	RxBusyTimeout = 1000 * time.Millisecond
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
	ID              string
	ClientSessionID string // Client-provided session ID for echo prevention
	Callsign        string
	Module          string
	State           SessionState
	Authenticated   bool
	Conn            *websocket.Conn
	SharedClient    *SharedVoiceClient // Changed from VoiceClient to SharedVoiceClient
	ClientPool      *VoiceClientPool   // Reference to the pool
	Config          *SessionConfig
	DB              *gorm.DB
	Hub             Broadcaster

	mu                 sync.RWMutex // Protects session state
	writeMu            sync.Mutex   // Protects WebSocket writes (concurrent writes cause panics)
	lastActivity       time.Time
	txStartTime        time.Time
	activeTransmit     bool
	hearingID          uint        // Database ID for current transmission
	lastHearingID      uint        // Last completed hearing ID (for late recording updates)
	lastAudioRxTime    time.Time   // Last time audio was received (for rx_busy timeout)
	rxBusyTimer        *time.Timer // Timer to clear stuck rx_busy state
	rxBusyTimerMu      sync.Mutex  // Protects rxBusyTimer
	audioPacketCounter int         // Counter for logging audio packets sent to urfd
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
	SessionID     string `json:"session_id,omitempty"` // Client-provided session ID
	Password      string `json:"password,omitempty"`
	Opus          string `json:"opus,omitempty"` // base64 encoded
	State         string `json:"state,omitempty"`
	From          string `json:"from,omitempty"`
	FromSessionID string `json:"from_session_id,omitempty"` // Session ID of sender (for echo prevention)
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

	// Stop the rx_busy timeout timer if active
	s.stopRxBusyTimer()

	// If transmitting, send PTT stop and release PTT lock
	if state == StateTransmitting && sharedClient != nil {
		sharedClient.SendPTTStop(module, callsign)
		sharedClient.ReleasePTT(s.ID, callsign)
		// Notify peers that transmission ended
		sharedClient.NotifyPeerTransmissionEnd(s.ID, callsign, module)
	}

	// Send voice_session_stop to urfd (if we have an active session)
	if sharedClient != nil && callsign != "" {
		if err := sharedClient.SendSessionStop(callsign); err != nil {
			log.Printf("Session %s: Warning: Failed to send voice_session_stop to urfd: %v", s.ID, err)
		}
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

	// TEMPORARY DEBUG: log all incoming WS messages
	log.Printf("Session %s: Received WS msg: type=%s, module=%s, callsign=%s, raw=%s", s.ID, msg.Type, msg.Module, msg.Callsign, string(data))

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
	s.ClientSessionID = msg.SessionID // Store client's session ID for echo prevention
	s.SharedClient = sharedClient
	s.State = StateListening

	log.Printf("Session %s: Stored client session ID: %s", s.ID, s.ClientSessionID)

	// Register this session with the shared client
	sharedClient.RegisterSession(s)

	// Send voice_session_start to urfd (don't fail if urfd is down)
	if err := sharedClient.SendSessionStart(s.Module, s.Callsign, s.ClientSessionID); err != nil {
		log.Printf("Session %s: Warning: Failed to send voice_session_start to urfd: %v", s.ID, err)
		// Don't return error - peer audio will still work
	}

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

	// Send voice_session_stop to urfd (don't fail if urfd is down)
	if s.SharedClient != nil && s.Callsign != "" {
		if err := s.SharedClient.SendSessionStop(s.Callsign); err != nil {
			log.Printf("Session %s: Warning: Failed to send voice_session_stop to urfd: %v", s.ID, err)
		}
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

	// PTT granted, send PTT start to reflector via control socket (with ACK)
	response, err := s.SharedClient.SendPTTStartWithAck(s.Module, s.Callsign, s.ClientSessionID)
	if err != nil {
		// Control request failed (timeout, etc) - release PTT and fail
		s.SharedClient.ReleasePTT(s.ID, s.Callsign)
		log.Printf("Session %s: PTT control request failed for %s: %v", s.ID, s.Callsign, err)
		return s.sendMessage(WSMessage{
			Type:   "ptt_denied",
			Reason: fmt.Sprintf("control_error: %v", err),
		})
	}

	// Check response status
	if response.Status != "success" {
		// PTT denied by urfd (module busy, no session, etc) - release PTT and fail
		s.SharedClient.ReleasePTT(s.ID, s.Callsign)
		log.Printf("Session %s: PTT denied by urfd for %s: %s - %s", s.ID, s.Callsign, response.Reason, response.Message)

		// Send detailed error to browser
		reason := response.Reason
		if response.Reason == "module_busy" && response.ActiveUser != "" {
			reason = fmt.Sprintf("Module %s is currently in use by %s", s.Module, response.ActiveUser)
		} else if response.Message != "" {
			reason = response.Message
		}

		return s.sendMessage(WSMessage{
			Type:         "ptt_denied",
			Reason:       reason,
			ActiveTalker: response.ActiveUser,
		})
	}

	// Success! PTT acknowledged by urfd
	log.Printf("Session %s: PTT start ACK received (stream_id=%d, recording=%s)",
		s.ID, response.StreamID, response.RecordingFile)

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

	// Send PTT stop to reflector via control socket (with ACK)
	if s.SharedClient == nil {
		return fmt.Errorf("not connected to voice client")
	}
	response, err := s.SharedClient.SendPTTStopWithAck(s.Module, s.Callsign)
	if err != nil {
		// Log warning but don't fail - peer audio already stopped
		log.Printf("Session %s: Warning: Failed to send PTT stop to urfd: %v", s.ID, err)
	} else if response.Status != "success" {
		log.Printf("Session %s: Warning: PTT stop NACK from urfd: %s - %s", s.ID, response.Reason, response.Message)
	} else {
		log.Printf("Session %s: PTT stop ACK received", s.ID)
	}

	// NEW: Release PTT in SharedClient
	s.SharedClient.ReleasePTT(s.ID, s.Callsign)

	// Notify all receiving peers that this transmission has ended
	// This allows them to transition from rx_busy back to listening
	s.SharedClient.NotifyPeerTransmissionEnd(s.ID, s.Callsign, s.Module)

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
		s.lastHearingID = s.hearingID // Save for late recording updates
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

	// DEBUG: Log entry to confirm this function is being called
	log.Printf("Session %s: handleAudioData() called - State=%s, SharedClient=%v", s.ID, s.State, s.SharedClient != nil)

	if s.State != StateTransmitting {
		// Silently ignore audio packets that arrive shortly after PTT release
		// This is normal due to encoder buffering and network latency
		log.Printf("Session %s: DROPPED audio packet - State is %s (not transmitting)", s.ID, s.State)
		return nil
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
		s.SharedClient.BroadcastPeerAudio(opusData, s.ID, s.Callsign, s.ClientSessionID, s.Module)
	}

	// Send audio data to reflector (for recording)
	if s.SharedClient == nil {
		return fmt.Errorf("not connected to voice client")
	}

	// DEBUG: Log every 10th audio packet sent to urfd
	s.audioPacketCounter++
	if (s.audioPacketCounter % 10) == 1 {
		log.Printf("Session %s: Sending audio packet #%d to urfd (size: %d bytes)", s.ID, s.audioPacketCounter, len(opusData))
	}

	if err := s.SharedClient.SendAudioData(s.Module, s.Callsign, opusData); err != nil {
		// Log warning but don't fail - peer audio already sent
		log.Printf("Session %s: WARNING: Failed to send audio #%d to urfd: %v", s.ID, s.audioPacketCounter, err)
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
		s.lastAudioRxTime = time.Now()
		s.sendState(StateRxBusy)
		s.mu.Unlock()
		s.mu.RLock()

		// Start the rx_busy timeout timer
		s.startRxBusyTimer()
	} else if s.State == StateRxBusy {
		// Update last audio receive time and reset timer
		s.mu.RUnlock()
		s.mu.Lock()
		s.lastAudioRxTime = time.Now()
		s.mu.Unlock()
		s.mu.RLock()

		// Reset the timer to extend the timeout
		s.resetRxBusyTimer()
	}

	s.mu.RUnlock()

	// Encode Opus data as base64 for WebSocket
	opusB64 := base64.StdEncoding.EncodeToString(msg.Opus)

	wsMsg := WSMessage{
		Type:          "audio_data",
		Opus:          opusB64,
		From:          msg.Callsign,
		FromSessionID: msg.SessionID, // Include session ID for echo prevention
	}

	s.sendMessage(wsMsg)
}

// SendAudioFromPeer sends peer audio (from another browser) to this session
// This is for real-time audio multiplexing between web clients
func (s *Session) SendAudioFromPeer(opusData []byte, fromCallsign, fromSessionID string) error {
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

	// If we receive peer audio while in listening state, switch to rx_busy
	// This matches the behavior of reflector audio handling
	if s.State == StateListening {
		s.mu.RUnlock()
		s.mu.Lock()
		s.State = StateRxBusy
		s.lastAudioRxTime = time.Now()
		s.sendState(StateRxBusy)
		s.mu.Unlock()
		s.mu.RLock()

		// Start the rx_busy timeout timer
		s.startRxBusyTimer()
	} else if s.State == StateRxBusy {
		// Update last audio receive time and reset timer
		s.mu.RUnlock()
		s.mu.Lock()
		s.lastAudioRxTime = time.Now()
		s.mu.Unlock()
		s.mu.RLock()

		// Reset the timer to extend the timeout
		s.resetRxBusyTimer()
	}

	s.mu.RUnlock()

	// Encode Opus data as base64 for WebSocket
	opusB64 := base64.StdEncoding.EncodeToString(opusData)

	wsMsg := WSMessage{
		Type:          "peer_audio", // Different type to distinguish from reflector audio
		Opus:          opusB64,
		From:          fromCallsign,
		FromSessionID: fromSessionID, // Include session ID for echo prevention
	}

	return s.sendMessage(wsMsg)
}

// ClearPeerRxBusy clears the rx_busy state if currently receiving peer audio
// This is called when a peer transmission ends to allow other sessions to transmit
func (s *Session) ClearPeerRxBusy() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only clear if we're in rx_busy state (receiving audio)
	if s.State == StateRxBusy {
		s.State = StateListening
		// Stop the rx_busy timeout timer
		s.stopRxBusyTimer()
		return s.sendState(StateListening)
	}

	return nil
}

// handleStateChange handles state changes from the reflector
func (s *Session) handleStateChange(msg VoiceMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we receive a state indicating RX has ended, go back to listening
	if msg.State == "listening" && s.State == StateRxBusy {
		s.State = StateListening
		// Stop the rx_busy timeout timer when reflector signals end of audio
		s.stopRxBusyTimer()
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

	// Protect WebSocket writes - gorilla/websocket requires serialized access
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

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

// startRxBusyTimer starts the timer to automatically clear rx_busy state after timeout
func (s *Session) startRxBusyTimer() {
	s.rxBusyTimerMu.Lock()
	defer s.rxBusyTimerMu.Unlock()

	// Cancel existing timer if any
	if s.rxBusyTimer != nil {
		s.rxBusyTimer.Stop()
	}

	// Create new timer
	s.rxBusyTimer = time.AfterFunc(RxBusyTimeout, func() {
		s.clearRxBusyOnTimeout()
	})
}

// resetRxBusyTimer resets the rx_busy timeout timer
func (s *Session) resetRxBusyTimer() {
	s.rxBusyTimerMu.Lock()
	defer s.rxBusyTimerMu.Unlock()

	if s.rxBusyTimer != nil {
		s.rxBusyTimer.Reset(RxBusyTimeout)
	}
}

// stopRxBusyTimer stops and clears the rx_busy timer
func (s *Session) stopRxBusyTimer() {
	s.rxBusyTimerMu.Lock()
	defer s.rxBusyTimerMu.Unlock()

	if s.rxBusyTimer != nil {
		s.rxBusyTimer.Stop()
		s.rxBusyTimer = nil
	}
}

// clearRxBusyOnTimeout is called when the rx_busy timeout expires
func (s *Session) clearRxBusyOnTimeout() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only transition if still in rx_busy state
	if s.State == StateRxBusy {
		timeSinceLastAudio := time.Since(s.lastAudioRxTime)
		log.Printf("Session %s: rx_busy timeout triggered (%.0fms since last audio)", s.ID, timeSinceLastAudio.Seconds()*1000)

		s.State = StateListening
		s.sendState(StateListening)
		log.Printf("Session %s: Cleared stuck rx_busy state, transitioned to listening", s.ID)
	}
}

// HandleRecordingComplete updates the hearing record with the audio file path
// This is called when urfd finishes recording a transmission
func (s *Session) HandleRecordingComplete(audioFile string) {
	if s.DB == nil {
		log.Printf("Session %s: Received recording complete but no database connection", s.ID)
		return
	}

	// Find the oldest hearing without an audio_file for this callsign/module
	// This handles rapid-fire transmissions where multiple recordings arrive
	var hearing Hearing
	err := s.DB.Where("my = ? AND module = ? AND (audio_file IS NULL OR audio_file = '')", s.Callsign, s.Module).
		Order("created_at ASC").
		First(&hearing).Error

	if err != nil {
		log.Printf("Session %s: No matching hearing found for recording %s (callsign=%s, module=%s): %v",
			s.ID, audioFile, s.Callsign, s.Module, err)
		return
	}

	log.Printf("Session %s: Processing recording complete for hearing %d: %s", s.ID, hearing.ID, audioFile)

	// Update the hearing record with the audio file path
	if err := s.DB.Model(&Hearing{}).Where("id = ?", hearing.ID).Update("audio_file", audioFile).Error; err != nil {
		log.Printf("Session %s: Failed to update hearing %d with audio_file: %v", s.ID, hearing.ID, err)
		return
	}

	log.Printf("Session %s: Updated hearing %d with audio_file: %s", s.ID, hearing.ID, audioFile)

	// Broadcast the update to all connected WebSocket clients
	if s.Hub != nil {
		s.Hub.BroadcastJSON(map[string]interface{}{
			"type":       "hearing_update",
			"id":         hearing.ID,
			"audio_file": audioFile,
		})
	}
}
