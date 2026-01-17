package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dbehnke/urfd-nng-dashboard/internal/assets"
	"github.com/dbehnke/urfd-nng-dashboard/internal/callbook"
	"github.com/dbehnke/urfd-nng-dashboard/internal/config"
	"github.com/dbehnke/urfd-nng-dashboard/internal/logger"
	"github.com/dbehnke/urfd-nng-dashboard/internal/nng"
	"github.com/dbehnke/urfd-nng-dashboard/internal/server"
	"github.com/dbehnke/urfd-nng-dashboard/internal/store"
	"github.com/dbehnke/urfd-nng-dashboard/internal/voice"
)

var (
	// ldflags
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// 3. Load Config
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	// 2. Initialize Logger
	logCfg := logger.Config{
		Level:      cfg.Logging.Level,
		FilePath:   cfg.Logging.FilePath,
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAgeDays: cfg.Logging.MaxAgeDays,
		Compress:   cfg.Logging.Compress,
		Console:    cfg.Logging.Console,
	}
	if err := logger.Init(logCfg); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	logger.Log.Info("Config loaded",
		zap.String("reflector_name", cfg.Reflector.Name),
		zap.Int("configured_modules", len(cfg.Reflector.Modules)),
	)
	defer logger.Sync()

	logger.Log.Info("Starting URFD Dashboard",
		zap.String("version", Version),
		zap.String("commit", Commit),
		zap.String("date", Date),
	)

	// 3. Initialize Store
	s, err := store.NewStore(cfg.Server.DBPath)
	if err != nil {
		logger.Log.Fatal("Failed to initialize store", zap.Error(err))
	}

	// 4. Initialize Hub
	hub := server.NewHub()
	go hub.Run()

	// 5. Initialize Callbook Manager
	cbMgr := callbook.NewManager(s, cfg.Callbook.QRZUsername, cfg.Callbook.QRZPassword)
	cbMgr.Start()

	// State retention & Session management
	var (
		lastState nng.Event
		stateMu   sync.RWMutex
		sessions  = make(map[string]*ActiveSession)
		sessMu    sync.Mutex
	)

	// Session cleanup and persistence ticker (Safety Net)
	go func() {
		for range time.Tick(2 * time.Second) {
			sessMu.Lock()
			now := time.Now().UTC()
			for key, sess := range sessions {
				// Safety Net: Use a longer timeout (30s) if state messages are missing
				if now.Sub(sess.LastSeen) > 30*time.Second {
					// Session ended!
					duration := now.Sub(sess.StartTime).Seconds()
					if err := s.DB.Model(&store.Hearing{}).Where("id = ?", sess.ID).Update("duration", duration).Error; err != nil {
						logger.Log.Error("Failed to update session duration", zap.Error(err))
					}
					hub.BroadcastJSON(nng.Event{
						Type:      "hearing",
						Status:    "ended",
						ID:        sess.ID,
						My:        sess.Callsign,
						Module:    sess.Module,
						Protocol:  sess.Protocol,
						Ur:        sess.Ur,
						Rpt2:      sess.Rpt2,
						Duration:  duration,
						CreatedAt: sess.StartTime.UTC(),
					})
					logger.Log.Info("Session timed out (safety net)", zap.Uint("id", sess.ID))
					delete(sessions, key)
				}
			}
			sessMu.Unlock()
		}
	}()

	// 5. Initialize NNG Subscriber
	sub, err := nng.NewSubscriber(cfg.Server.NNGURL)
	if err != nil {
		logger.Log.Error("Failed to connect to NNG; dashboard will run without live NNG events", zap.Error(err))
		sub = nil
	}

	// 6. Listen for events (only if NNG subscriber was created)
	if sub != nil {
		go func() {
			if err := sub.Listen(func(ev nng.Event) {
				// Pre-process: Trim spaces
				ev.My = strings.TrimSpace(ev.My)
				ev.Module = strings.TrimSpace(ev.Module)

				// Log hearing events to DB (Session-aware)
				if ev.Type == "hearing" || ev.Type == "closing" {
					if ev.My == "" {
						return
					}
					sessKey := ev.My + ":" + ev.Module
					sessMu.Lock()

					// Better matching: Find session by callsign if exact key fails
					sess, exists := sessions[sessKey]
					if !exists && ev.Type == "closing" {
						// Search by callsign
						for _, s := range sessions {
							if s.Callsign == ev.My {
								sess = s
								exists = true
								break
							}
						}
					}

					if ev.Type == "hearing" {
						if !exists {
							h := store.Hearing{
								My:        ev.My,
								Ur:        ev.Ur,
								Rpt1:      ev.Rpt1,
								Rpt2:      ev.Rpt2,
								Module:    ev.Module,
								Protocol:  ev.Protocol,
								CreatedAt: time.Now().UTC(),
							}
							if err := s.DB.Create(&h).Error; err != nil {
								logger.Log.Error("Failed to save hearing", zap.Error(err))
							}
							sess = &ActiveSession{
								ID:        h.ID,
								Callsign:  h.My,
								Module:    h.Module,
								Protocol:  h.Protocol,
								Ur:        h.Ur,
								Rpt2:      h.Rpt2,
								StartTime: h.CreatedAt,
								LastSeen:  time.Now().UTC(),
							}
							sessions[sessKey] = sess
						} else {
							sess.LastSeen = time.Now().UTC()
						}
						ev.ID = sess.ID
						ev.Protocol = sess.Protocol
						ev.CreatedAt = sess.StartTime.UTC()
						ev.Status = "active"
					} else if ev.Type == "closing" && exists {
						duration := time.Since(sess.StartTime).Seconds()
						updates := map[string]interface{}{"duration": duration}
						if ev.Recording != "" {
							updates["audio_file"] = ev.Recording
						}

						if err := s.DB.Model(&store.Hearing{}).Where("id = ?", sess.ID).Updates(updates).Error; err != nil {
							logger.Log.Error("Failed to update session", zap.Error(err))
						}
						ev.ID = sess.ID
						ev.Status = "ended"
						ev.Duration = duration
						ev.CreatedAt = sess.StartTime.UTC()
						ev.My = sess.Callsign
						ev.Module = sess.Module
						ev.Protocol = sess.Protocol
						ev.Ur = sess.Ur
						ev.Rpt2 = sess.Rpt2
						ev.Protocol = sess.Protocol
						ev.Ur = sess.Ur
						ev.Rpt2 = sess.Rpt2
						// ev.Recording is already set from Unmarshal if present
						// and persisted to DB above. It will be broadcasted below.

						// Clean up all sessions for this callsign to be safe
						for k, s := range sessions {
							if s.Callsign == ev.My {
								delete(sessions, k)
							}
						}
						logger.Log.Info("Session closed via closing event", zap.Uint("id", sess.ID))
					}
					sessMu.Unlock()
				}

				if ev.Type == "state" {
					stateMu.Lock()
					lastState = ev
					stateMu.Unlock()

					sessMu.Lock()
					activeTalkersByCall := make(map[string]nng.ActiveTalker)
					for _, talker := range ev.ActiveTalkers {
						call := strings.TrimSpace(talker.Callsign)
						if call != "" {
							talker.Module = strings.TrimSpace(talker.Module)
							activeTalkersByCall[call] = talker
						}
					}

					now := time.Now().UTC()

					// A. Update/End existing sessions based on State
					for key, sess := range sessions {
						talker, ok := activeTalkersByCall[sess.Callsign]
						if ok {
							// User still talking. Correct module if needed.
							if sess.Module != talker.Module {
								logger.Log.Info("Correcting session module",
									zap.String("callsign", sess.Callsign),
									zap.String("old", sess.Module),
									zap.String("new", talker.Module))
								sess.Module = talker.Module
								if err := s.DB.Model(&store.Hearing{}).Where("id = ?", sess.ID).Update("module", sess.Module).Error; err != nil {
									logger.Log.Error("DB fix failed", zap.Error(err))
								}
								delete(sessions, key)
								sessions[sess.Callsign+":"+sess.Module] = sess
							}
							sess.LastSeen = now
							// Synthetic heartbeat
							hub.BroadcastJSON(nng.Event{
								Type:      "hearing",
								Status:    "active",
								ID:        sess.ID,
								My:        sess.Callsign,
								Ur:        sess.Ur,
								Module:    sess.Module,
								Rpt2:      sess.Rpt2,
								Protocol:  sess.Protocol,
								CreatedAt: sess.StartTime,
							})
						} else {
							// User NOT in state.
							// Give them a 3-second grace to allow 'closing' event to arrive or for state jitter
							if now.Sub(sess.LastSeen) > 3*time.Second {
								duration := now.Sub(sess.StartTime).Seconds()
								if err := s.DB.Model(&store.Hearing{}).Where("id = ?", sess.ID).Update("duration", duration).Error; err != nil {
									logger.Log.Error("Duration update failed", zap.Error(err))
								}
								hub.BroadcastJSON(nng.Event{
									Type:      "hearing",
									Status:    "ended",
									ID:        sess.ID,
									My:        sess.Callsign,
									Module:    sess.Module,
									Protocol:  sess.Protocol,
									Ur:        sess.Ur,
									Rpt2:      sess.Rpt2,
									Duration:  duration,
									CreatedAt: sess.StartTime.UTC(),
								})
								logger.Log.Info("Session ended via state sync", zap.Uint("id", sess.ID))
								delete(sessions, key)
							}
						}
					}

					// B. Recovery: Start missing sessions from State
					for call, talker := range activeTalkersByCall {
						found := false
						for _, sess := range sessions {
							if sess.Callsign == call {
								found = true
								break
							}
						}
						if !found {
							h := store.Hearing{
								My:        call,
								Module:    talker.Module,
								Protocol:  talker.Protocol,
								Ur:        "CQCQCQ",
								Rpt1:      "SIMULATOR",
								Rpt2:      "URFD " + talker.Module,
								CreatedAt: now,
							}
							if err := s.DB.Create(&h).Error; err != nil {
								logger.Log.Error("Recovery failed", zap.Error(err))
							}
							sessions[call+":"+talker.Module] = &ActiveSession{
								ID:        h.ID,
								Callsign:  h.My,
								Module:    h.Module,
								Protocol:  h.Protocol,
								Ur:        h.Ur,
								Rpt2:      h.Rpt2,
								StartTime: h.CreatedAt,
								LastSeen:  now,
							}
							logger.Log.Info("Recovered session from State", zap.String("callsign", call))
						}
					}
					sessMu.Unlock()
				}

				hub.BroadcastJSON(ev)
			}); err != nil {
				logger.Log.Fatal("NNG subscriber failed", zap.Error(err))
			}
		}()
	}

	// 7. Start HTTP Server
	srv := server.NewServer(hub, assets.GetAssets())

	// Configure voice if enabled
	if cfg.Voice.Enable {
		srv.VoiceConfig = &voice.SessionConfig{
			RequirePassword:  cfg.Voice.TransmitPassword != "",
			TransmitPassword: cfg.Voice.TransmitPassword,
			MaxTxDuration:    time.Duration(cfg.Voice.MaxTxDuration) * time.Second,
			OpusBitrate:      cfg.Voice.OpusBitrate,
			ReflectorAddr:    cfg.Voice.ReflectorAddr,
		}
		logger.Log.Info("Voice chat enabled", zap.String("reflector_addr", cfg.Voice.ReflectorAddr))
	}

	// API Routes
	http.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		// Parse query params
		query := r.URL.Query()
		// limit is ignored in favor of "25 per module" rule

		cursor := 0
		if c := query.Get("cursor"); c != "" {
			if parsed, err := strconv.Atoi(c); err == nil {
				cursor = parsed
			}
		}

		// Enforce 48-hour limit
		since := time.Now().Add(-48 * time.Hour)
		if s := query.Get("since"); s != "" {
			if parsed, err := time.Parse(time.RFC3339, s); err == nil {
				if parsed.After(since) {
					since = parsed
				}
			}
		}

		// Use Window Function to get last 25 from EACH module to ensure diversity
		// This prevents one busy module from pushing out all history of quiet modules.
		var hearings []store.Hearing

		// Note: glebarez/sqlite supports window functions.
		// We use a raw query because GORM doesn't support recursive limits easily.
		// We still respect 'since' (48h) and 'cursor'.

		queryStr := `
			SELECT * FROM (
				SELECT *, ROW_NUMBER() OVER (PARTITION BY module ORDER BY id DESC) as rn 
				FROM hearings 
				WHERE created_at > ?
		`
		args := []interface{}{since}

		if cursor > 0 {
			queryStr += ` AND id < ? `
			args = append(args, cursor)
		}

		queryStr += `
			) WHERE rn <= 100
			ORDER BY id DESC
		`

		if err := s.DB.Raw(queryStr, args...).Scan(&hearings).Error; err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(hearings); err != nil {
			logger.Log.Error("Failed to encode history response", zap.Error(err))
		}
	})

	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"version":   Version,
			"commit":    Commit,
			"date":      Date,
			"reflector": cfg.Reflector,
		}); err != nil {
			logger.Log.Error("Failed to encode config response", zap.Error(err))
		}
	})

	http.HandleFunc("/api/callbook/", func(w http.ResponseWriter, r *http.Request) {
		callsign := strings.TrimPrefix(r.URL.Path, "/api/callbook/")
		callsign = strings.TrimSpace(callsign)
		callsign = strings.ToUpper(callsign)

		if callsign == "" {
			http.Error(w, "Missing callsign", 400)
			return
		}

		book, err := cbMgr.Lookup(callsign)
		if err != nil {
			// Not found or error
			http.Error(w, "Not found", 404)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(book); err != nil {
			logger.Log.Error("Failed to encode callbook response", zap.Error(err))
		}
	})

	srv.OnConnect = func(client *server.Client) {
		stateMu.RLock()
		defer stateMu.RUnlock()
		if lastState.Type != "" {
			data, _ := json.Marshal(lastState)
			client.Send <- data
		}
	}

	logger.Log.Info("HTTP server starting", zap.String("addr", cfg.Server.Addr))

	// Serve Audio Files
	if cfg.Audio.Enable && cfg.Audio.Path != "" {
		logger.Log.Info("Serving audio files", zap.String("path", cfg.Audio.Path), zap.String("url", "/audio/"))
		// Use http.FileServer to serve files from the directory
		fs := http.FileServer(http.Dir(cfg.Audio.Path))
		// Strip the /audio/ prefix so the file server sees the relative path
		http.Handle("/audio/", http.StripPrefix("/audio/", fs))
	}

	if err := srv.Start(cfg.Server.Addr); err != nil {
		logger.Log.Fatal("Server failed", zap.Error(err))
	}
}

type ActiveSession struct {
	ID        uint
	Callsign  string
	Module    string
	Protocol  string
	Ur        string
	Rpt2      string
	StartTime time.Time
	LastSeen  time.Time
}
