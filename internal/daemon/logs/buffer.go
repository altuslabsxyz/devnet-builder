package logs

import (
	"sync"
	"time"
)

// RingBuffer is a thread-safe circular buffer for log entries.
// It maintains a fixed capacity and overwrites oldest entries when full.
type RingBuffer struct {
	entries  []*LogEntry
	capacity int
	head     int  // Write position
	size     int  // Current number of entries
	mu       sync.RWMutex
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000 // Default capacity
	}
	return &RingBuffer{
		entries:  make([]*LogEntry, capacity),
		capacity: capacity,
	}
}

// Add appends a log entry to the buffer.
// If the buffer is full, the oldest entry is overwritten.
func (r *RingBuffer) Add(entry *LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[r.head] = entry
	r.head = (r.head + 1) % r.capacity

	if r.size < r.capacity {
		r.size++
	}
}

// GetAll returns all entries in chronological order.
func (r *RingBuffer) GetAll() []*LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getEntriesLocked(r.size)
}

// GetLast returns the last n entries in chronological order.
func (r *RingBuffer) GetLast(n int) []*LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if n > r.size {
		n = r.size
	}
	return r.getEntriesLocked(n)
}

// getEntriesLocked returns the last n entries (must hold lock).
func (r *RingBuffer) getEntriesLocked(n int) []*LogEntry {
	if n == 0 || r.size == 0 {
		return nil
	}

	result := make([]*LogEntry, n)

	// Calculate start position for reading
	// head points to next write position, so head-1 is newest entry
	// We want to read n entries ending at head-1
	start := (r.head - n + r.capacity) % r.capacity

	for i := 0; i < n; i++ {
		idx := (start + i) % r.capacity
		result[i] = r.entries[idx]
	}

	return result
}

// Size returns the current number of entries in the buffer.
func (r *RingBuffer) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

// Clear removes all entries from the buffer.
func (r *RingBuffer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.entries {
		r.entries[i] = nil
	}
	r.head = 0
	r.size = 0
}

// Filter returns entries matching the predicate in chronological order.
func (r *RingBuffer) Filter(predicate func(*LogEntry) bool) []*LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*LogEntry
	entries := r.getEntriesLocked(r.size)

	for _, entry := range entries {
		if entry != nil && predicate(entry) {
			result = append(result, entry)
		}
	}

	return result
}

// Since returns entries with timestamp >= the given time.
func (r *RingBuffer) Since(t time.Time) []*LogEntry {
	return r.Filter(func(e *LogEntry) bool {
		return !e.Timestamp.Before(t)
	})
}

// ForNode returns entries for a specific node index.
func (r *RingBuffer) ForNode(nodeIndex int) []*LogEntry {
	return r.Filter(func(e *LogEntry) bool {
		return e.NodeIndex == nodeIndex
	})
}

// ForLevel returns entries with the specified log level.
func (r *RingBuffer) ForLevel(level string) []*LogEntry {
	return r.Filter(func(e *LogEntry) bool {
		return e.Level == level
	})
}

// Subscribe returns a channel that receives new log entries.
// The channel is closed when the buffer is cleared.
// Caller must drain the channel to prevent blocking.
func (r *RingBuffer) Subscribe(bufSize int) chan *LogEntry {
	// This is a placeholder for streaming functionality.
	// In a full implementation, we'd track subscribers and broadcast.
	ch := make(chan *LogEntry, bufSize)
	return ch
}
