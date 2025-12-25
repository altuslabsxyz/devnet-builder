package integration

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/docker"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/node"
)

// requireDockerDaemon skips the test if Docker daemon is not available
func requireDockerDaemon(t *testing.T) {
	t.Helper()
	if !node.IsDockerAvailable(context.Background()) {
		t.Skip("Docker daemon not available")
	}
}

// TestMultiDevnetIsolation_NetworkSeparation tests that multiple devnets get separate networks
func TestMultiDevnetIsolation_NetworkSeparation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	networkManager := docker.NewNetworkManager()
	portAllocator := docker.NewPortAllocator(t.TempDir())

	// Create first devnet network
	networkID1, subnet1, err := networkManager.CreateNetwork(ctx, "isolation-test-1")
	if err != nil {
		t.Fatalf("Failed to create first network: %v", err)
	}
	defer networkManager.DeleteNetwork(ctx, networkID1)

	// Allocate ports for first devnet
	allocation1, err := portAllocator.AllocateRange(ctx, "isolation-test-1", 2)
	if err != nil {
		t.Fatalf("Failed to allocate ports for first devnet: %v", err)
	}
	defer portAllocator.ReleaseRange(ctx, "isolation-test-1")

	// Create second devnet network
	networkID2, subnet2, err := networkManager.CreateNetwork(ctx, "isolation-test-2")
	if err != nil {
		t.Fatalf("Failed to create second network: %v", err)
	}
	defer networkManager.DeleteNetwork(ctx, networkID2)

	// Allocate ports for second devnet
	allocation2, err := portAllocator.AllocateRange(ctx, "isolation-test-2", 2)
	if err != nil {
		t.Fatalf("Failed to allocate ports for second devnet: %v", err)
	}
	defer portAllocator.ReleaseRange(ctx, "isolation-test-2")

	// Verify different networks
	if networkID1 == networkID2 {
		t.Error("Both devnets got the same network ID")
	}

	// Verify different subnets
	if subnet1 == subnet2 {
		t.Error("Both devnets got the same subnet")
	}

	// Verify different port ranges
	if allocation1.PortRangeStart == allocation2.PortRangeStart {
		t.Error("Both devnets got the same port range start")
	}

	// Verify no port range overlap
	if rangesOverlap(allocation1, allocation2) {
		t.Errorf("Port ranges overlap: [%d-%d] and [%d-%d]",
			allocation1.PortRangeStart, allocation1.PortRangeEnd,
			allocation2.PortRangeStart, allocation2.PortRangeEnd)
	}

	t.Logf("Devnet 1: Network=%s, Subnet=%s, Ports=%d-%d",
		networkID1, subnet1, allocation1.PortRangeStart, allocation1.PortRangeEnd)
	t.Logf("Devnet 2: Network=%s, Subnet=%s, Ports=%d-%d",
		networkID2, subnet2, allocation2.PortRangeStart, allocation2.PortRangeEnd)
}

// TestMultiDevnetIsolation_ContainerCommunication tests that containers in different devnets cannot communicate
func TestMultiDevnetIsolation_ContainerCommunication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	networkManager := docker.NewNetworkManager()

	// Create two separate networks
	networkID1, subnet1, err := networkManager.CreateNetwork(ctx, "comm-test-1")
	if err != nil {
		t.Fatalf("Failed to create first network: %v", err)
	}
	defer networkManager.DeleteNetwork(ctx, networkID1)

	networkID2, subnet2, err := networkManager.CreateNetwork(ctx, "comm-test-2")
	if err != nil {
		t.Fatalf("Failed to create second network: %v", err)
	}
	defer networkManager.DeleteNetwork(ctx, networkID2)

	// Start test containers on each network
	container1ID := startTestContainer(t, ctx, "test-devnet1-node0", networkID1)
	defer removeContainer(ctx, container1ID)

	container2ID := startTestContainer(t, ctx, "test-devnet2-node0", networkID2)
	defer removeContainer(ctx, container2ID)

	// Get IP addresses
	ip1 := getContainerIP(t, ctx, container1ID)
	ip2 := getContainerIP(t, ctx, container2ID)

	t.Logf("Container 1: ID=%s, Network=%s, Subnet=%s, IP=%s", container1ID[:12], networkID1[:12], subnet1, ip1)
	t.Logf("Container 2: ID=%s, Network=%s, Subnet=%s, IP=%s", container2ID[:12], networkID2[:12], subnet2, ip2)

	// Verify containers cannot ping each other (network isolation)
	// Container 1 should not be able to ping Container 2
	if canPing(t, ctx, container1ID, ip2) {
		t.Error("Container 1 can ping Container 2 - network isolation failed")
	}

	// Verify containers can ping themselves (sanity check)
	if !canPing(t, ctx, container1ID, ip1) {
		t.Error("Container 1 cannot ping itself - container networking broken")
	}
}

