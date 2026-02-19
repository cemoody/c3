package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// PTYManager manages terminal I/O for a tmux pane.
//
// Reading uses `tmux pipe-pane` piping through a FIFO to capture the raw byte
// stream. Direct PTY slave reads don't work because the shell and our process
// compete for data when tmux holds the master end.
//
// Writing uses direct PTY slave writes to inject input bytes.
//
// Resize uses ioctl on the PTY slave fd.
type PTYManager struct {
	tmuxTarget string
	ring       *RingBuffer
	writeCh    chan []byte
	resizeCh   chan [2]uint16 // [cols, rows]
	logger     *slog.Logger

	// onOutput is called with each chunk of PTY output data.
	// Set before calling Open.
	onOutput func(data []byte)

	mu       sync.Mutex
	ptyFile  *os.File // PTY slave fd for writes and resize
	fifoPath string   // path to the FIFO for pipe-pane output
	fifoFile *os.File // read end of the FIFO
	epoch    int64
	stopCh   chan struct{}
}

func NewPTYManager(tmuxTarget string, ring *RingBuffer, logger *slog.Logger) *PTYManager {
	return &PTYManager{
		tmuxTarget: tmuxTarget,
		ring:       ring,
		writeCh:    make(chan []byte, 64),
		resizeCh:   make(chan [2]uint16, 8),
		logger:     logger,
	}
}

// SetTarget updates the tmux target used for pipe-pane commands.
func (p *PTYManager) SetTarget(target string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tmuxTarget = target
}

// Target returns the current tmux target.
func (p *PTYManager) Target() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tmuxTarget
}

// Epoch returns the current session epoch.
func (p *PTYManager) Epoch() int64 {
	return atomic.LoadInt64(&p.epoch)
}

// Open attaches to the PTY for writes/resize and starts tmux pipe-pane for reads.
func (p *PTYManager) Open(ttyPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Open PTY slave for writing input and resize
	f, err := os.OpenFile(ttyPath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open pty slave: %w", err)
	}
	p.ptyFile = f

	// Create a FIFO for tmux pipe-pane output
	tmpDir := os.TempDir()
	fifoPath := filepath.Join(tmpDir, fmt.Sprintf("c3-pipe-%d", os.Getpid()))
	os.Remove(fifoPath) // clean up any stale FIFO
	if err := unix.Mkfifo(fifoPath, 0600); err != nil {
		f.Close()
		return fmt.Errorf("mkfifo: %w", err)
	}
	p.fifoPath = fifoPath

	atomic.AddInt64(&p.epoch, 1)
	p.stopCh = make(chan struct{})

	p.logger.Info("pty opened", "path", ttyPath, "fifo", fifoPath, "epoch", p.Epoch())

	// Start tmux pipe-pane writing to our FIFO.
	pipeCmd := fmt.Sprintf("cat > %s", fifoPath)
	cmd := exec.Command("tmux", "pipe-pane", "-t", p.tmuxTarget, pipeCmd)
	if err := cmd.Run(); err != nil {
		f.Close()
		os.Remove(fifoPath)
		return fmt.Errorf("tmux pipe-pane: %w", err)
	}

	// Open FIFO for reading in a goroutine. The open blocks in O_RDONLY mode
	// until tmux's cat process opens the write end (which happens when the
	// pane first produces output). This is correct POSIX FIFO behavior.
	go p.fifoReadGoroutine(fifoPath, p.stopCh)
	go p.writeLoop(f, p.stopCh)
	go p.resizeLoop(f, p.stopCh)

	return nil
}

// Close stops all goroutines and releases resources.
func (p *PTYManager) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *PTYManager) closeLocked() {
	if p.stopCh != nil {
		close(p.stopCh)
		p.stopCh = nil
	}

	// Stop pipe-pane in tmux
	exec.Command("tmux", "pipe-pane", "-t", p.tmuxTarget).Run()

	if p.fifoFile != nil {
		p.fifoFile.Close()
		p.fifoFile = nil
	}
	if p.fifoPath != "" {
		os.Remove(p.fifoPath)
		p.fifoPath = ""
	}
	if p.ptyFile != nil {
		p.ptyFile.Close()
		p.ptyFile = nil
	}
}

// Reattach closes the existing PTY and opens a new one.
func (p *PTYManager) Reattach(newTTYPath string) error {
	p.Close()
	return p.Open(newTTYPath)
}

// WriteInput sends raw bytes to be written to the PTY.
func (p *PTYManager) WriteInput(data []byte) {
	select {
	case p.writeCh <- data:
	default:
		p.logger.Warn("pty write channel full, dropping input")
	}
}

// Resize sends new dimensions to the PTY.
func (p *PTYManager) Resize(cols, rows uint16) {
	select {
	case p.resizeCh <- [2]uint16{cols, rows}:
	default:
	}
}

func (p *PTYManager) fifoReadGoroutine(fifoPath string, stop chan struct{}) {
	// Open in blocking mode — blocks until a writer (tmux's cat) connects.
	fifoFile, err := os.OpenFile(fifoPath, os.O_RDONLY, 0)
	if err != nil {
		select {
		case <-stop:
			return
		default:
			p.logger.Error("failed to open fifo", "path", fifoPath, "error", err)
			return
		}
	}

	p.mu.Lock()
	p.fifoFile = fifoFile
	p.mu.Unlock()

	p.logger.Info("fifo reader connected", "path", fifoPath)
	p.readLoop(fifoFile, stop)
}

func (p *PTYManager) readLoop(r io.Reader, stop chan struct{}) {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-stop:
			return
		default:
		}

		n, err := r.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			p.ring.Write(data)
			if p.onOutput != nil {
				p.onOutput(data)
			}
		}
		if err != nil {
			select {
			case <-stop:
				return
			default:
				p.logger.Warn("fifo read ended", "error", err)
				return
			}
		}
	}
}

func (p *PTYManager) writeLoop(f *os.File, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case data := <-p.writeCh:
			p.mu.Lock()
			target := p.tmuxTarget
			p.mu.Unlock()

			if target == "" {
				continue
			}

			// Use tmux send-keys -l to inject input as keystrokes.
			// Writing to the PTY slave goes to the output side (display),
			// not the input side (shell). tmux send-keys writes to the
			// master side which the shell reads from.
			cmd := exec.Command("tmux", "send-keys", "-t", target, "-l", "--", string(data))
			if err := cmd.Run(); err != nil {
				select {
				case <-stop:
					return
				default:
					p.logger.Error("tmux send-keys error", "error", err)
				}
			}
		}
	}
}

func (p *PTYManager) resizeLoop(f *os.File, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case dims := <-p.resizeCh:
			ws := &unix.Winsize{
				Col: dims[0],
				Row: dims[1],
			}
			err := unix.IoctlSetWinsize(int(f.Fd()), unix.TIOCSWINSZ, ws)
			if err != nil {
				p.logger.Error("pty resize error", "error", err, "cols", dims[0], "rows", dims[1])
			} else {
				p.logger.Info("pty resized", "cols", dims[0], "rows", dims[1])
			}
		}
	}
}
