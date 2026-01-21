package voice

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

// ByteSlice is a custom type that marshals to JSON as array instead of base64
type ByteSlice []byte

// MarshalJSON ensures the byte slice is marshaled as a JSON array, not base64
func (b ByteSlice) MarshalJSON() ([]byte, error) {
	if b == nil {
		return []byte("null"), nil
	}
	// Marshal as array of integers
	result := make([]byte, 0, len(b)*4+2)
	result = append(result, '[')
	for i, v := range b {
		if i > 0 {
			result = append(result, ',')
		}
		result = append(result, []byte(fmt.Sprintf("%d", v))...)
	}
	result = append(result, ']')
	return result, nil
}

// VoiceMessage represents a message sent/received over NNG voice endpoint
type VoiceMessage struct {
	Type      string    `json:"type"`                 // "audio_data", "ptt_start", "ptt_stop", "state", "recording_complete"
	Module    string    `json:"module,omitempty"`     // Module identifier (A, B, C, D)
	Callsign  string    `json:"callsign,omitempty"`   // User callsign
	Source    string    `json:"source,omitempty"`     // "web" for web clients
	Opus      ByteSlice `json:"opus,omitempty"`       // Opus encoded audio data (marshaled as array)
	State     string    `json:"state,omitempty"`      // "listening", "transmitting", "rx_busy"
	AudioFile string    `json:"audio_file,omitempty"` // Path to recorded audio file
	SessionID string    `json:"session_id,omitempty"` // Session identifier for matching recordings
}

// ControlResponse represents a response from urfd control socket (REP)
type ControlResponse struct {
	Status        string `json:"status"`                   // "success" or "error"
	StreamID      int    `json:"stream_id,omitempty"`      // Stream ID assigned by urfd (on ptt_start success)
	RecordingFile string `json:"recording_file,omitempty"` // Recording filename (on ptt_start success)
	Reason        string `json:"reason,omitempty"`         // Error reason: "module_busy", "no_session", "internal_error"
	ActiveUser    string `json:"active_user,omitempty"`    // Callsign of user currently transmitting (on module_busy)
	Message       string `json:"message,omitempty"`        // Human-readable error message
}

// VoiceClient manages NNG PAIR connection to reflector voice endpoint
type VoiceClient struct {
	url           string
	sock          mangos.Socket // PAIR socket for audio data
	controlURL    string
	controlSock   mangos.Socket // REQ socket for PTT control
	mu            sync.RWMutex
	handlers      map[string]func(VoiceMessage)
	running       bool
	reconnecting  bool
	reconnectStop chan struct{}
	sendCounter   int // Debug counter for logging
	sendCounterMu sync.Mutex
}

// NewVoiceClient creates a new NNG PAIR client for voice communication
func NewVoiceClient(url, controlURL string) (*VoiceClient, error) {
	sock, err := pair.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create PAIR socket: %w", err)
	}

	// Set socket options - negative timeout means wait forever
	if err := sock.SetOption(mangos.OptionRecvDeadline, time.Duration(-1)); err != nil {
		sock.Close()
		return nil, fmt.Errorf("failed to set recv deadline: %w", err)
	}

	// Create REQ socket for control messages
	controlSock, err := req.NewSocket()
	if err != nil {
		sock.Close()
		return nil, fmt.Errorf("failed to create REQ socket: %w", err)
	}

	// Set timeout for REQ/REP operations (2 seconds)
	if err := controlSock.SetOption(mangos.OptionSendDeadline, 2*time.Second); err != nil {
		sock.Close()
		controlSock.Close()
		return nil, fmt.Errorf("failed to set control send deadline: %w", err)
	}
	if err := controlSock.SetOption(mangos.OptionRecvDeadline, 2*time.Second); err != nil {
		sock.Close()
		controlSock.Close()
		return nil, fmt.Errorf("failed to set control recv deadline: %w", err)
	}

	client := &VoiceClient{
		url:           url,
		sock:          sock,
		controlURL:    controlURL,
		controlSock:   controlSock,
		handlers:      make(map[string]func(VoiceMessage)),
		running:       false,
		reconnecting:  false,
		reconnectStop: make(chan struct{}),
	}

	return client, nil
}

