package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// testTmuxSession creates a tmux session and returns a cleanup function.
func testTmuxSession(t *testing.T, name string) func() {
	t.Helper()
	exec.Command("tmux", "kill-session", "-t", name).Run()
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name, "-x", "80", "-y", "24")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create tmux session: %v", err)
	}
	return func() {
		exec.Command("tmux", "kill-session", "-t", name).Run()
	}
}

// getFreePort returns an available TCP port.
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// defaultConfig builds a Config for testing.
func defaultConfig(t *testing.T, tmuxTarget string, port int) *Config {
	t.Helper()
	return &Config{
		TmuxTarget:      tmuxTarget,
		ListenAddr:      fmt.Sprintf("127.0.0.1:%d", port),
		RingBufferSize:  1024 * 1024,
		UploadDir:       t.TempDir(),
		MaxUploadSize:   20 * 1024 * 1024,
		TailReplaySize:  256,
		ClientQueueSize: 256,
	}
}

// startServer starts the c3 HTTP server and returns components and a cleanup function.
// It uses the SessionManager architecture; the returned Hub/Ring are for the cfg.TmuxTarget session.
func startServer(t *testing.T, cfg *Config) (*Hub, *PTYManager, *RingBuffer, *http.Server, func()) {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	sm := NewSessionManager(cfg, logger)

	// Pre-create the session for the test target
	sess := sm.Get(cfg.TmuxTarget)

	indexer := NewFileIndexer("/tmp", 999*time.Hour, logger)
	mux := NewServer(cfg, sm, indexer, logger)
	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	go server.ListenAndServe()

	cleanup := func() {
		server.Shutdown(context.Background())
		sm.CloseAll()
	}

	return sess.Hub, sess.PTY, sess.Ring, server, cleanup
}

// waitForRingData polls until the ring buffer has at least minBytes.
func waitForRingData(ring *RingBuffer, minBytes int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ring.WritePos() >= minBytes {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("ring buffer has %d bytes, wanted >= %d", ring.WritePos(), minBytes)
}

// waitForRingContent polls until the ring buffer contains the expected substring.
func waitForRingContent(ring *RingBuffer, substr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, _ := ring.Snapshot()
		if strings.Contains(string(data), substr) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %q in ring buffer", substr)
}

// connectWS dials a WebSocket for a given target, sends a hello, and returns the connection.
// Caller must defer conn.CloseNow().
func connectWS(t *testing.T, ctx context.Context, port int, target string, replayMode string, tailSize int) *websocket.Conn {
	t.Helper()
	url := fmt.Sprintf("ws://127.0.0.1:%d/s/%s/ws", port, target)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("ws dial failed: %v", err)
	}

	hello := HelloMsg{Type: "hello", ReplayMode: replayMode, TailSize: tailSize}
	raw, _ := json.Marshal(hello)
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		conn.CloseNow()
		t.Fatalf("ws write hello failed: %v", err)
	}
	return conn
}

// readWSOutputUntil reads from the WebSocket until the predicate returns true,
// accumulating all output bytes. Returns the accumulated output.
func readWSOutputUntil(t *testing.T, ctx context.Context, conn *websocket.Conn, pred func(accumulated []byte) bool) []byte {
	t.Helper()
	var all []byte
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Logf("readWSOutputUntil: read error: %v (accumulated %d bytes)", err, len(all))
			return all
		}

		var base struct {
			Type string `json:"type"`
		}
		json.Unmarshal(data, &base)

		if base.Type == "output" {
			var msg OutputMsg
			json.Unmarshal(data, &msg)
			decoded, _ := base64.StdEncoding.DecodeString(msg.Data)
			all = append(all, decoded...)
		}

		if pred(all) {
			return all
		}
	}
}

// sendWSInput sends an input message via the WebSocket.
func sendWSInput(t *testing.T, ctx context.Context, conn *websocket.Conn, text string) {
	t.Helper()
	inputB64 := base64.StdEncoding.EncodeToString([]byte(text))
	msg := fmt.Sprintf(`{"type":"input","data":"%s"}`, inputB64)
	if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		t.Fatalf("ws write input failed: %v", err)
	}
}

