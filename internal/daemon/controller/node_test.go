// internal/daemon/controller/node_test.go
package controller

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestNodeController_Reconcile_PendingToStarting(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

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

	// Reconcile
	err := nc.Reconcile(context.Background(), "test/0")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Starting
	got, _ := ms.GetNode(context.Background(), "test", 0)
	if got.Status.Phase != types.NodePhaseStarting {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseStarting)
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
	got, _ := ms.GetNode(context.Background(), "test", 0)
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
	got, _ := ms.GetNode(context.Background(), "test", 0)
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
	got, _ := ms.GetNode(context.Background(), "test", 0)
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
	got, _ := ms.GetNode(context.Background(), "test", 0)
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
	got, _ := ms.GetNode(context.Background(), "test", 0)
	if got.Status.Phase != types.NodePhaseStopped {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.NodePhaseStopped)
	}
}

func TestNodeController_Reconcile_FullLifecycle(t *testing.T) {
	ms := store.NewMemoryStore()
	nc := NewNodeController(ms, nil)

	// Create a new node (Pending -> Starting -> Running)
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

	// Step 1: Pending -> Starting
	if err := nc.Reconcile(context.Background(), "test/0"); err != nil {
		t.Fatalf("Reconcile 1: %v", err)
	}
	node, _ = ms.GetNode(context.Background(), "test", 0)
	if node.Status.Phase != types.NodePhaseStarting {
		t.Fatalf("After step 1: Phase = %q, want %q", node.Status.Phase, types.NodePhaseStarting)
	}

	// Step 2: Starting -> Running
	if err := nc.Reconcile(context.Background(), "test/0"); err != nil {
		t.Fatalf("Reconcile 2: %v", err)
	}
	node, _ = ms.GetNode(context.Background(), "test", 0)
	if node.Status.Phase != types.NodePhaseRunning {
		t.Fatalf("After step 2: Phase = %q, want %q", node.Status.Phase, types.NodePhaseRunning)
	}

	// Step 3: Request stop
	node.Spec.Desired = types.NodePhaseStopped
	if err := ms.UpdateNode(context.Background(), node); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}

	// Step 4: Running -> Stopping
	if err := nc.Reconcile(context.Background(), "test/0"); err != nil {
		t.Fatalf("Reconcile 3: %v", err)
	}
	node, _ = ms.GetNode(context.Background(), "test", 0)
	if node.Status.Phase != types.NodePhaseStopping {
		t.Fatalf("After step 4: Phase = %q, want %q", node.Status.Phase, types.NodePhaseStopping)
	}

	// Step 5: Stopping -> Stopped
	if err := nc.Reconcile(context.Background(), "test/0"); err != nil {
		t.Fatalf("Reconcile 4: %v", err)
	}
	node, _ = ms.GetNode(context.Background(), "test", 0)
	if node.Status.Phase != types.NodePhaseStopped {
		t.Fatalf("After step 5: Phase = %q, want %q", node.Status.Phase, types.NodePhaseStopped)
	}
}

func TestParseNodeKey(t *testing.T) {
	tests := []struct {
		key         string
		wantDevnet  string
		wantIndex   int
		wantErr     bool
	}{
		{"mydevnet/0", "mydevnet", 0, false},
		{"mydevnet/10", "mydevnet", 10, false},
		{"my-devnet/5", "my-devnet", 5, false},
		{"invalid", "", 0, true},
		{"devnet/abc", "", 0, true},
		{"", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			devnet, index, err := ParseNodeKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNodeKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
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
