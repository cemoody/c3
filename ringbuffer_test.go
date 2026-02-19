package main

import (
	"bytes"
	"testing"
)

func TestRingBufferBasicWriteRead(t *testing.T) {
	rb := NewRingBuffer(64)

	rb.Write([]byte("hello world"))
	if rb.WritePos() != 11 {
		t.Fatalf("expected writePos 11, got %d", rb.WritePos())
	}

	data, offset := rb.Snapshot()
	if offset != 0 {
		t.Fatalf("expected offset 0, got %d", offset)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(data))
	}
}

func TestRingBufferWrap(t *testing.T) {
	rb := NewRingBuffer(16)

	// Write 20 bytes into a 16-byte buffer
	rb.Write([]byte("0123456789"))
	rb.Write([]byte("abcdefghij"))

	if rb.WritePos() != 20 {
		t.Fatalf("expected writePos 20, got %d", rb.WritePos())
	}

	// Oldest available should be offset 4 (20 - 16)
	data, offset := rb.Snapshot()
	if offset != 4 {
		t.Fatalf("expected offset 4, got %d", offset)
	}
	if string(data) != "456789abcdefghij" {
		t.Fatalf("expected '456789abcdefghij', got %q", string(data))
	}
}

func TestRingBufferTail(t *testing.T) {
	rb := NewRingBuffer(64)
	rb.Write([]byte("hello world"))

	data, offset := rb.Tail(5)
	if offset != 6 {
		t.Fatalf("expected offset 6, got %d", offset)
	}
	if string(data) != "world" {
		t.Fatalf("expected 'world', got %q", string(data))
	}
}

func TestRingBufferTailExceedsAvailable(t *testing.T) {
	rb := NewRingBuffer(64)
	rb.Write([]byte("hi"))

	data, offset := rb.Tail(100)
	if offset != 0 {
		t.Fatalf("expected offset 0, got %d", offset)
	}
	if string(data) != "hi" {
		t.Fatalf("expected 'hi', got %q", string(data))
	}
}

func TestRingBufferReadFrom(t *testing.T) {
	rb := NewRingBuffer(64)
	rb.Write([]byte("hello world"))

	dst := make([]byte, 5)
	n, next, err := rb.ReadFrom(6, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}
	if next != 11 {
		t.Fatalf("expected next offset 11, got %d", next)
	}
	if string(dst[:n]) != "world" {
		t.Fatalf("expected 'world', got %q", string(dst[:n]))
	}
}

func TestRingBufferReadFromOverwritten(t *testing.T) {
	rb := NewRingBuffer(16)
	rb.Write([]byte("0123456789abcdefghij")) // 20 bytes, oldest = 4

	dst := make([]byte, 10)
	_, next, err := rb.ReadFrom(0, dst)
	if err == nil {
		t.Fatal("expected overwrite error")
	}
	if next != 4 {
		t.Fatalf("expected fast-forward to offset 4, got %d", next)
	}
}

func TestRingBufferLargeWrap(t *testing.T) {
	rb := NewRingBuffer(1024)

	// Write 5000 bytes in chunks
	total := 0
	for total < 5000 {
		chunk := bytes.Repeat([]byte{byte(total / 100)}, 100)
		rb.Write(chunk)
		total += 100
	}

	if rb.WritePos() != 5000 {
		t.Fatalf("expected writePos 5000, got %d", rb.WritePos())
	}

	data, offset := rb.Snapshot()
	if offset != 5000-1024 {
		t.Fatalf("expected offset %d, got %d", 5000-1024, offset)
	}
	if len(data) != 1024 {
		t.Fatalf("expected 1024 bytes, got %d", len(data))
	}
}

func TestRingBufferEmpty(t *testing.T) {
	rb := NewRingBuffer(64)

	data, offset := rb.Snapshot()
	if offset != 0 {
		t.Fatalf("expected offset 0, got %d", offset)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(data))
	}

	data, offset = rb.Tail(10)
	if len(data) != 0 {
		t.Fatalf("expected empty tail, got %d bytes", len(data))
	}
}