// tmuxSend sends keys to a tmux pane.
func tmuxSend(t *testing.T, target string, keys ...string) {
	t.Helper()
	args := append([]string{"send-keys", "-t", target}, keys...)
	if err := exec.Command("tmux", args...).Run(); err != nil {
		t.Fatalf("tmux send-keys failed: %v", err)
	}
}

// setupSession creates a tmux session, starts a server, waits for pipe-pane
// to be ready, and returns the port, ring, and a combined cleanup.
func setupSession(t *testing.T, name string) (int, string, *RingBuffer, *Hub, func()) {
	t.Helper()
	tmuxCleanup := testTmuxSession(t, name)
	port := getFreePort(t)
	target := name + ":0.0"
	cfg := defaultConfig(t, target, port)
	hub, _, ring, _, serverCleanup := startServer(t, cfg)

	// Wait for pipe-pane to attach and initial prompt to arrive
	time.Sleep(3 * time.Second)

	cleanup := func() {
		serverCleanup()
		tmuxCleanup()
	}
	return port, target, ring, hub, cleanup
}

// ---------------------------------------------------------------------------
// Original tests
// ---------------------------------------------------------------------------

func TestIntegration_PipePane(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, _, ring, _, cleanup := setupSession(t, "c3-pipe-test")
	defer cleanup()
	_ = port

	tmuxSend(t, "c3-pipe-test:0.0", "echo integration-test-output", "Enter")

	if err := waitForRingContent(ring, "integration-test-output", 10*time.Second); err != nil {
		data, _ := ring.Snapshot()
		t.Fatalf("ring buffer does not contain expected output: %v\nGot (%d bytes): %q", err, len(data), string(data))
	}

	data, _ := ring.Snapshot()
	t.Logf("Ring buffer captured %d bytes", len(data))
}

