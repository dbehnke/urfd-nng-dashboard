package voice

import (
	"fmt"
	"log"
	"sync"
)

// VoiceClientPool manages shared VoiceClient connections per module
// This solves the NNG PAIR 1:1 limitation by sharing a single connection
// across multiple web client sessions
type VoiceClientPool struct {
	clients        map[string]*SharedVoiceClient // key: module (e.g., "A", "B")
	mu             sync.RWMutex
	baseURL        string // Base URL template for audio (e.g., "tcp://urfd:5556")
	controlBaseURL string // Base URL template for control (e.g., "tcp://urfd:6556")
}

// Shutdown gracefully closes all connections in the pool
// Should be called when the application is shutting down
func (p *VoiceClientPool) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("VoiceClientPool: Shutting down %d client(s)", len(p.clients))

	for module, shared := range p.clients {
		if shared.client != nil {
			log.Printf("VoiceClientPool: Disconnecting module %s", module)
			if err := shared.client.Disconnect(); err != nil {
				log.Printf("VoiceClientPool: Error disconnecting module %s: %v", module, err)
			}
		}
	}

	p.clients = make(map[string]*SharedVoiceClient)
	log.Printf("VoiceClientPool: Shutdown complete")
}

// SharedVoiceClient wraps a VoiceClient with session management
type SharedVoiceClient struct {
	client         *VoiceClient
	sessions       map[string]*Session // key: session ID
	mu             sync.RWMutex
	refCount       int
	module         string
	activeTalker   string       // callsign of current transmitter (empty = none)
	activeTalkerMu sync.RWMutex // protects activeTalker
	sessionsMu     sync.RWMutex // protects sessions map
}

// NewVoiceClientPool creates a new voice client pool
func NewVoiceClientPool(baseURL, controlBaseURL string) *VoiceClientPool {
	return &VoiceClientPool{
		clients:        make(map[string]*SharedVoiceClient),
		baseURL:        baseURL,
		controlBaseURL: controlBaseURL,
	}
}

// GetClient gets or creates a shared client for a module
func (p *VoiceClientPool) GetClient(module string) (*SharedVoiceClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if client already exists
	if shared, exists := p.clients[module]; exists {
		shared.mu.Lock()
		shared.refCount++
		refCount := shared.refCount
		shared.mu.Unlock()
		log.Printf("VoiceClientPool: Reusing existing client for module %s (refCount: %d)", module, refCount)
		return shared, nil
	}

	// Create module-specific URL
	// The baseURL should be like "tcp://127.0.0.1:5556"
	// We need to add (module - 'A') to the port number
	// For example: Module A = 5556, Module B = 5557, Module C = 5558
	url, err := p.getModuleURL(module, p.baseURL)
	if err != nil {
		log.Printf("VoiceClientPool: Failed to generate URL for module %s: %v", module, err)
		return nil, fmt.Errorf("failed to generate module URL: %w", err)
	}

	// Create module-specific control URL (e.g., 6556 for module A)
	controlURL, err := p.getModuleURL(module, p.controlBaseURL)
	if err != nil {
		log.Printf("VoiceClientPool: Failed to generate control URL for module %s: %v", module, err)
		return nil, fmt.Errorf("failed to generate module control URL: %w", err)
	}

	log.Printf("VoiceClientPool: Creating new voice client for module %s with URL: %s (audio), %s (control)", module, url, controlURL)

	client, err := NewVoiceClient(url, controlURL)
	if err != nil {
		log.Printf("VoiceClientPool: Failed to create voice client for module %s: %v", module, err)
		return nil, fmt.Errorf("failed to create voice client: %w", err)
	}

	// Connect to reflector
	if err := client.Connect(); err != nil {
		log.Printf("VoiceClientPool: Failed to connect to reflector for module %s: %v", module, err)
		return nil, fmt.Errorf("failed to connect to reflector: %w", err)
	}

	shared := &SharedVoiceClient{
		client:   client,
		sessions: make(map[string]*Session),
		refCount: 1,
		module:   module,
	}

	// Register message handlers ONCE for this shared client
	client.OnMessage("audio_data", func(msg VoiceMessage) {
		shared.BroadcastAudioToSessions(msg)
	})
	client.OnMessage("peer_audio", func(msg VoiceMessage) {
		// Binary audio from M17/other protocols via urfd
		shared.BroadcastAudioToSessions(msg)
	})
	client.OnMessage("state", func(msg VoiceMessage) {
		shared.BroadcastStateToSessions(msg)
	})
	client.OnMessage("recording_complete", func(msg VoiceMessage) {
		// Handle recording completion notification from urfd
		shared.HandleRecordingComplete(msg)
	})

	// Start listening for messages from reflector
	go func() {
		log.Printf("SharedVoiceClient[%s]: Starting listen loop", module)
		if err := client.Listen(); err != nil {
			log.Printf("SharedVoiceClient[%s]: Listen error: %v", module, err)
		}
		log.Printf("SharedVoiceClient[%s]: Listen loop ended", module)
	}()

	p.clients[module] = shared
	log.Printf("VoiceClientPool: Created new client for module %s", module)
	return shared, nil
}

