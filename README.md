# c3 — Cloud Code Web UI

c3 is a mobile-first web interface for a long-lived Claude Code session running inside a tmux pane on a VPS. It gives you full terminal fidelity on desktop via xterm.js and a comfortable touch experience on mobile with a bottom-anchored input composer and quick-action buttons (y, n, Ctrl-C, Ctrl-D). The entire system compiles to a single Go binary with the Svelte 5 frontend embedded — just copy it to your server and run it.

## Getting Started

You need Go 1.22+, Node.js, and tmux installed on the machine where you want to run c3. Start by cloning the repo and building:

```
make
```

This runs `npm ci && npm run build` for the frontend, then `go build` to produce the `c3` binary with everything embedded. Next, start a tmux session and launch Claude Code inside it:

```
tmux new-session -s claude
claude
```

In a separate terminal (or via systemd), start c3 pointing at that tmux pane:

```
./c3 --tmux-target=claude:0.0 --listen-addr=:8080
```

Open `http://<your-server>:8080` in a browser. On desktop you get a full interactive terminal. On mobile you'll see a replay mode selector, then a readable terminal stream with a composer at the bottom. The server reads terminal output via `tmux pipe-pane` and writes input directly to the PTY slave, so what you see in the browser is byte-for-byte identical to what a real terminal would render.

## Configuration

All settings can be passed as flags or environment variables. The key ones are `--tmux-target` / `TMUX_TARGET` (required — the tmux pane to attach to, e.g. `claude:0.0`), `--listen-addr` / `LISTEN_ADDR` (default `:8080`), `--upload-dir` / `UPLOAD_DIR` (default `./uploads`), and `--ring-buffer-size` / `RING_BUFFER_SIZE` (default 16 MB). A systemd unit file is included at `c3.service` — edit the `ExecStart` line with your tmux target and install it to run c3 as a service.

## Running Tests

The test suite includes unit tests for the ring buffer and integration tests that spin up real tmux sessions, start the server, and exercise the full pipeline over WebSocket:

```
go test -v -timeout 300s ./...
```

The integration tests cover replay modes, multi-client fan-out, image upload with content-addressed dedup, pane destruction/recreation with epoch tracking, ANSI fidelity, control character injection, rapid resize spam, and concurrent connect/disconnect. They require tmux to be installed on the machine running the tests.
