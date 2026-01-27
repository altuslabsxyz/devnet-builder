// internal/daemon/runtime/process_test.go
package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestProcessRuntimeStartStop(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	// Create a test node that runs sleep
	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "test-node",
		},
		Spec: types.NodeSpec{
			BinaryPath: "sleep",
			HomeDir:    tempDir,
		},
	}

	// Custom start command for testing
	pr.SetCommandOverride("test-node", []string{"sleep", "30"})

	ctx := context.Background()

	// Start the node
	err := pr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	// Give process time to start (supervisor runs in background goroutine)
	time.Sleep(100 * time.Millisecond)

	// Check status
	status, err := pr.GetNodeStatus(ctx, "test-node")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}

	if !status.Running {
		t.Error("Node should be running")
	}

	if status.PID <= 0 {
		t.Error("Node should have a valid PID")
	}

	// Stop the node
	err = pr.StopNode(ctx, "test-node", true)
	if err != nil {
		t.Fatalf("StopNode failed: %v", err)
	}

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Check status again
	status, err = pr.GetNodeStatus(ctx, "test-node")
	if err != nil {
		t.Fatalf("GetNodeStatus after stop failed: %v", err)
	}

	if status.Running {
		t.Error("Node should not be running after stop")
	}
}

func TestProcessRuntimeAlreadyRunning(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "test-node-2",
		},
		Spec: types.NodeSpec{
			BinaryPath: "sleep",
			HomeDir:    tempDir,
		},
	}

	pr.SetCommandOverride("test-node-2", []string{"sleep", "30"})

	ctx := context.Background()

	// Start the node
	err := pr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Try to start again - should fail
	err = pr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err == nil {
		t.Error("Expected error when starting already running node")
	}

	// Cleanup
	_ = pr.StopNode(ctx, "test-node-2", true)
}

func TestProcessRuntimeCleanup(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	// Start multiple nodes
	for i := 0; i < 3; i++ {
		nodeID := "cleanup-test-node-" + string(rune('a'+i))
		node := &types.Node{
			Metadata: types.ResourceMeta{
				Name: nodeID,
			},
			Spec: types.NodeSpec{
				BinaryPath: "sleep",
				HomeDir:    tempDir,
			},
		}

		pr.SetCommandOverride(nodeID, []string{"sleep", "30"})

		err := pr.StartNode(context.Background(), node, StartOptions{
			RestartPolicy: RestartPolicy{Policy: "never"},
		})
		if err != nil {
			t.Fatalf("StartNode for %s failed: %v", nodeID, err)
		}
	}

	// Give processes time to start
	time.Sleep(100 * time.Millisecond)

	// Cleanup should stop all
	err := pr.Cleanup(context.Background())
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Give it a moment
	time.Sleep(100 * time.Millisecond)

	// All nodes should be stopped
	for i := 0; i < 3; i++ {
		nodeID := "cleanup-test-node-" + string(rune('a'+i))
		status, err := pr.GetNodeStatus(context.Background(), nodeID)
		if err != nil {
			t.Errorf("GetNodeStatus for %s failed: %v", nodeID, err)
			continue
		}
		if status.Running {
			t.Errorf("Node %s should not be running after cleanup", nodeID)
		}
	}
}

func TestProcessRuntimeStopNotFound(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	ctx := context.Background()

	// Stop a non-existent node
	err := pr.StopNode(ctx, "nonexistent-node", true)
	if err == nil {
		t.Error("Expected error when stopping non-existent node")
	}
}

func TestProcessRuntimeRestartNotRunning(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "restart-test-node",
		},
		Spec: types.NodeSpec{
			BinaryPath: "echo",
			HomeDir:    tempDir,
		},
	}

	pr.SetCommandOverride("restart-test-node", []string{"echo", "hello"})

	ctx := context.Background()

	// Start the node (it will exit immediately)
	err := pr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	// Wait for process to exit
	time.Sleep(200 * time.Millisecond)

	// Try to restart - should fail because process is not running
	err = pr.RestartNode(ctx, "restart-test-node")
	if err == nil {
		t.Error("Expected error when restarting non-running node")
	}
}
