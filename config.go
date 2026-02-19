package main

import (
	"flag"
	"os"
	"strconv"
)

type Config struct {
	TmuxTarget      string
	ListenAddr      string
	RingBufferSize  int
	UploadDir       string
	MaxUploadSize   int64
	TailReplaySize  int
	ClientQueueSize int
}

func ParseConfig() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.TmuxTarget, "tmux-target", "", "tmux pane target (e.g., claude:0.0)")
	flag.StringVar(&cfg.ListenAddr, "listen-addr", ":8080", "HTTP listen address")
	flag.IntVar(&cfg.RingBufferSize, "ring-buffer-size", 16*1024*1024, "ring buffer size in bytes")
	flag.StringVar(&cfg.UploadDir, "upload-dir", "./uploads", "directory for uploaded images")
	flag.Int64Var(&cfg.MaxUploadSize, "max-upload-size", 20*1024*1024, "max upload file size in bytes")
	flag.IntVar(&cfg.TailReplaySize, "tail-replay-size", 256*1024, "tail replay size in bytes for mobile")
	flag.IntVar(&cfg.ClientQueueSize, "client-queue-size", 256, "max outbound messages per client")
	flag.Parse()

	// Environment variable overrides
	if v := os.Getenv("TMUX_TARGET"); v != "" {
		cfg.TmuxTarget = v
	}
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("RING_BUFFER_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RingBufferSize = n
		}
	}
	if v := os.Getenv("UPLOAD_DIR"); v != "" {
		cfg.UploadDir = v
	}
	if v := os.Getenv("MAX_UPLOAD_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.MaxUploadSize = n
		}
	}
	if v := os.Getenv("TAIL_REPLAY_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.TailReplaySize = n
		}
	}
	if v := os.Getenv("CLIENT_QUEUE_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ClientQueueSize = n
		}
	}

	// TmuxTarget is optional — if empty, the session picker UI will be shown.

	return cfg, nil
}
