// internal/daemon/logs/parser_test.go
package logs

import (
	"testing"
	"time"
)

func TestLogParser_ParseJSON(t *testing.T) {
	parser := NewLogParser()

	line := `{"level":"info","module":"consensus","time":"2024-01-25T10:30:00Z","msg":"committed state","height":1234}`
	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if entry.Level != "info" {
		t.Errorf("expected level info, got %s", entry.Level)
	}
	if entry.Module != "consensus" {
		t.Errorf("expected module consensus, got %s", entry.Module)
	}
	if entry.Message != "committed state" {
		t.Errorf("expected message 'committed state', got %s", entry.Message)
	}
	if entry.Fields["height"] != float64(1234) {
		t.Errorf("expected height 1234, got %v", entry.Fields["height"])
	}
}

func TestLogParser_ParsePlainText(t *testing.T) {
	parser := NewLogParser()

	line := `INF committed state module=consensus height=1234`
	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if entry.Level != "info" {
		t.Errorf("expected level info, got %s", entry.Level)
	}
	if entry.Module != "consensus" {
		t.Errorf("expected module consensus, got %s", entry.Module)
	}
	if entry.Message != "committed state" {
		t.Errorf("expected message 'committed state', got %s", entry.Message)
	}
}

func TestLogParser_ParseDebugLevel(t *testing.T) {
	parser := NewLogParser()

	tests := []struct {
		line  string
		level string
	}{
		{`{"level":"debug","msg":"test"}`, "debug"},
		{`{"level":"info","msg":"test"}`, "info"},
		{`{"level":"warn","msg":"test"}`, "warn"},
		{`{"level":"error","msg":"test"}`, "error"},
		{`DBG test message`, "debug"},
		{`INF test message`, "info"},
		{`WRN test message`, "warn"},
		{`ERR test message`, "error"},
	}

	for _, tt := range tests {
		entry, err := parser.Parse(tt.line)
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", tt.line, err)
			continue
		}
		if entry.Level != tt.level {
			t.Errorf("Parse(%q): expected level %s, got %s", tt.line, tt.level, entry.Level)
		}
	}
}

func TestLogParser_ParseWithTimestamp(t *testing.T) {
	parser := NewLogParser()

	line := `{"level":"info","time":"2024-01-25T10:30:00.123456789Z","msg":"test"}`
	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	expected := time.Date(2024, 1, 25, 10, 30, 0, 123456789, time.UTC)
	if !entry.Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, entry.Timestamp)
	}
}

func TestLogParser_ParseInvalidJSON(t *testing.T) {
	parser := NewLogParser()

	// Invalid JSON should be treated as plain text
	line := `not valid json at all`
	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should capture as raw message
	if entry.Message != "not valid json at all" {
		t.Errorf("expected raw message, got %s", entry.Message)
	}
}

func TestLogEntry_ToJSON(t *testing.T) {
	entry := &LogEntry{
		Timestamp: time.Date(2024, 1, 25, 10, 30, 0, 0, time.UTC),
		Level:     "info",
		Module:    "consensus",
		Message:   "committed state",
		NodeIndex: 0,
		Fields: map[string]interface{}{
			"height": 1234,
		},
	}

	json := entry.ToJSON()
	if json == "" {
		t.Error("ToJSON returned empty string")
	}
}

func TestLogEntry_String(t *testing.T) {
	entry := &LogEntry{
		Timestamp: time.Date(2024, 1, 25, 10, 30, 0, 0, time.UTC),
		Level:     "info",
		Module:    "consensus",
		Message:   "committed state",
		NodeIndex: 0,
	}

	str := entry.String()
	if str == "" {
		t.Error("String returned empty string")
	}
}
