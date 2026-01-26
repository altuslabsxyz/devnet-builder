package provisioner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
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

// =============================================================================
// Orchestrator Integration Tests
// =============================================================================

// mockOrchestrator implements the Orchestrator interface for testing
type mockOrchestrator struct {
	executeCalled bool
	executeOpts   ports.ProvisionOptions
	executeResult *ports.ProvisionResult
	executeErr    error
}

func (m *mockOrchestrator) Execute(ctx context.Context, opts ports.ProvisionOptions) (*ports.ProvisionResult, error) {
	m.executeCalled = true
	m.executeOpts = opts
	return m.executeResult, m.executeErr
}

func (m *mockOrchestrator) OnProgress(callback ProgressCallback) {
	// No-op for tests
}

func (m *mockOrchestrator) CurrentPhase() ProvisioningPhase {
	return PhaseRunning
}

func TestDevnetProvisioner_ProvisionWithNilOrchestrator(t *testing.T) {
	// Test that existing behavior is preserved when orchestrator is nil
	s := store.NewMemoryStore()
	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: nil, // Explicitly nil
	})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			FullNodes:  1,
			Mode:       "docker",
		},
	}

	// Provision should still work (resource-only behavior)
	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed with nil orchestrator: %v", err)
	}

	// Verify nodes were created
	nodes, _ := s.ListNodes(context.Background(), "test-devnet")
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}
}

func TestDevnetProvisioner_ProvisionWithOrchestrator(t *testing.T) {
	s := store.NewMemoryStore()
	mockOrch := &mockOrchestrator{
		executeResult: &ports.ProvisionResult{
			DevnetName:     "test-devnet",
			ChainID:        "test-chain-1",
			BinaryPath:     "/path/to/binary",
			GenesisPath:    "/tmp/devnet/test-devnet/genesis.json",
			NodeCount:      3,
			ValidatorCount: 2,
			FullNodeCount:  1,
			DataDir:        "/tmp/devnet/test-devnet",
		},
	}

	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: mockOrch,
	})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			FullNodes:  1,
			Mode:       "local",
			BinarySource: types.BinarySource{
				Type:    "local",
				Path:    "/path/to/binary",
				Version: "v1.0.0",
			},
		},
	}

	// Provision should call orchestrator
	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed with orchestrator: %v", err)
	}

	// Verify orchestrator was called
	if !mockOrch.executeCalled {
		t.Error("Expected orchestrator.Execute to be called")
	}

	// Verify nodes were created after orchestrator succeeded
	nodes, _ := s.ListNodes(context.Background(), "test-devnet")
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}
}

func TestDevnetProvisioner_ProvisionWithOrchestratorError(t *testing.T) {
	s := store.NewMemoryStore()
	mockOrch := &mockOrchestrator{
		executeErr: errors.New("orchestrator failed"),
	}

	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: mockOrch,
	})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			FullNodes:  0,
			Mode:       "local",
		},
	}

	// Provision should fail when orchestrator fails
	err := p.Provision(context.Background(), devnet)
	if err == nil {
		t.Fatal("Expected error when orchestrator fails")
	}

	if !strings.Contains(err.Error(), "orchestrator failed") {
		t.Errorf("Expected error to contain 'orchestrator failed', got: %v", err)
	}

	// Nodes should NOT be created when orchestrator fails
	nodes, _ := s.ListNodes(context.Background(), "test-devnet")
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes when orchestrator fails, got %d", len(nodes))
	}
}

func TestDevnetProvisioner_ProvisionConvertsDevnetToOptions(t *testing.T) {
	s := store.NewMemoryStore()
	mockOrch := &mockOrchestrator{
		executeResult: &ports.ProvisionResult{
			DevnetName:     "my-devnet",
			ChainID:        "my-chain",
			NodeCount:      4,
			ValidatorCount: 3,
			FullNodeCount:  1,
			DataDir:        "/tmp/devnet/my-devnet",
		},
	}

	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: mockOrch,
	})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "my-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 3,
			FullNodes:  1,
			Mode:       "local",
			BinarySource: types.BinarySource{
				Type:    "local",
				Path:    "/usr/local/bin/stabbed",
				Version: "v2.0.0",
			},
			GenesisPath: "/path/to/genesis.json",
		},
	}

	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify conversion to ProvisionOptions
	opts := mockOrch.executeOpts
	if opts.DevnetName != "my-devnet" {
		t.Errorf("Expected DevnetName 'my-devnet', got '%s'", opts.DevnetName)
	}
	if opts.NumValidators != 3 {
		t.Errorf("Expected NumValidators 3, got %d", opts.NumValidators)
	}
	if opts.NumFullNodes != 1 {
		t.Errorf("Expected NumFullNodes 1, got %d", opts.NumFullNodes)
	}
	if opts.BinaryPath != "/usr/local/bin/stabbed" {
		t.Errorf("Expected BinaryPath '/usr/local/bin/stabbed', got '%s'", opts.BinaryPath)
	}
	if opts.Network != "stable" {
		t.Errorf("Expected Network 'stable', got '%s'", opts.Network)
	}
}

