package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func NewServer(cfg *Config, sm *SessionManager, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	// Session list endpoint
	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		sessions, err := ListSessions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"sessions": sessions,
		})
	})

	// Per-session WebSocket: /s/{target}/ws
	// Target can contain colons and dots, e.g., "6:0.0"
	mux.HandleFunc("GET /s/{target}/ws", func(w http.ResponseWriter, r *http.Request) {
		target := r.PathValue("target")
		if target == "" {
			http.Error(w, "missing target", http.StatusBadRequest)
			return
		}

		sess := sm.Get(target)

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			logger.Error("websocket accept failed", "error", err, "target", target)
			return
		}

		client := NewClient(conn, sess.Hub, sess.PTY, sess.Ring, cfg, logger)
		client.Run(r.Context())
	})

	// Per-session upload: /s/{target}/upload
	mux.HandleFunc("POST /s/{target}/upload", func(w http.ResponseWriter, r *http.Request) {
		target := r.PathValue("target")
		if target == "" {
			http.Error(w, "missing target", http.StatusBadRequest)
			return
		}
		sess := sm.Get(target)
		NewUploadHandler(cfg, sess.PTY, logger)(w, r)
	})

	// Serve embedded frontend
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		logger.Error("failed to create sub filesystem", "error", err)
		return mux
	}

	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve exact static file (JS, CSS, etc.)
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		// Serve static assets directly
		if _, err := fs.Stat(distFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for all other routes
		// (including /s/{target}/ paths)
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	return mux
}
