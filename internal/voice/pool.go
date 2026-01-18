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
	clients map[string]*SharedVoiceClient // key: module (e.g., "A", "B")
	mu      sync.RWMutex
	baseURL string // Base URL template (e.g., "ipc:///tmp/voice_%s")
}

// SharedVoiceClient wraps a VoiceClient with session management
type SharedVoiceClient struct {
	client   *VoiceClient
	sessions map[string]*Session // key: session ID
	mu       sync.RWMutex
	refCount int
	module   string
}

// NewVoiceClientPool creates a new voice client pool
func NewVoiceClientPool(baseURL string) *VoiceClientPool {
	return &VoiceClientPool{
		clients: make(map[string]*SharedVoiceClient),
		baseURL: baseURL,
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
	url, err := p.getModuleURL(module)
	if err != nil {
		log.Printf("VoiceClientPool: Failed to generate URL for module %s: %v", module, err)
		return nil, fmt.Errorf("failed to generate module URL: %w", err)
	}

	log.Printf("VoiceClientPool: Creating new voice client for module %s with URL: %s", module, url)

	client, err := NewVoiceClient(url)
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
	client.OnMessage("state", func(msg VoiceMessage) {
		shared.BroadcastStateToSessions(msg)
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
func (p *VoiceClientPool) getModuleURL(module string) (string, error) {
	if len(module) != 1 {
		return "", fmt.Errorf("invalid module: %s (must be single character)", module)
	}

	moduleChar := module[0]
	if moduleChar < 'A' || moduleChar > 'Z' {
		return "", fmt.Errorf("invalid module: %s (must be A-Z)", module)
	}

	// Find the last colon (port separator)
	lastColon := -1
	for i := len(p.baseURL) - 1; i >= 0; i-- {
		if p.baseURL[i] == ':' {
			lastColon = i
			break
		}
	}

	if lastColon == -1 {
		return "", fmt.Errorf("base URL has no port: %s", p.baseURL)
	}

	// Parse the base port
	basePortStr := p.baseURL[lastColon+1:]
	basePort := 0
	for i := 0; i < len(basePortStr); i++ {
		if basePortStr[i] < '0' || basePortStr[i] > '9' {
			break
		}
		basePort = basePort*10 + int(basePortStr[i]-'0')
	}

	if basePort == 0 {
		return "", fmt.Errorf("invalid port in base URL: %s", p.baseURL)
	}

	// Calculate module-specific port: base_port + (module - 'A')
	moduleOffset := int(moduleChar - 'A')
	modulePort := basePort + moduleOffset

	// Construct the new URL
	url := p.baseURL[:lastColon+1] + fmt.Sprintf("%d", modulePort)

	return url, nil
}

// ReleaseClient releases a reference to a shared client
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

	// If no more sessions, disconnect and remove
	if refCount <= 0 {
		shared.client.Disconnect()
		delete(p.clients, module)
		log.Printf("VoiceClientPool: Removed client for module %s (no more sessions)", module)
	}
}

// RegisterSession registers a session with the shared client
func (s *SharedVoiceClient) RegisterSession(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	log.Printf("SharedVoiceClient[%s]: Registered session %s (total: %d)", s.module, session.ID, len(s.sessions))
}

// UnregisterSession unregisters a session
func (s *SharedVoiceClient) UnregisterSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	log.Printf("SharedVoiceClient[%s]: Unregistered session %s (remaining: %d)", s.module, sessionID, len(s.sessions))
}

// BroadcastAudioToSessions sends audio to all registered sessions on this module
func (s *SharedVoiceClient) BroadcastAudioToSessions(msg VoiceMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log.Printf("SharedVoiceClient[%s]: Broadcasting audio from %s to %d sessions", s.module, msg.Callsign, len(s.sessions))

	// Broadcast to all sessions except the sender
	broadcastCount := 0
	for sessionID, session := range s.sessions {
		log.Printf("SharedVoiceClient[%s]: Checking session %s (callsign=%s, module=%s)",
			s.module, sessionID, session.Callsign, session.Module)

		// Only forward if session is listening to this module and not the sender
		if session.Module == msg.Module && session.Callsign != msg.Callsign {
			log.Printf("SharedVoiceClient[%s]: Forwarding audio to session %s (%s)", s.module, sessionID, session.Callsign)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if session.Module == msg.Module {
			session.handleStateChange(msg)
		}
	}
}

// SendPTTStart forwards PTT start to reflector
func (s *SharedVoiceClient) SendPTTStart(module, callsign string) error {
	return s.client.SendPTTStart(module, callsign)
}

// SendPTTStop forwards PTT stop to reflector
func (s *SharedVoiceClient) SendPTTStop(module, callsign string) error {
	return s.client.SendPTTStop(module, callsign)
}

// SendAudioData forwards audio data to reflector
func (s *SharedVoiceClient) SendAudioData(module, callsign string, opusData []byte) error {
	return s.client.SendAudioData(module, callsign, opusData)
}
