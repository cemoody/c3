package main

import (
	"encoding/json"
	"fmt"
)

// Client -> Server messages

type HelloMsg struct {
	Type       string `json:"type"`
	ReplayMode string `json:"replayMode"`
	TailSize   int    `json:"tailSize,omitempty"`
}

type InputMsg struct {
	Type string `json:"type"`
	Data string `json:"data"` // base64-encoded
}

type ResizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// Server -> Client messages

type OutputMsg struct {
	Type string `json:"type"`
	Data string `json:"data"` // base64-encoded
}

type ErrorMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type StatusMsg struct {
	Type      string `json:"type"`
	PaneState string `json:"paneState"` // "connected", "missing"
	Epoch     int64  `json:"epoch"`
	Cols      int    `json:"cols,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}

// ParseClientMessage parses a raw JSON message from a client into the appropriate type.
func ParseClientMessage(raw []byte) (any, error) {
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	switch base.Type {
	case "hello":
		var msg HelloMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case "input":
		var msg InputMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	case "resize":
		var msg ResizeMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	default:
		return nil, fmt.Errorf("unknown message type: %q", base.Type)
	}
}
