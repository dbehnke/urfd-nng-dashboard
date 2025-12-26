package server

import (
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

type Server struct {
	Hub       *Hub
	Assets    fs.FS
	OnConnect func(*Client)
}

func NewServer(hub *Hub, assets fs.FS) *Server {
	return &Server{Hub: hub, Assets: assets}
}

func (s *Server) Start(addr string) error {
	// Handle WS
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		_, client := UpgradeAndRegister(s.Hub, w, r)
		if client != nil {
			if s.OnConnect != nil {
				s.OnConnect(client)
			}
			go client.WritePump()
		}
	})

	// Handle Static Files (with SPA routing support)
	fileServer := http.FileServer(http.FS(s.Assets))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If requesting a file that doesn't exist, serve index.html for SPA
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		f, err := s.Assets.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			// Serve index.html for unknown routes (SPA)
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})

	log.Printf("HTTP Server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for now
	},
}
