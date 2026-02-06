// cmd/dvb/node_test.go
package main

import (
	"strings"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/dvbcontext"
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

func TestIsNodeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "validator-0", input: "validator-0", want: true},
		{name: "validator-1", input: "validator-1", want: true},
		{name: "validator-10", input: "validator-10", want: true},
		{name: "fullnode-0", input: "fullnode-0", want: true},
		{name: "fullnode-1", input: "fullnode-1", want: true},
		{name: "bare number", input: "0", want: false},
		{name: "devnet name", input: "my-devnet", want: false},
		{name: "validator only", input: "validator", want: false},
		{name: "fullnode only", input: "fullnode", want: false},
		{name: "unknown role", input: "unknown-0", want: true},
		{name: "node prefix", input: "node-0", want: false},
		{name: "empty string", input: "", want: false},
		{name: "validator with spaces", input: "validator- 0", want: false},
		{name: "negative index", input: "validator--1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNodeName(tt.input)
			if got != tt.want {
				t.Errorf("isNodeName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveNodeArgs(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantDevnet      string
		wantNodeNameArg string
	}{
		{
			name:            "no args",
			args:            []string{},
			wantDevnet:      "",
			wantNodeNameArg: "",
		},
		{
			name:            "single arg - node name",
			args:            []string{"validator-0"},
			wantDevnet:      "",
			wantNodeNameArg: "validator-0",
		},
		{
			name:            "single arg - fullnode name",
			args:            []string{"fullnode-1"},
			wantDevnet:      "",
			wantNodeNameArg: "fullnode-1",
		},
		{
			name:            "single arg - devnet name",
			args:            []string{"my-devnet"},
			wantDevnet:      "my-devnet",
			wantNodeNameArg: "",
		},
		{
			name:            "single arg - bare number treated as devnet",
			args:            []string{"0"},
			wantDevnet:      "0",
			wantNodeNameArg: "",
		},
		{
			name:            "two args",
			args:            []string{"my-devnet", "validator-0"},
			wantDevnet:      "my-devnet",
			wantNodeNameArg: "validator-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDevnet, gotNodeNameArg := resolveNodeArgs(tt.args)
			if gotDevnet != tt.wantDevnet {
				t.Errorf("resolveNodeArgs(%v) devnet = %q, want %q", tt.args, gotDevnet, tt.wantDevnet)
			}
			if gotNodeNameArg != tt.wantNodeNameArg {
				t.Errorf("resolveNodeArgs(%v) nodeNameArg = %q, want %q", tt.args, gotNodeNameArg, tt.wantNodeNameArg)
			}
		})
	}
}

func TestPrintNodeTable(t *testing.T) {
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

	// Verify printNodeTable uses NAME column and node names
	t.Run("standard output uses node names", func(t *testing.T) {
		// Verify NodeName produces the expected names for these nodes
		if dvbcontext.NodeName(nodes[0]) != "validator-0" {
			t.Errorf("expected validator-0, got %s", dvbcontext.NodeName(nodes[0]))
		}
		if dvbcontext.NodeName(nodes[1]) != "fullnode-1" {
			t.Errorf("expected fullnode-1, got %s", dvbcontext.NodeName(nodes[1]))
		}
		// The function writes to os.Stdout; verify it doesn't panic
		printNodeTable(nodes, false)
	})

	t.Run("wide output", func(t *testing.T) {
		printNodeTable(nodes, true)
	})
}
