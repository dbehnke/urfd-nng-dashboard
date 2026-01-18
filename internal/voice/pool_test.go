package voice

import (
	"fmt"
	"testing"
)

// TestRequestPTT_FirstCaller_Granted tests that the first caller is granted PTT when no one is transmitting
func TestRequestPTT_FirstCaller_Granted(t *testing.T) {
	// Create a SharedVoiceClient (without actual NNG connection)
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	// No active talker initially
	// Request PTT for session1 with callsign KF8S
	err := shared.RequestPTT("session1", "KF8S")

	// Should be granted (no error)
	if err != nil {
		t.Errorf("Expected PTT to be granted, but got error: %v", err)
	}

	// Check that activeTalker is set to "KF8S"
	if shared.activeTalker != "KF8S" {
		t.Errorf("Expected activeTalker to be 'KF8S', got '%s'", shared.activeTalker)
	}
}

// TestRequestPTT_SecondCaller_Denied tests that the second caller is denied when someone is already transmitting
func TestRequestPTT_SecondCaller_Denied(t *testing.T) {
	// Create a SharedVoiceClient
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	// First caller gets PTT
	shared.activeTalker = "KF8S"

	// Second caller tries to get PTT
	err := shared.RequestPTT("session2", "W8EAP")

	// Should be denied (error returned)
	if err == nil {
		t.Error("Expected PTT to be denied, but it was granted")
	}

	// Error message should mention who is transmitting
	expectedMsg := "PTT denied - KF8S is transmitting"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// activeTalker should still be "KF8S"
	if shared.activeTalker != "KF8S" {
		t.Errorf("Expected activeTalker to still be 'KF8S', got '%s'", shared.activeTalker)
	}
}

// TestReleasePTT_ClearsActiveTalker tests that releasing PTT clears the active talker
func TestReleasePTT_ClearsActiveTalker(t *testing.T) {
	// Create a SharedVoiceClient
	shared := &SharedVoiceClient{
		module:       "A",
		sessions:     make(map[string]*Session),
		activeTalker: "KF8S",
	}

	// Release PTT
	shared.ReleasePTT("session1", "KF8S")

	// activeTalker should be cleared
	if shared.activeTalker != "" {
		t.Errorf("Expected activeTalker to be empty, got '%s'", shared.activeTalker)
	}
}

// TestReleasePTT_WrongCallsign_NoEffect tests that only the actual talker can release PTT
func TestReleasePTT_WrongCallsign_NoEffect(t *testing.T) {
	// Create a SharedVoiceClient
	shared := &SharedVoiceClient{
		module:       "A",
		sessions:     make(map[string]*Session),
		activeTalker: "KF8S",
	}

	// Different caller tries to release PTT
	shared.ReleasePTT("session2", "W8EAP")

	// activeTalker should still be "KF8S" (unchanged)
	if shared.activeTalker != "KF8S" {
		t.Errorf("Expected activeTalker to still be 'KF8S', got '%s'", shared.activeTalker)
	}
}

// TestRegisterSession_AddsToMap tests that RegisterSession adds a session to the map
func TestRegisterSession_AddsToMap(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	session := &Session{
		ID:       "session1",
		Callsign: "KF8S",
		Module:   "A",
	}

	shared.RegisterSession(session)

	// Check that session was added
	shared.sessionsMu.RLock()
	storedSession, exists := shared.sessions["session1"]
	shared.sessionsMu.RUnlock()

	if !exists {
		t.Error("Expected session to be registered, but it was not found")
	}

	if storedSession != session {
		t.Error("Expected stored session to match the registered session")
	}
}

// TestUnregisterSession_RemovesFromMap tests that UnregisterSession removes a session from the map
func TestUnregisterSession_RemovesFromMap(t *testing.T) {
	session := &Session{
		ID:       "session1",
		Callsign: "KF8S",
		Module:   "A",
	}

	shared := &SharedVoiceClient{
		module: "A",
		sessions: map[string]*Session{
			"session1": session,
		},
	}

	shared.UnregisterSession("session1")

	// Check that session was removed
	shared.sessionsMu.RLock()
	_, exists := shared.sessions["session1"]
	shared.sessionsMu.RUnlock()

	if exists {
		t.Error("Expected session to be unregistered, but it was still found")
	}
}