// Connect establishes connection to the reflector voice endpoint
func (c *VoiceClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return fmt.Errorf("already connected")
	}

	// Connect audio socket (PAIR)
	if err := c.sock.Dial(c.url); err != nil {
		return fmt.Errorf("failed to dial audio socket %s: %w", c.url, err)
	}

	// Connect control socket (REQ)
	if err := c.controlSock.Dial(c.controlURL); err != nil {
		c.sock.Close()
		return fmt.Errorf("failed to dial control socket %s: %w", c.controlURL, err)
	}

	c.running = true
	log.Printf("Voice client connected to %s (audio) and %s (control)", c.url, c.controlURL)
	return nil
}

// Disconnect closes the NNG connection
func (c *VoiceClient) Disconnect() error {
	c.mu.Lock()

	if !c.running {
		c.mu.Unlock()
		return nil
	}

	c.running = false

	// Stop reconnection attempts
	if c.reconnecting {
		close(c.reconnectStop)
		c.reconnecting = false
	}

	c.mu.Unlock()

	// Close both sockets
	var err1, err2 error
	if err1 = c.sock.Close(); err1 != nil {
		log.Printf("Warning: failed to close audio socket: %v", err1)
	}
	if err2 = c.controlSock.Close(); err2 != nil {
		log.Printf("Warning: failed to close control socket: %v", err2)
	}

	log.Printf("Voice client disconnected from %s (audio) and %s (control)", c.url, c.controlURL)

	if err1 != nil {
		return fmt.Errorf("failed to close audio socket: %w", err1)
	}
	if err2 != nil {
		return fmt.Errorf("failed to close control socket: %w", err2)
	}
	return nil
}