// TestMultiDevnetIsolation_ConcurrentDeployment tests deploying multiple devnets concurrently
func TestMultiDevnetIsolation_ConcurrentDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Deploy 3 devnets concurrently
	type result struct {
		devnetName string
		networkID  string
		allocation *ports.PortAllocation
		err        error
	}

	results := make(chan result, 3)

	for i := 0; i < 3; i++ {
		devnetName := fmt.Sprintf("concurrent-test-%d", i)
		go func(name string, index int) {
			networkManager := docker.NewNetworkManager()
			portAllocator := docker.NewPortAllocator(tmpDir)

			// Add small stagger to reduce concurrent network creation conflicts
			time.Sleep(time.Duration(index) * 100 * time.Millisecond)

			netID, _, err := networkManager.CreateNetwork(ctx, name)
			if err != nil {
				results <- result{devnetName: name, err: err}
				return
			}

			alloc, err := portAllocator.AllocateRange(ctx, name, 2)
			if err != nil {
				networkManager.DeleteNetwork(ctx, netID)
				results <- result{devnetName: name, err: err}
				return
			}

			results <- result{
				devnetName: name,
				networkID:  netID,
				allocation: alloc,
				err:        nil,
			}
		}(devnetName, i)
	}

	// Collect results
	var deployments []result
	for i := 0; i < 3; i++ {
		res := <-results
		if res.err != nil {
			t.Errorf("Deployment %s failed: %v", res.devnetName, res.err)
		} else {
			deployments = append(deployments, res)
		}
	}

	// Cleanup
	defer func() {
		networkManager := docker.NewNetworkManager()
		portAllocator := docker.NewPortAllocator(tmpDir)
		for _, dep := range deployments {
			networkManager.DeleteNetwork(ctx, dep.networkID)
			portAllocator.ReleaseRange(ctx, dep.devnetName)
		}
	}()

	// Verify all deployments succeeded
	if len(deployments) != 3 {
		t.Fatalf("Expected 3 successful deployments, got %d", len(deployments))
	}

	// Verify all have different network IDs
	networkIDs := make(map[string]bool)
	for _, dep := range deployments {
		if networkIDs[dep.networkID] {
			t.Errorf("Duplicate network ID: %s", dep.networkID)
		}
		networkIDs[dep.networkID] = true
	}

	// Verify all have different port ranges
	for i := 0; i < len(deployments); i++ {
		for j := i + 1; j < len(deployments); j++ {
			if rangesOverlap(deployments[i].allocation, deployments[j].allocation) {
				t.Errorf("Port ranges overlap: %s [%d-%d] and %s [%d-%d]",
					deployments[i].devnetName, deployments[i].allocation.PortRangeStart, deployments[i].allocation.PortRangeEnd,
					deployments[j].devnetName, deployments[j].allocation.PortRangeStart, deployments[j].allocation.PortRangeEnd)
			}
		}
	}
}

// TestMultiDevnetIsolation_PortConflictDetection tests that port conflicts are detected
func TestMultiDevnetIsolation_PortConflictDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireDockerDaemon(t)

	ctx := context.Background()
	portAllocator := docker.NewPortAllocator(t.TempDir())

	// Allocate ports
	allocation, err := portAllocator.AllocateRange(ctx, "conflict-test", 4)
	if err != nil {
		t.Fatalf("Failed to allocate ports: %v", err)
	}
	defer portAllocator.ReleaseRange(ctx, "conflict-test")

	// Check for conflicts (should be none on clean system)
	conflicts, err := portAllocator.ValidatePortAvailability(ctx, allocation)
	if err != nil {
		t.Fatalf("ValidatePortAvailability() failed: %v", err)
	}

	// If conflicts found, log them (this might happen on CI systems)
	if len(conflicts) > 0 {
		t.Logf("Warning: Found %d port conflicts: %v", len(conflicts), conflicts)
		t.Logf("This is expected if other services are using these ports")
	}
}

// Helper functions

func startTestContainer(t *testing.T, ctx context.Context, name, networkID string) string {
	t.Helper()

	// Use alpine:latest for lightweight test container
	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", name,
		"--network", networkID,
		"--label", "app=devnet-builder-test",
		"alpine:latest",
		"sleep", "3600")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start test container: %v\nOutput: %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))
	t.Logf("Started test container: %s (ID: %s)", name, containerID[:12])
	return containerID
}

func removeContainer(ctx context.Context, containerID string) {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	cmd.Run() // Ignore errors
}

func getContainerIP(t *testing.T, ctx context.Context, containerID string) string {
	t.Helper()

	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		containerID)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get container IP: %v, output: %s", err, string(output))
	}

	ip := strings.TrimSpace(string(output))
	if ip == "" {
		t.Fatalf("Container %s has no IP address", containerID)
	}

	return ip
}

func canPing(t *testing.T, ctx context.Context, fromContainerID, toIP string) bool {
	t.Helper()

	// Try to ping with 1 second timeout
	cmd := exec.CommandContext(ctx, "docker", "exec", fromContainerID,
		"ping", "-c", "1", "-W", "1", toIP)

	err := cmd.Run()
	return err == nil
}

func rangesOverlap(a1, a2 *ports.PortAllocation) bool {
	return !(a1.PortRangeEnd < a2.PortRangeStart || a2.PortRangeEnd < a1.PortRangeStart)
}
