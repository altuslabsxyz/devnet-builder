// internal/daemon/controller/health_test.go
package controller

import (
	"context"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// mockHealthChecker implements HealthChecker for testing.
type mockHealthChecker struct {
	results map[string]*types.HealthCheckResult
	errors  map[string]error
}

func newMockHealthChecker() *mockHealthChecker {
	return &mockHealthChecker{
		results: make(map[string]*types.HealthCheckResult),
		errors:  make(map[string]error),
	}
}

func (m *mockHealthChecker) CheckHealth(ctx context.Context, node *types.Node) (*types.HealthCheckResult, error) {
	key := NodeKey(node.Spec.DevnetRef, node.Spec.Index)
	if err, ok := m.errors[key]; ok {
		return nil, err
	}
	if result, ok := m.results[key]; ok {
		return result, nil
	}
	// Default: healthy with some block height
	return &types.HealthCheckResult{
		NodeKey:     key,
		Healthy:     true,
		BlockHeight: node.Status.BlockHeight + 1,
		PeerCount:   5,
		CatchingUp:  false,
		CheckedAt:   time.Now(),
	}, nil
}

func (m *mockHealthChecker) SetResult(key string, result *types.HealthCheckResult) {
	m.results[key] = result
}

func (m *mockHealthChecker) SetError(key string, err error) {
	m.errors[key] = err
}

func TestHealthController_Reconcile_HealthyDevnet(t *testing.T) {
	ms := store.NewMemoryStore()
	checker := newMockHealthChecker()
	config := DefaultHealthControllerConfig()
	hc := NewHealthController(ms, checker, nil, config)

	// Create a devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase: types.PhaseRunning,
			Nodes: 2,
		},
	}
	if err := ms.CreateDevnet(context.Background(), devnet); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create running nodes
	for i := 0; i < 2; i++ {
		node := &types.Node{
			Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", i)},
			Spec: types.NodeSpec{
				DevnetRef: "test-devnet",
				Index:     i,
				Role:      "validator",
				Desired:   types.NodePhaseRunning,
			},
			Status: types.NodeStatus{
				Phase:         types.NodePhaseRunning,
				BlockHeight:   100,
				LastBlockTime: time.Now(),
			},
		}
		if err := ms.CreateNode(context.Background(), node); err != nil {
			t.Fatalf("CreateNode %d: %v", i, err)
		}
	}

	// Reconcile
	err := hc.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify devnet status updated
	got, _ := ms.GetDevnet(context.Background(), "", "test-devnet")
	if got.Status.ReadyNodes != 2 {
		t.Errorf("ReadyNodes = %d, want 2", got.Status.ReadyNodes)
	}
	if got.Status.Phase != types.PhaseRunning {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.PhaseRunning)
	}

	// Verify conditions
	var readyCond, healthyCond *types.Condition
	for i := range got.Status.Conditions {
		switch got.Status.Conditions[i].Type {
		case types.ConditionTypeReady:
			readyCond = &got.Status.Conditions[i]
		case types.ConditionTypeHealthy:
			healthyCond = &got.Status.Conditions[i]
		}
	}
	if readyCond == nil || readyCond.Status != types.ConditionTrue {
		t.Error("Expected Ready condition to be True")
	}
	if healthyCond == nil || healthyCond.Status != types.ConditionTrue {
		t.Error("Expected Healthy condition to be True")
	}
}

func TestHealthController_Reconcile_DegradedDevnet(t *testing.T) {
	ms := store.NewMemoryStore()
	checker := newMockHealthChecker()
	config := DefaultHealthControllerConfig()
	hc := NewHealthController(ms, checker, nil, config)

	// Create a devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase: types.PhaseRunning,
			Nodes: 2,
		},
	}
	if err := ms.CreateDevnet(context.Background(), devnet); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create one running and one crashed node
	runningNode := &types.Node{
		Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", 0)},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:         types.NodePhaseRunning,
			BlockHeight:   100,
			LastBlockTime: time.Now(),
		},
	}
	crashedNode := &types.Node{
		Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", 1)},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     1,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:   types.NodePhaseCrashed,
			Message: "Container failed",
		},
	}

	if err := ms.CreateNode(context.Background(), runningNode); err != nil {
		t.Fatalf("CreateNode 0: %v", err)
	}
	if err := ms.CreateNode(context.Background(), crashedNode); err != nil {
		t.Fatalf("CreateNode 1: %v", err)
	}

	// Reconcile
	err := hc.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify devnet becomes degraded
	got, _ := ms.GetDevnet(context.Background(), "", "test-devnet")
	if got.Status.ReadyNodes != 1 {
		t.Errorf("ReadyNodes = %d, want 1", got.Status.ReadyNodes)
	}
	if got.Status.Phase != types.PhaseDegraded {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.PhaseDegraded)
	}

	// Verify degraded condition
	var degradedCond *types.Condition
	for i := range got.Status.Conditions {
		if got.Status.Conditions[i].Type == types.ConditionTypeDegraded {
			degradedCond = &got.Status.Conditions[i]
			break
		}
	}
	if degradedCond == nil || degradedCond.Status != types.ConditionTrue {
		t.Error("Expected Degraded condition to be True")
	}
}

