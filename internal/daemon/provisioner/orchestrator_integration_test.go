//go:build integration

// internal/daemon/provisioner/orchestrator_integration_test.go
package provisioner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/builder"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Integration Test Mocks (with real file system operations)
// =============================================================================

// integrationBinaryBuilder simulates binary building with real file operations
type integrationBinaryBuilder struct {
	binaryPath string
}

func newIntegrationBinaryBuilder(tmpDir string) *integrationBinaryBuilder {
	binaryPath := filepath.Join(tmpDir, "bin", "stabled")
	return &integrationBinaryBuilder{binaryPath: binaryPath}
}

func (b *integrationBinaryBuilder) Build(ctx context.Context, spec builder.BuildSpec) (*builder.BuildResult, error) {
	// Create bin directory and placeholder binary
	binDir := filepath.Dir(b.binaryPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Create a placeholder binary file
	if err := os.WriteFile(b.binaryPath, []byte("#!/bin/bash\necho 'mock binary'\n"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create mock binary: %w", err)
	}

	return &builder.BuildResult{
		BinaryPath: b.binaryPath,
		GitCommit:  "abc123def456",
		GitRef:     spec.GitRef,
		BuiltAt:    time.Now(),
	}, nil
}

func (b *integrationBinaryBuilder) GetCached(ctx context.Context, spec builder.BuildSpec) (*builder.BuildResult, bool) {
	return nil, false
}

func (b *integrationBinaryBuilder) Clean(ctx context.Context, maxAge time.Duration) error {
	return nil
}

// integrationGenesisForker simulates genesis forking with real file operations
type integrationGenesisForker struct {
	dataDir string
}

func newIntegrationGenesisForker(dataDir string) *integrationGenesisForker {
	return &integrationGenesisForker{dataDir: dataDir}
}

func (f *integrationGenesisForker) Fork(ctx context.Context, opts ports.ForkOptions) (*ports.ForkResult, error) {
	chainID := opts.PatchOpts.ChainID
	if chainID == "" {
		chainID = "test-chain"
	}

	// Create a realistic genesis JSON
	genesis := fmt.Sprintf(`{
  "chain_id": %q,
  "genesis_time": %q,
  "initial_height": "1",
  "app_state": {
    "bank": {"balances": []},
    "staking": {"params": {"unbonding_time": "60s"}},
    "gov": {"voting_params": {"voting_period": "30s"}}
  }
}`, chainID, time.Now().UTC().Format(time.RFC3339))

	return &ports.ForkResult{
		Genesis:       []byte(genesis),
		SourceChainID: "mainnet-1",
		NewChainID:    chainID,
		SourceMode:    opts.Source.Mode,
		FetchedAt:     time.Now(),
	}, nil
}

// integrationNodeInitializer simulates node initialization with real file operations
type integrationNodeInitializer struct {
	dataDir  string
	nodeIDs  map[string]string
	initLogs []string
}

func newIntegrationNodeInitializer(dataDir string) *integrationNodeInitializer {
	return &integrationNodeInitializer{
		dataDir: dataDir,
		nodeIDs: make(map[string]string),
	}
}

func (i *integrationNodeInitializer) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	i.initLogs = append(i.initLogs, fmt.Sprintf("init: %s (%s)", moniker, nodeDir))

	// Create realistic node directory structure
	dirs := []string{
		filepath.Join(nodeDir, "config"),
		filepath.Join(nodeDir, "data"),
		filepath.Join(nodeDir, "keyring-test"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write config.toml
	configContent := fmt.Sprintf(`# Config for %s
moniker = %q
[p2p]
persistent_peers = ""
[consensus]
timeout_commit = "1s"
`, moniker, moniker)
	if err := os.WriteFile(filepath.Join(nodeDir, "config", "config.toml"), []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config.toml: %w", err)
	}

	// Write app.toml
	appContent := `# App config
minimum-gas-prices = "0stake"
`
	if err := os.WriteFile(filepath.Join(nodeDir, "config", "app.toml"), []byte(appContent), 0644); err != nil {
		return fmt.Errorf("failed to write app.toml: %w", err)
	}

	// Write genesis.json (placeholder - orchestrator will copy the real one)
	genesisContent := fmt.Sprintf(`{"chain_id": %q}`, chainID)
	if err := os.WriteFile(filepath.Join(nodeDir, "config", "genesis.json"), []byte(genesisContent), 0644); err != nil {
		return fmt.Errorf("failed to write genesis.json: %w", err)
	}

	// Write node_key.json
	nodeKeyContent := fmt.Sprintf(`{"priv_key": {"type": "tendermint/PrivKeyEd25519", "value": "mock_%s"}}`, moniker)
	if err := os.WriteFile(filepath.Join(nodeDir, "config", "node_key.json"), []byte(nodeKeyContent), 0644); err != nil {
		return fmt.Errorf("failed to write node_key.json: %w", err)
	}

	// Store node ID for later retrieval
	nodeID := fmt.Sprintf("node_%s_id_%d", moniker, time.Now().UnixNano()%10000)
	i.nodeIDs[nodeDir] = nodeID

	return nil
}

func (i *integrationNodeInitializer) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	if nodeID, ok := i.nodeIDs[nodeDir]; ok {
		return nodeID, nil
	}
	return "", fmt.Errorf("node ID not found for %s", nodeDir)
}

func (i *integrationNodeInitializer) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return &ports.AccountKeyInfo{
		Name:    keyName,
		Address: fmt.Sprintf("cosmos1%s", keyName[:8]),
	}, nil
}

func (i *integrationNodeInitializer) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	return &ports.AccountKeyInfo{
		Name:    keyName,
		Address: fmt.Sprintf("cosmos1%s", keyName[:8]),
	}, nil
}

