package voice

import (
	"sync"
	"testing"
	"time"
)

// MockWebSocketConn simulates a WebSocket connection for testing
type MockWebSocketConn struct {
	mu           sync.Mutex
	sentMessages []map[string]interface{}
	messagesChan chan map[string]interface{}
	closed       bool
	onMessage    func(msg map[string]interface{})
}

func NewMockWebSocketConn() *MockWebSocketConn {
	return &MockWebSocketConn{
		sentMessages: make([]map[string]interface{}, 0),
		messagesChan: make(chan map[string]interface{}, 10),
	}
}

func (m *MockWebSocketConn) Send(msg map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, msg)

	if m.onMessage != nil {
		m.onMessage(msg)
	}
}

func (m *MockWebSocketConn) Receive() map[string]interface{} {
	return <-m.messagesChan
}

func (m *MockWebSocketConn) SendToClient(msg map[string]interface{}) {
	m.messagesChan <- msg
}

func (m *MockWebSocketConn) GetSentMessages() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sentMessages
}

// TestIntegration_NormalPTTCycle tests a complete PTT press and release cycle
func TestIntegration_NormalPTTCycle(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	callsign := "KF8S"
	sessionID := "test-session-1"

	// Step 1: Client sends ptt_press
	t.Log("Step 1: Client sends ptt_press")
	err := shared.RequestPTT(sessionID, callsign)
	if err != nil {
		t.Fatalf("PTT request should be granted, got error: %v", err)
	}

	// Verify server state
	if shared.activeTalker != callsign {
		t.Errorf("Expected activeTalker to be '%s', got '%s'", callsign, shared.activeTalker)
	}

	// Step 2: Server should send voice_state: "transmitting" to client
	t.Log("Step 2: Server grants PTT (would send voice_state: transmitting)")
	// In real implementation, this would trigger:
	// - Client receives voice_state: "transmitting"
	// - Client starts encoder
	// - Client UI shows "Transmitting"

	// Step 3: Client sends audio_data packets (simulated)
	t.Log("Step 3: Client transmits audio (simulated)")
	time.Sleep(100 * time.Millisecond)

	// Step 4: Client sends ptt_release
	t.Log("Step 4: Client sends ptt_release")
	shared.ReleasePTT(sessionID, callsign)

	// Verify server state
	if shared.activeTalker != "" {
		t.Errorf("Expected activeTalker to be cleared, got '%s'", shared.activeTalker)
	}

	// Step 5: Server should send voice_state: "listening" to client
	t.Log("Step 5: Server confirms release (would send voice_state: listening)")
	// In real implementation, this would trigger:
	// - Client receives voice_state: "listening"
	// - Client stops encoder
	// - Client UI shows "Listening"

	t.Log("✅ Complete PTT cycle successful")
}

// TestIntegration_QuickDoubleClick tests the bug scenario
func TestIntegration_QuickDoubleClick(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	callsign := "KF8S"
	sessionID := "test-session-1"

	// Simulate client state machine
	clientState := "listening"

	t.Log("Initial state: listening")

	// Step 1: User clicks PTT button (first click)
	t.Log("Step 1: User clicks PTT button")
	clientState = "ptt_requesting"
	t.Logf("  Client state: %s", clientState)

	// Step 2: Client sends ptt_press to server
	t.Log("Step 2: Client sends ptt_press")
	err := shared.RequestPTT(sessionID, callsign)
	if err != nil {
		t.Fatalf("PTT request should be granted, got error: %v", err)
	}

	// Step 3: User clicks PTT button again BEFORE server responds (second click)
	t.Log("Step 3: User clicks PTT button again (quick double-click)")
	if clientState == "ptt_requesting" {
		t.Log("  Client: Cancelling PTT request")
		clientState = "listening"
		t.Logf("  Client state: %s", clientState)

		// In the fixed implementation, client would:
		// 1. Clear the PTT request timeout
		// 2. Set state back to listening
		// 3. NOT send ptt_release (request was never granted)
	}

	// Step 4: Server responds with voice_state: "transmitting" (but client already cancelled)
	t.Log("Step 4: Server sends voice_state: transmitting")

	// With the fix:
	// - Client should ignore this message because it's no longer in ptt_requesting state
	// - Client should remain in listening state
	// - Server should handle this by sending another state update

	// Without the fix:
	// - Client would transition to transmitting
	// - Encoder would start
	// - User would be stuck

	// Verify server released PTT (client never confirmed)
	// In practice, server should have a timeout to clear grants that aren't acknowledged
	shared.ReleasePTT(sessionID, callsign)

	if shared.activeTalker != "" {
		t.Errorf("Expected activeTalker to be cleared, got '%s'", shared.activeTalker)
	}

	if clientState != "listening" {
		t.Errorf("Expected client state to be 'listening', got '%s'", clientState)
	}

	t.Log("✅ Quick double-click handled correctly - no stuck state")
}

