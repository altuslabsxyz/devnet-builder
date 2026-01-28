package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestDevnetController_ReconcileNew(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Create a new devnet in Pending phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "test-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase: types.PhasePending,
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile
	err = ctrl.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Check that phase transitioned to Running (Pending -> Provisioning -> Running in one cycle)
	// Without a provisioner, reconcilePending continues directly to reconcileProvisioning
	// which completes immediately (stub behavior)
	updated, err := s.GetDevnet(context.Background(), "", "test-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	if updated.Status.Phase != types.PhaseRunning {
		t.Errorf("expected phase %s, got %s", types.PhaseRunning, updated.Status.Phase)
	}
}

func TestDevnetController_ReconcileProvisioning(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Create a devnet in Provisioning phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "test-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase: types.PhaseProvisioning,
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile - without provisioner, should transition to Running (stub behavior)
	err = ctrl.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	updated, err := s.GetDevnet(context.Background(), "", "test-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	// Without provisioner, we just mark it as Running for now
	if updated.Status.Phase != types.PhaseRunning {
		t.Errorf("expected phase %s, got %s", types.PhaseRunning, updated.Status.Phase)
	}

	// Should have correct node count set
	expectedNodes := devnet.Spec.Validators + devnet.Spec.FullNodes
	if updated.Status.Nodes != expectedNodes {
		t.Errorf("expected %d nodes, got %d", expectedNodes, updated.Status.Nodes)
	}
}

func TestDevnetController_ReconcileNotFound(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Reconcile non-existent devnet - should not error (item deleted)
	err := ctrl.Reconcile(context.Background(), "non-existent")
	if err != nil {
		t.Errorf("reconcile should not error for deleted devnet: %v", err)
	}
}

func TestDevnetController_ReconcileRunning(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Create a devnet already in Running phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "running-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase:      types.PhaseRunning,
			Nodes:      4,
			ReadyNodes: 4,
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile - should remain Running
	err = ctrl.Reconcile(context.Background(), "running-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	updated, err := s.GetDevnet(context.Background(), "", "running-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	if updated.Status.Phase != types.PhaseRunning {
		t.Errorf("expected phase %s, got %s", types.PhaseRunning, updated.Status.Phase)
	}
}

func TestDevnetController_ReconcileStopped(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Create a devnet in Stopped phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "stopped-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase:      types.PhaseStopped,
			Nodes:      4,
			ReadyNodes: 0,
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile - should remain Stopped (nothing to do until explicit start)
	err = ctrl.Reconcile(context.Background(), "stopped-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	updated, err := s.GetDevnet(context.Background(), "", "stopped-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	if updated.Status.Phase != types.PhaseStopped {
		t.Errorf("expected phase %s, got %s", types.PhaseStopped, updated.Status.Phase)
	}
}

func TestDevnetController_ReconcileDegraded(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Create a devnet in Degraded phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "degraded-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase:      types.PhaseDegraded,
			Nodes:      4,
			ReadyNodes: 2, // Only 2 of 4 nodes healthy
			Message:    "some nodes unhealthy",
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile - without provisioner, can't recover, stays degraded
	err = ctrl.Reconcile(context.Background(), "degraded-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	updated, err := s.GetDevnet(context.Background(), "", "degraded-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	// Should remain degraded (recovery would require provisioner)
	if updated.Status.Phase != types.PhaseDegraded {
		t.Errorf("expected phase %s, got %s", types.PhaseDegraded, updated.Status.Phase)
	}
}

func TestDevnetController_ReconcilePending_SetsConditions(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Create a new devnet in Pending phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "test-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase: types.PhasePending,
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile
	err = ctrl.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Get updated devnet
	updated, err := s.GetDevnet(context.Background(), types.DefaultNamespace, "test-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	// With the fix, reconcilePending continues directly to reconcileProvisioning,
	// so we end up with final conditions (not intermediate ones).
	// Progressing should be False (complete), Ready should be True.

	// Check that Progressing condition is set to False (provisioning complete)
	progressing := types.GetCondition(updated.Status.Conditions, types.ConditionTypeProgressing)
	if progressing == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if progressing.Status != types.ConditionFalse {
		t.Errorf("expected Progressing status %s, got %s", types.ConditionFalse, progressing.Status)
	}
	if progressing.Reason != "ProvisioningComplete" {
		t.Errorf("expected Progressing reason %s, got %s", "ProvisioningComplete", progressing.Reason)
	}

	// Check that Ready condition is set to True
	ready := types.GetCondition(updated.Status.Conditions, types.ConditionTypeReady)
	if ready == nil {
		t.Fatal("expected Ready condition to be set")
	}
	if ready.Status != types.ConditionTrue {
		t.Errorf("expected Ready status %s, got %s", types.ConditionTrue, ready.Status)
	}
	if ready.Reason != types.ReasonAllNodesReady {
		t.Errorf("expected Ready reason %s, got %s", types.ReasonAllNodesReady, ready.Reason)
	}

	// Check that events were added (should have both provisioning start and complete events)
	if len(updated.Status.Events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(updated.Status.Events))
	}

	// Last event should be ProvisioningComplete
	lastEvent := updated.Status.Events[len(updated.Status.Events)-1]
	if lastEvent.Type != types.EventTypeNormal {
		t.Errorf("expected event type %s, got %s", types.EventTypeNormal, lastEvent.Type)
	}
	if lastEvent.Reason != "ProvisioningComplete" {
		t.Errorf("expected event reason %s, got %s", "ProvisioningComplete", lastEvent.Reason)
	}
}

func TestDevnetController_ReconcileProvisioning_SetsConditionsOnSuccess(t *testing.T) {
	s := store.NewMemoryStore()
	ctrl := NewDevnetController(s, nil)

	// Create a devnet in Provisioning phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "test-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase: types.PhaseProvisioning,
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile - without provisioner, should succeed and transition to Running
	err = ctrl.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Get updated devnet
	updated, err := s.GetDevnet(context.Background(), types.DefaultNamespace, "test-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	// Check that Progressing condition is set to False (complete)
	progressing := types.GetCondition(updated.Status.Conditions, types.ConditionTypeProgressing)
	if progressing == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if progressing.Status != types.ConditionFalse {
		t.Errorf("expected Progressing status %s, got %s", types.ConditionFalse, progressing.Status)
	}

	// Check that NodesCreated condition is set to True
	nodesCreated := types.GetCondition(updated.Status.Conditions, types.ConditionTypeNodesCreated)
	if nodesCreated == nil {
		t.Fatal("expected NodesCreated condition to be set")
	}
	if nodesCreated.Status != types.ConditionTrue {
		t.Errorf("expected NodesCreated status %s, got %s", types.ConditionTrue, nodesCreated.Status)
	}

	// Check that Ready condition is set to True
	ready := types.GetCondition(updated.Status.Conditions, types.ConditionTypeReady)
	if ready == nil {
		t.Fatal("expected Ready condition to be set")
	}
	if ready.Status != types.ConditionTrue {
		t.Errorf("expected Ready status %s, got %s", types.ConditionTrue, ready.Status)
	}

	// Check that an event was added
	if len(updated.Status.Events) == 0 {
		t.Fatal("expected at least one event to be added")
	}

	lastEvent := updated.Status.Events[len(updated.Status.Events)-1]
	if lastEvent.Type != types.EventTypeNormal {
		t.Errorf("expected event type %s, got %s", types.EventTypeNormal, lastEvent.Type)
	}
	if lastEvent.Reason != "ProvisioningComplete" {
		t.Errorf("expected event reason %s, got %s", "ProvisioningComplete", lastEvent.Reason)
	}
}

// mockFailingProvisioner is a mock provisioner that always fails with a specific error.
type mockFailingProvisioner struct {
	err error
}

func (m *mockFailingProvisioner) Provision(ctx context.Context, devnet *types.Devnet) error {
	return m.err
}

func (m *mockFailingProvisioner) Deprovision(ctx context.Context, devnet *types.Devnet) error {
	return nil
}

func (m *mockFailingProvisioner) Start(ctx context.Context, devnet *types.Devnet) error {
	return nil
}

func (m *mockFailingProvisioner) Stop(ctx context.Context, devnet *types.Devnet) error {
	return nil
}

func (m *mockFailingProvisioner) GetStatus(ctx context.Context, devnet *types.Devnet) (*types.DevnetStatus, error) {
	return nil, nil
}

func TestDevnetController_ReconcileProvisioning_SetsConditionsOnFailure(t *testing.T) {
	s := store.NewMemoryStore()
	mockProvisioner := &mockFailingProvisioner{err: fmt.Errorf("image not found: cosmos/test:latest")}
	ctrl := NewDevnetController(s, mockProvisioner)

	// Create a devnet in Provisioning phase
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:      "test-devnet",
			CreatedAt: time.Now(),
		},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: types.DevnetStatus{
			Phase: types.PhaseProvisioning,
		},
	}

	err := s.CreateDevnet(context.Background(), devnet)
	if err != nil {
		t.Fatalf("failed to create devnet: %v", err)
	}

	// Reconcile - should fail and transition to Degraded
	err = ctrl.Reconcile(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	// Get updated devnet
	updated, err := s.GetDevnet(context.Background(), types.DefaultNamespace, "test-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	// Check phase is Degraded
	if updated.Status.Phase != types.PhaseDegraded {
		t.Errorf("expected phase %s, got %s", types.PhaseDegraded, updated.Status.Phase)
	}

	// Check that Progressing condition is set to False
	progressing := types.GetCondition(updated.Status.Conditions, types.ConditionTypeProgressing)
	if progressing == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if progressing.Status != types.ConditionFalse {
		t.Errorf("expected Progressing status %s, got %s", types.ConditionFalse, progressing.Status)
	}
	if progressing.Reason != types.ReasonImageNotFound {
		t.Errorf("expected Progressing reason %s, got %s", types.ReasonImageNotFound, progressing.Reason)
	}

	// Check that Degraded condition is set to True
	degraded := types.GetCondition(updated.Status.Conditions, types.ConditionTypeDegraded)
	if degraded == nil {
		t.Fatal("expected Degraded condition to be set")
	}
	if degraded.Status != types.ConditionTrue {
		t.Errorf("expected Degraded status %s, got %s", types.ConditionTrue, degraded.Status)
	}
	if degraded.Reason != types.ReasonImageNotFound {
		t.Errorf("expected Degraded reason %s, got %s", types.ReasonImageNotFound, degraded.Reason)
	}

	// Check that a warning event was added
	if len(updated.Status.Events) == 0 {
		t.Fatal("expected at least one event to be added")
	}

	lastEvent := updated.Status.Events[len(updated.Status.Events)-1]
	if lastEvent.Type != types.EventTypeWarning {
		t.Errorf("expected event type %s, got %s", types.EventTypeWarning, lastEvent.Type)
	}
	if lastEvent.Reason != types.ReasonImageNotFound {
		t.Errorf("expected event reason %s, got %s", types.ReasonImageNotFound, lastEvent.Reason)
	}
}

func TestDevnetController_ClassifyProvisioningError(t *testing.T) {
	ctrl := NewDevnetController(nil, nil)

	tests := []struct {
		name           string
		err            error
		expectedReason string
	}{
		{
			name:           "image not found",
			err:            fmt.Errorf("image not found: cosmos/test:latest"),
			expectedReason: types.ReasonImageNotFound,
		},
		{
			name:           "credentials error",
			err:            fmt.Errorf("credentials not available"),
			expectedReason: types.ReasonCredentialsNotFound,
		},
		{
			name:           "mode not supported",
			err:            fmt.Errorf("mode kubernetes not supported"),
			expectedReason: types.ReasonModeNotSupported,
		},
		{
			name:           "binary not found",
			err:            fmt.Errorf("binary cosmos not found in PATH"),
			expectedReason: types.ReasonBinaryNotFound,
		},
		{
			name:           "container failed",
			err:            fmt.Errorf("container failed to start"),
			expectedReason: types.ReasonContainerFailed,
		},
		{
			name:           "network error",
			err:            fmt.Errorf("network timeout"),
			expectedReason: types.ReasonNetworkError,
		},
		{
			name:           "connection error",
			err:            fmt.Errorf("connection refused"),
			expectedReason: types.ReasonNetworkError,
		},
		{
			name:           "unknown error",
			err:            fmt.Errorf("something went wrong"),
			expectedReason: "ProvisioningFailed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, _ := ctrl.classifyProvisioningError(tt.err)
			if reason != tt.expectedReason {
				t.Errorf("expected reason %s, got %s", tt.expectedReason, reason)
			}
		})
	}
}
