# c3

**Your tmux sessions, everywhere.**

c3 is a single-binary web UI that wraps tmux — built for running Claude Code on a VPS and accessing it from your phone, tablet, or any browser. Pair it with [Tailscale](https://tailscale.com) for secure access from anywhere.

- Full terminal on desktop via xterm.js
- Touch-friendly composer + icon quick actions on mobile
- Tab manager with drag-and-drop reorder and inline rename
- File browser with markdown preview and editing
- Image upload via paste or camera capture
- Arrow pad d-pad for mobile navigation
- One binary, zero dependencies (Svelte 5 frontend embedded)

## Get Started

**Download the latest binary:**

```bash
curl -LO https://github.com/cemoody/c3/releases/latest/download/c3
chmod +x c3
```

**Start a tmux session with Claude Code:**

```bash
tmux new-session -s claude
claude
```

**In another terminal, run c3:**

```bash
./c3 --tmux-target=claude:0.0 --listen-addr=:8080
```

**Open `http://your-server:8080` in any browser.** That's it.

### With Tailscale (recommended)

Install [Tailscale](https://tailscale.com/download) on your VPS and your devices. Then just hit `http://your-vps-tailscale-ip:8080` from anywhere — encrypted, no port forwarding, no VPN config.

## Build from Source

Requires Go 1.22+ and Node.js:

```bash
make
```

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--tmux-target` | `TMUX_TARGET` | — | tmux pane to attach to (e.g. `claude:0.0`) |
| `--listen-addr` | `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `--upload-dir` | `UPLOAD_DIR` | `./uploads` | Image upload directory |
| `--ring-buffer-size` | `RING_BUFFER_SIZE` | `16777216` | Ring buffer size in bytes |

A systemd unit file is included at `c3.service`.
