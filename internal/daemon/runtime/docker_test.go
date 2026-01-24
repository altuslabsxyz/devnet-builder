package runtime

import (
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// TestDockerRuntimeImplementsInterface verifies DockerRuntime implements NodeRuntime.
func TestDockerRuntimeImplementsInterface(t *testing.T) {
	// This is a compile-time check - if DockerRuntime doesn't implement
	// NodeRuntime, this won't compile.
	var _ controller.NodeRuntime = (*DockerRuntime)(nil)
}

func TestContainerName(t *testing.T) {
	tests := []struct {
		name     string
		node     *types.Node
		expected string
	}{
		{
			name: "basic node",
			node: &types.Node{
				Spec: types.NodeSpec{
					DevnetRef: "mydevnet",
					Index:     0,
				},
			},
			expected: "dvb-mydevnet-node-0",
		},
		{
			name: "multi-digit index",
			node: &types.Node{
				Spec: types.NodeSpec{
					DevnetRef: "testnet",
					Index:     42,
				},
			},
			expected: "dvb-testnet-node-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerName(tt.node)
			if got != tt.expected {
				t.Errorf("containerName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Integration tests would go here - they require Docker to be running.
// For now, we just test the interface compliance and helper functions.
