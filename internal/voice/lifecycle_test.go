package voice

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3/protocol/pair"
)

// testVoiceClient wraps VoiceClient for testing and captures sent messages
type testVoiceClient struct {
	*VoiceClient
	mu           sync.Mutex
	sentMessages []VoiceMessage
}

func newTestVoiceClient() *testVoiceClient {
	sock, _ := pair.NewSocket()
	vc := &VoiceClient{
		url:      "mock://test",
		sock:     sock,
		handlers: make(map[string]func(VoiceMessage)),
		running:  true,
	}

	return &testVoiceClient{
		VoiceClient:  vc,
		sentMessages: make([]VoiceMessage, 0),
	}
}

// Override Send to capture messages instead of sending over network
func (t *testVoiceClient) Send(msg VoiceMessage) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sentMessages = append(t.sentMessages, msg)
	return nil
}

func (t *testVoiceClient) GetSentMessages() []VoiceMessage {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]VoiceMessage{}, t.sentMessages...)
}

func (t *testVoiceClient) ClearMessages() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sentMessages = nil
}

// Helper to create a SharedVoiceClient with test client
func newTestSharedClient(module string) (*SharedVoiceClient, *testVoiceClient) {
	testClient := newTestVoiceClient()
	sharedClient := &SharedVoiceClient{
		client:   testClient.VoiceClient, // Use the underlying VoiceClient
		sessions: make(map[string]*Session),
		module:   module,
	}
	return sharedClient, testClient
}

// TestHandleVoiceStart_SendsSessionStartMessage tests that voice_start sends session_start to urfd
func TestHandleVoiceStart_SendsSessionStartMessage(t *testing.T) {
	sharedClient, testClient := newTestSharedClient("A")

	// Create test pool
	pool := &VoiceClientPool{
		clients: map[string]*SharedVoiceClient{
			"A": sharedClient,
		},
	}

	// Create session
	session := &Session{
		ID:         "test-session-1",
		ClientPool: pool,
		Config:     &SessionConfig{},
	}

	// Handle voice_start message
	msg := WSMessage{
		Type:     "voice_start",
		Module:   "A",
		Callsign: "KF8S",
	}

	err := session.handleVoiceStart(msg)
	if err != nil {
		t.Fatalf("handleVoiceStart failed: %v", err)
	}

	// Check that session was registered
	if session.Callsign != "KF8S" {
		t.Errorf("Expected callsign KF8S, got %s", session.Callsign)
	}
	if session.Module != "A" {
		t.Errorf("Expected module A, got %s", session.Module)
	}

	// Give it a moment for async operations
	time.Sleep(10 * time.Millisecond)

	// Check that voice_session_start message was sent to urfd
	messages := testClient.GetSentMessages()
	found := false
	for _, m := range messages {
		if m.Type == "voice_session_start" {
			found = true
			if m.Module != "A" {
				t.Errorf("Expected module A, got %s", m.Module)
			}
			if m.Callsign != "KF8S" {
				t.Errorf("Expected callsign KF8S, got %s", m.Callsign)
			}
			if m.Source != "web" {
				t.Errorf("Expected source web, got %s", m.Source)
			}
		}
	}

	if !found {
		t.Errorf("Expected voice_session_start message to be sent to urfd, but it was not found. Messages: %+v", messages)
	}
}

// TestHandleVoiceStop_SendsSessionStopMessage tests that voice_stop sends session_stop to urfd
func TestHandleVoiceStop_SendsSessionStopMessage(t *testing.T) {
	sharedClient, testClient := newTestSharedClient("A")

	// Create test pool
	pool := &VoiceClientPool{
		clients: map[string]*SharedVoiceClient{
			"A": sharedClient,
		},
	}

	// Create session with module/callsign already set
	session := &Session{
		ID:           "test-session-1",
		Callsign:     "KF8S",
		Module:       "A",
		State:        StateListening,
		SharedClient: sharedClient,
		ClientPool:   pool,
		Config:       &SessionConfig{},
	}

	// Register session
	sharedClient.RegisterSession(session)

	// Clear any previous messages
	testClient.ClearMessages()

	// Handle voice_stop
	err := session.handleVoiceStop()
	if err != nil {
		t.Fatalf("handleVoiceStop failed: %v", err)
	}

	// Give it a moment for async operations
	time.Sleep(10 * time.Millisecond)

	// Check that voice_session_stop message was sent to urfd
	messages := testClient.GetSentMessages()
	found := false
	for _, m := range messages {
		if m.Type == "voice_session_stop" {
			found = true
			if m.Callsign != "KF8S" {
				t.Errorf("Expected callsign KF8S, got %s", m.Callsign)
			}
		}
	}

	if !found {
		t.Errorf("Expected voice_session_stop message to be sent to urfd, but it was not found. Messages: %+v", messages)
	}
}

