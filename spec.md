Below is a revised version of your spec, incorporating:
    •   Tail-based fast replay for mobile (with full replay available)
    •   Explicit replay protocol and backpressure policy
    •   Content-addressed uploads with limits
    •   “Jump to live” and replay-scope UX controls
    •   Clarified PTY lifecycle and concurrency model

I’ve kept structure, but converted most sections to prose with targeted lists only where helpful.

⸻

Cloud Code Web UI — Revised Spec (v1)

Overview

Cloud Code Web UI is a mobile-first web interface for a long-lived Claude Code interactive session running inside a tmux pane on a single VPS. The UI must function well on both phones and desktops. Desktop users retain keystroke-level terminal fidelity via xterm.js. Mobile users receive a more ergonomic “product UI” experience: a readable terminal stream combined with a bottom-anchored input composer and quick actions.

The system is designed for a single trusted user operating over Tailscale. Simplicity, determinism, and terminal fidelity take priority over multi-user coordination or heavy lifecycle management.

⸻

Session Persistence & PTY Architecture

Claude Code runs inside a tmux pane managed manually by the user. The system does not attempt to survive VPS reboots or automatically recreate tmux sessions in v1.

The Go server does not attach as a tmux client. Instead, it resolves the pane’s underlying PTY path via tmux format queries (e.g., #{pane_tty}) and opens that PTY directly. All terminal I/O flows through this PTY file descriptor. This avoids tmux chrome and ensures that the byte stream seen by the browser is identical to what a real terminal would receive.

The tmux session remains independently accessible over SSH at all times.

The pane must be uniquely identifiable by configured session/window/pane identifiers. If the pane is destroyed and recreated, the server detects the change, resolves the new pane_tty, closes the old PTY file descriptor, and attaches to the new one.

When the PTY changes, the server begins a new internal “session epoch” while retaining the existing ring buffer unless explicitly configured otherwise. This allows scrollback continuity across pane recreation.

⸻

Backend (Go)

The backend is implemented in Go, runs as a single static binary, and is supervised via systemd.

On startup and periodically thereafter, the server verifies that the configured tmux pane exists. If it does not, the UI surfaces a clear “Session not found — start tmux + Claude Code” state. When the pane reappears, the server automatically attaches.

PTY I/O Model

The server maintains exactly one PTY file descriptor at a time. A dedicated read loop continuously reads raw bytes from the PTY and appends them to a server-side ring buffer.

All writes to the PTY are serialized through a single goroutine or channel to guarantee write ordering and prevent byte-level corruption under concurrent input. Multiple clients may send input simultaneously; interleaving is acceptable, but writes are strictly serialized before reaching the PTY.

Input is treated as raw bytes. Desktop keystrokes and composer submissions are transmitted as byte sequences without interpretation at the server layer.

⸻

Ring Buffer & Replay

The server maintains a configurable in-memory ring buffer of raw PTY bytes (default: 16 MB). This buffer is the canonical source of truth for terminal history.

The ring buffer serves two purposes:
    1.  Live fan-out of new PTY output to connected clients.
    2.  Scrollback replay when clients connect or reconnect.

Replay always consists of the original raw byte stream so that xterm.js reconstructs the terminal state exactly as if it had been connected live, including ANSI color, cursor movement, alternate screen transitions, and TUI behavior.

Each connected client maintains its own logical read cursor into the ring buffer. If a client falls behind beyond the oldest retained byte, it is fast-forwarded to the earliest available position. This event is logged and may optionally be surfaced in the UI as “history truncated.”

When the ring buffer wraps and overwrites old data, a structured “ring buffer wrap” event is logged.

⸻

Replay Modes & Fast Mobile Connect

To ensure fast mobile connections, the system supports two replay modes: full replay and tail replay.

On WebSocket connect, the client sends a hello message indicating its preferred replay mode:
    •   full: replay the entire available ring buffer (default for desktop).
    •   tail: replay only the most recent N bytes from the ring buffer, then attach live (default for mobile).

The server may clamp the requested tail size to configured minimum and maximum bounds. The default tail size for mobile is 256 KB. This keeps connection time and xterm parsing cost low on phones while still providing meaningful recent context.

After replaying the requested segment, the server streams live output as normal.

The frontend includes two UX affordances:
    •   A persistent “Jump to live” control that scrolls to the live bottom and resumes following output.
    •   A replay scope selector on connect or reconnect, allowing the user to choose between “Recent (fast)” and “Full history.” Mobile defaults to “Recent,” desktop defaults to “Full history.”

Replay frames may optionally be compressed (e.g., gzip or WebSocket permessage-deflate) to improve performance over slower links, though limiting replay size is the primary performance strategy in v1.

Checkpoint-based serialized terminal snapshots are explicitly out of scope for v1.

⸻

WebSocket Fan-Out & Backpressure

The server broadcasts PTY output to all connected WebSocket clients.

Each client has a bounded outbound queue. If a client cannot keep up and its queue exceeds configured limits, the server may either:
    •   Drop older queued frames and fast-forward the client to a more recent offset, or
    •   Disconnect the client with a clear reason.

A slow client must never block the PTY read loop or delay delivery to other clients.

Interleaved input from multiple clients is permitted. There is no locking or “control ownership” model in v1.

⸻

Terminal Resize

When a client sends a resize event, the server applies the new dimensions to the PTY immediately. Last resize wins.

Resize events are logged with the originating client identifier and the new dimensions.

Frequent resize events (e.g., mobile orientation changes) must not crash or corrupt terminal state. The server simply forwards the most recent dimensions to the PTY.

⸻

Frontend (Svelte + xterm.js)

The frontend is implemented in Svelte and uses xterm.js for terminal rendering. The compiled static assets are embedded into the Go binary via //go:embed.

xterm.js is configured with a large scrollback value (e.g., 50,000 lines) so that replayed history remains navigable using native scroll gestures on mobile and scroll wheel on desktop.

On desktop, the terminal captures direct keyboard input. On mobile, a bottom-anchored composer provides:
    •   A multi-line text area.
    •   A send button.
    •   Optional quick-action keys such as y, n, Ctrl-C, and Ctrl-D.

Composer submissions are transmitted as raw byte sequences to the server.

⸻

Image Upload

Image input is supported via content-addressed uploads.

Uploaded files are written to a stable directory such as:

/home/cloudcode/uploads/<sha256>.<ext>

The filename is derived from a deterministic SHA-256 hash of the file contents plus a validated extension. If the same file is uploaded multiple times, it resolves to the same path.

Uploads are subject to:
    •   A maximum file size (default: 20 MB, configurable).
    •   A type allowlist (e.g., png, jpg, webp).

Although the system operates within a trusted Tailscale environment and assumes a single user, the server still enforces size limits and basic validation to prevent accidental misuse.

After saving the file, the server injects a prompt line into the PTY referencing the absolute path, such as:

Analyze this image: /home/cloudcode/uploads/<sha256>.png

This leverages Claude Code’s documented image-path workflow and avoids clipboard emulation.

⸻

Networking & Trust Model

The server runs behind Tailscale (or equivalent WireGuard mesh). No TLS termination or authentication layer is required in v1. The service listens on plain HTTP/WebSocket bound to the Tailscale interface.

The deployment assumes a single trusted user within a secured tailnet. Additional authentication and multi-user isolation are out of scope for v1.

⸻

Observability & Logging

The server emits structured JSON logs.

Client lifecycle events include connection, disconnection (with reason), bytes written to PTY, bytes sent to client, dropped frames, and replay duration. Each client is assigned a connection ID for correlation.

Session-level events include tmux pane missing, pane discovered, TTY changed, PTY opened, PTY EOF, and resize events.

Server-level events include ring buffer wrap, active client count, uptime, and memory usage heartbeat.

When the PTY is reattached, a new session epoch identifier is generated and included in subsequent logs to correlate output with PTY lifecycle events.

⸻

Testing

Golden tests record raw PTY byte streams from representative Claude Code sessions, including color output, progress bars, alternate screen transitions, and TUI prompts. These streams are replayed into xterm.js in a headless browser, and rendered output is compared against expected snapshots to detect ANSI or replay regressions.

Chaos tests include destroying and recreating the tmux pane while clients are connected, simulating PTY rotation, triggering reconnect storms, throttling a slow client, spamming resize events, and sending concurrent input from multiple clients. The system must remain stable, avoid data corruption, and preserve PTY integrity under all scenarios.

⸻

Out of Scope (v1)

Claude Code lifecycle management, automatic restarts, file browsing or download endpoints, checkpoint-based serialized reconnect acceleration, and VPS reboot survival are explicitly deferred to future versions.