// Send sends a voice message to the reflector
// Automatically retries on transient failures
func (c *VoiceClient) Send(msg VoiceMessage) error {
	c.mu.RLock()

	// DEBUG: Log state at entry
	running := c.running
	reconnecting := c.reconnecting

	if !c.running {
		c.mu.RUnlock()
		log.Printf("VoiceClient.Send(): BLOCKED - not connected (running=%v, reconnecting=%v, msgType=%s)",
			running, reconnecting, msg.Type)
		return fmt.Errorf("not connected")
	}

	// Don't send while reconnecting
	if c.reconnecting {
		c.mu.RUnlock()
		log.Printf("VoiceClient.Send(): BLOCKED - reconnection in progress (running=%v, reconnecting=%v, msgType=%s)",
			running, reconnecting, msg.Type)
		return fmt.Errorf("reconnection in progress")
	}

	c.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Try sending with automatic retry on failure
	maxRetries := 2
	for i := 0; i < maxRetries; i++ {
		c.mu.RLock()
		sock := c.sock
		c.mu.RUnlock()

		if err := sock.Send(data); err != nil {
			log.Printf("Voice NNG: Send error (attempt %d/%d): %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			log.Printf("VoiceClient.Send(): FAILED after %d attempts for msgType=%s: %v", maxRetries, msg.Type, err)
			return fmt.Errorf("failed to send message after %d attempts: %w", maxRetries, err)
		}
		// DEBUG: Log successful sends (for audio_data, only log occasionally)
		if msg.Type == "audio_data" {
			// Already logged in SendAudioData
		} else {
			log.Printf("VoiceClient.Send(): SUCCESS for msgType=%s", msg.Type)
		}
		return nil // Success
	}

	return fmt.Errorf("failed to send message")
}

// SendAudioData sends Opus audio data to the reflector
func (c *VoiceClient) SendAudioData(module, callsign string, opusData []byte) error {
	msg := VoiceMessage{
		Type:     "audio_data",
		Module:   module,
		Callsign: callsign,
		Source:   "web",
		Opus:     opusData,
	}

	// DEBUG: Log periodic audio packet sends
	c.sendCounterMu.Lock()
	c.sendCounter++
	counter := c.sendCounter
	c.sendCounterMu.Unlock()

	if counter == 1 || (counter%50) == 0 {
		log.Printf("VoiceClient: Sending audio_data #%d for %s on module %s (opus size: %d)",
			counter, callsign, module, len(opusData))
	}

	return c.Send(msg)
}

// SendControlRequest sends a control message via REQ socket and waits for ACK/NACK
// Returns the response or error if request fails or times out
func (c *VoiceClient) SendControlRequest(msg VoiceMessage) (*ControlResponse, error) {
	c.mu.RLock()
	if !c.running {
		c.mu.RUnlock()
		return nil, fmt.Errorf("not connected")
	}
	controlSock := c.controlSock
	c.mu.RUnlock()

	// Marshal request
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal control request: %w", err)
	}

	// Send request with automatic retry on timeout
	maxRetries := 2
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Control REQ: Sending %s (attempt %d/%d)", msg.Type, attempt, maxRetries)

		// Send request
		if err := controlSock.Send(data); err != nil {
			log.Printf("Control REQ: Send failed (attempt %d/%d): %v", attempt, maxRetries, err)
			if attempt < maxRetries {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("failed to send control request after %d attempts: %w", maxRetries, err)
		}

		// Wait for response (2s timeout set in socket options)
		respData, err := controlSock.Recv()
		if err != nil {
			log.Printf("Control REP: Recv failed (attempt %d/%d): %v", attempt, maxRetries, err)
			if attempt < maxRetries {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("failed to receive control response after %d attempts: %w", maxRetries, err)
		}

		// Parse response
		var response ControlResponse
		if err := json.Unmarshal(respData, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal control response: %w", err)
		}

		log.Printf("Control REP: Received %s (status=%s)", msg.Type, response.Status)
		return &response, nil
	}

	return nil, fmt.Errorf("control request failed after %d attempts", maxRetries)
}

// SendPTTStart notifies reflector that PTT has been pressed (via control socket with ACK)
func (c *VoiceClient) SendPTTStartWithAck(module, callsign, sessionID string) (*ControlResponse, error) {
	msg := VoiceMessage{
		Type:      "ptt_start",
		Module:    module,
		Callsign:  callsign,
		Source:    "web",
		SessionID: sessionID,
	}
	return c.SendControlRequest(msg)
}

// SendPTTStop notifies reflector that PTT has been released (via control socket with ACK)
func (c *VoiceClient) SendPTTStopWithAck(module, callsign string) (*ControlResponse, error) {
	msg := VoiceMessage{
		Type:     "ptt_stop",
		Module:   module,
		Callsign: callsign,
		Source:   "web",
	}
	return c.SendControlRequest(msg)
}

// SendPTTStart notifies reflector that PTT has been pressed (DEPRECATED: use SendPTTStartWithAck)

func (c *VoiceClient) SendPTTStart(module, callsign string) error {
	return c.Send(VoiceMessage{
		Type:     "ptt_start",
		Module:   module,
		Callsign: callsign,
		Source:   "web",
	})
}

// SendPTTStop notifies reflector that PTT has been released
func (c *VoiceClient) SendPTTStop(module, callsign string) error {
	return c.Send(VoiceMessage{
		Type:     "ptt_stop",
		Module:   module,
		Callsign: callsign,
		Source:   "web",
	})
}

// SendSessionStart notifies reflector that a voice session has started
func (c *VoiceClient) SendSessionStart(module, callsign, sessionID string) error {
	return c.Send(VoiceMessage{
		Type:      "voice_session_start",
		Module:    module,
		Callsign:  callsign,
		Source:    "web",
		SessionID: sessionID,
	})
}

// SendSessionStop notifies reflector that a voice session has ended
func (c *VoiceClient) SendSessionStop(callsign string) error {
	return c.Send(VoiceMessage{
		Type:     "voice_session_stop",
		Callsign: callsign,
	})
}

// OnMessage registers a handler for a specific message type
func (c *VoiceClient) OnMessage(msgType string, handler func(VoiceMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

// Listen starts listening for incoming voice messages from reflector
// This should be run in a goroutine
// Implements automatic reconnection on connection failures
func (c *VoiceClient) Listen() error {
	consecutiveErrors := 0
	maxConsecutiveErrors := 3

	for {
		c.mu.RLock()
		if !c.running {
			c.mu.RUnlock()
			return nil
		}
		c.mu.RUnlock()

		msg, err := c.sock.Recv()
		if err != nil {
			// Check if we're shutting down
			c.mu.RLock()
			running := c.running
			c.mu.RUnlock()

			if !running {
				return nil
			}

			consecutiveErrors++
			log.Printf("Voice NNG Recv error (%d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			// If we hit too many consecutive errors, attempt reconnection
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Printf("Voice NNG: Too many consecutive errors, attempting reconnection...")
				if err := c.attemptReconnect(); err != nil {
					log.Printf("Voice NNG: Reconnection failed: %v", err)
					time.Sleep(5 * time.Second) // Wait before trying again
				} else {
					log.Printf("Voice NNG: Reconnection successful")
					consecutiveErrors = 0
				}
			} else {
				time.Sleep(100 * time.Millisecond)
			}
			continue
		}

		// Successfully received message, reset error counter
		consecutiveErrors = 0

		var voiceMsg VoiceMessage
		if err := json.Unmarshal(msg, &voiceMsg); err != nil {
			log.Printf("Voice JSON Unmarshal error: %v", err)
			continue
		}

		// Call registered handler for this message type
		c.mu.RLock()
		handler, exists := c.handlers[voiceMsg.Type]
		c.mu.RUnlock()

		if exists {
			go handler(voiceMsg)
		} else {
			log.Printf("No handler for voice message type: %s", voiceMsg.Type)
		}
	}
}

// attemptReconnect closes and reopens the connection to urfd
func (c *VoiceClient) attemptReconnect() error {
	c.mu.Lock()

	if c.reconnecting {
		c.mu.Unlock()
		return fmt.Errorf("reconnection already in progress")
	}

	if !c.running {
		c.mu.Unlock()
		return fmt.Errorf("client is not running")
	}

	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	log.Printf("Voice NNG: Closing existing sockets for reconnection...")

	// Close the old sockets
	if err := c.sock.Close(); err != nil {
		log.Printf("Voice NNG: Warning: Failed to close old audio socket: %v", err)
	}
	if err := c.controlSock.Close(); err != nil {
		log.Printf("Voice NNG: Warning: Failed to close old control socket: %v", err)
	}

	// Create new audio socket (PAIR)
	sock, err := pair.NewSocket()
	if err != nil {
		return fmt.Errorf("failed to create new PAIR socket: %w", err)
	}

	// Set socket options for audio
	if err := sock.SetOption(mangos.OptionRecvDeadline, time.Duration(-1)); err != nil {
		sock.Close()
		return fmt.Errorf("failed to set recv deadline: %w", err)
	}

	// Create new control socket (REQ)
	controlSock, err := req.NewSocket()
	if err != nil {
		sock.Close()
		return fmt.Errorf("failed to create new REQ socket: %w", err)
	}

	// Set socket options for control
	if err := controlSock.SetOption(mangos.OptionSendDeadline, 2*time.Second); err != nil {
		sock.Close()
		controlSock.Close()
		return fmt.Errorf("failed to set control send deadline: %w", err)
	}
	if err := controlSock.SetOption(mangos.OptionRecvDeadline, 2*time.Second); err != nil {
		sock.Close()
		controlSock.Close()
		return fmt.Errorf("failed to set control recv deadline: %w", err)
	}

	// Attempt to dial audio socket with retries
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := sock.Dial(c.url); err != nil {
			log.Printf("Voice NNG: Audio dial attempt %d/%d failed: %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
			} else {
				sock.Close()
				controlSock.Close()
				return fmt.Errorf("failed to dial audio socket after %d attempts: %w", maxRetries, err)
			}
		} else {
			break // Success
		}
	}

	// Attempt to dial control socket with retries
	for i := 0; i < maxRetries; i++ {
		if err := controlSock.Dial(c.controlURL); err != nil {
			log.Printf("Voice NNG: Control dial attempt %d/%d failed: %v", i+1, maxRetries, err)
			if i < maxRetries-1 {
				time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
			} else {
				sock.Close()
				controlSock.Close()
				return fmt.Errorf("failed to dial control socket after %d attempts: %w", maxRetries, err)
			}
		} else {
			break // Success
		}
	}

	// Update sockets
	c.mu.Lock()
	c.sock = sock
	c.controlSock = controlSock
	c.mu.Unlock()

	log.Printf("Voice NNG: Reconnected to %s (audio) and %s (control)", c.url, c.controlURL)
	return nil
}

// IsConnected returns whether the client is currently connected
func (c *VoiceClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}