func (i *integrationNodeInitializer) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return nil, nil
}

func (i *integrationNodeInitializer) GetTestMnemonic(validatorIndex int) string {
	return fmt.Sprintf("test mnemonic for validator %d", validatorIndex)
}

// integrationNodeRuntime simulates node runtime operations
type integrationNodeRuntime struct {
	startedNodes map[string]*nodeState
	startLogs    []string
}

type nodeState struct {
	node      *types.Node
	startedAt time.Time
	running   bool
}

func newIntegrationNodeRuntime() *integrationNodeRuntime {
	return &integrationNodeRuntime{
		startedNodes: make(map[string]*nodeState),
	}
}

func (r *integrationNodeRuntime) StartNode(ctx context.Context, node *types.Node, opts runtime.StartOptions) error {
	r.startLogs = append(r.startLogs, fmt.Sprintf("start: %s (%s)", node.Metadata.Name, node.Spec.Role))
	r.startedNodes[node.Metadata.Name] = &nodeState{
		node:      node,
		startedAt: time.Now(),
		running:   true,
	}
	return nil
}

func (r *integrationNodeRuntime) StopNode(ctx context.Context, nodeID string, graceful bool) error {
	if state, ok := r.startedNodes[nodeID]; ok {
		state.running = false
	}
	return nil
}

func (r *integrationNodeRuntime) RestartNode(ctx context.Context, nodeID string) error {
	if state, ok := r.startedNodes[nodeID]; ok {
		state.running = true
		state.startedAt = time.Now()
	}
	return nil
}

func (r *integrationNodeRuntime) GetNodeStatus(ctx context.Context, nodeID string) (*runtime.NodeStatus, error) {
	if state, ok := r.startedNodes[nodeID]; ok {
		return &runtime.NodeStatus{
			Running:   state.running,
			StartedAt: state.startedAt,
		}, nil
	}
	return nil, fmt.Errorf("node not found: %s", nodeID)
}

