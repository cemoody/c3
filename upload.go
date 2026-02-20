package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var allowedExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

func NewUploadHandler(cfg *Config, ptyMgr *PTYManager, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxUploadSize)

		if err := r.ParseMultipartForm(cfg.MaxUploadSize); err != nil {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}

		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(w, "missing image field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext == ".jpeg" {
			ext = ".jpg"
		}
		if !allowedExts[ext] {
			http.Error(w, fmt.Sprintf("unsupported file type: %s", ext), http.StatusBadRequest)
			return
		}

		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, "failed to read file", http.StatusInternalServerError)
			return
		}

		hash := sha256.Sum256(data)
		hexHash := hex.EncodeToString(hash[:])

		if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
			logger.Error("failed to create upload dir", "error", err)
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(cfg.UploadDir, hexHash+ext)
		absPath, err := filepath.Abs(destPath)
		if err != nil {
			absPath = destPath
		}

		// Write file (skip if already exists — content-addressed dedup)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				logger.Error("failed to write upload", "error", err, "path", destPath)
				http.Error(w, "failed to save file", http.StatusInternalServerError)
				return
			}
			logger.Info("image uploaded", "path", absPath, "hash", hexHash, "size", len(data))
		} else {
			logger.Info("image upload deduplicated", "path", absPath, "hash", hexHash)
		}

		// Inject prompt into PTY (if connected to a session)
		if ptyMgr != nil {
			prompt := fmt.Sprintf("Analyze this image: %s\n", absPath)
			ptyMgr.WriteInput([]byte(prompt))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"path": absPath,
			"hash": hexHash,
		})
	}
}
