// internal/daemon/runtime/logs_test.go
package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogManagerWrite(t *testing.T) {
	tempDir := t.TempDir()

	lm := NewLogManager(tempDir, LogConfig{
		MaxSize:  1024, // 1KB for testing
		MaxFiles: 3,
	})

	logPath := filepath.Join(tempDir, "test.log")
	writer, err := lm.GetWriter("test-node", logPath)
	if err != nil {
		t.Fatalf("GetWriter failed: %v", err)
	}
	defer writer.Close()

	// Write some data
	_, err = writer.Write([]byte("test log line\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file not created")
	}
}

func TestLogManagerRotation(t *testing.T) {
	tempDir := t.TempDir()

	lm := NewLogManager(tempDir, LogConfig{
		MaxSize:  100, // 100 bytes for quick rotation
		MaxFiles: 2,
	})

	logPath := filepath.Join(tempDir, "test.log")
	writer, err := lm.GetWriter("test-node", logPath)
	if err != nil {
		t.Fatalf("GetWriter failed: %v", err)
	}

	// Write enough data to trigger rotation
	bigLine := strings.Repeat("x", 50) + "\n"
	for i := 0; i < 5; i++ {
		writer.Write([]byte(bigLine))
	}
	writer.Close()

	// Check that rotation happened
	files, _ := filepath.Glob(filepath.Join(tempDir, "test.log*"))
	if len(files) < 2 {
		t.Errorf("Expected at least 2 log files after rotation, got %d", len(files))
	}
}

func TestLogManagerRead(t *testing.T) {
	tempDir := t.TempDir()

	lm := NewLogManager(tempDir, LogConfig{
		MaxSize:  1024,
		MaxFiles: 3,
	})

	logPath := filepath.Join(tempDir, "test.log")

	// Write some lines
	writer, _ := lm.GetWriter("test-node", logPath)
	writer.Write([]byte("line 1\n"))
	writer.Write([]byte("line 2\n"))
	writer.Write([]byte("line 3\n"))
	writer.Close()

	// Read last 2 lines
	reader, err := lm.GetReader(logPath, LogOptions{Lines: 2})
	if err != nil {
		t.Fatalf("GetReader failed: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	content := string(buf[:n])

	if !strings.Contains(content, "line 2") || !strings.Contains(content, "line 3") {
		t.Errorf("Expected last 2 lines, got: %s", content)
	}
}
