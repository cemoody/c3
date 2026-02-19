package main

import (
	"fmt"
	"sync"
)

// RingBuffer is a circular byte buffer with monotonically increasing write position.
// The actual ring index is writePos % size. A reader at offset C is behind if
// writePos - C > size (its data has been overwritten).
type RingBuffer struct {
	mu       sync.Mutex
	buf      []byte
	size     int
	writePos int64 // total bytes written (monotonically increasing)
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Write appends data to the ring buffer.
func (rb *RingBuffer) Write(data []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for len(data) > 0 {
		idx := int(rb.writePos % int64(rb.size))
		n := copy(rb.buf[idx:], data)
		rb.writePos += int64(n)
		data = data[n:]
	}
}

// WritePos returns the current write position (total bytes written).
func (rb *RingBuffer) WritePos() int64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.writePos
}

// oldestOffset returns the offset of the oldest available byte (caller must hold mu).
func (rb *RingBuffer) oldestOffset() int64 {
	if rb.writePos <= int64(rb.size) {
		return 0
	}
	return rb.writePos - int64(rb.size)
}

// ReadFrom reads bytes starting at the given offset into dst.
// Returns the number of bytes read and the next offset to read from.
// If offset is behind the oldest available byte, returns ErrOverwritten
// with the fast-forwarded offset.
func (rb *RingBuffer) ReadFrom(offset int64, dst []byte) (int, int64, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	oldest := rb.oldestOffset()

	if offset < oldest {
		return 0, oldest, fmt.Errorf("data overwritten: requested offset %d, oldest available %d", offset, oldest)
	}
	if offset >= rb.writePos {
		// Nothing new to read.
		return 0, offset, nil
	}

	available := int(rb.writePos - offset)
	if available > len(dst) {
		available = len(dst)
	}

	read := 0
	pos := offset
	for read < available {
		idx := int(pos % int64(rb.size))
		end := idx + (available - read)
		if end > rb.size {
			end = rb.size
		}
		n := copy(dst[read:], rb.buf[idx:end])
		read += n
		pos += int64(n)
	}

	return read, offset + int64(read), nil
}

// Tail returns the last n bytes from the buffer (or fewer if less data is available)
// and the offset at which the returned data begins.
func (rb *RingBuffer) Tail(n int) ([]byte, int64) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	oldest := rb.oldestOffset()
	available := int(rb.writePos - oldest)
	if n > available {
		n = available
	}
	if n == 0 {
		return nil, rb.writePos
	}

	startOffset := rb.writePos - int64(n)
	result := make([]byte, n)

	read := 0
	pos := startOffset
	for read < n {
		idx := int(pos % int64(rb.size))
		end := idx + (n - read)
		if end > rb.size {
			end = rb.size
		}
		copied := copy(result[read:], rb.buf[idx:end])
		read += copied
		pos += int64(copied)
	}

	return result, startOffset
}

// TailFromRedraw searches backwards through the last maxSearch bytes for the
// most recent terminal redraw point (clear screen, alternate screen enter, or
// cursor home) and returns data from that point. This gives xterm.js a clean
// starting state without needing the full buffer.
// Falls back to the last maxSearch bytes if no redraw point is found.
func (rb *RingBuffer) TailFromRedraw(maxSearch int) ([]byte, int64) {
	// Get the raw tail to search through
	tail, startOffset := rb.Tail(maxSearch)
	if len(tail) == 0 {
		return tail, startOffset
	}

	// Search backwards for redraw markers.
	// \x1b[2J     = clear entire screen
	// \x1b[?1049h = enter alternate screen buffer
	// \x1b[H\x1b[2J or \x1b[2J\x1b[H = home + clear (common combo)
	markers := [][]byte{
		{0x1b, '[', '2', 'J'},      // clear screen
		{0x1b, '[', '?', '1', '0', '4', '9', 'h'}, // alternate screen
	}

	bestPos := -1
	for _, marker := range markers {
		// Search backwards by finding the last occurrence
		for i := len(tail) - len(marker); i >= 0; i-- {
			match := true
			for j := 0; j < len(marker); j++ {
				if tail[i+j] != marker[j] {
					match = false
					break
				}
			}
			if match {
				if i > bestPos {
					bestPos = i
				}
				break // found the last occurrence of this marker
			}
		}
	}

	if bestPos >= 0 {
		return tail[bestPos:], startOffset + int64(bestPos)
	}

	// No redraw point found; return the full tail
	return tail, startOffset
}

// Snapshot returns the entire available buffer contents in write order
// and the offset at which the data begins.
func (rb *RingBuffer) Snapshot() ([]byte, int64) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	oldest := rb.oldestOffset()
	available := int(rb.writePos - oldest)
	if available == 0 {
		return nil, rb.writePos
	}

	result := make([]byte, available)

	read := 0
	pos := oldest
	for read < available {
		idx := int(pos % int64(rb.size))
		end := idx + (available - read)
		if end > rb.size {
			end = rb.size
		}
		copied := copy(result[read:], rb.buf[idx:end])
		read += copied
		pos += int64(copied)
	}

	return result, oldest
}
