package voice

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
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
	Type     string    `json:"type"`               // "audio_data", "ptt_start", "ptt_stop", "state"
	Module   string    `json:"module,omitempty"`   // Module identifier (A, B, C, D)
	Callsign string    `json:"callsign,omitempty"` // User callsign
	Source   string    `json:"source,omitempty"`   // "web" for web clients
	Opus     ByteSlice `json:"opus,omitempty"`     // Opus encoded audio data (marshaled as array)
	State    string    `json:"state,omitempty"`    // "listening", "transmitting", "rx_busy"
}

// VoiceClient manages NNG PAIR connection to reflector voice endpoint
type VoiceClient struct {
	url      string
	sock     mangos.Socket
	mu       sync.RWMutex
	handlers map[string]func(VoiceMessage)
	running  bool
}

// NewVoiceClient creates a new NNG PAIR client for voice communication
func NewVoiceClient(url string) (*VoiceClient, error) {
	sock, err := pair.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create PAIR socket: %w", err)
	}

	// Set socket options - negative timeout means wait forever
	if err := sock.SetOption(mangos.OptionRecvDeadline, time.Duration(-1)); err != nil {
		sock.Close()
		return nil, fmt.Errorf("failed to set recv deadline: %w", err)
	}

	client := &VoiceClient{
		url:      url,
		sock:     sock,
		handlers: make(map[string]func(VoiceMessage)),
		running:  false,
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

	if err := c.sock.Dial(c.url); err != nil {
		return fmt.Errorf("failed to dial %s: %w", c.url, err)
	}

	c.running = true
	log.Printf("Voice client connected to %s", c.url)
	return nil
}

// Disconnect closes the NNG connection
func (c *VoiceClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.running = false
	if err := c.sock.Close(); err != nil {
		return fmt.Errorf("failed to close socket: %w", err)
	}

	log.Printf("Voice client disconnected from %s", c.url)
	return nil
}

// Send sends a voice message to the reflector
func (c *VoiceClient) Send(msg VoiceMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.running {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := c.sock.Send(data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendAudioData sends Opus audio data to the reflector
func (c *VoiceClient) SendAudioData(module, callsign string, opusData []byte) error {
	return c.Send(VoiceMessage{
		Type:     "audio_data",
		Module:   module,
		Callsign: callsign,
		Source:   "web",
		Opus:     opusData,
	})
}

// SendPTTStart notifies reflector that PTT has been pressed
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
func (c *VoiceClient) SendSessionStart(module, callsign string) error {
	return c.Send(VoiceMessage{
		Type:     "voice_session_start",
		Module:   module,
		Callsign: callsign,
		Source:   "web",
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
func (c *VoiceClient) Listen() error {
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

			log.Printf("Voice NNG Recv error: %v", err)
			continue
		}

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

// IsConnected returns whether the client is currently connected
func (c *VoiceClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}
