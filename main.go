package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := ParseConfig()
	if err != nil {
		logger.Error("config error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logger.Info("starting c3",
		"listen_addr", cfg.ListenAddr,
		"ring_buffer_size", cfg.RingBufferSize,
	)

	sm := NewSessionManager(cfg, logger)
	defer sm.CloseAll()

	// If a default target was provided on the command line, pre-create the session.
	if cfg.TmuxTarget != "" {
		sm.Get(cfg.TmuxTarget)
		logger.Info("default session created", "target", cfg.TmuxTarget)
	}

	// File indexer — scans home directory in background
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "/"
	}
	indexer := NewFileIndexer(homeDir, 30*time.Second, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go indexer.Run(ctx)

	mux := NewServer(cfg, sm, indexer, logger)

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		logger.Info("shutting down")
		cancel()
		sm.CloseAll()
		server.Close()
	}()

	logger.Info("listening", "addr", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