func TestDevnetProvisioner_ProvisionWithBinaryVersion(t *testing.T) {
	s := store.NewMemoryStore()
	mockOrch := &mockOrchestrator{
		executeResult: &ports.ProvisionResult{
			DevnetName:     "test-devnet",
			ChainID:        "test-chain",
			NodeCount:      1,
			ValidatorCount: 1,
			DataDir:        "/tmp/devnet/test-devnet",
		},
	}

	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: mockOrch,
	})

	// Test with cache source (no path, but has version)
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 1,
			FullNodes:  0,
			Mode:       "local",
			BinarySource: types.BinarySource{
				Type:    "cache",
				Version: "v3.0.0",
			},
		},
	}

	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	opts := mockOrch.executeOpts
	if opts.BinaryVersion != "v3.0.0" {
		t.Errorf("Expected BinaryVersion 'v3.0.0', got '%s'", opts.BinaryVersion)
	}
	if opts.BinaryPath != "" {
		t.Errorf("Expected empty BinaryPath for cache source, got '%s'", opts.BinaryPath)
	}
}

func TestDevnetProvisioner_ProvisionWithGenesisPath(t *testing.T) {
	s := store.NewMemoryStore()
	mockOrch := &mockOrchestrator{
		executeResult: &ports.ProvisionResult{
			DevnetName:     "test-devnet",
			ChainID:        "test-chain",
			NodeCount:      1,
			ValidatorCount: 1,
			DataDir:        "/tmp/devnet/test-devnet",
		},
	}

	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: mockOrch,
	})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:      "stable",
			Validators:  1,
			FullNodes:   0,
			Mode:        "local",
			GenesisPath: "/custom/genesis.json",
		},
	}

	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	opts := mockOrch.executeOpts
	if opts.GenesisSource.Mode != plugintypes.GenesisModeLocal {
		t.Errorf("Expected GenesisSource.Mode 'local', got '%s'", opts.GenesisSource.Mode)
	}
	if opts.GenesisSource.LocalPath != "/custom/genesis.json" {
		t.Errorf("Expected GenesisSource.LocalPath '/custom/genesis.json', got '%s'", opts.GenesisSource.LocalPath)
	}
}

func TestDevnetProvisioner_ProvisionWithSnapshotURL(t *testing.T) {
	s := store.NewMemoryStore()
	mockOrch := &mockOrchestrator{
		executeResult: &ports.ProvisionResult{
			DevnetName:     "test-devnet",
			ChainID:        "test-chain",
			NodeCount:      1,
			ValidatorCount: 1,
			DataDir:        "/tmp/devnet/test-devnet",
		},
	}

	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: mockOrch,
	})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:      "stable",
			Validators:  1,
			FullNodes:   0,
			Mode:        "local",
			SnapshotURL: "https://snapshots.example.com/mainnet.tar.gz",
		},
	}

	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	opts := mockOrch.executeOpts
	if opts.GenesisSource.Mode != plugintypes.GenesisModeSnapshot {
		t.Errorf("Expected GenesisSource.Mode 'snapshot', got '%s'", opts.GenesisSource.Mode)
	}
	if opts.GenesisSource.SnapshotURL != "https://snapshots.example.com/mainnet.tar.gz" {
		t.Errorf("Expected GenesisSource.SnapshotURL, got '%s'", opts.GenesisSource.SnapshotURL)
	}
}

func TestDevnetProvisioner_ProvisionProgressCallback(t *testing.T) {
	s := store.NewMemoryStore()

	var progressPhase ProvisioningPhase
	var progressMessage string

	// Create a mock that triggers progress callback
	mockOrch := &mockOrchestrator{
		executeResult: &ports.ProvisionResult{
			DevnetName:     "test-devnet",
			ChainID:        "test-chain",
			NodeCount:      1,
			ValidatorCount: 1,
			DataDir:        "/tmp/devnet/test-devnet",
		},
	}

	p := NewDevnetProvisioner(s, Config{
		DataDir:      "/tmp/devnet",
		Orchestrator: mockOrch,
		OnProgress: func(phase ProvisioningPhase, message string) {
			progressPhase = phase
			progressMessage = message
		},
	})

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 1,
			FullNodes:  0,
			Mode:       "local",
		},
	}

	err := p.Provision(context.Background(), devnet)
	if err != nil {
		t.Fatalf("Provision failed: %v", err)
	}

	// Verify progress callback was set up
	// The actual callback would be triggered by the orchestrator
	_ = progressPhase
	_ = progressMessage
}

// =============================================================================
// DevnetToProvisionOptions Conversion Tests
// =============================================================================

