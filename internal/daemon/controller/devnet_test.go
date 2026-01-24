package controller

import (
	"context"
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

	// Check that phase transitioned to Provisioning
	updated, err := s.GetDevnet(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	if updated.Status.Phase != types.PhaseProvisioning {
		t.Errorf("expected phase %s, got %s", types.PhaseProvisioning, updated.Status.Phase)
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

	updated, err := s.GetDevnet(context.Background(), "test-devnet")
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

	updated, err := s.GetDevnet(context.Background(), "running-devnet")
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

	updated, err := s.GetDevnet(context.Background(), "stopped-devnet")
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

	updated, err := s.GetDevnet(context.Background(), "degraded-devnet")
	if err != nil {
		t.Fatalf("failed to get devnet: %v", err)
	}

	// Should remain degraded (recovery would require provisioner)
	if updated.Status.Phase != types.PhaseDegraded {
		t.Errorf("expected phase %s, got %s", types.PhaseDegraded, updated.Status.Phase)
	}
}
