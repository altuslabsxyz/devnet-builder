// internal/daemon/runtime/process_test.go
package runtime

import (
	"context"
	"os"
	"syscall"
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

func TestProcessRuntimeDetach(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	// Start multiple nodes
	var pids []int
	for i := 0; i < 2; i++ {
		nodeID := "detach-test-node-" + string(rune('a'+i))
		node := &types.Node{
			Metadata: types.ResourceMeta{
				Name: nodeID,
			},
			Spec: types.NodeSpec{
				BinaryPath: "sleep",
				HomeDir:    tempDir,
			},
		}

		pr.SetCommandOverride(nodeID, []string{"sleep", "60"})

		err := pr.StartNode(context.Background(), node, StartOptions{
			RestartPolicy: RestartPolicy{Policy: "never"},
		})
		if err != nil {
			t.Fatalf("StartNode for %s failed: %v", nodeID, err)
		}
	}

	// Give processes time to start
	time.Sleep(100 * time.Millisecond)

	// Get PIDs before detach
	for i := 0; i < 2; i++ {
		nodeID := "detach-test-node-" + string(rune('a'+i))
		status, err := pr.GetNodeStatus(context.Background(), nodeID)
		if err != nil {
			t.Fatalf("GetNodeStatus for %s failed: %v", nodeID, err)
		}
		if !status.Running || status.PID <= 0 {
			t.Fatalf("Node %s should be running with valid PID", nodeID)
		}
		pids = append(pids, status.PID)
	}

	// Detach should NOT stop processes
	err := pr.Detach()
	if err != nil {
		t.Fatalf("Detach failed: %v", err)
	}

	// Give a moment for supervisor to exit
	time.Sleep(100 * time.Millisecond)

	// Verify processes are still running (using signal 0)
	for i, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			t.Errorf("FindProcess for PID %d failed: %v", pid, err)
			continue
		}
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			t.Errorf("Process %d (PID %d) should still be running after detach: %v", i, pid, err)
		}
	}

	// Cleanup: kill the orphaned processes
	for _, pid := range pids {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}
}

func TestProcessRuntimeReconnect(t *testing.T) {
	tempDir := t.TempDir()

	// First runtime starts a process
	pr1 := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "reconnect-test-node",
		},
		Spec: types.NodeSpec{
			BinaryPath: "sleep",
			HomeDir:    tempDir,
		},
	}

	pr1.SetCommandOverride("reconnect-test-node", []string{"sleep", "60"})

	ctx := context.Background()

	err := pr1.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Get PID
	status, err := pr1.GetNodeStatus(ctx, "reconnect-test-node")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if !status.Running || status.PID <= 0 {
		t.Fatal("Node should be running with valid PID")
	}
	originalPID := status.PID

	// Detach (simulates daemon shutdown)
	err = pr1.Detach()
	if err != nil {
		t.Fatalf("Detach failed: %v", err)
	}

	// Create new runtime (simulates daemon restart)
	pr2 := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	// Update node with stored PID (simulating what ReconnectAll would do)
	node.Status.PID = originalPID
	node.Status.Phase = types.NodePhaseRunning

	// Reconnect to the running process
	ok, err := pr2.ReconnectNode(ctx, node, originalPID)
	if err != nil {
		t.Fatalf("ReconnectNode failed: %v", err)
	}
	if !ok {
		t.Fatal("ReconnectNode should return true for running process")
	}

	// Verify we can get status through the new runtime
	status2, err := pr2.GetNodeStatus(ctx, "reconnect-test-node")
	if err != nil {
		t.Fatalf("GetNodeStatus on new runtime failed: %v", err)
	}
	if !status2.Running {
		t.Error("Node should be running through reconnected supervisor")
	}
	if status2.PID != originalPID {
		t.Errorf("PID mismatch: got %d, want %d", status2.PID, originalPID)
	}

	// Cleanup: stop through new runtime
	err = pr2.StopNode(ctx, "reconnect-test-node", true)
	if err != nil {
		// Monitoring supervisor in detach mode doesn't kill, so use signal
		if proc, err := os.FindProcess(originalPID); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}
}

func TestProcessRuntimeReconnectDeadProcess(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "dead-reconnect-test",
		},
		Spec: types.NodeSpec{
			BinaryPath: "nonexistent-binary",
			HomeDir:    tempDir,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
			PID:   999999, // Almost certainly not running
		},
	}

	ctx := context.Background()

	// Try to reconnect to a dead PID
	ok, err := pr.ReconnectNode(ctx, node, 999999)
	if err != nil {
		t.Fatalf("ReconnectNode failed: %v", err)
	}
	if ok {
		t.Error("ReconnectNode should return false for dead process")
	}
}

func TestValidateProcessFallback(t *testing.T) {
	tempDir := t.TempDir()

	pr := NewProcessRuntime(ProcessRuntimeConfig{
		DataDir: tempDir,
	})

	// Start a real process to validate against
	pr.SetCommandOverride("validate-test", []string{"sleep", "60"})

	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "validate-test",
		},
		Spec: types.NodeSpec{
			BinaryPath: "sleep",
			HomeDir:    tempDir,
		},
	}

	ctx := context.Background()

	err := pr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	status, err := pr.GetNodeStatus(ctx, "validate-test")
	if err != nil || status.PID <= 0 {
		t.Fatalf("Failed to get running process: %v", err)
	}

	pid := status.PID

	// Validation should pass for matching process
	valid := pr.validateProcess(pid, node)
	if !valid {
		t.Error("validateProcess should return true for matching process")
	}

	// Validation with wrong binary should fail
	wrongNode := &types.Node{
		Spec: types.NodeSpec{
			BinaryPath: "/totally/wrong/binary",
			HomeDir:    "/wrong/home",
		},
	}
	valid = pr.validateProcess(pid, wrongNode)
	if valid {
		t.Error("validateProcess should return false for non-matching process")
	}

	// Cleanup
	_ = pr.StopNode(ctx, "validate-test", true)
}
