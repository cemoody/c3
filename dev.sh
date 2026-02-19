#!/usr/bin/env bash
# dev.sh — watch for changes, rebuild, and restart c3 with minimal downtime
#
# Usage: ./dev.sh [c3 flags...]
# Example: ./dev.sh --listen-addr=:8081
#
# Watches .go, .svelte, .ts, .css, .js, and .html files.
# On change: rebuilds frontend + backend, then restarts c3.
# Ctrl+C to stop.

set -euo pipefail

C3_ARGS=("$@")
C3_PID=""
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export PATH="/home/chris/go-sdk/go/bin:$PATH"

cleanup() {
    echo ""
    echo "[dev] shutting down..."
    if [[ -n "$C3_PID" ]] && kill -0 "$C3_PID" 2>/dev/null; then
        kill "$C3_PID" 2>/dev/null
        wait "$C3_PID" 2>/dev/null || true
    fi
    exit 0
}
trap cleanup EXIT INT TERM

build() {
    echo "[dev] building frontend..."
    (cd "$SCRIPT_DIR/frontend" && npm run build --silent 2>&1) || { echo "[dev] frontend build FAILED"; return 1; }
    echo "[dev] building backend..."
    (cd "$SCRIPT_DIR" && go build -o c3 . 2>&1) || { echo "[dev] go build FAILED"; return 1; }
    echo "[dev] build OK"
    return 0
}

start_c3() {
    echo "[dev] starting c3 ${C3_ARGS[*]:-}"
    "$SCRIPT_DIR/c3" "${C3_ARGS[@]}" &
    C3_PID=$!
    echo "[dev] c3 pid=$C3_PID"
}

stop_c3() {
    if [[ -n "$C3_PID" ]] && kill -0 "$C3_PID" 2>/dev/null; then
        echo "[dev] stopping c3 pid=$C3_PID"
        kill "$C3_PID" 2>/dev/null
        wait "$C3_PID" 2>/dev/null || true
        C3_PID=""
    fi
}

restart() {
    stop_c3
    if build; then
        start_c3
    else
        echo "[dev] build failed — c3 not restarted, waiting for next change..."
    fi
}

# Initial build and start
restart

echo "[dev] watching for changes... (Ctrl+C to stop)"

# Use inotifywait if available, otherwise fall back to polling
if command -v inotifywait &>/dev/null; then
    while true; do
        inotifywait -q -r -e modify,create,delete \
            --include '\.(go|svelte|ts|css|js|html)$' \
            --exclude '(node_modules|dist|\.git)' \
            "$SCRIPT_DIR"
        echo "[dev] change detected, rebuilding..."
        sleep 0.5  # debounce
        restart
    done
else
    echo "[dev] inotifywait not found, using polling (2s interval)"
    LAST_HASH=""
    while true; do
        sleep 2
        # Hash all source files
        HASH=$(find "$SCRIPT_DIR" \
            -path '*/node_modules' -prune -o \
            -path '*/dist' -prune -o \
            -path '*/.git' -prune -o \
            -path '*/e2e/node_modules' -prune -o \
            \( -name '*.go' -o -name '*.svelte' -o -name '*.ts' -o -name '*.css' -o -name '*.html' \) \
            -newer "$SCRIPT_DIR/c3" -print 2>/dev/null | head -1)
        if [[ -n "$HASH" ]]; then
            echo "[dev] change detected, rebuilding..."
            restart
        fi
    done
fi