// getModuleURL generates the URL for a specific module by adding the module offset to the port
func (p *VoiceClientPool) getModuleURL(module string, baseURL string) (string, error) {
	if len(module) != 1 {
		return "", fmt.Errorf("invalid module: %s (must be single character)", module)
	}

	moduleChar := module[0]
	if moduleChar < 'A' || moduleChar > 'Z' {
		return "", fmt.Errorf("invalid module: %s (must be A-Z)", module)
	}

	// Find the last colon (port separator)
	lastColon := -1
	for i := len(baseURL) - 1; i >= 0; i-- {
		if baseURL[i] == ':' {
			lastColon = i
			break
		}
	}

	if lastColon == -1 {
		return "", fmt.Errorf("base URL has no port: %s", baseURL)
	}

	// Parse the base port
	basePortStr := baseURL[lastColon+1:]
	basePort := 0
	for i := 0; i < len(basePortStr); i++ {
		if basePortStr[i] < '0' || basePortStr[i] > '9' {
			break
		}
		basePort = basePort*10 + int(basePortStr[i]-'0')
	}

	if basePort == 0 {
		return "", fmt.Errorf("invalid port in base URL: %s", baseURL)
	}

	// Calculate module-specific port: base_port + (module - 'A') * 3
	// Module A=0, B=3, C=6, D=9, M=36, S=54, Z=75, etc.
	// Examples: A=5556, D=5559 (+3), M=5568 (+12), S=5574 (+18), Z=5581 (+25)
	moduleOffset := int(moduleChar-'A') * 3
	modulePort := basePort + moduleOffset

	// Construct the new URL
	url := baseURL[:lastColon+1] + fmt.Sprintf("%d", modulePort)

	return url, nil
}

// ReleaseClient releases a reference to a shared client
// NOTE: We keep connections alive even when refCount hits 0 for resilience
// Connections are persistent service-to-service pipes that should stay open
func (p *VoiceClientPool) ReleaseClient(module, sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	shared, exists := p.clients[module]
	if !exists {
		return
	}

	shared.mu.Lock()
	delete(shared.sessions, sessionID)
	shared.refCount--
	refCount := shared.refCount
	shared.mu.Unlock()

	log.Printf("VoiceClientPool: Released client for module %s, session %s (refCount: %d)", module, sessionID, refCount)

	// CHANGED: Keep connection alive even when no sessions
	// The NNG connection is a persistent pipe between dashboard and urfd
	// It should remain open for fast reconnection when new sessions arrive
	if refCount <= 0 {
		log.Printf("VoiceClientPool: Module %s has no active sessions, but keeping connection alive", module)
	}
}

// RegisterSession registers a session with the shared client
func (s *SharedVoiceClient) RegisterSession(session *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[session.ID] = session
	log.Printf("SharedVoiceClient[%s]: Registered session %s (%s on module %s)", s.module, session.ID, session.Callsign, session.Module)
}

// UnregisterSession unregisters a session
func (s *SharedVoiceClient) UnregisterSession(sessionID string) {
	s.sessionsMu.Lock()
	session, exists := s.sessions[sessionID]
	if !exists {
		s.sessionsMu.Unlock()
		return
	}

	// Get callsign before deleting
	callsign := session.Callsign
	delete(s.sessions, sessionID)
	remaining := len(s.sessions)
	s.sessionsMu.Unlock()

	// If this session was the active talker, release PTT
	if callsign != "" {
		s.ReleasePTT(sessionID, callsign)
	}

	log.Printf("SharedVoiceClient[%s]: Unregistered session %s (remaining: %d)", s.module, sessionID, remaining)
}

