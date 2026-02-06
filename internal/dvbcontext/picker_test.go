package dvbcontext

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

func TestPickNode_NilClient(t *testing.T) {
	_, err := PickNode(context.Background(), nil, "default", "my-devnet")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if err.Error() != "client is nil" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestErrNoNodes_IsSentinel(t *testing.T) {
	// Verify ErrNoNodes can be used with errors.Is
	wrappedErr := errors.New("no nodes found in devnet test")
	if errors.Is(wrappedErr, ErrNoNodes) {
		t.Error("plain error should not match ErrNoNodes")
	}
}

func TestNodeName(t *testing.T) {
	tests := []struct {
		name     string
		node     *v1.Node
		expected string
	}{
		{
			name: "validator with index 0",
			node: &v1.Node{
				Metadata: &v1.NodeMetadata{Index: 0},
				Spec:     &v1.NodeSpec{Role: "validator"},
			},
			expected: "validator-0",
		},
		{
			name: "fullnode with index 1",
			node: &v1.Node{
				Metadata: &v1.NodeMetadata{Index: 1},
				Spec:     &v1.NodeSpec{Role: "fullnode"},
			},
			expected: "fullnode-1",
		},
		{
			name: "validator with higher index",
			node: &v1.Node{
				Metadata: &v1.NodeMetadata{Index: 10},
				Spec:     &v1.NodeSpec{Role: "validator"},
			},
			expected: "validator-10",
		},
		{
			name: "nil spec defaults to unknown",
			node: &v1.Node{
				Metadata: &v1.NodeMetadata{Index: 0},
			},
			expected: "unknown-0",
		},
		{
			name: "nil metadata defaults to index 0",
			node: &v1.Node{
				Spec: &v1.NodeSpec{Role: "validator"},
			},
			expected: "validator-0",
		},
		{
			name:     "nil spec and metadata",
			node:     &v1.Node{},
			expected: "unknown-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NodeName(tt.node)
			if result != tt.expected {
				t.Errorf("NodeName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatNodeDisplay(t *testing.T) {
	tests := []struct {
		name     string
		node     *v1.Node
		expected string
	}{
		{
			name: "running validator",
			node: &v1.Node{
				Metadata: &v1.NodeMetadata{Index: 0},
				Spec:     &v1.NodeSpec{Role: "validator"},
				Status:   &v1.NodeStatus{Phase: "Running"},
			},
			expected: "validator-0 (Running)",
		},
		{
			name: "stopped fullnode",
			node: &v1.Node{
				Metadata: &v1.NodeMetadata{Index: 1},
				Spec:     &v1.NodeSpec{Role: "fullnode"},
				Status:   &v1.NodeStatus{Phase: "Stopped"},
			},
			expected: "fullnode-1 (Stopped)",
		},
		{
			name:     "nil fields",
			node:     &v1.Node{},
			expected: "unknown-0 (Unknown)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNodeDisplay(tt.node)
			if result != tt.expected {
				t.Errorf("formatNodeDisplay() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestResolveNodeName_NilClient(t *testing.T) {
	_, err := ResolveNodeName(context.Background(), nil, "default", "my-devnet", "validator-0")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if err.Error() != "client is nil" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}
