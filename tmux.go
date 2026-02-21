package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ResolvePaneTTY queries tmux for the PTY device path of a given pane target.
func ResolvePaneTTY(target string) (string, error) {
	cmd := exec.Command("tmux", "display-message", "-p", "-t", target, "#{pane_tty}")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux query failed: %w", err)
	}
	tty := strings.TrimSpace(string(out))
	if tty == "" {
		return "", fmt.Errorf("empty pane_tty for target %q", target)
	}
	if !strings.HasPrefix(tty, "/dev/") {
		return "", fmt.Errorf("unexpected pane_tty value: %q", tty)
	}
	return tty, nil
}

// RenameWindow renames the tmux window containing the given pane target.
func RenameWindow(target, name string) error {
	cmd := exec.Command("tmux", "rename-window", "-t", target, name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux rename-window failed: %w", err)
	}
	return nil
}

// CreateSession creates a new detached tmux session with the given name.
func CreateSession(name string) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux new-session failed: %w", err)
	}
	return nil
}

// PaneDimensions returns the current cols and rows of a tmux pane.
func PaneDimensions(target string) (cols, rows int, err error) {
	cmd := exec.Command("tmux", "display-message", "-p", "-t", target, "#{pane_width} #{pane_height}")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	_, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%d %d", &cols, &rows)
	return
}

// CursorPosition returns the cursor position (0-indexed col, row) of a tmux pane.
func CursorPosition(target string) (col, row int, err error) {
	cmd := exec.Command("tmux", "display-message", "-p", "-t", target, "#{cursor_x} #{cursor_y}")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	_, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%d %d", &col, &row)
	return
}

// CapturePane returns the visible content plus scrollback history of a tmux
// pane with ANSI escape sequences intact. The scrollbackLines parameter
// controls how many lines of history before the visible area to include.
func CapturePane(target string, scrollbackLines int) ([]byte, error) {
	// -e: include escape sequences (colors, etc.)
	// -p: output to stdout
	// -t: target pane
	// -S: start line (negative = lines before visible area)
	startLine := fmt.Sprintf("-%d", scrollbackLines)
	cmd := exec.Command("tmux", "capture-pane", "-e", "-p", "-t", target, "-S", startLine)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tmux capture-pane failed: %w", err)
	}
	return out, nil
}

// TmuxSession represents a tmux session with its windows and panes.
type TmuxSession struct {
	Name    string      `json:"name"`
	Windows []TmuxWindow `json:"windows"`
}

type TmuxWindow struct {
	Index  string     `json:"index"`
	Name   string     `json:"name"`
	Panes  []TmuxPane `json:"panes"`
}

type TmuxPane struct {
	Index      string `json:"index"`
	CurrentCmd string `json:"currentCommand"`
	Target     string `json:"target"` // "session:window.pane"
}

// ListSessions returns all tmux sessions with their windows and panes.
func ListSessions() ([]TmuxSession, error) {
	// List all panes across all sessions with format fields
	cmd := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{session_name}\t#{window_index}\t#{window_name}\t#{pane_index}\t#{pane_current_command}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes failed: %w", err)
	}

	sessionMap := make(map[string]*TmuxSession)
	windowMap := make(map[string]*TmuxWindow) // key: "session:window"
	var sessionOrder []string

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}
		sessName, winIdx, winName, paneIdx, paneCmd := parts[0], parts[1], parts[2], parts[3], parts[4]

		sess, ok := sessionMap[sessName]
		if !ok {
			sess = &TmuxSession{Name: sessName}
			sessionMap[sessName] = sess
			sessionOrder = append(sessionOrder, sessName)
		}

		winKey := sessName + ":" + winIdx
		win, ok := windowMap[winKey]
		if !ok {
			win = &TmuxWindow{Index: winIdx, Name: winName}
			windowMap[winKey] = win
			sess.Windows = append(sess.Windows, TmuxWindow{}) // placeholder
		}

		target := fmt.Sprintf("%s:%s.%s", sessName, winIdx, paneIdx)
		pane := TmuxPane{
			Index:      paneIdx,
			CurrentCmd: paneCmd,
			Target:     target,
		}
		win.Panes = append(win.Panes, pane)
	}

	// Rebuild sessions with proper window references
	var result []TmuxSession
	for _, name := range sessionOrder {
		sess := sessionMap[name]
		var windows []TmuxWindow
		for i := range sess.Windows {
			winKey := name + ":" + fmt.Sprint(i)
			if win, ok := windowMap[winKey]; ok {
				windows = append(windows, *win)
			}
		}
		// Fallback: iterate windowMap for this session if indices aren't sequential
		if len(windows) == 0 {
			for key, win := range windowMap {
				if strings.HasPrefix(key, name+":") {
					windows = append(windows, *win)
				}
			}
		}
		result = append(result, TmuxSession{Name: name, Windows: windows})
	}

	return result, nil
}

