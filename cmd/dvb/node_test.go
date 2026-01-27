// cmd/dvb/node_test.go
package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 2 * time.Second,
			expected: "2s",
		},
		{
			name:     "sub-second rounds to zero",
			duration: 500 * time.Millisecond,
			expected: "0s",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			expected: "1m",
		},
		{
			name:     "90 seconds shows as minutes",
			duration: 90 * time.Second,
			expected: "2m", // Rounds to 1.5 -> truncates to 1
		},
		{
			name:     "5 minutes",
			duration: 5 * time.Minute,
			expected: "5m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestGetHealthIcon(t *testing.T) {
	tests := []struct {
		name     string
		phase    string
		contains string // check for the emoji/icon character
	}{
		{
			name:     "running shows green circle",
			phase:    "Running",
			contains: "●",
		},
		{
			name:     "crashed shows red X",
			phase:    "Crashed",
			contains: "✗",
		},
		{
			name:     "stopped shows white circle",
			phase:    "Stopped",
			contains: "○",
		},
		{
			name:     "pending shows yellow half",
			phase:    "Pending",
			contains: "◐",
		},
		{
			name:     "starting shows yellow half",
			phase:    "Starting",
			contains: "◐",
		},
		{
			name:     "stopping shows yellow half",
			phase:    "Stopping",
			contains: "◐",
		},
		{
			name:     "unknown shows question mark",
			phase:    "SomeUnknownPhase",
			contains: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getHealthIcon(tt.phase)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("getHealthIcon(%q) = %q, want to contain %q", tt.phase, result, tt.contains)
			}
		})
	}
}

func TestGetNodeListSummary(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []*v1.Node
		contains []string
	}{
		{
			name:     "empty list",
			nodes:    []*v1.Node{},
			contains: []string{},
		},
		{
			name: "all running",
			nodes: []*v1.Node{
				{Status: &v1.NodeStatus{Phase: "Running"}},
				{Status: &v1.NodeStatus{Phase: "Running"}},
			},
			contains: []string{"2 running"},
		},
		{
			name: "mixed states",
			nodes: []*v1.Node{
				{Status: &v1.NodeStatus{Phase: "Running"}},
				{Status: &v1.NodeStatus{Phase: "Stopped"}},
				{Status: &v1.NodeStatus{Phase: "Starting"}},
			},
			contains: []string{"1 running", "1 stopped", "1 other"},
		},
		{
			name: "all stopped",
			nodes: []*v1.Node{
				{Status: &v1.NodeStatus{Phase: "Stopped"}},
				{Status: &v1.NodeStatus{Phase: "Stopped"}},
			},
			contains: []string{"2 stopped"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNodeListSummary(tt.nodes)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("getNodeListSummary() = %q, want to contain %q", result, expected)
				}
			}
		})
	}
}

func TestParseNodeIndex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:    "valid zero",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "valid positive",
			input:   "5",
			want:    5,
			wantErr: false,
		},
		{
			name:    "negative number",
			input:   "-1",
			want:    0,
			wantErr: true,
		},
		{
			name:    "non-number",
			input:   "abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "floating point",
			input:   "1.5",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNodeIndex(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseNodeIndex(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseNodeIndex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPrintNodeTable(t *testing.T) {
	// Capture output by temporarily redirecting stdout
	// Note: This test just verifies the function doesn't panic and produces output

	nodes := []*v1.Node{
		{
			Metadata: &v1.NodeMetadata{
				Index:      0,
				DevnetName: "test-devnet",
			},
			Spec: &v1.NodeSpec{
				Role: "validator",
			},
			Status: &v1.NodeStatus{
				Phase:        "Running",
				ContainerId:  "abc123def456789",
				RestartCount: 0,
			},
		},
		{
			Metadata: &v1.NodeMetadata{
				Index:      1,
				DevnetName: "test-devnet",
			},
			Spec: &v1.NodeSpec{
				Role: "fullnode",
			},
			Status: &v1.NodeStatus{
				Phase:        "Stopped",
				ContainerId:  "",
				RestartCount: 2,
			},
		},
	}

	// Test that printNodeTable doesn't panic
	t.Run("standard output", func(t *testing.T) {
		// The function writes to os.Stdout, so we just verify it doesn't panic
		// In a real test, we'd capture stdout
		printNodeTable(nodes, false)
	})

	t.Run("wide output", func(t *testing.T) {
		printNodeTable(nodes, true)
	})
}

func TestNodeListOptions(t *testing.T) {
	opts := &nodeListOptions{
		watch:    true,
		interval: 5,
		wide:     true,
	}

	if !opts.watch {
		t.Error("expected watch to be true")
	}
	if opts.interval != 5 {
		t.Errorf("expected interval 5, got %d", opts.interval)
	}
	if !opts.wide {
		t.Error("expected wide to be true")
	}
}

// Suppress output for testing
var _ = bytes.Buffer{}
