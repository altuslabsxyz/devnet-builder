package integration

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/docker"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/node"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// TestRollback_NetworkCleanup tests that networks are cleaned up on rollback
func TestRollback_NetworkCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	logger := output.NewLogger()
	networkManager := docker.NewNetworkManager()
	portAllocator := docker.NewPortAllocator(t.TempDir())
	dockerManager := node.NewDockerManager("alpine:latest", logger)

	orchestrator := docker.NewOrchestrator(networkManager, portAllocator, dockerManager, logger)

	// Create deployment state with network and ports
	networkID, _, err := networkManager.CreateNetwork(ctx, "rollback-test-network")
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	allocation, err := portAllocator.AllocateRange(ctx, "rollback-test", 2)
	if err != nil {
		networkManager.DeleteNetwork(ctx, networkID)
		t.Fatalf("Failed to allocate ports: %v", err)
	}

	// Build state for rollback
	state := &ports.DeploymentState{
		DevnetName:        "rollback-test",
		NetworkID:         &networkID,
		PortRange:         allocation,
		StartedContainers: []string{},
		Phase:             ports.PhaseNetworkCreating,
	}

	// Verify network exists before rollback
	exists, err := networkManager.NetworkExists(ctx, networkID)
	if err != nil {
		t.Fatalf("NetworkExists() failed: %v", err)
	}
	if !exists {
		t.Fatal("Network doesn't exist before rollback")
	}

	// Execute rollback
	err = orchestrator.Rollback(ctx, state)
	if err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	// Verify network was deleted
	exists, err = networkManager.NetworkExists(ctx, networkID)
	if err != nil {
		t.Fatalf("NetworkExists() after rollback failed: %v", err)
	}
	if exists {
		t.Error("Network still exists after rollback")
	}

	// Verify ports were released
	retrieved, err := portAllocator.GetAllocation(ctx, "rollback-test")
	if err != nil {
		t.Fatalf("GetAllocation() after rollback failed: %v", err)
	}
	if retrieved != nil {
		t.Error("Port allocation still exists after rollback")
	}
}

// TestRollback_ContainerCleanup tests that containers are cleaned up on rollback
func TestRollback_ContainerCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	logger := output.NewLogger()
	networkManager := docker.NewNetworkManager()
	portAllocator := docker.NewPortAllocator(t.TempDir())
	dockerManager := node.NewDockerManager("alpine:latest", logger)

	orchestrator := docker.NewOrchestrator(networkManager, portAllocator, dockerManager, logger)

	// Create network
	networkID, _, err := networkManager.CreateNetwork(ctx, "rollback-container-test")
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}
	defer networkManager.DeleteNetwork(ctx, networkID)

	// Allocate ports
	allocation, err := portAllocator.AllocateRange(ctx, "rollback-container-test", 2)
	if err != nil {
		t.Fatalf("Failed to allocate ports: %v", err)
	}
	defer portAllocator.ReleaseRange(ctx, "rollback-container-test")

	// Start test containers
	container1 := startTestContainer(t, ctx, "rollback-test-container-1", networkID)
	container2 := startTestContainer(t, ctx, "rollback-test-container-2", networkID)

	// Verify containers are running
	if !containerIsRunning(ctx, container1) {
		t.Fatal("Container 1 is not running")
	}
	if !containerIsRunning(ctx, container2) {
		t.Fatal("Container 2 is not running")
	}

	// Build state with started containers
	state := &ports.DeploymentState{
		DevnetName:        "rollback-container-test",
		NetworkID:         &networkID,
		PortRange:         allocation,
		StartedContainers: []string{container1, container2},
		Phase:             ports.PhaseContainerStarting,
	}

	// Execute rollback
	err = orchestrator.Rollback(ctx, state)
	if err != nil {
		t.Logf("Rollback returned error (may be expected): %v", err)
	}

	// Wait a moment for containers to stop
	time.Sleep(2 * time.Second)

	// Verify containers are stopped and removed
	if containerExists(ctx, container1) {
		t.Error("Container 1 still exists after rollback")
	}
	if containerExists(ctx, container2) {
		t.Error("Container 2 still exists after rollback")
	}
}