func TestHealthController_CrashRecovery(t *testing.T) {
	ms := store.NewMemoryStore()
	checker := newMockHealthChecker()
	mgr := NewManager()

	config := DefaultHealthControllerConfig()
	config.RestartPolicy.Enabled = true
	config.RestartPolicy.MaxRestarts = 3
	config.RestartPolicy.BackoffInitial = 1 * time.Millisecond // Fast for testing

	hc := NewHealthController(ms, checker, mgr, config)
	mgr.Register("health", hc)
	mgr.Register("nodes", &noopController{})

	// Create a devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Status:   types.DevnetStatus{Phase: types.PhaseRunning},
	}
	if err := ms.CreateDevnet(context.Background(), devnet); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create a crashed node with 0 restarts
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", 0)},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:        types.NodePhaseCrashed,
			RestartCount: 0,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := hc.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify node transitioned to Pending for restart
	got, _ := ms.GetNode(context.Background(), "", "test-devnet", 0)
	if got.Status.Phase != types.NodePhasePending {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhasePending)
	}
	if got.Status.RestartCount != 1 {
		t.Errorf("RestartCount = %d, want 1", got.Status.RestartCount)
	}
}

func TestHealthController_CrashRecovery_MaxRestartsExceeded(t *testing.T) {
	ms := store.NewMemoryStore()
	checker := newMockHealthChecker()

	config := DefaultHealthControllerConfig()
	config.RestartPolicy.Enabled = true
	config.RestartPolicy.MaxRestarts = 3

	hc := NewHealthController(ms, checker, nil, config)

	// Create a devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Status:   types.DevnetStatus{Phase: types.PhaseRunning},
	}
	if err := ms.CreateDevnet(context.Background(), devnet); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create a crashed node that exceeded max restarts
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", 0)},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:        types.NodePhaseCrashed,
			RestartCount: 3, // Already at max
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := hc.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify node stays crashed (not restarted)
	got, _ := ms.GetNode(context.Background(), "", "test-devnet", 0)
	if got.Status.Phase != types.NodePhaseCrashed {
		t.Errorf("Phase = %q, want %q (should not restart)", got.Status.Phase, types.NodePhaseCrashed)
	}
	if got.Status.RestartCount != 3 {
		t.Errorf("RestartCount = %d, want 3 (should not increment)", got.Status.RestartCount)
	}
}

func TestHealthController_StuckChainDetection(t *testing.T) {
	ms := store.NewMemoryStore()
	checker := newMockHealthChecker()

	config := DefaultHealthControllerConfig()
	config.StuckThreshold = 1 * time.Second // Short for testing

	hc := NewHealthController(ms, checker, nil, config)

	// Create a devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Status:   types.DevnetStatus{Phase: types.PhaseRunning},
	}
	if err := ms.CreateDevnet(context.Background(), devnet); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create a node with old block time (stuck)
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", 0)},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:         types.NodePhaseRunning,
			BlockHeight:   100,
			LastBlockTime: time.Now().Add(-5 * time.Second), // Old block time
			CatchingUp:    false,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Mock checker returns same block height
	checker.SetResult("test-devnet/0", &types.HealthCheckResult{
		NodeKey:     "test-devnet/0",
		Healthy:     true,
		BlockHeight: 100, // Same as before
		PeerCount:   5,
		CatchingUp:  false,
		CheckedAt:   time.Now(),
	})

	// Reconcile
	err := hc.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify devnet is degraded
	got, _ := ms.GetDevnet(context.Background(), "", "test-devnet")
	if got.Status.Phase != types.PhaseDegraded {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.PhaseDegraded)
	}

	// Verify degraded condition mentions stuck
	var degradedCond *types.Condition
	for i := range got.Status.Conditions {
		if got.Status.Conditions[i].Type == types.ConditionTypeDegraded {
			degradedCond = &got.Status.Conditions[i]
			break
		}
	}
	if degradedCond == nil || degradedCond.Status != types.ConditionTrue {
		t.Error("Expected Degraded condition to be True")
	}
	if degradedCond != nil && degradedCond.Reason != "ChainStuck" {
		t.Errorf("Reason = %q, want 'ChainStuck'", degradedCond.Reason)
	}
}