// TestSessionStop_OnDisconnect_SendsMessage tests that Stop() sends session_stop before cleanup
func TestSessionStop_OnDisconnect_SendsMessage(t *testing.T) {
	sharedClient, testClient := newTestSharedClient("A")

	// Create test pool
	pool := &VoiceClientPool{
		clients: map[string]*SharedVoiceClient{
			"A": sharedClient,
		},
	}

	// Create session with module/callsign already set
	session := &Session{
		ID:           "test-session-1",
		Callsign:     "W8EAP",
		Module:       "A",
		State:        StateListening,
		SharedClient: sharedClient,
		ClientPool:   pool,
		Config:       &SessionConfig{},
	}

	// Register session
	sharedClient.RegisterSession(session)

	// Clear any previous messages
	testClient.ClearMessages()

	// Simulate disconnect - Stop() should be called
	err := session.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Give it a moment for async operations
	time.Sleep(10 * time.Millisecond)

	// Check that voice_session_stop message was sent to urfd
	messages := testClient.GetSentMessages()
	found := false
	for _, m := range messages {
		if m.Type == "voice_session_stop" {
			found = true
			if m.Callsign != "W8EAP" {
				t.Errorf("Expected callsign W8EAP, got %s", m.Callsign)
			}
		}
	}

	if !found {
		t.Errorf("Expected voice_session_stop message to be sent on disconnect, but it was not found. Messages: %+v", messages)
	}
}

// TestReconnectToUrfd_ResendsActiveSessions tests that reconnection resyncs sessions
func TestReconnectToUrfd_ResendsActiveSessions(t *testing.T) {
	sharedClient, testClient := newTestSharedClient("A")

	// Create two active sessions
	session1 := &Session{
		ID:       "session-1",
		Callsign: "KF8S",
		Module:   "A",
		State:    StateListening,
	}
	session2 := &Session{
		ID:       "session-2",
		Callsign: "W8EAP",
		Module:   "A",
		State:    StateTransmitting,
	}

	sharedClient.RegisterSession(session1)
	sharedClient.RegisterSession(session2)

	// Clear any previous messages
	testClient.ClearMessages()

	// Simulate urfd reconnection
	err := sharedClient.OnUrfdReconnect()
	if err != nil {
		t.Fatalf("OnUrfdReconnect failed: %v", err)
	}

	// Give it a moment for async operations
	time.Sleep(10 * time.Millisecond)

	// Check that voice_session_start messages were sent for both sessions
	messages := testClient.GetSentMessages()
	sessionStarts := 0
	callsignsSeen := make(map[string]bool)

	for _, m := range messages {
		if m.Type == "voice_session_start" {
			sessionStarts++
			callsignsSeen[m.Callsign] = true

			if m.Module != "A" {
				t.Errorf("Expected module A, got %s", m.Module)
			}
			if m.Source != "web" {
				t.Errorf("Expected source web, got %s", m.Source)
			}
		}
	}

	if sessionStarts != 2 {
		t.Errorf("Expected 2 voice_session_start messages on reconnect, got %d. Messages: %+v", sessionStarts, messages)
	}

	if !callsignsSeen["KF8S"] {
		t.Errorf("Expected voice_session_start for KF8S, but it was not found")
	}
	if !callsignsSeen["W8EAP"] {
		t.Errorf("Expected voice_session_start for W8EAP, but it was not found")
	}
}

// TestSessionStartMessageFormat tests the exact format of session_start message
func TestSessionStartMessageFormat(t *testing.T) {
	msg := VoiceMessage{
		Type:     "voice_session_start",
		Module:   "A",
		Callsign: "KF8S",
		Source:   "web",
	}

	// Marshal to JSON to verify format
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Unmarshal back to verify
	var parsed VoiceMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.Type != "voice_session_start" {
		t.Errorf("Expected type voice_session_start, got %s", parsed.Type)
	}
	if parsed.Module != "A" {
		t.Errorf("Expected module A, got %s", parsed.Module)
	}
	if parsed.Callsign != "KF8S" {
		t.Errorf("Expected callsign KF8S, got %s", parsed.Callsign)
	}
	if parsed.Source != "web" {
		t.Errorf("Expected source web, got %s", parsed.Source)
	}
}

// TestSessionStopMessageFormat tests the exact format of session_stop message
func TestSessionStopMessageFormat(t *testing.T) {
	msg := VoiceMessage{
		Type:     "voice_session_stop",
		Callsign: "KF8S",
	}

	// Marshal to JSON to verify format
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Unmarshal back to verify
	var parsed VoiceMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.Type != "voice_session_stop" {
		t.Errorf("Expected type voice_session_stop, got %s", parsed.Type)
	}
	if parsed.Callsign != "KF8S" {
		t.Errorf("Expected callsign KF8S, got %s", parsed.Callsign)
	}
}