// BroadcastAudioToSessions sends audio to all registered sessions on this module
// This is for audio coming FROM the urfd reflector
func (s *SharedVoiceClient) BroadcastAudioToSessions(msg VoiceMessage) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	log.Printf("SharedVoiceClient[%s]: Broadcasting reflector audio from %s (source=%s) to %d sessions", s.module, msg.Callsign, msg.Source, len(s.sessions))

	// Broadcast to all sessions except the sender
	broadcastCount := 0
	for sessionID, session := range s.sessions {
		log.Printf("SharedVoiceClient[%s]: Checking session %s (callsign=%s, module=%s)",
			s.module, sessionID, session.Callsign, session.Module)

		// Skip if this is a web client's own audio echoed back from reflector
		// Web audio should never come back from the reflector
		if msg.Source == "web" && session.Callsign == msg.Callsign {
			log.Printf("SharedVoiceClient[%s]: Skipping session %s - web audio echo from reflector (callsign=%s)",
				s.module, sessionID, msg.Callsign)
			continue
		}

		// Only forward if session is listening to this module and not the sender
		if session.Module == msg.Module && session.Callsign != msg.Callsign {
			log.Printf("SharedVoiceClient[%s]: Forwarding reflector audio to session %s (%s)", s.module, sessionID, session.Callsign)
			session.handleAudioFromReflector(msg)
			broadcastCount++
		} else {
			log.Printf("SharedVoiceClient[%s]: Skipping session %s (module match=%v, not sender=%v)",
				s.module, sessionID, session.Module == msg.Module, session.Callsign != msg.Callsign)
		}
	}

	log.Printf("SharedVoiceClient[%s]: Broadcast complete, forwarded to %d sessions", s.module, broadcastCount)
}

// BroadcastStateToSessions sends state updates to all sessions
func (s *SharedVoiceClient) BroadcastStateToSessions(msg VoiceMessage) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	for _, session := range s.sessions {
		if session.Module == msg.Module {
			session.handleStateChange(msg)
		}
	}
}

// SendPTTStart forwards PTT start to reflector (DEPRECATED: use SendPTTStartWithAck)
func (s *SharedVoiceClient) SendPTTStart(module, callsign string) error {
	return s.client.SendPTTStart(module, callsign)
}

// SendPTTStartWithAck sends PTT start via control socket and waits for ACK/NACK
func (s *SharedVoiceClient) SendPTTStartWithAck(module, callsign, sessionID string) (*ControlResponse, error) {
	return s.client.SendPTTStartWithAck(module, callsign, sessionID)
}

// SendPTTStop forwards PTT stop to reflector (DEPRECATED: use SendPTTStopWithAck)
func (s *SharedVoiceClient) SendPTTStop(module, callsign string) error {
	return s.client.SendPTTStop(module, callsign)
}

// SendPTTStopWithAck sends PTT stop via control socket and waits for ACK/NACK
func (s *SharedVoiceClient) SendPTTStopWithAck(module, callsign string) (*ControlResponse, error) {
	return s.client.SendPTTStopWithAck(module, callsign)
}

// SendAudioData forwards audio data to reflector
func (s *SharedVoiceClient) SendAudioData(module, callsign string, opusData []byte) error {
	return s.client.SendAudioData(module, callsign, opusData)
}

// SendSessionStart forwards session start to reflector
func (s *SharedVoiceClient) SendSessionStart(module, callsign, sessionID string) error {
	return s.client.SendSessionStart(module, callsign, sessionID)
}

// SendSessionStop forwards session stop to reflector
func (s *SharedVoiceClient) SendSessionStop(callsign string) error {
	return s.client.SendSessionStop(callsign)
}

// OnUrfdReconnect resyncs all active sessions after urfd reconnects
func (s *SharedVoiceClient) OnUrfdReconnect() error {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	log.Printf("urfd reconnected, resyncing %d active sessions", len(s.sessions))

	for _, session := range s.sessions {
		msg := VoiceMessage{
			Type:     "voice_session_start",
			Module:   session.Module,
			Callsign: session.Callsign,
			Source:   "web",
		}
		if err := s.client.Send(msg); err != nil {
			log.Printf("Warning: Failed to resync session %s: %v", session.Callsign, err)
		}
	}

	return nil
}