// TestUnregisterSession_ClearsActiveTalkerIfMatch tests that unregistering clears PTT if the session was transmitting
func TestUnregisterSession_ClearsActiveTalkerIfMatch(t *testing.T) {
	session := &Session{
		ID:       "session1",
		Callsign: "KF8S",
		Module:   "A",
	}

	shared := &SharedVoiceClient{
		module:       "A",
		activeTalker: "KF8S",
		sessions: map[string]*Session{
			"session1": session,
		},
	}

	shared.UnregisterSession("session1")

	// activeTalker should be cleared
	shared.activeTalkerMu.RLock()
	talker := shared.activeTalker
	shared.activeTalkerMu.RUnlock()

	if talker != "" {
		t.Errorf("Expected activeTalker to be cleared, got '%s'", talker)
	}

	// Session should also be removed
	shared.sessionsMu.RLock()
	_, exists := shared.sessions["session1"]
	shared.sessionsMu.RUnlock()

	if exists {
		t.Error("Expected session to be unregistered")
	}
}

// TestRegisterSession_Concurrent_ThreadSafe tests that concurrent registration is thread-safe
func TestRegisterSession_Concurrent_ThreadSafe(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	// Register 100 sessions concurrently
	const numSessions = 100
	done := make(chan bool, numSessions)

	for i := 0; i < numSessions; i++ {
		go func(id int) {
			sessionID := fmt.Sprintf("session%d", id)
			session := &Session{
				ID:       sessionID,
				Callsign: fmt.Sprintf("CALL%d", id),
				Module:   "A",
			}
			shared.RegisterSession(session)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numSessions; i++ {
		<-done
	}

	// Check that all sessions were registered
	shared.sessionsMu.RLock()
	count := len(shared.sessions)
	shared.sessionsMu.RUnlock()

	if count != numSessions {
		t.Errorf("Expected %d sessions to be registered, got %d", numSessions, count)
	}
}

// TestBroadcastAudioToSessions_SameModule_ReceivesAudio tests that sessions on the same module receive audio
func TestBroadcastAudioToSessions_SameModule_ReceivesAudio(t *testing.T) {
	// Create mock sessions
	session1 := &Session{
		ID:       "session1",
		Callsign: "KF8S",
		Module:   "A",
	}

	session2 := &Session{
		ID:       "session2",
		Callsign: "W8EAP",
		Module:   "A",
	}

	session3 := &Session{
		ID:       "session3",
		Callsign: "N0CALL",
		Module:   "A",
	}

	session4 := &Session{
		ID:       "session4",
		Callsign: "K9ABC",
		Module:   "D", // Different module
	}

	shared := &SharedVoiceClient{
		module: "A",
		sessions: map[string]*Session{
			"session1": session1,
			"session2": session2,
			"session3": session3,
			"session4": session4,
		},
	}

	opusData := []byte{1, 2, 3, 4, 5}

	// This should broadcast to session2 and session3 (same module A), but not session4 (module D) or session1 (sender)
	shared.BroadcastPeerAudio(opusData, "session1", "KF8S", "A")

	// The test passes if no panic occurs
	// In a real implementation, we would mock SendAudioFromPeer to verify it was called for the correct sessions
}

// TestBroadcastAudioToSessions_ExcludesSender tests that the sender doesn't receive their own audio
func TestBroadcastAudioToSessions_ExcludesSender(t *testing.T) {
	session1 := &Session{
		ID:       "session1",
		Callsign: "KF8S",
		Module:   "A",
	}

	session2 := &Session{
		ID:       "session2",
		Callsign: "W8EAP",
		Module:   "A",
	}

	shared := &SharedVoiceClient{
		module: "A",
		sessions: map[string]*Session{
			"session1": session1,
			"session2": session2,
		},
	}

	opusData := []byte{1, 2, 3, 4, 5}

	// Broadcast from session1
	shared.BroadcastPeerAudio(opusData, "session1", "KF8S", "A")

	// Test passes if no panic - actual verification would require mocking SendAudioFromPeer
}

// TestBroadcastAudioToSessions_EmptySessions_NoError tests that broadcasting with no sessions doesn't error
func TestBroadcastAudioToSessions_EmptySessions_NoError(t *testing.T) {
	shared := &SharedVoiceClient{
		module:   "A",
		sessions: make(map[string]*Session),
	}

	opusData := []byte{1, 2, 3, 4, 5}

	// Should not panic or error with empty sessions
	shared.BroadcastPeerAudio(opusData, "session1", "KF8S", "A")
}
