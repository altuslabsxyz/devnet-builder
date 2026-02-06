// internal/daemon/controller/node_test.go
package controller

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// mockNodeRuntime implements runtime.NodeRuntime for testing.
type mockNodeRuntime struct {
	startNodeFn     func(ctx context.Context, node *types.Node, opts runtime.StartOptions) error
	stopNodeFn      func(ctx context.Context, nodeID string, graceful bool) error
	restartNodeFn   func(ctx context.Context, nodeID string) error
	getNodeStatusFn func(ctx context.Context, nodeID string) (*runtime.NodeStatus, error)
	getLogsFn       func(ctx context.Context, nodeID string, opts runtime.LogOptions) (io.ReadCloser, error)
	cleanupFn       func(ctx context.Context) error
}

func (m *mockNodeRuntime) StartNode(ctx context.Context, node *types.Node, opts runtime.StartOptions) error {
	if m.startNodeFn != nil {
		return m.startNodeFn(ctx, node, opts)
	}
	return nil
}

func (m *mockNodeRuntime) StopNode(ctx context.Context, nodeID string, graceful bool) error {
	if m.stopNodeFn != nil {
		return m.stopNodeFn(ctx, nodeID, graceful)
	}
	return nil
}

func (m *mockNodeRuntime) RestartNode(ctx context.Context, nodeID string) error {
	if m.restartNodeFn != nil {
		return m.restartNodeFn(ctx, nodeID)
	}
	return nil
}

func (m *mockNodeRuntime) GetNodeStatus(ctx context.Context, nodeID string) (*runtime.NodeStatus, error) {
	if m.getNodeStatusFn != nil {
		return m.getNodeStatusFn(ctx, nodeID)
	}
	return &runtime.NodeStatus{Running: true}, nil
}

func (m *mockNodeRuntime) GetLogs(ctx context.Context, nodeID string, opts runtime.LogOptions) (io.ReadCloser, error) {
	if m.getLogsFn != nil {
		return m.getLogsFn(ctx, nodeID, opts)
	}
	return nil, nil
}

func (m *mockNodeRuntime) Cleanup(ctx context.Context) error {
	if m.cleanupFn != nil {
		return m.cleanupFn(ctx)
	}
	return nil
}

func (m *mockNodeRuntime) ExecInNode(ctx context.Context, nodeID string, command []string, timeout time.Duration) (*runtime.ExecResult, error) {
	return nil, fmt.Errorf("exec not implemented in mock")
}

// Ensure mockNodeRuntime implements runtime.NodeRuntime
var _ runtime.NodeRuntime = (*mockNodeRuntime)(nil)

func TestNodeController_Reconcile_PendingToRunning(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil) // No runtime, so transitions directly to Running

	// Create a node in Pending phase with desired Running
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhasePending,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile - should go Pending -> Starting -> Running in one call
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Running (without runtime, continues directly)
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseRunning {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseRunning)
	}
}

func TestNodeController_Reconcile_StartingToRunning(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil) // No runtime, so no actual container start

	// Create a node in Starting phase
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseStarting,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Running
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseRunning {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseRunning)
	}
	if got.Status.Message != "Node is running" {
		t.Errorf("Message = %q, want %q", got.Status.Message, "Node is running")
	}
}

func TestNodeController_Reconcile_RunningToStopping(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

	// Create a running node with desired Stopped
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseStopped,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Stopping
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseStopping {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseStopping)
	}
}

func TestNodeController_Reconcile_StoppingToStopped(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

	// Create a stopping node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseStopping,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Stopped
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseStopped {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseStopped)
	}
	if got.Status.Message != "Node stopped" {
		t.Errorf("Message = %q, want %q", got.Status.Message, "Node stopped")
	}
}

func TestNodeController_Reconcile_StoppedToRestarting(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

	// Create a stopped node with desired Running
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:        types.NodePhaseStopped,
			RestartCount: 0,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Pending (restart)
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhasePending {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhasePending)
	}
	if got.Status.RestartCount != 1 {
		t.Errorf("RestartCount = %d, want 1", got.Status.RestartCount)
	}
}

func TestNodeController_Reconcile_NodeNotFound(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

	// Reconcile a non-existent node should not error
	err := nc.Reconcile(context.Background(), "nonexistent/0")
	if err != nil {
		t.Errorf("Expected no error for non-existent node, got: %v", err)
	}
}

func TestNodeController_Reconcile_InvalidKey(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

	// Invalid key format
	err := nc.Reconcile(context.Background(), "invalid-key")
	if err == nil {
		t.Error("Expected error for invalid key format")
	}

	// Invalid index
	err = nc.Reconcile(context.Background(), "devnet/abc")
	if err == nil {
		t.Error("Expected error for invalid index")
	}
}

func TestNodeController_Reconcile_CrashedToStopped(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

	// Create a crashed node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
		},
		Status: types.NodeStatus{
			Phase:   types.NodePhaseCrashed,
			Message: "Container failed",
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Stopped
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseStopped {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseStopped)
	}
}

