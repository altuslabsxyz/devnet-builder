// cmd/dvb/logs_test.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
)

// --- Mocks ---

// mockNodeLogStreamer implements nodeLogStreamer for testing.
type mockNodeLogStreamer struct {
	entries      []client.LogEntry
	err          error
	calledDevnet string
	calledIndex  int
	calledFollow bool
	calledTail   int
}

func (m *mockNodeLogStreamer) StreamNodeLogs(ctx context.Context, devnetName string, index int, follow bool, since string, tail int, callback func(*client.LogEntry) error) error {
	m.calledDevnet = devnetName
	m.calledIndex = index
	m.calledFollow = follow
	m.calledTail = tail
	if m.err != nil {
		return m.err
	}
	for i := range m.entries {
		if err := callback(&m.entries[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Pure function tests ---

func TestIsValidNodeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", "validator-0", true},
		{"valid numeric start", "node0", true},
		{"valid underscore", "full_node_1", true},
		{"valid single char", "a", true},
		{"empty string", "", false},
		{"path traversal dots", "..", false},
		{"path traversal slash", "validator/../etc", false},
		{"backslash", "node\\0", false},
		{"single dot", ".", false},
		{"starts with hyphen", "-node", false},
		{"starts with underscore", "_node", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidNodeName(tt.input); got != tt.want {
				t.Errorf("isValidNodeName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsPathWithinBase(t *testing.T) {
	tests := []struct {
		name     string
		resolved string
		base     string
		want     bool
	}{
		{"within base", "/home/user/.devnet-builder/devnets/test/node.log", "/home/user/.devnet-builder/devnets", true},
		{"exact base", "/home/user/.devnet-builder/devnets", "/home/user/.devnet-builder/devnets", true},
		{"outside base", "/etc/passwd", "/home/user/.devnet-builder", false},
		{"prefix match not subdir", "/home/user/.devnet-builder-evil/file", "/home/user/.devnet-builder", false},
		{"parent escape", "/home/user/.devnet-builder/../secret", "/home/user/.devnet-builder", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPathWithinBase(tt.resolved, tt.base); got != tt.want {
				t.Errorf("isPathWithinBase(%q, %q) = %v, want %v", tt.resolved, tt.base, got, tt.want)
			}
		})
	}
}

func TestLooksLikeNodeIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"0", true},
		{"5", true},
		{"validator-0", true},
		{"node-1", true},
		{"full-3", true},
		{"my-devnet", false},
		{"cosmos", false},
		{"mainnet", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := looksLikeNodeIdentifier(tt.input); got != tt.want {
				t.Errorf("looksLikeNodeIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetNodeColor(t *testing.T) {
	// Same input should always return same color (deterministic)
	c1 := getNodeColor("validator-0")
	c2 := getNodeColor("validator-0")
	if c1("test") != c2("test") {
		t.Error("getNodeColor should be deterministic for the same input")
	}
	// Different nodes should not panic
	_ = getNodeColor("node-1")
	_ = getNodeColor("full-0")
	_ = getNodeColor("")
}

// --- printLogLine tests (Bug 1 fix) ---

func TestPrintLogLine(t *testing.T) {
	tests := []struct {
		name          string
		node          string
		line          string
		showTimestamp bool
		wantNode      bool
		wantTimestamp bool
		wantContains  string
	}{
		{"plain line no node", "", "hello world", false, false, false, "hello world"},
		{"with node prefix", "validator-0", "started", false, true, false, "started"},
		{"with timestamp no node", "", "log line", true, false, true, "log line"},
		{"with timestamp and node", "node-1", "running", true, true, true, "running"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printLogLine(&buf, tt.node, tt.line, tt.showTimestamp)
			output := buf.String()

			if tt.wantContains != "" && !strings.Contains(output, tt.wantContains) {
				t.Errorf("output %q should contain %q", output, tt.wantContains)
			}
			if tt.wantNode && !strings.Contains(output, "["+tt.node+"]") {
				t.Errorf("output %q should contain node prefix [%s]", output, tt.node)
			}
			if tt.wantTimestamp {
				// Check for today's date pattern (YYYY-MM-DD)
				today := time.Now().Format("2006-01-02")
				if !strings.Contains(output, today) {
					t.Errorf("output %q should contain today's date %q for timestamp", output, today)
				}
			}
			if !tt.wantTimestamp && tt.node == "" {
				// Plain line should be just the line + newline
				if strings.TrimSpace(output) != tt.line {
					t.Errorf("plain output should be %q, got %q", tt.line, strings.TrimSpace(output))
				}
			}
		})
	}
}

// --- streamLogsFromDaemonWithClient tests (DI) ---

func TestStreamLogsFromDaemonWithClient(t *testing.T) {
	t.Run("streams entries with timestamps", func(t *testing.T) {
		now := time.Now()
		mock := &mockNodeLogStreamer{
			entries: []client.LogEntry{
				{Timestamp: now, Stream: "stdout", Message: "block 1"},
				{Timestamp: now, Stream: "stderr", Message: "warning!"},
			},
		}
		var buf bytes.Buffer
		opts := &logsOptions{devnet: "test", node: "0", timestamp: true}

		err := streamLogsFromDaemonWithClient(context.Background(), opts, 0, mock, &buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "block 1") {
			t.Errorf("output should contain 'block 1', got: %s", output)
		}
		if !strings.Contains(output, "warning!") {
			t.Errorf("output should contain 'warning!', got: %s", output)
		}
		if !strings.Contains(output, now.Format(time.RFC3339)) {
			t.Errorf("output should contain timestamp, got: %s", output)
		}
	})

	t.Run("streams entries without timestamps", func(t *testing.T) {
		mock := &mockNodeLogStreamer{
			entries: []client.LogEntry{
				{Message: "hello"},
			},
		}
		var buf bytes.Buffer
		opts := &logsOptions{devnet: "test", node: "0", timestamp: false}

		err := streamLogsFromDaemonWithClient(context.Background(), opts, 0, mock, &buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "hello") {
			t.Errorf("output should contain 'hello', got: %s", output)
		}
	})

	t.Run("nil client returns error", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &logsOptions{devnet: "test", node: "0"}
		err := streamLogsFromDaemonWithClient(context.Background(), opts, 0, nil, &buf)
		if err == nil {
			t.Error("expected error for nil client")
		}
		if !strings.Contains(err.Error(), "daemon client not available") {
			t.Errorf("error should mention client, got: %v", err)
		}
	})

	t.Run("propagates stream error", func(t *testing.T) {
		mock := &mockNodeLogStreamer{err: fmt.Errorf("connection lost")}
		var buf bytes.Buffer
		opts := &logsOptions{devnet: "test", node: "0"}

		err := streamLogsFromDaemonWithClient(context.Background(), opts, 0, mock, &buf)
		if err == nil || !strings.Contains(err.Error(), "connection lost") {
			t.Errorf("expected 'connection lost' error, got: %v", err)
		}
	})

	t.Run("passes correct parameters to client", func(t *testing.T) {
		mock := &mockNodeLogStreamer{}
		var buf bytes.Buffer
		opts := &logsOptions{devnet: "my-devnet", node: "3", follow: true, tail: 50}

		_ = streamLogsFromDaemonWithClient(context.Background(), opts, 3, mock, &buf)

		if mock.calledDevnet != "my-devnet" {
			t.Errorf("devnet = %q, want my-devnet", mock.calledDevnet)
		}
		if mock.calledIndex != 3 {
			t.Errorf("index = %d, want 3", mock.calledIndex)
		}
		if !mock.calledFollow {
			t.Error("follow should be true")
		}
		if mock.calledTail != 50 {
			t.Errorf("tail = %d, want 50", mock.calledTail)
		}
	})

	t.Run("zero timestamp falls back to no-timestamp format", func(t *testing.T) {
		mock := &mockNodeLogStreamer{
			entries: []client.LogEntry{
				{Message: "no time"},
			},
		}
		var buf bytes.Buffer
		opts := &logsOptions{devnet: "test", node: "0", timestamp: true}

		err := streamLogsFromDaemonWithClient(context.Background(), opts, 0, mock, &buf)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "no time") {
			t.Errorf("output should contain message, got: %s", output)
		}
		// Should NOT contain RFC3339 timestamp since entry.Timestamp is zero
		if strings.Contains(output, "T") && strings.Contains(output, "+") {
			t.Errorf("output should not contain timestamp for zero-time entry, got: %s", output)
		}
	})
}

// --- streamLogFile tests (file-based) ---

func TestStreamLogFile(t *testing.T) {
	t.Run("reads all lines", func(t *testing.T) {
		dir := t.TempDir()
		logFile := filepath.Join(dir, "node.log")
		os.WriteFile(logFile, []byte("line1\nline2\nline3\n"), 0644)

		var buf bytes.Buffer
		opts := &logsOptions{}
		err := streamLogFile(context.Background(), logFile, "node-0", opts, &buf)
		if err != nil {
			t.Fatal(err)
		}

		output := buf.String()
		if !strings.Contains(output, "line1") || !strings.Contains(output, "line3") {
			t.Errorf("should contain all lines, got: %s", output)
		}
	})

	t.Run("tail last 2 lines", func(t *testing.T) {
		dir := t.TempDir()
		logFile := filepath.Join(dir, "node.log")
		os.WriteFile(logFile, []byte("line1\nline2\nline3\n"), 0644)

		var buf bytes.Buffer
		opts := &logsOptions{tail: 2}
		err := streamLogFile(context.Background(), logFile, "node-0", opts, &buf)
		if err != nil {
			t.Fatal(err)
		}

		output := buf.String()
		if strings.Contains(output, "line1") {
			t.Errorf("should not contain line1 with tail=2, got: %s", output)
		}
		if !strings.Contains(output, "line2") || !strings.Contains(output, "line3") {
			t.Errorf("should contain line2 and line3, got: %s", output)
		}
	})

	t.Run("timestamp flag works in file mode", func(t *testing.T) {
		dir := t.TempDir()
		logFile := filepath.Join(dir, "node.log")
		os.WriteFile(logFile, []byte("hello\n"), 0644)

		var buf bytes.Buffer
		opts := &logsOptions{timestamp: true}
		err := streamLogFile(context.Background(), logFile, "node-0", opts, &buf)
		if err != nil {
			t.Fatal(err)
		}

		output := buf.String()
		today := time.Now().Format("2006-01-02")
		if !strings.Contains(output, today) {
			t.Errorf("timestamp should be present, got: %s", output)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &logsOptions{}
		err := streamLogFile(context.Background(), "/nonexistent/file.log", "", opts, &buf)
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("empty file produces no output", func(t *testing.T) {
		dir := t.TempDir()
		logFile := filepath.Join(dir, "node.log")
		os.WriteFile(logFile, []byte(""), 0644)

		var buf bytes.Buffer
		opts := &logsOptions{}
		err := streamLogFile(context.Background(), logFile, "node-0", opts, &buf)
		if err != nil {
			t.Fatal(err)
		}

		if buf.Len() != 0 {
			t.Errorf("expected no output for empty file, got: %s", buf.String())
		}
	})
}

// --- followLogFile tests ---

func TestFollowLogFile(t *testing.T) {
	t.Run("picks up new content written to file", func(t *testing.T) {
		dir := t.TempDir()
		logFile := filepath.Join(dir, "node.log")
		os.WriteFile(logFile, []byte("initial\n"), 0644)

		var buf bytes.Buffer
		opts := &logsOptions{}
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			done <- followLogFile(ctx, logFile, "node-0", opts, &buf)
		}()

		// Give watcher time to start
		time.Sleep(50 * time.Millisecond)

		// Append new content to the file
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(f, "new line from follow")
		f.Close()

		// Wait for the watcher to pick it up
		time.Sleep(200 * time.Millisecond)
		cancel()

		if err := <-done; err != nil && err != context.Canceled {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "new line from follow") {
			t.Errorf("should contain new content, got: %s", output)
		}
		// Should NOT contain "initial" since followLogFile starts at EOF
		if strings.Contains(output, "initial") {
			t.Errorf("should not contain initial content, got: %s", output)
		}
	})

	t.Run("returns context error on cancellation", func(t *testing.T) {
		dir := t.TempDir()
		logFile := filepath.Join(dir, "node.log")
		os.WriteFile(logFile, []byte("data\n"), 0644)

		var buf bytes.Buffer
		opts := &logsOptions{}
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			done <- followLogFile(ctx, logFile, "node-0", opts, &buf)
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()

		err := <-done
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	})
}

// --- findLogFile tests ---

func TestFindLogFile(t *testing.T) {
	t.Run("finds node.log", func(t *testing.T) {
		dir := t.TempDir()
		nodeDir := filepath.Join(dir, "validator-0")
		os.MkdirAll(nodeDir, 0755)
		os.WriteFile(filepath.Join(nodeDir, "node.log"), []byte("log"), 0644)

		result := findLogFile(dir, "validator-0")
		if result == "" {
			t.Error("expected to find node.log")
		}
		if !strings.HasSuffix(result, "node.log") {
			t.Errorf("expected node.log path, got: %s", result)
		}
	})

	t.Run("finds stdout.log when node.log missing", func(t *testing.T) {
		dir := t.TempDir()
		nodeDir := filepath.Join(dir, "validator-0")
		os.MkdirAll(nodeDir, 0755)
		os.WriteFile(filepath.Join(nodeDir, "stdout.log"), []byte("log"), 0644)

		result := findLogFile(dir, "validator-0")
		if result == "" {
			t.Error("expected to find stdout.log")
		}
		if !strings.HasSuffix(result, "stdout.log") {
			t.Errorf("expected stdout.log path, got: %s", result)
		}
	})

	t.Run("finds output.log as fallback", func(t *testing.T) {
		dir := t.TempDir()
		nodeDir := filepath.Join(dir, "validator-0")
		os.MkdirAll(nodeDir, 0755)
		os.WriteFile(filepath.Join(nodeDir, "output.log"), []byte("log"), 0644)

		result := findLogFile(dir, "validator-0")
		if result == "" {
			t.Error("expected to find output.log")
		}
	})

	t.Run("finds arbitrary .log file", func(t *testing.T) {
		dir := t.TempDir()
		nodeDir := filepath.Join(dir, "validator-0")
		os.MkdirAll(nodeDir, 0755)
		os.WriteFile(filepath.Join(nodeDir, "custom.log"), []byte("log"), 0644)

		result := findLogFile(dir, "validator-0")
		if result == "" {
			t.Error("expected to find custom.log")
		}
	})

	t.Run("returns empty for nonexistent node", func(t *testing.T) {
		dir := t.TempDir()
		result := findLogFile(dir, "missing-node")
		if result != "" {
			t.Errorf("expected empty, got: %s", result)
		}
	})

	t.Run("finds data/node.log", func(t *testing.T) {
		dir := t.TempDir()
		nodeDir := filepath.Join(dir, "validator-0")
		dataDir := filepath.Join(nodeDir, "data")
		os.MkdirAll(dataDir, 0755)
		os.WriteFile(filepath.Join(dataDir, "node.log"), []byte("log"), 0644)

		result := findLogFile(dir, "validator-0")
		if result == "" {
			t.Error("expected to find data/node.log")
		}
	})
}