// RequestPTT attempts to acquire PTT for a session
// Returns error if PTT is already held by another callsign
func (s *SharedVoiceClient) RequestPTT(sessionID, callsign string) error {
	s.activeTalkerMu.Lock()
	defer s.activeTalkerMu.Unlock()

	// Check if someone else is already transmitting
	if s.activeTalker != "" && s.activeTalker != callsign {
		log.Printf("PTT denied for %s (active: %s)", callsign, s.activeTalker)
		return fmt.Errorf("PTT denied - %s is transmitting", s.activeTalker)
	}

	// Grant PTT
	s.activeTalker = callsign
	log.Printf("PTT granted to %s", callsign)
	return nil
}

// ReleasePTT releases PTT for a session
// Only clears activeTalker if the callsign matches
func (s *SharedVoiceClient) ReleasePTT(sessionID, callsign string) {
	s.activeTalkerMu.Lock()
	defer s.activeTalkerMu.Unlock()

	// Only clear if this is the active talker
	if s.activeTalker == callsign {
		s.activeTalker = ""
		log.Printf("PTT released by %s", callsign)
	} else {
		log.Printf("PTT release ignored for %s (active talker: %s)", callsign, s.activeTalker)
	}
}

// BroadcastPeerAudio broadcasts audio from one session to all other sessions on the same module
// This is for real-time peer-to-peer audio (browser to browser) without going through urfd
func (s *SharedVoiceClient) BroadcastPeerAudio(opusData []byte, fromSessionID, fromCallsign, fromClientSessionID, module string) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	log.Printf("SharedVoiceClient[%s]: Broadcasting peer audio from %s (clientSession=%s) to %d sessions",
		s.module, fromCallsign, fromClientSessionID, len(s.sessions))

	// Broadcast to all sessions except the sender
	broadcastCount := 0
	for sessionID, session := range s.sessions {
		// Skip if this is the sender (by server session ID)
		if sessionID == fromSessionID {
			continue
		}

		// Skip if session is on a different module (module isolation)
		if session.Module != module {
			continue
		}

		// Send audio to this peer with the sender's client session ID
		if err := session.SendAudioFromPeer(opusData, fromCallsign, fromClientSessionID); err != nil {
			log.Printf("SharedVoiceClient[%s]: Failed to send peer audio to session %s: %v", s.module, sessionID, err)
			// Continue trying to send to other sessions
		} else {
			broadcastCount++
		}
	}

	log.Printf("SharedVoiceClient[%s]: Broadcast peer audio from %s to %d peers on module %s", s.module, fromCallsign, broadcastCount, module)
}

// NotifyPeerTransmissionEnd notifies all receiving sessions that peer transmission has ended
// This allows them to transition from rx_busy back to listening state
func (s *SharedVoiceClient) NotifyPeerTransmissionEnd(fromSessionID, fromCallsign, module string) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	log.Printf("SharedVoiceClient[%s]: Notifying peers that %s transmission ended", s.module, fromCallsign)

	// Notify all sessions except the sender
	for sessionID, session := range s.sessions {
		// Skip if this is the sender
		if sessionID == fromSessionID {
			continue
		}

		// Skip if session is on a different module
		if session.Module != module {
			continue
		}

		// Notify session to return to listening state if in rx_busy
		if err := session.ClearPeerRxBusy(); err != nil {
			log.Printf("SharedVoiceClient[%s]: Failed to clear rx_busy for session %s: %v", s.module, sessionID, err)
		}
	}
}

// HandleRecordingComplete processes recording completion notifications from urfd
// This is called when urfd finishes recording a transmission and has saved the audio file
func (s *SharedVoiceClient) HandleRecordingComplete(msg VoiceMessage) {
	log.Printf("SharedVoiceClient[%s]: Recording complete for %s: %s", s.module, msg.Callsign, msg.AudioFile)

	// Forward to all sessions for the callsign/module
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	for _, session := range s.sessions {
		// Match by callsign and module
		if session.Callsign == msg.Callsign && session.Module == msg.Module {
			session.HandleRecordingComplete(msg.AudioFile)
		}
	}
}
