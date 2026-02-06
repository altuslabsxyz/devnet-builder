package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// mockServiceBackend implements ServiceBackend for testing.
type mockServiceBackend struct {
	mu        sync.Mutex
	installed map[string]*ServiceDefinition
	running   map[string]bool
	pids      map[string]int

	// Error injection
	installErr error
	startErr   error
	stopErr    error
	statusErr  error
}

func newMockServiceBackend() *mockServiceBackend {
	return &mockServiceBackend{
		installed: make(map[string]*ServiceDefinition),
		running:   make(map[string]bool),
		pids:      make(map[string]int),
	}
}

func (m *mockServiceBackend) ServiceID(nodeID string) string {
	return "mock-" + nodeID
}

func (m *mockServiceBackend) InstallService(_ context.Context, def *ServiceDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.installErr != nil {
		return m.installErr
	}
	m.installed[def.ID] = def
	return nil
}

func (m *mockServiceBackend) UninstallService(_ context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.installed, serviceID)
	delete(m.running, serviceID)
	delete(m.pids, serviceID)
	return nil
}

func (m *mockServiceBackend) StartService(_ context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.running[serviceID] = true
	m.pids[serviceID] = 12345
	return nil
}

func (m *mockServiceBackend) StopService(_ context.Context, serviceID string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopErr != nil {
		return m.stopErr
	}
	m.running[serviceID] = false
	return nil
}

func (m *mockServiceBackend) RestartService(_ context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running[serviceID] = true
	m.pids[serviceID] = 12346
	return nil
}

func (m *mockServiceBackend) GetServiceStatus(_ context.Context, serviceID string) (*ServiceStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	running := m.running[serviceID]
	pid := m.pids[serviceID]
	return &ServiceStatus{
		Running: running,
		PID:     pid,
	}, nil
}

func (m *mockServiceBackend) IsInstalled(_ context.Context, serviceID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.installed[serviceID]
	return ok, nil
}

// Helper to create a ServiceRuntime with mock backend.
func newTestServiceRuntime(t *testing.T) (*ServiceRuntime, *mockServiceBackend) {
	t.Helper()
	backend := newMockServiceBackend()
	sr := &ServiceRuntime{
		config: ServiceRuntimeConfig{
			DataDir: t.TempDir(),
			Logger:  slog.Default(),
		},
		backend:    backend,
		logManager: NewLogManager(t.TempDir(), DefaultLogConfig()),
		services:   make(map[string]serviceInfo),
	}
	return sr, backend
}

func testNode(name string) *types.Node {
	return &types.Node{
		Metadata: types.ResourceMeta{
			Name: name,
		},
		Spec: types.NodeSpec{
			BinaryPath: "/usr/bin/stabled",
			HomeDir:    "/tmp/test-home",
			Network:    "stable",
		},
	}
}

func TestServiceRuntimeStartStop(t *testing.T) {
	sr, backend := newTestServiceRuntime(t)
	ctx := context.Background()
	node := testNode("test-node")

	// Start
	err := sr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "on-failure"},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	// Verify service was installed and started
	serviceID := backend.ServiceID("test-node")
	backend.mu.Lock()
	if _, ok := backend.installed[serviceID]; !ok {
		t.Error("service should be installed")
	}
	if !backend.running[serviceID] {
		t.Error("service should be running")
	}
	backend.mu.Unlock()

	// Check status
	status, err := sr.GetNodeStatus(ctx, "test-node")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if !status.Running {
		t.Error("node should be running")
	}

	// Stop
	err = sr.StopNode(ctx, "test-node", true)
	if err != nil {
		t.Fatalf("StopNode failed: %v", err)
	}

	// Verify service was uninstalled
	backend.mu.Lock()
	if _, ok := backend.installed[serviceID]; ok {
		t.Error("service should be uninstalled after stop")
	}
	backend.mu.Unlock()

	// Status should show not running
	status, err = sr.GetNodeStatus(ctx, "test-node")
	if err != nil {
		t.Fatalf("GetNodeStatus after stop failed: %v", err)
	}
	if status.Running {
		t.Error("node should not be running after stop")
	}
}

func TestServiceRuntimeAlreadyRunning(t *testing.T) {
	sr, _ := newTestServiceRuntime(t)
	ctx := context.Background()
	node := testNode("test-node")

	err := sr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err != nil {
		t.Fatalf("first StartNode failed: %v", err)
	}

	// Second start should fail
	err = sr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err == nil {
		t.Error("expected error starting already-running node")
	}
}

func TestServiceRuntimeStopNotFound(t *testing.T) {
	sr, _ := newTestServiceRuntime(t)
	ctx := context.Background()

	err := sr.StopNode(ctx, "nonexistent", true)
	if err == nil {
		t.Error("expected error stopping nonexistent node")
	}
}

func TestServiceRuntimeRestart(t *testing.T) {
	sr, _ := newTestServiceRuntime(t)
	ctx := context.Background()
	node := testNode("test-node")

	err := sr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "on-failure"},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	err = sr.RestartNode(ctx, "test-node")
	if err != nil {
		t.Fatalf("RestartNode failed: %v", err)
	}

	status, err := sr.GetNodeStatus(ctx, "test-node")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if !status.Running {
		t.Error("node should be running after restart")
	}
}

func TestServiceRuntimeRestartNotFound(t *testing.T) {
	sr, _ := newTestServiceRuntime(t)
	ctx := context.Background()

	err := sr.RestartNode(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error restarting nonexistent node")
	}
}

