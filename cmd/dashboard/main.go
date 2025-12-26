package main

import (
	"encoding/json"
	"flag"
	"log"
	"sync"
	"time"

	"github.com/dbehnke/urfd-nng-dashboard/internal/assets"
	"github.com/dbehnke/urfd-nng-dashboard/internal/nng"
	"github.com/dbehnke/urfd-nng-dashboard/internal/server"
	"github.com/dbehnke/urfd-nng-dashboard/internal/store"
)

var (
	nngURL = flag.String("nng-url", "tcp://127.0.0.1:5555", "URFD NNG Publisher URL")
	dbPath = flag.String("db", "data/dashboard.db", "Path to SQLite database")
	listen = flag.String("listen", ":8080", "HTTP listen address")
)

func main() {
	flag.Parse()

	// 1. Initialize Store
	s, err := store.NewStore(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// 2. Initialize Hub
	hub := server.NewHub()
	go hub.Run()

	// State retention & Session management
	var (
		lastState nng.Event
		stateMu   sync.RWMutex
		sessions  = make(map[string]*ActiveSession)
		sessMu    sync.Mutex
	)

	// Session cleanup and persistence ticker
	go func() {
		for range time.Tick(2 * time.Second) {
			sessMu.Lock()
			now := time.Now()
			for key, sess := range sessions {
				if now.Sub(sess.LastSeen) > 3*time.Second {
					// Session ended!
					duration := now.Sub(sess.StartTime).Seconds()

					// Update DB
					if err := s.DB.Model(&store.Hearing{}).Where("id = ?", sess.ID).Update("duration", duration).Error; err != nil {
						log.Printf("Failed to update session duration: %v", err)
					}

					// Broadcast "ended" event
					hub.BroadcastJSON(nng.Event{
						Type:     "hearing",
						Status:   "ended",
						ID:       sess.ID,
						Duration: duration,
					})

					log.Printf("Session %d ended (duration: %.1fs)", sess.ID, duration)
					delete(sessions, key)
				}
			}
			sessMu.Unlock()
		}
	}()

	// 3. Initialize NNG Subscriber
	sub, err := nng.NewSubscriber(*nngURL)
	if err != nil {
		log.Fatalf("Failed to connect to NNG: %v", err)
	}

	// 4. Listen for events
	go sub.Listen(func(ev nng.Event) {
		// Log hearing events to DB (Session-aware)
		if ev.Type == "hearing" {
			sessKey := ev.My + ":" + ev.Module
			sessMu.Lock()
			sess, exists := sessions[sessKey]
			if !exists {
				h := store.Hearing{
					My:       ev.My,
					Ur:       ev.Ur,
					Rpt1:     ev.Rpt1,
					Rpt2:     ev.Rpt2,
					Module:   ev.Module,
					Protocol: ev.Protocol,
				}
				if err := s.DB.Create(&h).Error; err != nil {
					log.Printf("Failed to save hearing: %v", err)
				}
				sess = &ActiveSession{
					ID:        h.ID,
					Protocol:  h.Protocol,
					StartTime: h.CreatedAt,
					LastSeen:  time.Now(),
				}
				sessions[sessKey] = sess
			} else {
				sess.LastSeen = time.Now()
			}
			ev.ID = sess.ID
			ev.Protocol = sess.Protocol
			ev.CreatedAt = sess.StartTime
			ev.Status = "active"
			sessMu.Unlock()
		}

		if ev.Type == "state" {
			stateMu.Lock()
			lastState = ev
			stateMu.Unlock()
		}

		// Broadcast all events to Websocket
		hub.BroadcastJSON(ev)
	})

	// 5. Start HTTP Server
	srv := server.NewServer(hub, assets.GetAssets())
	srv.OnConnect = func(client *server.Client) {
		stateMu.RLock()
		defer stateMu.RUnlock()
		if lastState.Type != "" {
			data, _ := json.Marshal(lastState)
			client.Send <- data
		}
	}

	if err := srv.Start(*listen); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

type ActiveSession struct {
	ID        uint
	Protocol  string
	StartTime time.Time
	LastSeen  time.Time
}