func TestDevnetToProvisionOptions_BasicConversion(t *testing.T) {
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "basic-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			FullNodes:  2,
			Mode:       "local",
		},
	}

	opts := devnetToProvisionOptions(devnet, "/data")

	if opts.DevnetName != "basic-devnet" {
		t.Errorf("Expected DevnetName 'basic-devnet', got '%s'", opts.DevnetName)
	}
	if opts.NumValidators != 4 {
		t.Errorf("Expected NumValidators 4, got %d", opts.NumValidators)
	}
	if opts.NumFullNodes != 2 {
		t.Errorf("Expected NumFullNodes 2, got %d", opts.NumFullNodes)
	}
	if opts.Network != "stable" {
		t.Errorf("Expected Network 'stable', got '%s'", opts.Network)
	}
	if opts.DataDir != "/data/basic-devnet" {
		t.Errorf("Expected DataDir '/data/basic-devnet', got '%s'", opts.DataDir)
	}
}

func TestDevnetToProvisionOptions_LocalBinarySource(t *testing.T) {
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 1,
			Mode:       "local",
			BinarySource: types.BinarySource{
				Type:    "local",
				Path:    "/opt/bin/stabbed",
				Version: "v1.0.0",
			},
		},
	}

	opts := devnetToProvisionOptions(devnet, "/data")

	if opts.BinaryPath != "/opt/bin/stabbed" {
		t.Errorf("Expected BinaryPath '/opt/bin/stabbed', got '%s'", opts.BinaryPath)
	}
	if opts.BinaryVersion != "v1.0.0" {
		t.Errorf("Expected BinaryVersion 'v1.0.0', got '%s'", opts.BinaryVersion)
	}
}

func TestDevnetToProvisionOptions_CacheBinarySource(t *testing.T) {
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 1,
			Mode:       "local",
			BinarySource: types.BinarySource{
				Type:    "cache",
				Version: "v2.0.0",
			},
		},
	}

	opts := devnetToProvisionOptions(devnet, "/data")

	if opts.BinaryPath != "" {
		t.Errorf("Expected empty BinaryPath for cache, got '%s'", opts.BinaryPath)
	}
	if opts.BinaryVersion != "v2.0.0" {
		t.Errorf("Expected BinaryVersion 'v2.0.0', got '%s'", opts.BinaryVersion)
	}
}

func TestDevnetToProvisionOptions_LocalGenesis(t *testing.T) {
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test"},
		Spec: types.DevnetSpec{
			Plugin:      "stable",
			Validators:  1,
			Mode:        "local",
			GenesisPath: "/path/to/genesis.json",
		},
	}

	opts := devnetToProvisionOptions(devnet, "/data")

	if opts.GenesisSource.Mode != plugintypes.GenesisModeLocal {
		t.Errorf("Expected GenesisMode 'local', got '%s'", opts.GenesisSource.Mode)
	}
	if opts.GenesisSource.LocalPath != "/path/to/genesis.json" {
		t.Errorf("Expected LocalPath '/path/to/genesis.json', got '%s'", opts.GenesisSource.LocalPath)
	}
}

func TestDevnetToProvisionOptions_SnapshotGenesis(t *testing.T) {
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test"},
		Spec: types.DevnetSpec{
			Plugin:      "stable",
			Validators:  1,
			Mode:        "local",
			SnapshotURL: "https://snapshots.example.com/chain.tar.gz",
		},
	}

	opts := devnetToProvisionOptions(devnet, "/data")

	if opts.GenesisSource.Mode != plugintypes.GenesisModeSnapshot {
		t.Errorf("Expected GenesisMode 'snapshot', got '%s'", opts.GenesisSource.Mode)
	}
	if opts.GenesisSource.SnapshotURL != "https://snapshots.example.com/chain.tar.gz" {
		t.Errorf("Expected SnapshotURL, got '%s'", opts.GenesisSource.SnapshotURL)
	}
}

func TestDevnetToProvisionOptions_RPCGenesisDefault(t *testing.T) {
	// When no genesis path or snapshot URL is provided, default to RPC mode
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 1,
			Mode:       "local",
		},
	}

	opts := devnetToProvisionOptions(devnet, "/data")

	if opts.GenesisSource.Mode != plugintypes.GenesisModeRPC {
		t.Errorf("Expected default GenesisMode 'rpc', got '%s'", opts.GenesisSource.Mode)
	}
}

func TestDevnetToProvisionOptions_ChainIDGeneration(t *testing.T) {
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "my-awesome-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 1,
			Mode:       "local",
		},
	}

	opts := devnetToProvisionOptions(devnet, "/data")

	// ChainID should be generated from devnet name
	if opts.ChainID != "my-awesome-devnet-1" {
		t.Errorf("Expected ChainID 'my-awesome-devnet-1', got '%s'", opts.ChainID)
	}
}