func (r *integrationNodeRuntime) GetLogs(ctx context.Context, nodeID string, opts runtime.LogOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (r *integrationNodeRuntime) Cleanup(ctx context.Context) error {
	r.startedNodes = make(map[string]*nodeState)
	return nil
}

func (r *integrationNodeRuntime) ExecInNode(ctx context.Context, nodeID string, command []string, timeout time.Duration) (*runtime.ExecResult, error) {
	return &runtime.ExecResult{
		ExitCode: 0,
		Stdout:   "mock exec output",
		Stderr:   "",
	}, nil
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestProvisioningOrchestrator_FullIntegration(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "provisioner-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create integration test components
	binaryBuilder := newIntegrationBinaryBuilder(tmpDir)
	genesisForker := newIntegrationGenesisForker(tmpDir)
	nodeInitializer := newIntegrationNodeInitializer(tmpDir)
	nodeRuntime := newIntegrationNodeRuntime()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := OrchestratorConfig{
		BinaryBuilder:   binaryBuilder,
		GenesisForker:   genesisForker,
		NodeInitializer: nodeInitializer,
		NodeRuntime:     nodeRuntime,
		DataDir:         tmpDir,
		Logger:          logger,
	}

	orch := NewProvisioningOrchestrator(config)

	// Track progress
	var phases []ProvisioningPhase
	var messages []string
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		phases = append(phases, phase)
		messages = append(messages, message)
		t.Logf("Phase: %s - %s", phase, message)
	})

	// Execute provisioning
	dataDir := filepath.Join(tmpDir, "devnet-data")
	opts := ports.ProvisionOptions{
		DevnetName:    "integration-test-devnet",
		ChainID:       "integration-chain-1",
		Network:       "stable",
		BinaryVersion: "v1.0.0",
		NumValidators: 3,
		NumFullNodes:  1,
		DataDir:       dataDir,
		GenesisSource: plugintypes.GenesisSource{
			Mode:        plugintypes.GenesisModeRPC,
			NetworkType: "mainnet",
		},
		GenesisPatchOpts: plugintypes.GenesisPatchOptions{
			ChainID: "integration-chain-1",
		},
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result
	t.Run("ResultValidation", func(t *testing.T) {
		assert.Equal(t, "integration-test-devnet", result.DevnetName)
		assert.Equal(t, "integration-chain-1", result.ChainID)
		assert.Equal(t, 4, result.NodeCount) // 3 validators + 1 fullnode
		assert.Equal(t, 3, result.ValidatorCount)
		assert.Equal(t, 1, result.FullNodeCount)
		assert.Equal(t, dataDir, result.DataDir)
		assert.NotEmpty(t, result.BinaryPath)
	})

	// Verify phase transitions
	t.Run("PhaseTransitions", func(t *testing.T) {
		expectedPhases := []ProvisioningPhase{
			PhaseBuilding,
			PhaseForking,
			PhaseInitializing,
			PhaseStarting,
			PhaseHealthChecking,
			PhaseRunning,
		}
		assert.Equal(t, expectedPhases, phases)
		assert.Equal(t, PhaseRunning, orch.CurrentPhase())
	})

	// Verify binary was created
	t.Run("BinaryCreated", func(t *testing.T) {
		assert.FileExists(t, result.BinaryPath)
	})

	// Verify genesis was written
	t.Run("GenesisCreated", func(t *testing.T) {
		genesisPath := filepath.Join(dataDir, "genesis.json")
		assert.FileExists(t, genesisPath)

		genesisData, err := os.ReadFile(genesisPath)
		require.NoError(t, err)
		assert.Contains(t, string(genesisData), `"chain_id": "integration-chain-1"`)
	})

	// Verify node directories were created
	t.Run("NodeDirectoriesCreated", func(t *testing.T) {
		nodesDir := filepath.Join(dataDir, "nodes")

		// Check validator directories
		for i := 0; i < 3; i++ {
			nodeDir := filepath.Join(nodesDir, fmt.Sprintf("integration-test-devnet-validator-%d", i))
			assert.DirExists(t, nodeDir)
			assert.DirExists(t, filepath.Join(nodeDir, "config"))
			assert.DirExists(t, filepath.Join(nodeDir, "data"))
			assert.FileExists(t, filepath.Join(nodeDir, "config", "config.toml"))
		}

		// Check fullnode directory
		fullnodeDir := filepath.Join(nodesDir, "integration-test-devnet-fullnode-3")
		assert.DirExists(t, fullnodeDir)
	})

	// Verify forked genesis was distributed to all nodes (critical fix validation)
	t.Run("ForkedGenesisDistributedToNodes", func(t *testing.T) {
		nodesDir := filepath.Join(dataDir, "nodes")

		// Read the main genesis to compare
		mainGenesis, err := os.ReadFile(filepath.Join(dataDir, "genesis.json"))
		require.NoError(t, err)
		require.NotEmpty(t, mainGenesis)

		// Check all validator nodes have the full forked genesis (not placeholder)
		for i := 0; i < 3; i++ {
			nodeDir := filepath.Join(nodesDir, fmt.Sprintf("integration-test-devnet-validator-%d", i))
			nodeGenesis, err := os.ReadFile(filepath.Join(nodeDir, "config", "genesis.json"))
			require.NoError(t, err, "Node %d genesis should be readable", i)

			// The node genesis should match the main forked genesis exactly
			assert.Equal(t, mainGenesis, nodeGenesis,
				"Node %d genesis should match the forked genesis, not be a placeholder", i)

			// Should contain the full chain_id and app_state (not just placeholder)
			assert.Contains(t, string(nodeGenesis), `"chain_id": "integration-chain-1"`)
			assert.Contains(t, string(nodeGenesis), `"app_state"`,
				"Node %d genesis should have full app_state, not placeholder", i)
		}

		// Check fullnode has the full forked genesis too
		fullnodeDir := filepath.Join(nodesDir, "integration-test-devnet-fullnode-3")
		fullnodeGenesis, err := os.ReadFile(filepath.Join(fullnodeDir, "config", "genesis.json"))
		require.NoError(t, err)
		assert.Equal(t, mainGenesis, fullnodeGenesis,
			"Fullnode genesis should match the forked genesis")
	})

	// Verify nodes were started
	t.Run("NodesStarted", func(t *testing.T) {
		assert.Len(t, nodeRuntime.startedNodes, 4)
		assert.Len(t, nodeRuntime.startLogs, 4)

		// Verify all nodes are running
		for name, state := range nodeRuntime.startedNodes {
			assert.True(t, state.running, "Node %s should be running", name)
		}
	})

	// Verify initialization logs
	t.Run("InitializationLogs", func(t *testing.T) {
		assert.Len(t, nodeInitializer.initLogs, 4)
	})
}

func TestProvisioningOrchestrator_WithPrebuiltBinary(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "provisioner-prebuilt-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a pre-built binary
	prebuiltBinaryPath := filepath.Join(tmpDir, "prebuilt", "stabled")
	require.NoError(t, os.MkdirAll(filepath.Dir(prebuiltBinaryPath), 0755))
	require.NoError(t, os.WriteFile(prebuiltBinaryPath, []byte("#!/bin/bash\necho 'prebuilt'\n"), 0755))

	// Create integration components
	binaryBuilder := newIntegrationBinaryBuilder(tmpDir) // Won't be called
	genesisForker := newIntegrationGenesisForker(tmpDir)
	nodeInitializer := newIntegrationNodeInitializer(tmpDir)
	nodeRuntime := newIntegrationNodeRuntime()

	config := OrchestratorConfig{
		BinaryBuilder:   binaryBuilder,
		GenesisForker:   genesisForker,
		NodeInitializer: nodeInitializer,
		NodeRuntime:     nodeRuntime,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	var phases []ProvisioningPhase
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		phases = append(phases, phase)
	})

	dataDir := filepath.Join(tmpDir, "devnet-data")
	opts := ports.ProvisionOptions{
		DevnetName:    "prebuilt-test",
		ChainID:       "prebuilt-chain-1",
		Network:       "stable",
		BinaryPath:    prebuiltBinaryPath, // Pre-built binary provided
		NumValidators: 2,
		NumFullNodes:  0,
		DataDir:       dataDir,
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify building phase was skipped
	assert.NotContains(t, phases, PhaseBuilding)
	assert.Contains(t, phases, PhaseForking)
	assert.Contains(t, phases, PhaseInitializing)
	assert.Contains(t, phases, PhaseStarting)
	assert.Contains(t, phases, PhaseRunning)

	// Verify pre-built binary was used
	assert.Equal(t, prebuiltBinaryPath, result.BinaryPath)
}

func TestProvisioningOrchestrator_ValidatorsAndFullNodes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "provisioner-nodes-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	binaryBuilder := newIntegrationBinaryBuilder(tmpDir)
	genesisForker := newIntegrationGenesisForker(tmpDir)
	nodeInitializer := newIntegrationNodeInitializer(tmpDir)
	nodeRuntime := newIntegrationNodeRuntime()

	config := OrchestratorConfig{
		BinaryBuilder:   binaryBuilder,
		GenesisForker:   genesisForker,
		NodeInitializer: nodeInitializer,
		NodeRuntime:     nodeRuntime,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	dataDir := filepath.Join(tmpDir, "devnet-data")
	opts := ports.ProvisionOptions{
		DevnetName:    "node-test",
		ChainID:       "node-chain-1",
		Network:       "stable",
		NumValidators: 5,
		NumFullNodes:  3,
		DataDir:       dataDir,
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	// Verify counts
	assert.Equal(t, 8, result.NodeCount)
	assert.Equal(t, 5, result.ValidatorCount)
	assert.Equal(t, 3, result.FullNodeCount)

	// Verify all nodes were started
	assert.Len(t, nodeRuntime.startedNodes, 8)

	// Verify node roles
	validatorCount := 0
	fullnodeCount := 0
	for _, state := range nodeRuntime.startedNodes {
		switch state.node.Spec.Role {
		case "validator":
			validatorCount++
		case "fullnode":
			fullnodeCount++
		}
	}
	assert.Equal(t, 5, validatorCount)
	assert.Equal(t, 3, fullnodeCount)
}

func TestProvisioningOrchestrator_ConcurrentExecutions(t *testing.T) {
	// Test that multiple orchestrators can run concurrently without interference
	const numOrchestrators = 3

	results := make(chan *ports.ProvisionResult, numOrchestrators)
	errors := make(chan error, numOrchestrators)

	for i := 0; i < numOrchestrators; i++ {
		go func(idx int) {
			tmpDir, err := os.MkdirTemp("", fmt.Sprintf("provisioner-concurrent-%d-*", idx))
			if err != nil {
				errors <- err
				return
			}
			defer os.RemoveAll(tmpDir)

			config := OrchestratorConfig{
				BinaryBuilder:   newIntegrationBinaryBuilder(tmpDir),
				GenesisForker:   newIntegrationGenesisForker(tmpDir),
				NodeInitializer: newIntegrationNodeInitializer(tmpDir),
				NodeRuntime:     newIntegrationNodeRuntime(),
				DataDir:         tmpDir,
				Logger:          slog.Default(),
			}

			orch := NewProvisioningOrchestrator(config)

			dataDir := filepath.Join(tmpDir, "devnet-data")
			opts := ports.ProvisionOptions{
				DevnetName:    fmt.Sprintf("concurrent-devnet-%d", idx),
				ChainID:       fmt.Sprintf("concurrent-chain-%d", idx),
				Network:       "stable",
				NumValidators: 2,
				NumFullNodes:  1,
				DataDir:       dataDir,
			}

			result, err := orch.Execute(context.Background(), opts)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	// Collect results
	var successCount int
	var errCount int
	for i := 0; i < numOrchestrators; i++ {
		select {
		case result := <-results:
			assert.NotNil(t, result)
			successCount++
		case err := <-errors:
			t.Errorf("Concurrent execution failed: %v", err)
			errCount++
		case <-time.After(30 * time.Second):
			t.Fatal("Concurrent executions timed out")
		}
	}

	assert.Equal(t, numOrchestrators, successCount)
	assert.Equal(t, 0, errCount)
}

func TestProvisioningOrchestrator_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "provisioner-cancel-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a slow forker that respects context cancellation
	slowForker := &slowGenesisForker{delay: 5 * time.Second}

	config := OrchestratorConfig{
		BinaryBuilder:   newIntegrationBinaryBuilder(tmpDir),
		GenesisForker:   slowForker,
		NodeInitializer: newIntegrationNodeInitializer(tmpDir),
		NodeRuntime:     newIntegrationNodeRuntime(),
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	dataDir := filepath.Join(tmpDir, "devnet-data")
	opts := ports.ProvisionOptions{
		DevnetName:    "cancel-test",
		ChainID:       "cancel-chain-1",
		Network:       "stable",
		BinaryPath:    "/path/to/binary", // Skip build phase
		NumValidators: 1,
		DataDir:       dataDir,
	}

	result, err := orch.Execute(ctx, opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, PhaseFailed, orch.CurrentPhase())
}

// slowGenesisForker is a forker that takes time and respects context
type slowGenesisForker struct {
	delay time.Duration
}

func (f *slowGenesisForker) Fork(ctx context.Context, opts ports.ForkOptions) (*ports.ForkResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(f.delay):
		return &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test"}`),
			NewChainID: "test",
		}, nil
	}
}