func TestServiceRuntimeCleanup(t *testing.T) {
	sr, backend := newTestServiceRuntime(t)
	ctx := context.Background()

	// Start multiple nodes
	for i := 0; i < 3; i++ {
		node := testNode(fmt.Sprintf("cleanup-node-%d", i))
		err := sr.StartNode(ctx, node, StartOptions{
			RestartPolicy: RestartPolicy{Policy: "never"},
		})
		if err != nil {
			t.Fatalf("StartNode failed: %v", err)
		}
	}

	// Cleanup all
	err := sr.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// All services should be uninstalled
	backend.mu.Lock()
	if len(backend.installed) != 0 {
		t.Errorf("expected 0 installed services, got %d", len(backend.installed))
	}
	backend.mu.Unlock()
}

func TestServiceRuntimeDiscoverExisting(t *testing.T) {
	sr, backend := newTestServiceRuntime(t)
	ctx := context.Background()

	// Pre-install services in the mock backend (simulating services from previous session)
	backend.mu.Lock()
	for _, name := range []string{"node-a", "node-b"} {
		svcID := backend.ServiceID(name)
		backend.installed[svcID] = &ServiceDefinition{ID: svcID, NodeID: name}
		backend.running[svcID] = true
	}
	backend.mu.Unlock()

	nodes := []*types.Node{
		{
			Metadata: types.ResourceMeta{Name: "node-a"},
			Status:   types.NodeStatus{Phase: types.NodePhaseRunning},
		},
		{
			Metadata: types.ResourceMeta{Name: "node-b"},
			Status:   types.NodeStatus{Phase: types.NodePhaseRunning},
		},
		{
			Metadata: types.ResourceMeta{Name: "node-c"},
			Status:   types.NodeStatus{Phase: types.NodePhaseStopped}, // Should be skipped
		},
	}

	discovered, err := sr.DiscoverExisting(ctx, nodes)
	if err != nil {
		t.Fatalf("DiscoverExisting failed: %v", err)
	}

	if discovered != 2 {
		t.Errorf("expected 2 discovered, got %d", discovered)
	}

	// Verify they're tracked
	sr.mu.RLock()
	if len(sr.services) != 2 {
		t.Errorf("expected 2 tracked services, got %d", len(sr.services))
	}
	sr.mu.RUnlock()

	// Now we should be able to get status
	status, err := sr.GetNodeStatus(ctx, "node-a")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if !status.Running {
		t.Error("discovered node should show as running")
	}
}

func TestServiceRuntimeDiscoverMissing(t *testing.T) {
	sr, _ := newTestServiceRuntime(t)
	ctx := context.Background()

	// No pre-installed services
	nodes := []*types.Node{
		{
			Metadata: types.ResourceMeta{Name: "orphan-node"},
			Status:   types.NodeStatus{Phase: types.NodePhaseRunning},
		},
	}

	discovered, err := sr.DiscoverExisting(ctx, nodes)
	if err != nil {
		t.Fatalf("DiscoverExisting failed: %v", err)
	}

	if discovered != 0 {
		t.Errorf("expected 0 discovered, got %d", discovered)
	}
}

func TestServiceRuntimeInstallError(t *testing.T) {
	sr, backend := newTestServiceRuntime(t)
	ctx := context.Background()
	node := testNode("fail-node")

	backend.installErr = fmt.Errorf("disk full")

	err := sr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err == nil {
		t.Error("expected error when install fails")
	}
}

func TestServiceRuntimeStartError(t *testing.T) {
	sr, backend := newTestServiceRuntime(t)
	ctx := context.Background()
	node := testNode("fail-node")

	backend.startErr = fmt.Errorf("service manager error")

	err := sr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "never"},
	})
	if err == nil {
		t.Error("expected error when start fails")
	}

	// Service should be cleaned up (uninstalled) on start failure
	backend.mu.Lock()
	if _, ok := backend.installed[backend.ServiceID("fail-node")]; ok {
		t.Error("service should be uninstalled after start failure")
	}
	backend.mu.Unlock()
}

func TestServiceRuntimeServiceDefinition(t *testing.T) {
	sr, backend := newTestServiceRuntime(t)
	ctx := context.Background()

	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "def-test"},
		Spec: types.NodeSpec{
			BinaryPath: "/usr/bin/stabled",
			HomeDir:    "/home/test/.stabled",
			Network:    "stable",
		},
	}

	err := sr.StartNode(ctx, node, StartOptions{
		RestartPolicy: RestartPolicy{Policy: "on-failure"},
		Env: map[string]string{
			"CUSTOM_VAR": "value",
		},
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	// Inspect the installed service definition
	serviceID := backend.ServiceID("def-test")
	backend.mu.Lock()
	def, ok := backend.installed[serviceID]
	backend.mu.Unlock()

	if !ok {
		t.Fatal("service not installed")
	}

	if def.ID != serviceID {
		t.Errorf("ID = %q, want %q", def.ID, serviceID)
	}
	if def.NodeID != "def-test" {
		t.Errorf("NodeID = %q, want %q", def.NodeID, "def-test")
	}
	if def.WorkingDirectory != "/home/test/.stabled" {
		t.Errorf("WorkingDirectory = %q, want %q", def.WorkingDirectory, "/home/test/.stabled")
	}
	if !def.RestartOnFailure {
		t.Error("RestartOnFailure should be true for on-failure policy")
	}
	if v, ok := def.Environment["CUSTOM_VAR"]; !ok || v != "value" {
		t.Errorf("Environment[CUSTOM_VAR] = %q, want %q", v, "value")
	}
}

func TestServiceRuntimeExecInNode(t *testing.T) {
	sr, _ := newTestServiceRuntime(t)
	ctx := context.Background()

	_, err := sr.ExecInNode(ctx, "any-node", []string{"echo", "hi"}, time.Second)
	if err == nil {
		t.Error("expected error from ExecInNode")
	}
}