// TestRollback_PartialState tests rollback with partial deployment state
func TestRollback_PartialState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	logger := output.NewLogger()
	networkManager := docker.NewNetworkManager()
	portAllocator := docker.NewPortAllocator(t.TempDir())
	dockerManager := node.NewDockerManager("alpine:latest", logger)

	orchestrator := docker.NewOrchestrator(networkManager, portAllocator, dockerManager, logger)

	tests := []struct {
		name       string
		setupState func(t *testing.T) *ports.DeploymentState
		verify     func(t *testing.T, ctx context.Context, state *ports.DeploymentState)
	}{
		{
			name: "rollback with only network",
			setupState: func(t *testing.T) *ports.DeploymentState {
				netID, _, err := networkManager.CreateNetwork(ctx, "partial-network-only")
				if err != nil {
					t.Fatalf("Failed to create network: %v", err)
				}
				return &ports.DeploymentState{
					DevnetName: "partial-network-only",
					NetworkID:  &netID,
					Phase:      ports.PhaseNetworkCreating,
				}
			},
			verify: func(t *testing.T, ctx context.Context, state *ports.DeploymentState) {
				exists, _ := networkManager.NetworkExists(ctx, *state.NetworkID)
				if exists {
					t.Error("Network still exists after rollback")
				}
			},
		},
		{
			name: "rollback with only ports",
			setupState: func(t *testing.T) *ports.DeploymentState {
				alloc, err := portAllocator.AllocateRange(ctx, "partial-ports-only", 2)
				if err != nil {
					t.Fatalf("Failed to allocate ports: %v", err)
				}
				return &ports.DeploymentState{
					DevnetName: "partial-ports-only",
					PortRange:  alloc,
					Phase:      ports.PhasePortAllocating,
				}
			},
			verify: func(t *testing.T, ctx context.Context, state *ports.DeploymentState) {
				alloc, _ := portAllocator.GetAllocation(ctx, state.DevnetName)
				if alloc != nil {
					t.Error("Port allocation still exists after rollback")
				}
			},
		},
		{
			name: "rollback with network and ports",
			setupState: func(t *testing.T) *ports.DeploymentState {
				netID, _, err := networkManager.CreateNetwork(ctx, "partial-both")
				if err != nil {
					t.Fatalf("Failed to create network: %v", err)
				}
				alloc, err := portAllocator.AllocateRange(ctx, "partial-both", 2)
				if err != nil {
					networkManager.DeleteNetwork(ctx, netID)
					t.Fatalf("Failed to allocate ports: %v", err)
				}
				return &ports.DeploymentState{
					DevnetName: "partial-both",
					NetworkID:  &netID,
					PortRange:  alloc,
					Phase:      ports.PhasePortAllocating,
				}
			},
			verify: func(t *testing.T, ctx context.Context, state *ports.DeploymentState) {
				exists, _ := networkManager.NetworkExists(ctx, *state.NetworkID)
				if exists {
					t.Error("Network still exists after rollback")
				}
				alloc, _ := portAllocator.GetAllocation(ctx, state.DevnetName)
				if alloc != nil {
					t.Error("Port allocation still exists after rollback")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.setupState(t)

			// Execute rollback
			err := orchestrator.Rollback(ctx, state)
			if err != nil {
				t.Logf("Rollback returned error (may be expected): %v", err)
			}

			// Verify cleanup
			tt.verify(t, ctx, state)
		})
	}
}

// TestRollback_MultipleFailures tests that rollback continues even if some cleanup fails
func TestRollback_MultipleFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	logger := output.NewLogger()
	networkManager := docker.NewNetworkManager()
	portAllocator := docker.NewPortAllocator(t.TempDir())
	dockerManager := node.NewDockerManager("alpine:latest", logger)

	orchestrator := docker.NewOrchestrator(networkManager, portAllocator, dockerManager, logger)

	// Create valid network and ports
	networkID, _, err := networkManager.CreateNetwork(ctx, "multi-fail-test")
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	allocation, err := portAllocator.AllocateRange(ctx, "multi-fail-test", 2)
	if err != nil {
		networkManager.DeleteNetwork(ctx, networkID)
		t.Fatalf("Failed to allocate ports: %v", err)
	}

	// Build state with mix of valid and invalid resources
	state := &ports.DeploymentState{
		DevnetName: "multi-fail-test",
		NetworkID:  &networkID,
		PortRange:  allocation,
		StartedContainers: []string{
			"non-existent-container-1",
			"non-existent-container-2",
		},
		Phase: ports.PhaseContainerStarting,
	}

	// Execute rollback - should continue despite container removal failures
	err = orchestrator.Rollback(ctx, state)
	if err == nil {
		t.Log("Rollback succeeded despite non-existent containers (good - continued cleanup)")
	} else {
		t.Logf("Rollback returned error: %v (expected for non-existent containers)", err)
	}

	// Verify that valid resources were still cleaned up
	exists, _ := networkManager.NetworkExists(ctx, networkID)
	if exists {
		t.Error("Network still exists - cleanup was abandoned")
	}

	alloc, _ := portAllocator.GetAllocation(ctx, "multi-fail-test")
	if alloc != nil {
		t.Error("Port allocation still exists - cleanup was abandoned")
	}
}

// TestRollback_Idempotency tests that rollback can be called multiple times safely
func TestRollback_Idempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	logger := output.NewLogger()
	networkManager := docker.NewNetworkManager()
	portAllocator := docker.NewPortAllocator(t.TempDir())
	dockerManager := node.NewDockerManager("alpine:latest", logger)

	orchestrator := docker.NewOrchestrator(networkManager, portAllocator, dockerManager, logger)

	// Create resources
	networkID, _, err := networkManager.CreateNetwork(ctx, "idempotent-test")
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	allocation, err := portAllocator.AllocateRange(ctx, "idempotent-test", 2)
	if err != nil {
		networkManager.DeleteNetwork(ctx, networkID)
		t.Fatalf("Failed to allocate ports: %v", err)
	}

	state := &ports.DeploymentState{
		DevnetName: "idempotent-test",
		NetworkID:  &networkID,
		PortRange:  allocation,
		Phase:      ports.PhasePortAllocating,
	}

	// First rollback
	err = orchestrator.Rollback(ctx, state)
	if err != nil {
		t.Fatalf("First rollback failed: %v", err)
	}

	// Second rollback (should not panic or error badly)
	err = orchestrator.Rollback(ctx, state)
	if err != nil {
		t.Logf("Second rollback returned error (expected): %v", err)
	} else {
		t.Log("Second rollback succeeded (idempotent)")
	}

	// Verify resources are gone
	exists, _ := networkManager.NetworkExists(ctx, networkID)
	if exists {
		t.Error("Network still exists after rollbacks")
	}

	alloc, _ := portAllocator.GetAllocation(ctx, "idempotent-test")
	if alloc != nil {
		t.Error("Port allocation still exists after rollbacks")
	}
}

// Helper functions

func containerIsRunning(ctx context.Context, containerID string) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Running}}",
		containerID)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

func containerExists(ctx context.Context, containerID string) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	err := cmd.Run()
	return err == nil
}
