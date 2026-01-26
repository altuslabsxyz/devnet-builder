// internal/daemon/logs/collector.go
package logs

import (
	"bufio"
	"context"
	"io"
	"sync"
	"time"
)

// LogCollector collects logs from readers (e.g., process stdout) and stores them
// in a ring buffer. It handles parsing, tagging with node metadata, and provides
// filtering methods for querying logs.
type LogCollector struct {
	buffer *RingBuffer
	parser *LogParser
	mu     sync.RWMutex
}

// LogStats contains statistics about collected logs.
type LogStats struct {
	Total   int
	ByLevel map[string]int
	ByNode  map[int]int
}

// NewLogCollector creates a new log collector with the given buffer.
func NewLogCollector(buffer *RingBuffer) *LogCollector {
	return &LogCollector{
		buffer: buffer,
		parser: NewLogParser(),
	}
}

// CollectFromReader reads log lines from the reader, parses them, and stores
// them in the buffer. It tags each entry with the given nodeIndex.
// Returns the number of lines collected and any error encountered.
// This function respects context cancellation even with blocking readers.
func (c *LogCollector) CollectFromReader(ctx context.Context, reader io.Reader, nodeIndex int) (int, error) {
	lineCh := make(chan string)
	errCh := make(chan error, 1)

	// Scanner goroutine - reads lines and sends them to channel
	go func() {
		defer close(lineCh)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			select {
			case lineCh <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	count := 0
	for {
		select {
		case <-ctx.Done():
			return count, ctx.Err()
		case line, ok := <-lineCh:
			if !ok {
				// Channel closed, done reading
				select {
				case err := <-errCh:
					return count, err
				default:
					return count, nil
				}
			}

			if line == "" {
				continue
			}

			entry, err := c.parser.Parse(line)
			if err != nil {
				// For unparseable lines, create a basic entry
				entry = &LogEntry{
					Timestamp: time.Now(),
					Level:     "unknown",
					Message:   line,
					Raw:       line,
				}
			}

			entry.NodeIndex = nodeIndex
			c.buffer.Add(entry)
			count++
		}
	}
}

// GetAll returns all log entries in chronological order.
func (c *LogCollector) GetAll() []*LogEntry {
	return c.buffer.GetAll()
}

// GetLast returns the last n log entries in chronological order.
func (c *LogCollector) GetLast(n int) []*LogEntry {
	return c.buffer.GetLast(n)
}

// GetSince returns log entries with timestamp >= the given time.
func (c *LogCollector) GetSince(t time.Time) []*LogEntry {
	return c.buffer.Since(t)
}

// GetByLevel returns log entries matching the given level.
func (c *LogCollector) GetByLevel(level string) []*LogEntry {
	return c.buffer.ForLevel(level)
}

// GetByNode returns log entries for a specific node index.
func (c *LogCollector) GetByNode(nodeIndex int) []*LogEntry {
	return c.buffer.ForNode(nodeIndex)
}

// Stats returns statistics about the collected logs.
func (c *LogCollector) Stats() LogStats {
	entries := c.buffer.GetAll()

	stats := LogStats{
		Total:   len(entries),
		ByLevel: make(map[string]int),
		ByNode:  make(map[int]int),
	}

	for _, entry := range entries {
		if entry != nil {
			stats.ByLevel[entry.Level]++
			stats.ByNode[entry.NodeIndex]++
		}
	}

	return stats
}

// Clear removes all log entries from the buffer.
func (c *LogCollector) Clear() {
	c.buffer.Clear()
}
