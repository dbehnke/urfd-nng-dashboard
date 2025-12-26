package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestHub(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	NewServer(hub, nil)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	}))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Test Broadcast
	msg := map[string]string{"type": "test"}
	hub.BroadcastJSON(msg)

	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(p, &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if resp["type"] != "test" {
		t.Errorf("Expected type test, got %s", resp["type"])
	}
}
