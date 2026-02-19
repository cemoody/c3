package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

var clientCounter atomic.Int64

// Client represents a single WebSocket client connection.
type Client struct {
	id      string
	conn    *websocket.Conn
	hub     *Hub
	pty     *PTYManager
	ring    *RingBuffer
	cfg     *Config
	sendCh  chan []byte
	logger  *slog.Logger
	dropped int
}

func NewClient(conn *websocket.Conn, hub *Hub, pty *PTYManager, ring *RingBuffer, cfg *Config, logger *slog.Logger) *Client {
	id := fmt.Sprintf("c%d", clientCounter.Add(1))
	return &Client{
		id:     id,
		conn:   conn,
		hub:    hub,
		pty:    pty,
		ring:   ring,
		cfg:    cfg,
		sendCh: make(chan []byte, cfg.ClientQueueSize),
		logger: logger.With("client_id", id),
	}
}

// Run starts the client read and write pumps. Blocks until the client disconnects.
func (c *Client) Run(ctx context.Context) {
	c.logger.Info("client connected")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.writePump(ctx)
	c.readPump(ctx)
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		c.conn.CloseNow()
	}()

	// First message must be a hello.
	_, raw, err := c.conn.Read(ctx)
	if err != nil {
		c.logger.Error("read hello failed", "error", err)
		return
	}

	msg, err := ParseClientMessage(raw)
	if err != nil {
		c.sendError(ctx, fmt.Sprintf("invalid hello: %v", err))
		return
	}

	hello, ok := msg.(*HelloMsg)
	if !ok {
		c.sendError(ctx, "first message must be hello")
		return
	}

	// Perform replay.
	if err := c.replay(ctx, hello); err != nil {
		c.logger.Error("replay failed", "error", err)
		return
	}

	// Register for live fan-out after replay completes.
	c.hub.Register(c)

	// Send current status.
	c.sendStatus(ctx)

	// Read loop for input/resize messages.
	// Note: we intentionally do NOT forward resize to the PTY.
	// The web client adapts to the pane's dimensions (sent in status),
	// rather than resizing the pane to match the browser. This prevents
	// TUI rendering corruption from mid-animation resize races.
	for {
		_, raw, err := c.conn.Read(ctx)
		if err != nil {
			c.logger.Info("client disconnected", "error", err)
			return
		}

		msg, err := ParseClientMessage(raw)
		if err != nil {
			c.logger.Warn("invalid message", "error", err)
			continue
		}

		switch m := msg.(type) {
		case *InputMsg:
			data, err := base64.StdEncoding.DecodeString(m.Data)
			if err != nil {
				c.logger.Warn("invalid base64 input", "error", err)
				continue
			}
			c.pty.WriteInput(data)
		case *ResizeMsg:
			// Ignored — the pane dimensions are authoritative.
			// The client should match its terminal to the pane size.
			_ = m
		default:
			c.logger.Warn("unexpected message type in read loop", "msg", msg)
		}
	}
}

func (c *Client) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}
			err := c.conn.Write(ctx, websocket.MessageText, msg)
			if err != nil {
				// Don't log errors when context is cancelled (normal disconnect)
				if ctx.Err() == nil {
					c.logger.Error("write failed", "error", err)
				}
				return
			}
		}
	}
}

func (c *Client) replay(ctx context.Context, hello *HelloMsg) error {
	start := time.Now()

	// For fast connect (tail mode, the default): send a tmux capture-pane
	// snapshot of the current screen, then stream live. This is instant
	// and gives a clean render. The ring buffer tail is not sent because
	// it starts mid-stream and xterm.js can't reconstruct state from it.
	//
	// For full replay: send the entire ring buffer. This takes longer but
	// gives complete scrollback history.
	if hello.ReplayMode != "full" {
		// For fast connect: skip replay entirely. The client will send a
		// resize, which triggers the TUI to receive SIGWINCH and redraw.
		// We also inject Ctrl-L after a short delay to force a full redraw
		// at the client's terminal dimensions. This avoids width mismatches
		// between the old pane size and the browser's terminal size.
		c.logger.Info("fast connect, skipping replay — will redraw on resize")
		return nil
	}

	var data []byte
	switch hello.ReplayMode {
	case "full":
		data, _ = c.ring.Snapshot()
	default: // "tail" or default
		tailSize := hello.TailSize
		if tailSize <= 0 {
			tailSize = c.cfg.TailReplaySize
		}
		if tailSize > c.cfg.RingBufferSize {
			tailSize = c.cfg.RingBufferSize
		}
		data, _ = c.ring.Tail(tailSize)
	}

	if len(data) > 0 {
		c.logger.Info("replaying", "mode", hello.ReplayMode, "bytes", len(data))

		const chunkSize = 64 * 1024
		for i := 0; i < len(data); i += chunkSize {
			end := i + chunkSize
			if end > len(data) {
				end = len(data)
			}
			if err := c.sendOutputFrame(ctx, data[i:end]); err != nil {
				return fmt.Errorf("replay write error: %w", err)
			}
		}

		c.logger.Info("replay complete", "bytes", len(data), "duration", time.Since(start))
	}

	return nil
}

func (c *Client) sendOutputFrame(ctx context.Context, data []byte) error {
	msg := OutputMsg{
		Type: "output",
		Data: base64.StdEncoding.EncodeToString(data),
	}
	raw, _ := json.Marshal(msg)
	return c.conn.Write(ctx, websocket.MessageText, raw)
}

func (c *Client) sendError(ctx context.Context, message string) {
	msg := ErrorMsg{Type: "error", Message: message}
	raw, _ := json.Marshal(msg)
	c.conn.Write(ctx, websocket.MessageText, raw)
}

func (c *Client) sendStatus(ctx context.Context) {
	msg := StatusMsg{
		Type:      "status",
		PaneState: "connected",
		Epoch:     c.pty.Epoch(),
	}
	// Include pane dimensions so the client can match them
	target := c.pty.Target()
	if target != "" {
		if cols, rows, err := PaneDimensions(target); err == nil {
			msg.Cols = cols
			msg.Rows = rows
		}
	}
	raw, _ := json.Marshal(msg)
	select {
	case c.sendCh <- raw:
	default:
	}
}
