package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

func NewFilesHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Query().Get("path")
		if reqPath == "" {
			reqPath = os.Getenv("HOME")
		}

		// Resolve to absolute and clean
		absPath, err := filepath.Abs(reqPath)
		if err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(absPath)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if !info.IsDir() {
			http.Error(w, "not a directory", http.StatusBadRequest)
			return
		}

		entries, err := os.ReadDir(absPath)
		if err != nil {
			http.Error(w, "cannot read directory", http.StatusForbidden)
			return
		}

		var files []FileEntry
		for _, e := range entries {
			// Skip hidden files
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, FileEntry{
				Name:  e.Name(),
				IsDir: e.IsDir(),
				Size:  info.Size(),
			})
		}

		// Sort: directories first, then alphabetical
		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir
			}
			return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"path":  absPath,
			"files": files,
		})
	}
}

func NewFileContentHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Query().Get("path")
		if reqPath == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}

		absPath, err := filepath.Abs(reqPath)
		if err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(absPath)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if info.IsDir() {
			http.Error(w, "is a directory", http.StatusBadRequest)
			return
		}

		http.ServeFile(w, r, absPath)
	}
}

func NewFileSaveHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Query().Get("path")
		if reqPath == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}

		absPath, err := filepath.Abs(reqPath)
		if err != nil {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		// Only allow saving to existing files (no creating new files)
		if _, err := os.Stat(absPath); err != nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(absPath, body, 0644); err != nil {
			logger.Error("failed to save file", "path", absPath, "error", err)
			http.Error(w, "save failed", http.StatusInternalServerError)
			return
		}

		logger.Info("file saved", "path", absPath, "bytes", len(body))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true", "path": absPath})
	}
}
