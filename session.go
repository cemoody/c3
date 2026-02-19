package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Session holds the per-target PTY pipeline: monitor, pty manager, ring buffer, and hub.
type Session struct {
	Target  string
	Ring    *RingBuffer
	Hub     *Hub
	PTY     *PTYManager
	Monitor *PaneMonitor
	cancel  context.CancelFunc
}

// SessionManager creates and caches sessions by tmux target.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	cfg      *Config
	logger   *slog.Logger
}

func NewSessionManager(cfg *Config, logger *slog.Logger) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		cfg:      cfg,
		logger:   logger,
	}
}

// Get returns an existing session or creates a new one for the given target.
func (sm *SessionManager) Get(target string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.sessions[target]; ok {
		return s
	}

	s := sm.createLocked(target)
	sm.sessions[target] = s
	return s
}

func (sm *SessionManager) createLocked(target string) *Session {
	logger := sm.logger.With("target", target)

	ring := NewRingBuffer(sm.cfg.RingBufferSize)
	hub := NewHub(logger)
	ptyMgr := NewPTYManager(target, ring, logger)
	ptyMgr.onOutput = func(data []byte) { hub.Broadcast(data) }

	ctx, cancel := context.WithCancel(context.Background())

	monitor := NewPaneMonitor(target, 5*time.Second, logger)
	go monitor.Run(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-monitor.Events():
				switch ev.State {
				case PaneStateConnected:
					if ev.NewTTY {
						logger.Info("attaching to PTY", "tty", ev.TTY)
						if err := ptyMgr.Reattach(ev.TTY); err != nil {
							logger.Error("failed to attach PTY", "tty", ev.TTY, "error", err)
						}
						hub.BroadcastStatus("connected", ptyMgr.Epoch())
					}
				case PaneStateMissing:
					logger.Warn("pane missing, closing PTY")
					ptyMgr.Close()
					hub.BroadcastStatus("missing", ptyMgr.Epoch())
				}
			}
		}
	}()

	logger.Info("session created", "target", target)

	return &Session{
		Target:  target,
		Ring:    ring,
		Hub:     hub,
		PTY:     ptyMgr,
		Monitor: monitor,
		cancel:  cancel,
	}
}

// Close shuts down a session.
func (s *Session) Close() {
	s.cancel()
	s.PTY.Close()
}

// CloseAll shuts down all sessions.
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, s := range sm.sessions {
		s.Close()
	}
}
