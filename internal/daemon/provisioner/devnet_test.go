package provisioner

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestDevnetProvisioner_Provision(t *testing.T) {
	s := store.NewMemoryStore()
	p := NewDevnetProvisioner(s, Config{DataDir: "/tmp/devnet"})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 3,
			FullNodes:  1,
			Mode:       "docker",
		},
	}

	// Provision
	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify nodes were created
	nodes, err := s.ListNodes(context.Background(), "", "test-devnet")
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(nodes) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(nodes))
	}

	// Check validator nodes
	validators := 0
	fullnodes := 0
	for _, node := range nodes {
		if node.Spec.Role == "validator" {
			validators++
		} else if node.Spec.Role == "fullnode" {
			fullnodes++
		}
	}

	if validators != 3 {
		t.Errorf("Expected 3 validators, got %d", validators)
	}
	if fullnodes != 1 {
		t.Errorf("Expected 1 fullnode, got %d", fullnodes)
	}
}

func TestDevnetProvisioner_ProvisionIdempotent(t *testing.T) {
	s := store.NewMemoryStore()
	p := NewDevnetProvisioner(s, Config{DataDir: "/tmp/devnet"})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			FullNodes:  0,
			Mode:       "docker",
		},
	}

	// Provision twice
	if err := p.Provision(context.Background(), devnet); err != nil {
		t.Fatalf("First Provision failed: %v", err)
	}
	if err := p.Provision(context.Background(), devnet); err != nil {
		t.Fatalf("Second Provision failed: %v", err)
	}

	// Should still have only 2 nodes
	nodes, _ := s.ListNodes(context.Background(), "", "test-devnet")
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes after idempotent provision, got %d", len(nodes))
	}
}

func TestDevnetProvisioner_Deprovision(t *testing.T) {
	s := store.NewMemoryStore()
	p := NewDevnetProvisioner(s, Config{DataDir: "/tmp/devnet"})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			FullNodes:  1,
			Mode:       "docker",
		},
	}

	// Provision first
	if err := p.Provision(context.Background(), devnet); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Deprovision
	if err := p.Deprovision(context.Background(), devnet); err != nil {
		t.Fatalf("Deprovision failed: %v", err)
	}

	// Verify nodes were deleted
	nodes, _ := s.ListNodes(context.Background(), "", "test-devnet")
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes after deprovision, got %d", len(nodes))
	}
}

func TestDevnetProvisioner_StartStop(t *testing.T) {
	s := store.NewMemoryStore()
	p := NewDevnetProvisioner(s, Config{DataDir: "/tmp/devnet"})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			FullNodes:  0,
			Mode:       "docker",
		},
	}

	// Provision
	if err := p.Provision(context.Background(), devnet); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Stop
	if err := p.Stop(context.Background(), devnet); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify all nodes have desired=Stopped
	nodes, _ := s.ListNodes(context.Background(), "", "test-devnet")
	for _, node := range nodes {
		if node.Spec.Desired != types.NodePhaseStopped {
			t.Errorf("Node %d: expected Desired=Stopped, got %s", node.Spec.Index, node.Spec.Desired)
		}
	}

	// Start
	if err := p.Start(context.Background(), devnet); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify all nodes have desired=Running
	nodes, _ = s.ListNodes(context.Background(), "", "test-devnet")
	for _, node := range nodes {
		if node.Spec.Desired != types.NodePhaseRunning {
			t.Errorf("Node %d: expected Desired=Running, got %s", node.Spec.Index, node.Spec.Desired)
		}
	}
}

func TestDevnetProvisioner_GetStatus(t *testing.T) {
	s := store.NewMemoryStore()
	p := NewDevnetProvisioner(s, Config{DataDir: "/tmp/devnet"})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 3,
			FullNodes:  0,
			Mode:       "docker",
		},
	}

	// Provision
	if err := p.Provision(context.Background(), devnet); err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// All nodes are Pending initially
	status, err := p.GetStatus(context.Background(), devnet)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Nodes != 3 {
		t.Errorf("Expected 3 nodes, got %d", status.Nodes)
	}
	if status.ReadyNodes != 0 {
		t.Errorf("Expected 0 ready nodes (all pending), got %d", status.ReadyNodes)
	}

	// Simulate one node becoming Running
	nodes, _ := s.ListNodes(context.Background(), "", "test-devnet")
	nodes[0].Status.Phase = types.NodePhaseRunning
	nodes[0].Status.BlockHeight = 100
	s.UpdateNode(context.Background(), nodes[0])

	// Check status again
	status, _ = p.GetStatus(context.Background(), devnet)
	if status.ReadyNodes != 1 {
		t.Errorf("Expected 1 ready node, got %d", status.ReadyNodes)
	}
	if status.CurrentHeight != 100 {
		t.Errorf("Expected height 100, got %d", status.CurrentHeight)
	}
	if status.Phase != types.PhaseDegraded {
		t.Errorf("Expected phase Degraded (1/3 ready), got %s", status.Phase)
	}

	// Make all nodes running
	for _, node := range nodes {
		node.Status.Phase = types.NodePhaseRunning
		s.UpdateNode(context.Background(), node)
	}

	status, _ = p.GetStatus(context.Background(), devnet)
	if status.Phase != types.PhaseRunning {
		t.Errorf("Expected phase Running (3/3 ready), got %s", status.Phase)
	}
}