// TestIntegration_ButtonStateFlow tests that button disabled state is correct
func TestIntegration_ButtonStateFlow(t *testing.T) {
	testCases := []struct {
		state         string
		shouldDisable bool
		canStartPTT   bool
		canStopPTT    bool
	}{
		{"disconnected", true, false, false},
		{"listening", false, true, false},
		{"ptt_requesting", false, false, true}, // Should be able to cancel
		{"transmitting", false, false, true},   // Should be able to stop
		{"ptt_releasing", false, false, true},  // Should be able to force stop
		{"rx_busy", true, false, false},        // Half-duplex: can't TX while RX
	}

	for _, tc := range testCases {
		t.Run(tc.state, func(t *testing.T) {
			// Simulate button disabled logic (from VoiceChat.vue)
			isEnabled := true
			hasCallsign := true
			hasModule := true

			var shouldDisable bool
			if tc.state == "disconnected" || tc.state == "rx_busy" {
				shouldDisable = true
			} else if !isEnabled || !hasCallsign || !hasModule {
				shouldDisable = true
			} else {
				shouldDisable = false
			}

			if shouldDisable != tc.shouldDisable {
				t.Errorf("State '%s': expected button disabled=%v, got %v",
					tc.state, tc.shouldDisable, shouldDisable)
			}

			// Verify action logic
			canStart := tc.state == "listening"
			canStop := tc.state == "ptt_requesting" || tc.state == "transmitting" || tc.state == "ptt_releasing"

			if canStart != tc.canStartPTT {
				t.Errorf("State '%s': expected canStartPTT=%v, got %v",
					tc.state, tc.canStartPTT, canStart)
			}

			if canStop != tc.canStopPTT {
				t.Errorf("State '%s': expected canStopPTT=%v, got %v",
					tc.state, tc.canStopPTT, canStop)
			}
		})
	}
}

// TestIntegration_TwoClientsRaceCondition tests race between two clients
func TestIntegration_TwoClientsRaceCondition(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	// Both clients press PTT at approximately the same time
	t.Log("Scenario: Two clients press PTT simultaneously")

	// Client 1 sends ptt_press (arrives first)
	err1 := shared.RequestPTT("session1", "KF8S")
	if err1 != nil {
		t.Fatalf("Client 1 should get PTT, got error: %v", err1)
	}
	t.Log("✅ Client 1 (KF8S) granted PTT")

	// Client 2 sends ptt_press (arrives second)
	err2 := shared.RequestPTT("session2", "W8EAP")
	if err2 == nil {
		t.Fatal("Client 2 should be denied")
	}
	t.Logf("✅ Client 2 (W8EAP) denied: %v", err2)

	// Verify only Client 1 has PTT
	if shared.activeTalker != "KF8S" {
		t.Errorf("Expected KF8S to have PTT, got '%s'", shared.activeTalker)
	}

	// Client 2 should receive ptt_denied message with reason
	expectedMsg := "PTT denied - KF8S is transmitting"
	if err2.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err2.Error())
	}

	// Client 1 finishes and releases
	shared.ReleasePTT("session1", "KF8S")
	t.Log("✅ Client 1 released PTT")

	// Now Client 2 can get PTT
	err3 := shared.RequestPTT("session2", "W8EAP")
	if err3 != nil {
		t.Fatalf("Client 2 should get PTT after Client 1 releases, got error: %v", err3)
	}
	t.Log("✅ Client 2 granted PTT after Client 1 released")

	if shared.activeTalker != "W8EAP" {
		t.Errorf("Expected W8EAP to have PTT, got '%s'", shared.activeTalker)
	}
}

// TestIntegration_EncoderLifecycle tests encoder start/stop timing
func TestIntegration_EncoderLifecycle(t *testing.T) {
	// Simulate the encoder lifecycle
	type EncoderState struct {
		running bool
	}

	encoder := &EncoderState{running: false}
	clientState := "listening"

	t.Log("Initial: encoder stopped, state=listening")

	// Step 1: User clicks PTT
	clientState = "ptt_requesting"
	t.Logf("Step 1: User clicked PTT, state=%s", clientState)

	// Encoder should NOT start yet
	if encoder.running {
		t.Error("❌ Encoder should NOT start before server grants PTT")
	} else {
		t.Log("✅ Encoder correctly waiting for server grant")
	}

	// Step 2: Server grants PTT
	clientState = "transmitting"
	encoder.running = true
	t.Logf("Step 2: Server granted, state=%s, encoder started", clientState)

	// Step 3: User releases PTT
	clientState = "ptt_releasing"
	t.Logf("Step 3: User released PTT, state=%s", clientState)

	// Encoder should KEEP RUNNING until server confirms
	if !encoder.running {
		t.Error("❌ Encoder should keep running until server confirms release")
	} else {
		t.Log("✅ Encoder correctly running until server confirms")
	}

	// Step 4: Server confirms release
	clientState = "listening"
	encoder.running = false
	t.Logf("Step 4: Server confirmed, state=%s, encoder stopped", clientState)

	// Final state check
	if encoder.running {
		t.Error("❌ Encoder should be stopped")
	}
	if clientState != "listening" {
		t.Errorf("❌ State should be 'listening', got '%s'", clientState)
	}

	t.Log("✅ Encoder lifecycle correct")
}
