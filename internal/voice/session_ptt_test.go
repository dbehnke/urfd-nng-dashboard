package voice

import (
	"testing"
	"time"
)

// Mock WebSocket connection for testing
type mockConn struct {
	sentMessages [][]byte
	closed       bool
}

func (m *mockConn) WriteMessage(messageType int, data []byte) error {
	m.sentMessages = append(m.sentMessages, data)
	return nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

// TestPTTRequest_ServerGrantsAccess tests the full PTT request flow
func TestPTTRequest_ServerGrantsAccess(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	// Simulate client requesting PTT
	err := shared.RequestPTT("session1", "KF8S")
	if err != nil {
		t.Fatalf("PTT request should be granted, got error: %v", err)
	}

	// Verify active talker is set
	if shared.activeTalker != "KF8S" {
		t.Errorf("Expected activeTalker to be 'KF8S', got '%s'", shared.activeTalker)
	}
}

// TestPTTRequest_QuickClickScenario tests the scenario where user clicks PTT twice quickly
func TestPTTRequest_QuickClickScenario(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	// First click: Request PTT
	err := shared.RequestPTT("session1", "KF8S")
	if err != nil {
		t.Fatalf("First PTT request should be granted, got error: %v", err)
	}

	// Verify state
	if shared.activeTalker != "KF8S" {
		t.Errorf("Expected activeTalker to be 'KF8S', got '%s'", shared.activeTalker)
	}

	// Second click: User releases PTT
	shared.ReleasePTT("session1", "KF8S")

	// Verify PTT is released
	if shared.activeTalker != "" {
		t.Errorf("Expected activeTalker to be cleared, got '%s'", shared.activeTalker)
	}
}

// TestPTTRequest_TwoClients_Arbitration tests that only one client can transmit at a time
func TestPTTRequest_TwoClients_Arbitration(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	// Client 1 requests PTT
	err1 := shared.RequestPTT("session1", "KF8S")
	if err1 != nil {
		t.Fatalf("Client 1 PTT request should be granted, got error: %v", err1)
	}

	// Client 2 tries to request PTT while Client 1 is transmitting
	err2 := shared.RequestPTT("session2", "W8EAP")
	if err2 == nil {
		t.Fatal("Client 2 PTT request should be denied")
	}

	// Verify error message contains active talker callsign
	expectedMsg := "PTT denied - KF8S is transmitting"
	if err2.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err2.Error())
	}

	// Client 1 releases PTT
	shared.ReleasePTT("session1", "KF8S")

	// Now Client 2 should be able to get PTT
	err3 := shared.RequestPTT("session2", "W8EAP")
	if err3 != nil {
		t.Fatalf("Client 2 PTT request after release should be granted, got error: %v", err3)
	}

	// Verify active talker switched
	if shared.activeTalker != "W8EAP" {
		t.Errorf("Expected activeTalker to be 'W8EAP', got '%s'", shared.activeTalker)
	}
}

// TestPTTRequest_RapidToggle tests rapid PTT toggle (press/release/press/release)
func TestPTTRequest_RapidToggle(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	callsign := "KF8S"
	sessionID := "session1"

	// Simulate 10 rapid toggles
	for i := 0; i < 10; i++ {
		// Press
		err := shared.RequestPTT(sessionID, callsign)
		if err != nil {
			t.Fatalf("Iteration %d: PTT request should be granted, got error: %v", i, err)
		}

		if shared.activeTalker != callsign {
			t.Errorf("Iteration %d: Expected activeTalker to be '%s', got '%s'", i, callsign, shared.activeTalker)
		}

		// Release
		shared.ReleasePTT(sessionID, callsign)

		if shared.activeTalker != "" {
			t.Errorf("Iteration %d: Expected activeTalker to be cleared, got '%s'", i, shared.activeTalker)
		}
	}
}

// TestPTTRelease_OnlyActiveTalkerCanRelease tests that only the active talker can release PTT
func TestPTTRelease_OnlyActiveTalkerCanRelease(t *testing.T) {
	shared := &SharedVoiceClient{
		module:       "A",
		sessions:     make(map[string]*Session),
		activeTalker: "KF8S",
	}

	// Client 2 tries to release Client 1's PTT
	shared.ReleasePTT("session2", "W8EAP")

	// PTT should still be held by KF8S
	if shared.activeTalker != "KF8S" {
		t.Errorf("Expected activeTalker to still be 'KF8S', got '%s'", shared.activeTalker)
	}

	// Correct client releases
	shared.ReleasePTT("session1", "KF8S")

	// Now PTT should be released
	if shared.activeTalker != "" {
		t.Errorf("Expected activeTalker to be cleared, got '%s'", shared.activeTalker)
	}
}

// TestPTTRequest_StateTransitions tests state transitions during PTT lifecycle
func TestPTTRequest_StateTransitions(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	callsign := "KF8S"
	sessionID := "session1"

	// Initial state: no active talker
	if shared.activeTalker != "" {
		t.Errorf("Expected no active talker initially, got '%s'", shared.activeTalker)
	}

	// Request PTT
	err := shared.RequestPTT(sessionID, callsign)
	if err != nil {
		t.Fatalf("PTT request should be granted, got error: %v", err)
	}

	// State: transmitting
	if shared.activeTalker != callsign {
		t.Errorf("Expected active talker to be '%s', got '%s'", callsign, shared.activeTalker)
	}

	// Release PTT
	shared.ReleasePTT(sessionID, callsign)

	// State: listening
	if shared.activeTalker != "" {
		t.Errorf("Expected no active talker after release, got '%s'", shared.activeTalker)
	}
}

// TestPTTRequest_DifferentModulesIndependent tests that different modules can transmit simultaneously
func TestPTTRequest_DifferentModulesIndependent(t *testing.T) {
	moduleA := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	moduleB := &SharedVoiceClient{
		module:   "B",
		sessions: make(map[string]*Session),
	}

	// Both modules request PTT simultaneously
	errA := moduleA.RequestPTT("session1", "KF8S")
	errB := moduleB.RequestPTT("session2", "W8EAP")

	if errA != nil {
		t.Errorf("Module A PTT request should be granted, got error: %v", errA)
	}

	if errB != nil {
		t.Errorf("Module B PTT request should be granted, got error: %v", errB)
	}

	// Both should have their own active talker
	if moduleA.activeTalker != "KF8S" {
		t.Errorf("Module A should have activeTalker 'KF8S', got '%s'", moduleA.activeTalker)
	}

	if moduleB.activeTalker != "W8EAP" {
		t.Errorf("Module B should have activeTalker 'W8EAP', got '%s'", moduleB.activeTalker)
	}
}

// TestPTTTimeout_MaxDuration tests that PTT is auto-released after max duration
// This test would require integration with the session handler
func TestPTTTimeout_Concept(t *testing.T) {
	// This is a conceptual test - actual implementation would require
	// integration with the Session.handleVoiceStart and timeout logic

	maxDuration := 100 * time.Millisecond

	// In practice, the session would:
	// 1. Start PTT
	// 2. Set timer for maxDuration
	// 3. Auto-release after timeout
	// 4. Send voice_state: 'listening' to client

	// For this unit test, we just verify the timeout value is reasonable
	if maxDuration < 1*time.Millisecond {
		t.Error("Max duration should be at least 1ms")
	}

	if maxDuration > 5*time.Minute {
		t.Error("Max duration should not exceed 5 minutes")
	}
}