type PaneState int

const (
	PaneStateMissing PaneState = iota
	PaneStateConnected
)

// PaneEvent is emitted by PaneMonitor when pane state changes.
type PaneEvent struct {
	State  PaneState
	TTY    string // non-empty when State == PaneStateConnected
	NewTTY bool   // true if the TTY path changed from the previous known path
}

// PaneMonitor periodically checks for the configured tmux pane.
type PaneMonitor struct {
	target   string
	interval time.Duration
	logger   *slog.Logger

	mu       sync.Mutex
	state    PaneState
	lastTTY  string
	eventsCh chan PaneEvent
}

func NewPaneMonitor(target string, interval time.Duration, logger *slog.Logger) *PaneMonitor {
	return &PaneMonitor{
		target:   target,
		interval: interval,
		logger:   logger,
		state:    PaneStateMissing,
		eventsCh: make(chan PaneEvent, 8),
	}
}

// Events returns the channel on which pane state changes are delivered.
func (m *PaneMonitor) Events() <-chan PaneEvent {
	return m.eventsCh
}

// State returns the current pane state.
func (m *PaneMonitor) State() PaneState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// CurrentTTY returns the last known TTY path.
func (m *PaneMonitor) CurrentTTY() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastTTY
}

// SetTarget changes the tmux target and resets state so the monitor
// will discover the new pane on the next check.
func (m *PaneMonitor) SetTarget(target string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.target = target
	m.state = PaneStateMissing
	m.lastTTY = ""
}

// Target returns the current tmux target.
func (m *PaneMonitor) Target() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.target
}

// ForceCheck triggers an immediate pane check outside the normal interval.
func (m *PaneMonitor) ForceCheck() {
	m.check()
}

// Run starts the monitor loop. It blocks until ctx is cancelled.
func (m *PaneMonitor) Run(ctx context.Context) {
	// Do an immediate check before entering the ticker loop.
	m.check()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check()
		}
	}
}

func (m *PaneMonitor) check() {
	m.mu.Lock()
	target := m.target
	m.mu.Unlock()

	if target == "" {
		return
	}

	tty, err := ResolvePaneTTY(target)

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		if m.state != PaneStateMissing {
			m.logger.Warn("tmux pane lost", "target", m.target, "error", err)
			m.state = PaneStateMissing
			m.emit(PaneEvent{State: PaneStateMissing})
		}
		return
	}

	// Pane exists.
	if m.state == PaneStateMissing {
		// Transition from missing → connected.
		// Always set NewTTY=true because the pane was destroyed and recreated,
		// so we must reattach even if Linux reused the same PTY number.
		m.logger.Info("tmux pane found", "target", m.target, "tty", tty)
		m.state = PaneStateConnected
		m.lastTTY = tty
		m.emit(PaneEvent{State: PaneStateConnected, TTY: tty, NewTTY: true})
		return
	}

	// Already connected — check if TTY changed.
	if tty != m.lastTTY {
		m.logger.Info("tmux pane TTY changed", "target", m.target, "old", m.lastTTY, "new", tty)
		m.lastTTY = tty
		m.emit(PaneEvent{State: PaneStateConnected, TTY: tty, NewTTY: true})
	}
}

func (m *PaneMonitor) emit(ev PaneEvent) {
	select {
	case m.eventsCh <- ev:
	default:
		m.logger.Warn("pane event channel full, dropping event")
	}
}
