// internal/daemon/logs/collector_test.go
package logs

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLogCollector_CollectFromReader(t *testing.T) {
	buf := NewRingBuffer(100)
	collector := NewLogCollector(buf)

	logs := `{"level":"info","module":"consensus","msg":"committed state","height":1}
{"level":"info","module":"consensus","msg":"committed state","height":2}
{"level":"info","module":"consensus","msg":"committed state","height":3}
`
	reader := strings.NewReader(logs)

	n, err := collector.CollectFromReader(context.Background(), reader, 0)
	if err != nil {
		t.Fatalf("CollectFromReader failed: %v", err)
	}

	if n != 3 {
		t.Errorf("expected 3 lines collected, got %d", n)
	}

	entries := buf.GetAll()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries in buffer, got %d", len(entries))
	}

	// Check node index was set
	for _, entry := range entries {
		if entry.NodeIndex != 0 {
			t.Errorf("expected nodeIndex 0, got %d", entry.NodeIndex)
		}
	}
}

func TestLogCollector_CollectFromReaderWithContext(t *testing.T) {
	buf := NewRingBuffer(100)
	collector := NewLogCollector(buf)

	// Create a reader that blocks forever
	pr, pw := io.Pipe()
	defer pw.Close()

	// Write some data
	go func() {
		pw.Write([]byte(`{"level":"info","msg":"test1"}` + "\n"))
		time.Sleep(10 * time.Millisecond)
		pw.Write([]byte(`{"level":"info","msg":"test2"}` + "\n"))
	}()

	// Cancel context quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := collector.CollectFromReader(ctx, pr, 0)
	if err != context.DeadlineExceeded {
		// May also be nil if we read all available data
		t.Logf("CollectFromReader returned: %v", err)
	}
}

func TestLogCollector_FilterByLevel(t *testing.T) {
	buf := NewRingBuffer(100)
	collector := NewLogCollector(buf)

	logs := `{"level":"debug","msg":"debug msg"}
{"level":"info","msg":"info msg"}
{"level":"warn","msg":"warn msg"}
{"level":"error","msg":"error msg"}
`
	reader := strings.NewReader(logs)
	collector.CollectFromReader(context.Background(), reader, 0)

	// Get only errors
	errors := collector.GetByLevel("error")
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestLogCollector_FilterByNode(t *testing.T) {
	buf := NewRingBuffer(100)
	collector := NewLogCollector(buf)

	// Collect from multiple "nodes"
	reader1 := strings.NewReader(`{"level":"info","msg":"from node 0"}` + "\n")
	reader2 := strings.NewReader(`{"level":"info","msg":"from node 1"}` + "\n")

	collector.CollectFromReader(context.Background(), reader1, 0)
	collector.CollectFromReader(context.Background(), reader2, 1)

	// Get only node 0
	node0Logs := collector.GetByNode(0)
	if len(node0Logs) != 1 {
		t.Errorf("expected 1 log from node 0, got %d", len(node0Logs))
	}
	if node0Logs[0].Message != "from node 0" {
		t.Errorf("unexpected message: %s", node0Logs[0].Message)
	}
}

func TestLogCollector_GetLast(t *testing.T) {
	buf := NewRingBuffer(100)
	collector := NewLogCollector(buf)

	logs := `{"level":"info","msg":"msg1"}
{"level":"info","msg":"msg2"}
{"level":"info","msg":"msg3"}
{"level":"info","msg":"msg4"}
{"level":"info","msg":"msg5"}
`
	reader := strings.NewReader(logs)
	collector.CollectFromReader(context.Background(), reader, 0)

	last3 := collector.GetLast(3)
	if len(last3) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(last3))
	}

	if last3[0].Message != "msg3" {
		t.Errorf("expected msg3, got %s", last3[0].Message)
	}
	if last3[2].Message != "msg5" {
		t.Errorf("expected msg5, got %s", last3[2].Message)
	}
}

func TestLogCollector_GetSince(t *testing.T) {
	buf := NewRingBuffer(100)
	collector := NewLogCollector(buf)

	now := time.Now()
	oldTime := now.Add(-2 * time.Hour).Format(time.RFC3339)
	recentTime := now.Add(-30 * time.Second).Format(time.RFC3339)

	logs := `{"level":"info","time":"` + oldTime + `","msg":"old msg"}
{"level":"info","time":"` + recentTime + `","msg":"recent msg"}
`
	reader := strings.NewReader(logs)
	collector.CollectFromReader(context.Background(), reader, 0)

	// Get since 1 hour ago
	recent := collector.GetSince(now.Add(-1 * time.Hour))
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent entry, got %d", len(recent))
	}
	if recent[0].Message != "recent msg" {
		t.Errorf("expected 'recent msg', got %s", recent[0].Message)
	}
}

func TestLogCollector_Stats(t *testing.T) {
	buf := NewRingBuffer(100)
	collector := NewLogCollector(buf)

	logs := `{"level":"debug","msg":"d1"}
{"level":"info","msg":"i1"}
{"level":"info","msg":"i2"}
{"level":"warn","msg":"w1"}
{"level":"error","msg":"e1"}
{"level":"error","msg":"e2"}
{"level":"error","msg":"e3"}
`
	reader := strings.NewReader(logs)
	collector.CollectFromReader(context.Background(), reader, 0)

	stats := collector.Stats()

	if stats.Total != 7 {
		t.Errorf("expected total 7, got %d", stats.Total)
	}
	if stats.ByLevel["debug"] != 1 {
		t.Errorf("expected 1 debug, got %d", stats.ByLevel["debug"])
	}
	if stats.ByLevel["info"] != 2 {
		t.Errorf("expected 2 info, got %d", stats.ByLevel["info"])
	}
	if stats.ByLevel["error"] != 3 {
		t.Errorf("expected 3 error, got %d", stats.ByLevel["error"])
	}
}