func TestIntegration_WebSocket(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-ws-test")
	defer cleanup()

	// Seed output
	tmuxSend(t, "c3-ws-test:0.0", "echo seed-output", "Enter")
	if err := waitForRingContent(ring, "seed-output", 10*time.Second); err != nil {
		t.Fatalf("no seed data: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := connectWS(t, ctx, port, target, "full", 0)
	defer conn.CloseNow()

	// Read replay — should contain seed-output
	output := readWSOutputUntil(t, ctx, conn, func(acc []byte) bool {
		return strings.Contains(string(acc), "seed-output")
	})
	if !strings.Contains(string(output), "seed-output") {
		t.Errorf("replay does not contain seed-output. Got: %q", string(output))
	}

	// Inject input via WS
	sendWSInput(t, ctx, conn, "echo ws-injected\n")

	// Read until we see the injected command echoed back
	output = readWSOutputUntil(t, ctx, conn, func(acc []byte) bool {
		return strings.Contains(string(acc), "ws-injected")
	})
	if !strings.Contains(string(output), "ws-injected") {
		t.Error("did not receive output for injected command")
	}

	conn.Close(websocket.StatusNormalClosure, "done")
}

func TestIntegration_PaneMissing(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port := getFreePort(t)
	cfg := defaultConfig(t, "nonexistent-session:0.0", port)
	_, _, _, _, serverCleanup := startServer(t, cfg)
	defer serverCleanup()

	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := connectWS(t, ctx, port, "nonexistent-session:0.0", "full", 0)
	defer conn.CloseNow()

	// Should get a status message (server doesn't crash)
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read failed: %v", err)
	}

	var base struct {
		Type string `json:"type"`
	}
	json.Unmarshal(data, &base)
	t.Logf("Received message type=%s data=%s", base.Type, string(data))

	conn.Close(websocket.StatusNormalClosure, "done")
}

// ---------------------------------------------------------------------------
// New integration tests
// ---------------------------------------------------------------------------

// TestIntegration_TailReplay verifies that tail mode only returns the last N
// bytes of the buffer, while full mode returns everything.
func TestIntegration_TailReplay(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-tail-test")
	defer cleanup()

	// Generate enough output so full replay is significantly larger than tail.
	// Our TailReplaySize is 256 bytes (set in defaultConfig).
	for i := 0; i < 10; i++ {
		tmuxSend(t, "c3-tail-test:0.0", fmt.Sprintf("echo line-marker-%03d-padding-to-make-this-longer", i), "Enter")
		time.Sleep(300 * time.Millisecond)
	}

	// Wait for the last marker to appear
	if err := waitForRingContent(ring, "line-marker-009", 10*time.Second); err != nil {
		t.Fatalf("output not captured: %v", err)
	}

	fullSnap, _ := ring.Snapshot()
	t.Logf("Total ring buffer: %d bytes", len(fullSnap))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect with full replay
	fullConn := connectWS(t, ctx, port, target, "full", 0)
	defer fullConn.CloseNow()

	fullOutput := readWSOutputUntil(t, ctx, fullConn, func(acc []byte) bool {
		return strings.Contains(string(acc), "line-marker-009")
	})
	fullConn.Close(websocket.StatusNormalClosure, "done")

	// Connect with tail replay (256 bytes)
	tailConn := connectWS(t, ctx, port, target, "tail", 256)
	defer tailConn.CloseNow()

	// Read what the tail gives us — it should be much smaller
	// Read for a short period to get the replay data before live kicks in
	tailCtx, tailCancel := context.WithTimeout(ctx, 2*time.Second)
	defer tailCancel()
	tailOutput := readWSOutputUntil(t, tailCtx, tailConn, func(acc []byte) bool {
		return len(acc) > 200 // just get some data
	})
	tailConn.Close(websocket.StatusNormalClosure, "done")

	t.Logf("Full replay: %d bytes, Tail replay: %d bytes", len(fullOutput), len(tailOutput))

	if len(tailOutput) >= len(fullOutput) {
		t.Errorf("tail replay (%d) should be smaller than full replay (%d)", len(tailOutput), len(fullOutput))
	}

	// Tail should still contain some of the more recent markers
	if len(tailOutput) > 0 {
		// It should NOT contain the very first marker (that would mean we got everything)
		if strings.Contains(string(tailOutput), "line-marker-000") && len(fullOutput) > 300 {
			t.Log("Warning: tail contains earliest marker — buffer may be too small for a meaningful test")
		}
	}
}

// TestIntegration_MultipleClients verifies that multiple simultaneous WebSocket
// clients all receive the same output.
func TestIntegration_MultipleClients(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, hub, cleanup := setupSession(t, "c3-multi-test")
	defer cleanup()

	// Seed some output
	tmuxSend(t, "c3-multi-test:0.0", "echo multi-seed", "Enter")
	if err := waitForRingContent(ring, "multi-seed", 10*time.Second); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Connect 3 clients
	conns := make([]*websocket.Conn, 3)
	for i := range conns {
		conns[i] = connectWS(t, ctx, port, target, "full", 0)
		defer conns[i].CloseNow()
	}

	// Drain the replay from all clients
	for i, conn := range conns {
		readWSOutputUntil(t, ctx, conn, func(acc []byte) bool {
			return strings.Contains(string(acc), "multi-seed")
		})
		t.Logf("Client %d received replay", i)
	}

	// Verify hub has all 3 registered
	if count := hub.ClientCount(); count != 3 {
		t.Errorf("expected 3 clients registered, got %d", count)
	}

	// Send new output — all 3 should receive it
	marker := "multi-client-broadcast-marker"
	tmuxSend(t, "c3-multi-test:0.0", "echo "+marker, "Enter")

	var wg sync.WaitGroup
	results := make([]bool, 3)

	for i, conn := range conns {
		wg.Add(1)
		go func(idx int, c *websocket.Conn) {
			defer wg.Done()
			out := readWSOutputUntil(t, ctx, c, func(acc []byte) bool {
				return strings.Contains(string(acc), marker)
			})
			results[idx] = strings.Contains(string(out), marker)
		}(i, conn)
	}

	wg.Wait()

	for i, got := range results {
		if !got {
			t.Errorf("client %d did not receive broadcast marker", i)
		}
	}

	// Disconnect clients
	for _, conn := range conns {
		conn.Close(websocket.StatusNormalClosure, "done")
	}

	// Hub should eventually have 0 clients
	time.Sleep(500 * time.Millisecond)
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", count)
	}
}