func TestNodeController_Reconcile_FullLifecycle(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil) // No runtime

	// Create a new node (Pending -> Running in one reconcile without runtime)
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhasePending,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Step 1: Pending -> Starting -> Running (continues directly without runtime)
	if err := nc.Reconcile(context.Background(), "test/0"); err != nil {
		t.Fatalf("Reconcile 1: %v", err)
	}
	node, _ = ms.GetNode(context.Background(), "", "test", 0)
	if node.Status.Phase != types.NodePhaseRunning {
		t.Fatalf("After step 1: Phase = %q, want %q", node.Status.Phase, types.NodePhaseRunning)
	}

	// Step 2: Request stop
	node.Spec.Desired = types.NodePhaseStopped
	if err := ms.UpdateNode(context.Background(), node); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}

	// Step 3: Running -> Stopping
	if err := nc.Reconcile(context.Background(), "test/0"); err != nil {
		t.Fatalf("Reconcile 2: %v", err)
	}
	node, _ = ms.GetNode(context.Background(), "", "test", 0)
	if node.Status.Phase != types.NodePhaseStopping {
		t.Fatalf("After step 3: Phase = %q, want %q", node.Status.Phase, types.NodePhaseStopping)
	}

	// Step 4: Stopping -> Stopped
	if err := nc.Reconcile(context.Background(), "test/0"); err != nil {
		t.Fatalf("Reconcile 3: %v", err)
	}
	node, _ = ms.GetNode(context.Background(), "", "test", 0)
	if node.Status.Phase != types.NodePhaseStopped {
		t.Fatalf("After step 4: Phase = %q, want %q", node.Status.Phase, types.NodePhaseStopped)
	}
}

func TestParseNodeKey(t *testing.T) {
	tests := []struct {
		key           string
		wantNamespace string
		wantDevnet    string
		wantIndex     int
		wantErr       bool
	}{
		{"mydevnet/0", "default", "mydevnet", 0, false},
		{"mydevnet/10", "default", "mydevnet", 10, false},
		{"my-devnet/5", "default", "my-devnet", 5, false},
		{"ns/mydevnet/0", "ns", "mydevnet", 0, false},
		{"invalid", "", "", 0, true},
		{"devnet/abc", "", "", 0, true},
		{"", "", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			namespace, devnet, index, err := ParseNodeKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNodeKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if namespace != tt.wantNamespace {
					t.Errorf("namespace = %q, want %q", namespace, tt.wantNamespace)
				}
				if devnet != tt.wantDevnet {
					t.Errorf("devnet = %q, want %q", devnet, tt.wantDevnet)
				}
				if index != tt.wantIndex {
					t.Errorf("index = %d, want %d", index, tt.wantIndex)
				}
			}
		})
	}
}

func TestNodeKey(t *testing.T) {
	tests := []struct {
		devnet string
		index  int
		want   string
	}{
		{"mydevnet", 0, "mydevnet/0"},
		{"mydevnet", 10, "mydevnet/10"},
		{"my-devnet", 5, "my-devnet/5"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := NodeKey(tt.devnet, tt.index)
			if got != tt.want {
				t.Errorf("NodeKey(%q, %d) = %q, want %q", tt.devnet, tt.index, got, tt.want)
			}
		})
	}
}

// Tests with mockNodeRuntime to verify runtime interactions

func TestNodeController_Reconcile_StartingWithRuntime(t *testing.T) {
	ms := store.NewMemoryStore()

	startCalled := false
	mock := &mockNodeRuntime{
		startNodeFn: func(ctx context.Context, node *types.Node, opts runtime.StartOptions) error {
			startCalled = true
			if node.Spec.DevnetRef != "test" {
				t.Errorf("StartNode called with wrong devnet: %q", node.Spec.DevnetRef)
			}
			return nil
		},
	}
	nc := NewNodeController(ms, mock)

	// Create a node in Starting phase
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseStarting,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify StartNode was called
	if !startCalled {
		t.Error("StartNode was not called")
	}

	// Verify transition to Running
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseRunning {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseRunning)
	}
}

func TestNodeController_Reconcile_StartingWithRuntimeError(t *testing.T) {
	ms := store.NewMemoryStore()

	mock := &mockNodeRuntime{
		startNodeFn: func(ctx context.Context, node *types.Node, opts runtime.StartOptions) error {
			return fmt.Errorf("docker start failed")
		},
	}
	nc := NewNodeController(ms, mock)

	// Create a node in Starting phase
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseStarting,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Crashed
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseCrashed {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseCrashed)
	}
	if got.Status.Message != "Failed to start: docker start failed" {
		t.Errorf("Message = %q, want %q", got.Status.Message, "Failed to start: docker start failed")
	}
}

func TestNodeController_Reconcile_RunningWithRuntimeNotRunning(t *testing.T) {
	ms := store.NewMemoryStore()

	mock := &mockNodeRuntime{
		getNodeStatusFn: func(ctx context.Context, nodeID string) (*runtime.NodeStatus, error) {
			return &runtime.NodeStatus{Running: false}, nil
		},
	}
	nc := NewNodeController(ms, mock)

	// Create a running node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
			Desired:   types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Crashed (node stopped unexpectedly)
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseCrashed {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseCrashed)
	}
}

func TestNodeController_Reconcile_StoppingWithRuntime(t *testing.T) {
	ms := store.NewMemoryStore()

	stopCalled := false
	mock := &mockNodeRuntime{
		stopNodeFn: func(ctx context.Context, nodeID string, graceful bool) error {
			stopCalled = true
			if nodeID != "test-0" {
				t.Errorf("StopNode called with wrong nodeID: %q", nodeID)
			}
			if !graceful {
				t.Error("StopNode called with graceful=false, want true")
			}
			return nil
		},
	}
	nc := NewNodeController(ms, mock)

	// Create a stopping node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
			Role:      "validator",
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseStopping,
		},
	}
	if err := ms.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify StopNode was called
	if !stopCalled {
		t.Error("StopNode was not called")
	}

	// Verify transition to Stopped
	got, _ := ms.GetNode(context.Background(), "", "test", 0)
	if got.Status.Phase != types.NodePhaseStopped {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseStopped)
	}
}