func TestHealthController_BackoffCalculation(t *testing.T) {
	config := DefaultHealthControllerConfig()
	config.RestartPolicy.BackoffInitial = 5 * time.Second
	config.RestartPolicy.BackoffMax = 5 * time.Minute
	config.RestartPolicy.BackoffMultiplier = 2.0

	hc := NewHealthController(nil, nil, nil, config)

	tests := []struct {
		restartCount int
		wantBackoff  time.Duration
	}{
		{0, 5 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 40 * time.Second},
		{4, 80 * time.Second},
		{5, 160 * time.Second},
		{10, 5 * time.Minute}, // Should cap at max
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := hc.calculateBackoff(tt.restartCount, config.RestartPolicy)
			if got != tt.wantBackoff {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.restartCount, got, tt.wantBackoff)
			}
		})
	}
}

func TestHealthController_Reconcile_DevnetNotFound(t *testing.T) {
	ms := store.NewMemoryStore()
	hc := NewHealthController(ms, nil, nil, DefaultHealthControllerConfig())

	// Reconcile a non-existent devnet should not error
	err := hc.Reconcile(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("Expected no error for non-existent devnet, got: %v", err)
	}
}

func TestHealthController_SkipsStoppedNodes(t *testing.T) {
	ms := store.NewMemoryStore()
	checker := newMockHealthChecker()
	hc := NewHealthController(ms, checker, nil, DefaultHealthControllerConfig())

	// Create a devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Status:   types.DevnetStatus{Phase: types.PhaseRunning, Nodes: 1},
	}
	if err := ms.CreateDevnet(context.Background(), devnet); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create a stopped node (desired = stopped)
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", 0)},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseStopped,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseStopped,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := hc.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify devnet shows all nodes healthy (stopped nodes are healthy if desired=stopped)
	got, _ := ms.GetDevnet(context.Background(), "", "test-devnet")
	if got.Status.ReadyNodes != 1 {
		t.Errorf("ReadyNodes = %d, want 1", got.Status.ReadyNodes)
	}
}

func TestHealthController_RestartBackoff(t *testing.T) {
	ms := store.NewMemoryStore()
	checker := newMockHealthChecker()

	config := DefaultHealthControllerConfig()
	config.RestartPolicy.Enabled = true
	config.RestartPolicy.BackoffInitial = 1 * time.Hour // Very long backoff

	hc := NewHealthController(ms, checker, nil, config)

	// Create a devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Status:   types.DevnetStatus{Phase: types.PhaseRunning},
	}
	if err := ms.CreateDevnet(context.Background(), devnet); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create a crashed node with backoff in the future
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: NodeKey("test-devnet", 0)},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:           types.NodePhaseCrashed,
			RestartCount:    1,
			NextRestartTime: time.Now().Add(1 * time.Hour), // Backoff not expired
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := hc.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify node stays crashed (backoff not expired)
	got, _ := ms.GetNode(context.Background(), "", "test-devnet", 0)
	if got.Status.Phase != types.NodePhaseCrashed {
		t.Errorf("Phase = %q, want %q (should wait for backoff)", got.Status.Phase, types.NodePhaseCrashed)
	}
	if got.Status.RestartCount != 1 {
		t.Errorf("RestartCount = %d, want 1 (should not increment)", got.Status.RestartCount)
	}
}

// noopController is a minimal mock for Controller interface.
type noopController struct{}

func (m *noopController) Reconcile(ctx context.Context, key string) error {
	return nil
}
