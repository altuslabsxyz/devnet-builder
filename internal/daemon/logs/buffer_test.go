// internal/daemon/logs/buffer_test.go
package logs

import (
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_Basic(t *testing.T) {
	buf := NewRingBuffer(5)

	// Add 3 entries
	for i := 0; i < 3; i++ {
		buf.Add(&LogEntry{Message: "msg" + string(rune('A'+i))})
	}

	entries := buf.GetAll()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Message != "msgA" {
		t.Errorf("expected msgA first, got %s", entries[0].Message)
	}
	if entries[2].Message != "msgC" {
		t.Errorf("expected msgC last, got %s", entries[2].Message)
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	buf := NewRingBuffer(3)

	// Add 5 entries to overflow
	for i := 0; i < 5; i++ {
		buf.Add(&LogEntry{Message: "msg" + string(rune('A'+i))})
	}

	entries := buf.GetAll()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Should have C, D, E (oldest A, B overwritten)
	if entries[0].Message != "msgC" {
		t.Errorf("expected msgC first, got %s", entries[0].Message)
	}
	if entries[2].Message != "msgE" {
		t.Errorf("expected msgE last, got %s", entries[2].Message)
	}
}

func TestRingBuffer_GetLast(t *testing.T) {
	buf := NewRingBuffer(10)

	for i := 0; i < 10; i++ {
		buf.Add(&LogEntry{Message: "msg" + string(rune('A'+i))})
	}

	// Get last 3
	entries := buf.GetLast(3)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Message != "msgH" {
		t.Errorf("expected msgH first, got %s", entries[0].Message)
	}
	if entries[2].Message != "msgJ" {
		t.Errorf("expected msgJ last, got %s", entries[2].Message)
	}
}

func TestRingBuffer_GetLastMoreThanAvailable(t *testing.T) {
	buf := NewRingBuffer(10)

	for i := 0; i < 3; i++ {
		buf.Add(&LogEntry{Message: "msg" + string(rune('A'+i))})
	}

	// Request more than available
	entries := buf.GetLast(100)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	buf := NewRingBuffer(5)

	entries := buf.GetAll()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	entries = buf.GetLast(10)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries from GetLast, got %d", len(entries))
	}
}

func TestRingBuffer_Size(t *testing.T) {
	buf := NewRingBuffer(5)

	if buf.Size() != 0 {
		t.Errorf("expected size 0, got %d", buf.Size())
	}

	buf.Add(&LogEntry{Message: "test"})
	if buf.Size() != 1 {
		t.Errorf("expected size 1, got %d", buf.Size())
	}

	for i := 0; i < 10; i++ {
		buf.Add(&LogEntry{Message: "test"})
	}
	if buf.Size() != 5 {
		t.Errorf("expected size 5 (capacity), got %d", buf.Size())
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	buf := NewRingBuffer(5)

	for i := 0; i < 3; i++ {
		buf.Add(&LogEntry{Message: "test"})
	}

	buf.Clear()

	if buf.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", buf.Size())
	}
}

func TestRingBuffer_Filter(t *testing.T) {
	buf := NewRingBuffer(10)

	buf.Add(&LogEntry{Level: "info", Message: "info1"})
	buf.Add(&LogEntry{Level: "error", Message: "error1"})
	buf.Add(&LogEntry{Level: "info", Message: "info2"})
	buf.Add(&LogEntry{Level: "error", Message: "error2"})

	// Filter for errors only
	entries := buf.Filter(func(e *LogEntry) bool {
		return e.Level == "error"
	})

	if len(entries) != 2 {
		t.Fatalf("expected 2 error entries, got %d", len(entries))
	}
	if entries[0].Message != "error1" {
		t.Errorf("expected error1, got %s", entries[0].Message)
	}
}

func TestRingBuffer_Since(t *testing.T) {
	buf := NewRingBuffer(10)

	now := time.Now()
	buf.Add(&LogEntry{Timestamp: now.Add(-2 * time.Minute), Message: "old"})
	buf.Add(&LogEntry{Timestamp: now.Add(-30 * time.Second), Message: "recent1"})
	buf.Add(&LogEntry{Timestamp: now, Message: "recent2"})

	// Get entries since 1 minute ago
	entries := buf.Since(now.Add(-1 * time.Minute))

	if len(entries) != 2 {
		t.Fatalf("expected 2 recent entries, got %d", len(entries))
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	buf := NewRingBuffer(100)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				buf.Add(&LogEntry{Message: "test"})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = buf.GetAll()
			}
		}()
	}

	wg.Wait()

	// Should not panic and should have at most capacity entries
	if buf.Size() > 100 {
		t.Errorf("size exceeded capacity: %d", buf.Size())
	}
}
