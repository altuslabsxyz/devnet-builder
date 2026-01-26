// Package logs provides log parsing, storage, and streaming for devnet nodes.
// It supports both JSON and plain text Cosmos SDK log formats.
package logs

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// LogEntry represents a parsed log entry with structured fields.
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Module    string                 `json:"module,omitempty"`
	Message   string                 `json:"message"`
	NodeIndex int                    `json:"nodeIndex"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Raw       string                 `json:"raw,omitempty"`
}

// ToJSON serializes the log entry to JSON.
func (e *LogEntry) ToJSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

// String returns a human-readable representation.
func (e *LogEntry) String() string {
	ts := e.Timestamp.Format("15:04:05.000")
	level := strings.ToUpper(e.Level[:3])
	if e.Module != "" {
		return fmt.Sprintf("[%s] %s [%s] %s", ts, level, e.Module, e.Message)
	}
	return fmt.Sprintf("[%s] %s %s", ts, level, e.Message)
}

// LogParser parses log lines into structured LogEntry objects.
type LogParser struct {
	// plainTextRegex matches Cosmos SDK plain text log format
	// Examples: "INF committed state module=consensus height=1234"
	plainTextRegex *regexp.Regexp
	// keyValueRegex extracts key=value pairs
	keyValueRegex *regexp.Regexp
}

// NewLogParser creates a new log parser.
func NewLogParser() *LogParser {
	return &LogParser{
		// Match: LEVEL message key=value key=value ...
		plainTextRegex: regexp.MustCompile(`^(DBG|INF|WRN|ERR)\s+(.+)$`),
		keyValueRegex:  regexp.MustCompile(`(\w+)=("[^"]*"|\S+)`),
	}
}

// Parse parses a log line and returns a structured LogEntry.
func (p *LogParser) Parse(line string) (*LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	// Try JSON format first
	if strings.HasPrefix(line, "{") {
		entry, err := p.parseJSON(line)
		if err == nil {
			return entry, nil
		}
	}

	// Fall back to plain text format
	return p.parsePlainText(line), nil
}

func (p *LogParser) parseJSON(line string) (*LogEntry, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, err
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
		Raw:       line,
	}

	// Extract standard fields
	if level, ok := raw["level"].(string); ok {
		entry.Level = level
		delete(raw, "level")
	}
	if module, ok := raw["module"].(string); ok {
		entry.Module = module
		delete(raw, "module")
	}
	if msg, ok := raw["msg"].(string); ok {
		entry.Message = msg
		delete(raw, "msg")
	}
	if timeStr, ok := raw["time"].(string); ok {
		if ts, err := time.Parse(time.RFC3339Nano, timeStr); err == nil {
			entry.Timestamp = ts
		}
		delete(raw, "time")
	}

	// All remaining fields go to Fields map
	for k, v := range raw {
		entry.Fields[k] = v
	}

	return entry, nil
}

func (p *LogParser) parsePlainText(line string) *LogEntry {
	entry := &LogEntry{
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
		Raw:       line,
	}

	// Try to match the level prefix
	matches := p.plainTextRegex.FindStringSubmatch(line)
	if matches == nil {
		// No recognized format, treat entire line as message
		entry.Level = "info"
		entry.Message = line
		return entry
	}

	// Map short level to full name
	entry.Level = p.shortLevelToFull(matches[1])
	rest := matches[2]

	// Extract key=value pairs
	kvMatches := p.keyValueRegex.FindAllStringSubmatch(rest, -1)
	var message strings.Builder
	lastEnd := 0

	for _, kv := range kvMatches {
		fullMatch := kv[0]
		key := kv[1]
		value := strings.Trim(kv[2], `"`)

		// Find position of this match
		pos := strings.Index(rest[lastEnd:], fullMatch)
		if pos > 0 {
			// Add text before this key=value to message
			message.WriteString(strings.TrimSpace(rest[lastEnd : lastEnd+pos]))
			message.WriteString(" ")
		}
		lastEnd = strings.Index(rest, fullMatch) + len(fullMatch)

		// Handle special keys
		switch key {
		case "module":
			entry.Module = value
		default:
			entry.Fields[key] = value
		}
	}

	// Get remaining message (before any key=value pairs)
	if len(kvMatches) > 0 {
		firstKV := kvMatches[0][0]
		pos := strings.Index(rest, firstKV)
		if pos > 0 {
			entry.Message = strings.TrimSpace(rest[:pos])
		}
	} else {
		entry.Message = rest
	}

	if entry.Message == "" {
		entry.Message = strings.TrimSpace(message.String())
	}

	return entry
}

func (p *LogParser) shortLevelToFull(short string) string {
	switch short {
	case "DBG":
		return "debug"
	case "INF":
		return "info"
	case "WRN":
		return "warn"
	case "ERR":
		return "error"
	default:
		return "info"
	}
}