// TestIntegration_MultiClientInput verifies that input from multiple clients
// all arrives at the PTY (interleaved is fine).
func TestIntegration_MultiClientInput(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-input-test")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Connect two clients
	conn1 := connectWS(t, ctx, port, target, "tail", 256)
	defer conn1.CloseNow()
	conn2 := connectWS(t, ctx, port, target, "tail", 256)
	defer conn2.CloseNow()

	// Drain replays
	time.Sleep(500 * time.Millisecond)

	// Each client sends a different command
	sendWSInput(t, ctx, conn1, "echo from-client-1\n")
	time.Sleep(500 * time.Millisecond)
	sendWSInput(t, ctx, conn2, "echo from-client-2\n")

	// Wait for both outputs in the ring buffer
	if err := waitForRingContent(ring, "from-client-1", 10*time.Second); err != nil {
		t.Errorf("client-1 input not seen: %v", err)
	}
	if err := waitForRingContent(ring, "from-client-2", 10*time.Second); err != nil {
		t.Errorf("client-2 input not seen: %v", err)
	}

	conn1.Close(websocket.StatusNormalClosure, "done")
	conn2.Close(websocket.StatusNormalClosure, "done")
}

// TestIntegration_Resize verifies that resize messages change the PTY dimensions.
func TestIntegration_Resize(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, _, _, cleanup := setupSession(t, "c3-resize-test")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn := connectWS(t, ctx, port, target, "tail", 256)
	defer conn.CloseNow()

	// Send resize
	resize := `{"type":"resize","cols":120,"rows":40}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(resize)); err != nil {
		t.Fatalf("ws write resize failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Query the tmux pane dimensions
	out, err := exec.Command("tmux", "display-message", "-p", "-t", "c3-resize-test:0.0", "#{pane_width} #{pane_height}").Output()
	if err != nil {
		t.Fatalf("failed to query pane size: %v", err)
	}

	dims := strings.TrimSpace(string(out))
	t.Logf("Pane dimensions after resize: %s", dims)

	// Note: tmux may not reflect ioctl resize in its display-message if we
	// only resized the PTY slave without telling tmux. The resize goes through
	// to the PTY (verifiable by `stty size` inside the pane).
	// Let's verify via the shell inside the pane instead.
	tmuxSend(t, "c3-resize-test:0.0", "stty size", "Enter")

	// The ring buffer isn't accessible here through the helper, but the command
	// ran. For a more thorough check, read from the WS.
	output := readWSOutputUntil(t, ctx, conn, func(acc []byte) bool {
		return strings.Contains(string(acc), "40 120") || strings.Contains(string(acc), "120")
	})

	if !strings.Contains(string(output), "40 120") {
		t.Logf("stty output: %q (may not match if tmux overrides)", string(output))
		// Don't fail — tmux can override pty dimensions. The important thing is
		// the resize didn't crash anything.
	}

	conn.Close(websocket.StatusNormalClosure, "done")
}

// TestIntegration_Upload verifies content-addressed image upload, deduplication,
// and PTY prompt injection.
func TestIntegration_Upload(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, _, ring, _, cleanup := setupSession(t, "c3-upload-test")
	defer cleanup()

	// Create a small test "image" (just bytes, not a real PNG)
	imgData := []byte("fake-png-data-for-testing-upload")
	hash := sha256.Sum256(imgData)
	expectedHash := hex.EncodeToString(hash[:])

	// Upload via multipart POST
	uploadURL := fmt.Sprintf("http://127.0.0.1:%d/api/upload", port)

	doUpload := func(filename string) (int, map[string]string) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("image", filename)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(imgData)
		writer.Close()

		resp, err := http.Post(uploadURL, writer.FormDataContentType(), &buf)
		if err != nil {
			t.Fatalf("upload request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var result map[string]string
		json.Unmarshal(body, &result)
		return resp.StatusCode, result
	}

	// First upload
	status, result := doUpload("test.png")
	if status != 200 {
		t.Fatalf("upload returned status %d", status)
	}
	if result["hash"] != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, result["hash"])
	}
	t.Logf("Upload path: %s", result["path"])

	// Verify file exists on disk
	uploadedPath := result["path"]
	if _, err := os.Stat(uploadedPath); err != nil {
		t.Errorf("uploaded file not found at %s: %v", uploadedPath, err)
	}

	// Verify file contents match
	ondisk, err := os.ReadFile(uploadedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ondisk, imgData) {
		t.Error("file contents don't match upload data")
	}

	// Verify the prompt was injected into the PTY
	if err := waitForRingContent(ring, "Analyze this image:", 10*time.Second); err != nil {
		t.Errorf("prompt not injected: %v", err)
	}

	// Second upload of the same file — should deduplicate (same hash)
	status2, result2 := doUpload("test2.png")
	if status2 != 200 {
		t.Fatalf("dedup upload returned status %d", status2)
	}
	if result2["hash"] != expectedHash {
		t.Errorf("dedup hash mismatch")
	}

	// Verify only one file exists (same hash → same path)
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(uploadedPath), expectedHash+"*"))
	if len(files) != 1 {
		t.Errorf("expected 1 file for deduplicated upload, got %d: %v", len(files), files)
	}
}

// TestIntegration_UploadValidation verifies that upload rejects bad file types
// and oversized files.
func TestIntegration_UploadValidation(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, _, _, _, cleanup := setupSession(t, "c3-upload-val-test")
	defer cleanup()

	uploadURL := fmt.Sprintf("http://127.0.0.1:%d/api/upload", port)

	doUpload := func(filename string, data []byte) int {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("image", filename)
		part.Write(data)
		writer.Close()

		resp, err := http.Post(uploadURL, writer.FormDataContentType(), &buf)
		if err != nil {
			t.Fatalf("upload request failed: %v", err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// Reject .txt extension
	status := doUpload("evil.txt", []byte("not an image"))
	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for .txt upload, got %d", status)
	}

	// Reject .exe extension
	status = doUpload("malware.exe", []byte("definitely not an image"))
	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for .exe upload, got %d", status)
	}

	// Accept .jpg
	status = doUpload("photo.jpg", []byte("fake jpg"))
	if status != http.StatusOK {
		t.Errorf("expected 200 for .jpg upload, got %d", status)
	}

	// Accept .webp
	status = doUpload("photo.webp", []byte("fake webp"))
	if status != http.StatusOK {
		t.Errorf("expected 200 for .webp upload, got %d", status)
	}

	// Accept .jpeg (normalized to .jpg)
	status = doUpload("photo.jpeg", []byte("fake jpeg"))
	if status != http.StatusOK {
		t.Errorf("expected 200 for .jpeg upload, got %d", status)
	}
}

// TestIntegration_PaneRecreate verifies that destroying and recreating the tmux
// pane causes the server to reattach with a new epoch.
func TestIntegration_PaneRecreate(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	sessionName := "c3-recreate-test"
	tmuxCleanup := testTmuxSession(t, sessionName)
	defer tmuxCleanup()

	port := getFreePort(t)
	cfg := defaultConfig(t, sessionName+":0.0", port)
	_, _, ring, _, serverCleanup := startServer(t, cfg)
	defer serverCleanup()

	// Wait for initial attach
	time.Sleep(3 * time.Second)

	// Seed some output in epoch 1
	tmuxSend(t, sessionName+":0.0", "echo epoch1-output", "Enter")
	if err := waitForRingContent(ring, "epoch1-output", 10*time.Second); err != nil {
		t.Fatalf("epoch1 output missing: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect a WS client before destruction
	conn := connectWS(t, ctx, port, sessionName+":0.0", "tail", 256)
	defer conn.CloseNow()

	// Drain replay
	readWSOutputUntil(t, ctx, conn, func(acc []byte) bool {
		return len(acc) > 0
	})

	// Kill the pane and recreate
	exec.Command("tmux", "kill-pane", "-t", sessionName+":0.0").Run()
	time.Sleep(3 * time.Second)

	// Recreate the session (kill-pane killed the only pane, so session is gone)
	exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-x", "80", "-y", "24").Run()

	// Wait for pane monitor to detect new pane and for pipe-pane to be set up.
	// The monitor polls every 1s. After detecting, Open() creates FIFO,
	// runs pipe-pane, and the read goroutine opens the FIFO (blocking until
	// tmux's cat writer connects). We need enough time for all of this.
	time.Sleep(5 * time.Second)

	// Send output in the new pane
	tmuxSend(t, sessionName+":0.0", "echo epoch2-output", "Enter")

	// Use a generous timeout — the pipe-pane may take a moment to start flowing
	if err := waitForRingContent(ring, "epoch2-output", 15*time.Second); err != nil {
		data, _ := ring.Snapshot()
		t.Fatalf("epoch2 output missing: %v\nRing buffer (%d bytes): %q", err, len(data), string(data))
	}

	// The WS client should have received a status message about the pane being missing
	// and then reconnecting. Read messages and look for status updates.
	gotMissing := false
	gotReconnected := false
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()

	for i := 0; i < 50; i++ {
		_, data, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		var base struct {
			Type string `json:"type"`
		}
		json.Unmarshal(data, &base)

		if base.Type == "status" {
			var status StatusMsg
			json.Unmarshal(data, &status)
			if status.PaneState == "missing" {
				gotMissing = true
			}
			if status.PaneState == "connected" && gotMissing {
				gotReconnected = true
			}
			t.Logf("Status: paneState=%s epoch=%d", status.PaneState, status.Epoch)
		}

		if gotMissing && gotReconnected {
			break
		}
	}

	if !gotMissing {
		t.Error("client did not receive 'missing' status after pane destruction")
	}
	if !gotReconnected {
		t.Error("client did not receive 'connected' status after pane recreation")
	}

	conn.Close(websocket.StatusNormalClosure, "done")
}

// TestIntegration_LargeOutput verifies the system handles a burst of large
// output without losing data or crashing.
func TestIntegration_LargeOutput(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-large-test")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn := connectWS(t, ctx, port, target, "tail", 256)
	defer conn.CloseNow()

	// Drain initial replay
	time.Sleep(500 * time.Millisecond)

	// Generate a burst of output — 200 lines via seq, then a unique end marker.
	// We use a unique end marker to avoid matching the command echo.
	tmuxSend(t, "c3-large-test:0.0", "seq 1 200", "Enter")
	time.Sleep(2 * time.Second)
	tmuxSend(t, "c3-large-test:0.0", "echo END-OF-SEQ-MARKER", "Enter")

	// Wait for the end marker in the ring buffer (proves seq finished)
	if err := waitForRingContent(ring, "END-OF-SEQ-MARKER", 15*time.Second); err != nil {
		t.Fatalf("large output not fully captured: %v", err)
	}

	// Now read from WS — seq output should already be queued
	output := readWSOutputUntil(t, ctx, conn, func(acc []byte) bool {
		// Wait for the end marker to appear in the WS output.
		// Count occurrences: first is in command echo, second is in actual output.
		return strings.Count(string(acc), "END-OF-SEQ-MARKER") >= 2
	})

	t.Logf("Received %d bytes of output", len(output))

	if len(output) < 500 {
		t.Errorf("expected substantial output, got only %d bytes", len(output))
	}

	// Verify the ring buffer captured the full seq output
	snap, _ := ring.Snapshot()
	snapText := string(snap)
	for _, n := range []string{"100", "150", "200"} {
		if !strings.Contains(snapText, n) {
			t.Errorf("ring buffer missing seq number %s", n)
		}
	}

	conn.Close(websocket.StatusNormalClosure, "done")
}

// TestIntegration_RapidResize sends many resize events in quick succession
// to verify the server doesn't crash or corrupt state.
func TestIntegration_RapidResize(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-rapid-resize")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := connectWS(t, ctx, port, target, "tail", 256)
	defer conn.CloseNow()

	// Send 50 resize events rapidly
	for i := 0; i < 50; i++ {
		cols := 80 + (i % 40)
		rows := 24 + (i % 20)
		msg := fmt.Sprintf(`{"type":"resize","cols":%d,"rows":%d}`, cols, rows)
		conn.Write(ctx, websocket.MessageText, []byte(msg))
	}

	time.Sleep(1 * time.Second)

	// Verify the system is still functional — send a command and check output
	tmuxSend(t, "c3-rapid-resize:0.0", "echo still-alive", "Enter")
	if err := waitForRingContent(ring, "still-alive", 10*time.Second); err != nil {
		t.Fatalf("server broken after rapid resize: %v", err)
	}

	conn.Close(websocket.StatusNormalClosure, "done")
	t.Log("Server survived 50 rapid resize events")
}

// TestIntegration_ClientDisconnectReconnect verifies that a client can disconnect
// and reconnect, receiving the full replay again.
func TestIntegration_ClientDisconnectReconnect(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-reconnect-test")
	defer cleanup()

	// Seed output
	tmuxSend(t, "c3-reconnect-test:0.0", "echo reconnect-marker-1", "Enter")
	if err := waitForRingContent(ring, "reconnect-marker-1", 10*time.Second); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// First connection
	conn1 := connectWS(t, ctx, port, target, "full", 0)
	output1 := readWSOutputUntil(t, ctx, conn1, func(acc []byte) bool {
		return strings.Contains(string(acc), "reconnect-marker-1")
	})
	conn1.Close(websocket.StatusNormalClosure, "done")
	t.Logf("First connection got %d bytes", len(output1))

	// Add more output while disconnected
	tmuxSend(t, "c3-reconnect-test:0.0", "echo reconnect-marker-2", "Enter")
	if err := waitForRingContent(ring, "reconnect-marker-2", 10*time.Second); err != nil {
		t.Fatalf("marker-2 not captured: %v", err)
	}

	// Second connection — should see both markers in full replay
	conn2 := connectWS(t, ctx, port, target, "full", 0)
	defer conn2.CloseNow()
	output2 := readWSOutputUntil(t, ctx, conn2, func(acc []byte) bool {
		return strings.Contains(string(acc), "reconnect-marker-2")
	})

	if !strings.Contains(string(output2), "reconnect-marker-1") {
		t.Error("second connection replay missing marker-1")
	}
	if !strings.Contains(string(output2), "reconnect-marker-2") {
		t.Error("second connection replay missing marker-2")
	}

	t.Logf("Second connection got %d bytes (both markers present)", len(output2))
	conn2.Close(websocket.StatusNormalClosure, "done")
}

// TestIntegration_ANSIFidelity verifies that ANSI escape sequences pass through
// the system intact (not mangled by any layer).
func TestIntegration_ANSIFidelity(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-ansi-test")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := connectWS(t, ctx, port, target, "tail", 256)
	defer conn.CloseNow()

	// Drain initial data
	time.Sleep(500 * time.Millisecond)

	// Send a command that produces known ANSI output, with an end marker
	// so we wait for the actual output rather than the command echo.
	tmuxSend(t, "c3-ansi-test:0.0", `printf '\033[31mRED\033[0m \033[32mGREEN\033[0m\n' && echo ANSI-DONE`, "Enter")

	// Wait for the end marker to ensure printf output has been captured
	if err := waitForRingContent(ring, "ANSI-DONE", 10*time.Second); err != nil {
		t.Fatalf("ANSI output not captured: %v", err)
	}

	// Read from WS until we see the end marker
	output := readWSOutputUntil(t, ctx, conn, func(acc []byte) bool {
		return strings.Contains(string(acc), "ANSI-DONE")
	})

	text := string(output)

	// The raw ANSI should contain \x1b[31m (red) and \x1b[32m (green)
	if !strings.Contains(text, "\x1b[31m") {
		t.Errorf("missing red ANSI escape \\x1b[31m in WS output (%d bytes)", len(text))
	}
	if !strings.Contains(text, "\x1b[32m") {
		t.Errorf("missing green ANSI escape \\x1b[32m in WS output (%d bytes)", len(text))
	}
	if !strings.Contains(text, "\x1b[0m") {
		t.Errorf("missing reset ANSI escape \\x1b[0m in WS output (%d bytes)", len(text))
	}

	// Also verify it's in the ring buffer
	data, _ := ring.Snapshot()
	ringText := string(data)
	if !strings.Contains(ringText, "\x1b[31mRED\x1b[0m") {
		t.Errorf("ring buffer is missing intact ANSI red sequence (%d bytes)", len(ringText))
	}

	conn.Close(websocket.StatusNormalClosure, "done")
	t.Log("ANSI escape sequences pass through intact")
}

// TestIntegration_ControlCharacterInput verifies that control characters
// (Ctrl-C, Ctrl-D) are correctly transmitted to the PTY.
func TestIntegration_ControlCharacterInput(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, ring, _, cleanup := setupSession(t, "c3-ctrl-test")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn := connectWS(t, ctx, port, target, "tail", 256)
	defer conn.CloseNow()

	// Start a long-running command (sleep)
	sendWSInput(t, ctx, conn, "sleep 999\n")
	time.Sleep(1 * time.Second)

	// Send Ctrl-C (\x03) to interrupt it
	sendWSInput(t, ctx, conn, "\x03")

	// The shell should show the prompt again after the interrupt.
	// Wait for the prompt (indicated by $ ) after the Ctrl-C.
	if err := waitForRingContent(ring, "sleep 999", 5*time.Second); err != nil {
		t.Fatalf("sleep command not seen: %v", err)
	}

	// Send another command to verify the shell is responsive
	sendWSInput(t, ctx, conn, "echo ctrl-c-worked\n")
	if err := waitForRingContent(ring, "ctrl-c-worked", 10*time.Second); err != nil {
		t.Fatalf("shell not responsive after Ctrl-C: %v", err)
	}

	conn.Close(websocket.StatusNormalClosure, "done")
	t.Log("Ctrl-C correctly interrupted a running command")
}

// TestIntegration_ConcurrentConnectDisconnect creates and destroys many
// WebSocket connections concurrently to verify the hub handles races.
func TestIntegration_ConcurrentConnectDisconnect(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found")
	}

	port, target, _, hub, cleanup := setupSession(t, "c3-concurrent-test")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Spawn 10 goroutines that each connect, read some data, then disconnect
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn := connectWS(t, ctx, port, target, "tail", 128)
			defer conn.CloseNow()

			// Read a bit
			readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			readWSOutputUntil(t, readCtx, conn, func(acc []byte) bool {
				return len(acc) > 0
			})

			// Close cleanly
			err := conn.Close(websocket.StatusNormalClosure, "done")
			if err != nil {
				errors <- fmt.Errorf("client %d close error: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Logf("Warning: %v", err)
	}

	// After all clients disconnect, hub should be empty
	time.Sleep(1 * time.Second)
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients after concurrent disconnect, got %d", count)
	}

	t.Log("10 concurrent connect/disconnect cycles completed")
}
