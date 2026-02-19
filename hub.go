package main

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"sync"
)

// Hub manages all connected WebSocket clients and broadcasts PTY output.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
	logger  *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]*Client),
		logger:  logger,
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.id] = c
	h.logger.Info("client registered", "client_id", c.id, "total", len(h.clients))
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c.id]; ok {
		delete(h.clients, c.id)
		close(c.sendCh)
		h.logger.Info("client unregistered", "client_id", c.id, "total", len(h.clients))
	}
}

// Broadcast sends raw PTY output data to all connected clients as an OutputMsg.
func (h *Hub) Broadcast(data []byte) {
	msg := OutputMsg{
		Type: "output",
		Data: base64.StdEncoding.EncodeToString(data),
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal output message", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		select {
		case c.sendCh <- raw:
		default:
			c.dropped++
			if c.dropped >= 10 {
				h.logger.Warn("client too slow, will disconnect", "client_id", c.id, "dropped", c.dropped)
				go c.conn.CloseNow()
			}
		}
	}
}

// BroadcastStatus sends a status message to all connected clients.
func (h *Hub) BroadcastStatus(paneState string, epoch int64) {
	msg := StatusMsg{
		Type:      "status",
		PaneState: paneState,
		Epoch:     epoch,
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		select {
		case c.sendCh <- raw:
		default:
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
