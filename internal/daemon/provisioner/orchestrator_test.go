// internal/daemon/provisioner/orchestrator_test.go
package provisioner

import (
	"context"
	"errors"
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
// Mock Implementations
// =============================================================================

// mockBinaryBuilder implements builder.BinaryBuilder for testing
type mockBinaryBuilder struct {
	buildCalled bool
	buildSpec   builder.BuildSpec
	buildResult *builder.BuildResult
	buildErr    error
}

func (m *mockBinaryBuilder) Build(ctx context.Context, spec builder.BuildSpec) (*builder.BuildResult, error) {
	m.buildCalled = true
	m.buildSpec = spec
	return m.buildResult, m.buildErr
}

func (m *mockBinaryBuilder) GetCached(ctx context.Context, spec builder.BuildSpec) (*builder.BuildResult, bool) {
	return nil, false
}

func (m *mockBinaryBuilder) Clean(ctx context.Context, maxAge time.Duration) error {
	return nil
}

// mockGenesisForker implements ports.GenesisForker for testing
type mockGenesisForker struct {
	forkCalled bool
	forkOpts   ports.ForkOptions
	forkResult *ports.ForkResult
	forkErr    error
}

func (m *mockGenesisForker) Fork(ctx context.Context, opts ports.ForkOptions, progress ports.ProgressReporter) (*ports.ForkResult, error) {
	m.forkCalled = true
	m.forkOpts = opts
	return m.forkResult, m.forkErr
}

// mockNodeInitializer implements ports.NodeInitializer for testing
type mockNodeInitializer struct {
	initializeCalls []struct {
		nodeDir string
		moniker string
		chainID string
	}
	initializeErr error
	nodeIDResult  string
	nodeIDErr     error
	nodeIDMap     map[string]string // nodeDir -> nodeID (per-node override)
}

func (m *mockNodeInitializer) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	m.initializeCalls = append(m.initializeCalls, struct {
		nodeDir string
		moniker string
		chainID string
	}{nodeDir, moniker, chainID})

	// Create config directory and stub files like the real initializer does
	if m.initializeErr == nil {
		configDir := filepath.Join(nodeDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}

		// Create a minimal priv_validator_key.json (needed by readValidatorKeys)
		hexAddr := fmt.Sprintf("%040x", len(m.initializeCalls))
		keyJSON := fmt.Sprintf(`{
  "address": "%s",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "dGVzdHB1YmtleSVk"
  }
}`, hexAddr)
		if err := os.WriteFile(filepath.Join(configDir, "priv_validator_key.json"), []byte(keyJSON), 0644); err != nil {
			return err
		}

		// Create a minimal config.toml (needed by configureNodeNetworking)
		configTOML := `proxy_app = "tcp://127.0.0.1:26658"
pprof_laddr = "localhost:6060"
persistent_peers = ""
timeout_propose = "3s"
timeout_prevote = "1s"
timeout_precommit = "1s"
timeout_commit = "5s"

[rpc]
laddr = "tcp://127.0.0.1:26657"

[p2p]
laddr = "tcp://0.0.0.0:26656"
addr_book_strict = true
allow_duplicate_ip = false
`
		if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configTOML), 0644); err != nil {
			return err
		}

		// Create a minimal app.toml (needed by SetPortsWithHost, EnableNode0Services)
		appTOML := `[api]
enable = false
address = "tcp://0.0.0.0:1317"
enabled-unsafe-cors = false

[grpc]
enable = true
address = "0.0.0.0:9090"

[json-rpc]
enable = false
address = "0.0.0.0:8545"
ws-address = "0.0.0.0:8546"
`
		if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(appTOML), 0644); err != nil {
			return err
		}
	}

	return m.initializeErr
}

func (m *mockNodeInitializer) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	if m.nodeIDMap != nil {
		if id, ok := m.nodeIDMap[nodeDir]; ok {
			return id, m.nodeIDErr
		}
	}
	return m.nodeIDResult, m.nodeIDErr
}

func (m *mockNodeInitializer) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return nil, nil
}

func (m *mockNodeInitializer) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	return nil, nil
}

func (m *mockNodeInitializer) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return nil, nil
}

func (m *mockNodeInitializer) GetTestMnemonic(validatorIndex int) string {
	return ""
}

// mockNodeRuntime implements runtime.NodeRuntime for testing
type mockNodeRuntime struct {
	startCalls []struct {
		node *types.Node
		opts runtime.StartOptions
	}
	startErr error
	stopErr  error
}

func (m *mockNodeRuntime) StartNode(ctx context.Context, node *types.Node, opts runtime.StartOptions) error {
	m.startCalls = append(m.startCalls, struct {
		node *types.Node
		opts runtime.StartOptions
	}{node, opts})
	return m.startErr
}

func (m *mockNodeRuntime) StopNode(ctx context.Context, nodeID string, graceful bool) error {
	return m.stopErr
}

func (m *mockNodeRuntime) RestartNode(ctx context.Context, nodeID string) error {
	return nil
}

func (m *mockNodeRuntime) GetNodeStatus(ctx context.Context, nodeID string) (*runtime.NodeStatus, error) {
	return nil, nil
}

func (m *mockNodeRuntime) GetLogs(ctx context.Context, nodeID string, opts runtime.LogOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockNodeRuntime) Cleanup(ctx context.Context) error {
	return nil
}

func (m *mockNodeRuntime) ExecInNode(ctx context.Context, nodeID string, command []string, timeout time.Duration) (*runtime.ExecResult, error) {
	return nil, fmt.Errorf("exec not implemented in mock")
}

// mockHealthChecker implements controller.HealthChecker for testing
type mockHealthChecker struct {
	checkHealthCalls        []*types.Node
	healthResults           map[string]*types.HealthCheckResult // keyed by node name
	checkHealthErr          error
	callCount               int
	returnHealthyAfterCalls int // return healthy after this many calls (0 = always return configured result)
}

func newMockHealthChecker() *mockHealthChecker {
	return &mockHealthChecker{
		healthResults: make(map[string]*types.HealthCheckResult),
	}
}

func (m *mockHealthChecker) setHealthyForNode(nodeName string, healthy bool, catchingUp bool) {
	m.healthResults[nodeName] = &types.HealthCheckResult{
		NodeKey:    nodeName,
		Healthy:    healthy,
		CatchingUp: catchingUp,
		CheckedAt:  time.Now(),
	}
}

func (m *mockHealthChecker) setAllHealthy() {
	for name := range m.healthResults {
		m.healthResults[name].Healthy = true
		m.healthResults[name].CatchingUp = false
	}
}

func (m *mockHealthChecker) CheckHealth(ctx context.Context, node *types.Node) (*types.HealthCheckResult, error) {
	m.checkHealthCalls = append(m.checkHealthCalls, node)
	m.callCount++

	if m.checkHealthErr != nil {
		return nil, m.checkHealthErr
	}

	// If configured to return healthy after N calls, do so
	if m.returnHealthyAfterCalls > 0 && m.callCount >= m.returnHealthyAfterCalls {
		return &types.HealthCheckResult{
			NodeKey:    node.Metadata.Name,
			Healthy:    true,
			CatchingUp: false,
			CheckedAt:  time.Now(),
		}, nil
	}

	result, ok := m.healthResults[node.Metadata.Name]
	if !ok {
		// Default to healthy if not configured
		return &types.HealthCheckResult{
			NodeKey:    node.Metadata.Name,
			Healthy:    true,
			CatchingUp: false,
			CheckedAt:  time.Now(),
		}, nil
	}
	return result, nil
}

// =============================================================================
// Phase Constants Tests
// =============================================================================

func TestProvisioningPhaseConstants(t *testing.T) {
	tests := []struct {
		name     string
		phase    ProvisioningPhase
		expected string
	}{
		{"PhasePending", PhasePending, "Pending"},
		{"PhaseBuilding", PhaseBuilding, "Building"},
		{"PhaseForking", PhaseForking, "Forking"},
		{"PhaseInitializing", PhaseInitializing, "Initializing"},
		{"PhaseStarting", PhaseStarting, "Starting"},
		{"PhaseHealthChecking", PhaseHealthChecking, "HealthChecking"},
		{"PhaseRunning", PhaseRunning, "Running"},
		{"PhaseDegraded", PhaseDegraded, "Degraded"},
		{"PhaseFailed", PhaseFailed, "Failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.phase))
		})
	}
}

// =============================================================================
// OrchestratorConfig Tests
// =============================================================================

func TestNewProvisioningOrchestrator(t *testing.T) {
	config := OrchestratorConfig{
		BinaryBuilder:   &mockBinaryBuilder{},
		GenesisForker:   &mockGenesisForker{},
		NodeInitializer: &mockNodeInitializer{},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         "/tmp/test",
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)
	require.NotNil(t, orch)
	assert.Equal(t, PhasePending, orch.CurrentPhase())
}

func TestNewProvisioningOrchestrator_DefaultLogger(t *testing.T) {
	config := OrchestratorConfig{
		BinaryBuilder:   &mockBinaryBuilder{},
		GenesisForker:   &mockGenesisForker{},
		NodeInitializer: &mockNodeInitializer{},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         "/tmp/test",
		Logger:          nil, // should use default
	}

	orch := NewProvisioningOrchestrator(config)
	require.NotNil(t, orch)
}

// =============================================================================
// Execute Tests - Phase Transitions
// =============================================================================

func TestExecute_PhaseTransitions(t *testing.T) {
	tmpDir := t.TempDir()

	mockBuilder := &mockBinaryBuilder{
		buildResult: &builder.BuildResult{
			BinaryPath: "/path/to/binary",
			GitCommit:  "abc123",
		},
	}
	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:       []byte(`{"chain_id": "test-chain"}`),
			SourceChainID: "source-chain",
			NewChainID:    "test-chain",
		},
	}
	mockInit := &mockNodeInitializer{
		nodeIDResult: "node123",
	}
	mockRuntime := &mockNodeRuntime{}

	config := OrchestratorConfig{
		BinaryBuilder:   mockBuilder,
		GenesisForker:   mockForker,
		NodeInitializer: mockInit,
		NodeRuntime:     mockRuntime,
		DataDir:         tmpDir,
		Logger:          slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}

	orch := NewProvisioningOrchestrator(config)

	// Track phase transitions
	var phases []ProvisioningPhase
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		phases = append(phases, phase)
	})

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 2,
		NumFullNodes:  0,
		DataDir:       tmpDir,
		GenesisSource: plugintypes.GenesisSource{
			Mode: plugintypes.GenesisModeRPC,
		},
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify phase progression (health checking is included but skips immediately when no checker configured)
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
}

func TestExecute_SkipBuildingWhenBinaryProvided(t *testing.T) {
	tmpDir := t.TempDir()

	mockBuilder := &mockBinaryBuilder{
		buildResult: &builder.BuildResult{
			BinaryPath: "/path/to/binary",
		},
	}
	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test-chain"}`),
			NewChainID: "test-chain",
		},
	}
	mockInit := &mockNodeInitializer{
		nodeIDResult: "node123",
	}
	mockRuntime := &mockNodeRuntime{}

	config := OrchestratorConfig{
		BinaryBuilder:   mockBuilder,
		GenesisForker:   mockForker,
		NodeInitializer: mockInit,
		NodeRuntime:     mockRuntime,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	var phases []ProvisioningPhase
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		phases = append(phases, phase)
	})

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		BinaryPath:    "/pre-built/binary", // Pre-built binary provided
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	// Building phase should NOT be called since binary is provided
	assert.False(t, mockBuilder.buildCalled, "Build should not be called when binary is provided")

	// But other phases should still occur
	assert.Contains(t, phases, PhaseForking)
	assert.Contains(t, phases, PhaseInitializing)
	assert.Contains(t, phases, PhaseStarting)
	assert.Contains(t, phases, PhaseHealthChecking)
	assert.Contains(t, phases, PhaseRunning)
}

// =============================================================================
// Execute Tests - Component Calls
// =============================================================================

func TestExecute_CallsComponentsInOrder(t *testing.T) {
	tmpDir := t.TempDir()

	var callOrder []string

	mockBuilder := &mockBinaryBuilder{
		buildResult: &builder.BuildResult{
			BinaryPath: "/path/to/binary",
		},
	}
	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test-chain"}`),
			NewChainID: "test-chain",
		},
	}
	mockInit := &mockNodeInitializer{
		nodeIDResult: "node123",
	}
	mockRuntime := &mockNodeRuntime{}

	config := OrchestratorConfig{
		BinaryBuilder:   mockBuilder,
		GenesisForker:   mockForker,
		NodeInitializer: mockInit,
		NodeRuntime:     mockRuntime,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	// Use progress callback to track order
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		callOrder = append(callOrder, string(phase))
	})

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 2,
		NumFullNodes:  1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	// Verify components were called
	assert.True(t, mockBuilder.buildCalled)
	assert.True(t, mockForker.forkCalled)
	assert.Len(t, mockInit.initializeCalls, 3) // 2 validators + 1 fullnode
	assert.Len(t, mockRuntime.startCalls, 3)   // 2 validators + 1 fullnode
}

func TestExecute_BuilderReceivesCorrectSpec(t *testing.T) {
	tmpDir := t.TempDir()

	mockBuilder := &mockBinaryBuilder{
		buildResult: &builder.BuildResult{
			BinaryPath: "/path/to/binary",
		},
	}
	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test-chain"}`),
			NewChainID: "test-chain",
		},
	}

	config := OrchestratorConfig{
		BinaryBuilder:   mockBuilder,
		GenesisForker:   mockForker,
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		BinaryVersion: "v1.0.0",
		Network:       "stable",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", mockBuilder.buildSpec.GitRef)
	assert.Equal(t, "stable", mockBuilder.buildSpec.PluginName)
}

func TestExecute_ForkerReceivesCorrectOptions(t *testing.T) {
	tmpDir := t.TempDir()

	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test-chain"}`),
			NewChainID: "test-chain",
		},
	}

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker:   mockForker,
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName: "test-devnet",
		ChainID:    "my-chain-id",
		GenesisSource: plugintypes.GenesisSource{
			Mode:        plugintypes.GenesisModeRPC,
			NetworkType: "mainnet",
		},
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	assert.True(t, mockForker.forkCalled)
	assert.Equal(t, plugintypes.GenesisModeRPC, mockForker.forkOpts.Source.Mode)
	assert.Equal(t, "mainnet", mockForker.forkOpts.Source.NetworkType)
	assert.Equal(t, "my-chain-id", mockForker.forkOpts.PatchOpts.ChainID)
}

func TestExecute_ForkerReceivesBinaryVersion(t *testing.T) {
	tmpDir := t.TempDir()

	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test-chain"}`),
			NewChainID: "test-chain",
		},
	}

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker:   mockForker,
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "my-chain-id",
		BinaryVersion: "v1.2.3",
		GenesisSource: plugintypes.GenesisSource{
			Mode: plugintypes.GenesisModeRPC,
		},
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	// Verify BinaryVersion is propagated to PatchOpts
	assert.True(t, mockForker.forkCalled)
	assert.Equal(t, "v1.2.3", mockForker.forkOpts.PatchOpts.BinaryVersion)
}

func TestExecute_ForkerReceivesBinaryVersionFromPatchOpts(t *testing.T) {
	tmpDir := t.TempDir()

	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test-chain"}`),
			NewChainID: "test-chain",
		},
	}

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker:   mockForker,
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	// BinaryVersion explicitly set in GenesisPatchOpts should take precedence
	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "my-chain-id",
		BinaryVersion: "v1.0.0", // This should be overridden
		GenesisPatchOpts: plugintypes.GenesisPatchOptions{
			BinaryVersion: "v2.0.0", // This takes precedence
		},
		GenesisSource: plugintypes.GenesisSource{
			Mode: plugintypes.GenesisModeRPC,
		},
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	// Verify BinaryVersion from GenesisPatchOpts takes precedence
	assert.True(t, mockForker.forkCalled)
	assert.Equal(t, "v2.0.0", mockForker.forkOpts.PatchOpts.BinaryVersion)
}

// =============================================================================
// Execute Tests - Error Handling
// =============================================================================

func TestExecute_BuilderError_FailsToFailed(t *testing.T) {
	tmpDir := t.TempDir()

	mockBuilder := &mockBinaryBuilder{
		buildErr: errors.New("build failed"),
	}

	config := OrchestratorConfig{
		BinaryBuilder:   mockBuilder,
		GenesisForker:   &mockGenesisForker{},
		NodeInitializer: &mockNodeInitializer{},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	result, err := orch.Execute(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "build failed")
	assert.Equal(t, PhaseFailed, orch.CurrentPhase())
}

func TestExecute_ForkerError_FailsToFailed(t *testing.T) {
	tmpDir := t.TempDir()

	mockForker := &mockGenesisForker{
		forkErr: errors.New("fork failed"),
	}

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker:   mockForker,
		NodeInitializer: &mockNodeInitializer{},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	result, err := orch.Execute(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "fork failed")
	assert.Equal(t, PhaseFailed, orch.CurrentPhase())
}

func TestExecute_InitializerError_FailsToFailed(t *testing.T) {
	tmpDir := t.TempDir()

	mockInit := &mockNodeInitializer{
		initializeErr: errors.New("init failed"),
	}

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: mockInit,
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	result, err := orch.Execute(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "init failed")
	assert.Equal(t, PhaseFailed, orch.CurrentPhase())
}

func TestExecute_RuntimeError_FailsToFailed(t *testing.T) {
	tmpDir := t.TempDir()

	mockRuntime := &mockNodeRuntime{
		startErr: errors.New("start failed"),
	}

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     mockRuntime,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	result, err := orch.Execute(context.Background(), opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "start failed")
	assert.Equal(t, PhaseFailed, orch.CurrentPhase())
}

// =============================================================================
// Execute Tests - Context Cancellation
// =============================================================================

func TestExecute_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker:   &mockGenesisForker{},
		NodeInitializer: &mockNodeInitializer{},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	result, err := orch.Execute(ctx, opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, PhaseFailed, orch.CurrentPhase())
}

// =============================================================================
// Execute Tests - Result Validation
// =============================================================================

func TestExecute_ReturnsCorrectResult(t *testing.T) {
	tmpDir := t.TempDir()

	mockBuilder := &mockBinaryBuilder{
		buildResult: &builder.BuildResult{
			BinaryPath: "/path/to/binary",
		},
	}
	mockForker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:    []byte(`{"chain_id": "test-chain"}`),
			NewChainID: "test-chain",
		},
	}

	config := OrchestratorConfig{
		BinaryBuilder:   mockBuilder,
		GenesisForker:   mockForker,
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "my-devnet",
		ChainID:       "my-chain",
		NumValidators: 3,
		NumFullNodes:  2,
		DataDir:       tmpDir,
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "my-devnet", result.DevnetName)
	assert.Equal(t, "my-chain", result.ChainID)
	assert.Equal(t, "/path/to/binary", result.BinaryPath)
	assert.Equal(t, 5, result.NodeCount) // 3 validators + 2 fullnodes
	assert.Equal(t, 3, result.ValidatorCount)
	assert.Equal(t, 2, result.FullNodeCount)
	assert.Equal(t, tmpDir, result.DataDir)
}

// =============================================================================
// Progress Callback Tests
// =============================================================================

func TestOnProgress_ReceivesAllPhaseMessages(t *testing.T) {
	tmpDir := t.TempDir()

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	var messages []string
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		messages = append(messages, message)
	})

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	// Should have at least one message per phase
	assert.GreaterOrEqual(t, len(messages), 5)
}

func TestOnProgress_NoCallback_DoesNotPanic(t *testing.T) {
	tmpDir := t.TempDir()

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)
	// Intentionally not setting OnProgress callback

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	// Should not panic
	assert.NotPanics(t, func() {
		_, _ = orch.Execute(context.Background(), opts)
	})
}

// =============================================================================
// GetError Tests
// =============================================================================

func TestGetError_ReturnsLastError(t *testing.T) {
	tmpDir := t.TempDir()

	expectedErr := errors.New("test error")
	mockBuilder := &mockBinaryBuilder{
		buildErr: expectedErr,
	}

	config := OrchestratorConfig{
		BinaryBuilder:   mockBuilder,
		GenesisForker:   &mockGenesisForker{},
		NodeInitializer: &mockNodeInitializer{},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, _ = orch.Execute(context.Background(), opts)

	assert.Error(t, orch.GetError())
	assert.Contains(t, orch.GetError().Error(), "test error")
}

func TestGetError_ReturnsNilOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:    "test-devnet",
		ChainID:       "test-chain",
		NumValidators: 1,
		DataDir:       tmpDir,
	}

	_, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)

	assert.NoError(t, orch.GetError())
}

// =============================================================================
// Health Checking Phase Tests
// =============================================================================

func TestExecute_HealthPhase_AllHealthy(t *testing.T) {
	tmpDir := t.TempDir()

	mockChecker := newMockHealthChecker()
	// Default returns healthy=true for all nodes

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		HealthChecker:   mockChecker,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	var phases []ProvisioningPhase
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		phases = append(phases, phase)
	})

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      2,
		DataDir:            tmpDir,
		HealthCheckTimeout: 10 * time.Second,
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should progress through health checking to Running
	assert.Contains(t, phases, PhaseHealthChecking)
	assert.Contains(t, phases, PhaseRunning)
	assert.Equal(t, PhaseRunning, orch.CurrentPhase())

	// Health checker should have been called
	assert.Greater(t, len(mockChecker.checkHealthCalls), 0)
}

func TestExecute_HealthPhase_Timeout_TransitionsToDegraded(t *testing.T) {
	tmpDir := t.TempDir()

	mockChecker := newMockHealthChecker()
	// Configure nodes to never become healthy
	mockChecker.setHealthyForNode("test-devnet-validator-0", false, false)

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		HealthChecker:   mockChecker,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      1,
		DataDir:            tmpDir,
		HealthCheckTimeout: 100 * time.Millisecond, // Very short timeout
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err) // Should not error, just transition to Degraded
	require.NotNil(t, result)

	// Should end in Degraded phase, not Failed
	assert.Equal(t, PhaseDegraded, orch.CurrentPhase())
}

func TestExecute_HealthPhase_CatchingUp_WaitsForSync(t *testing.T) {
	tmpDir := t.TempDir()

	mockChecker := newMockHealthChecker()
	// Node is healthy but still catching up - should not count as fully healthy
	mockChecker.setHealthyForNode("test-devnet-validator-0", true, true) // catching up
	// After a few calls, stop catching up
	mockChecker.returnHealthyAfterCalls = 3

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		HealthChecker:   mockChecker,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      1,
		DataDir:            tmpDir,
		HealthCheckTimeout: 20 * time.Second, // Allow enough time for 3+ health check iterations
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should reach Running after nodes stop catching up
	assert.Equal(t, PhaseRunning, orch.CurrentPhase())

	// Should have called health check multiple times while waiting (at least 3 per returnHealthyAfterCalls)
	assert.GreaterOrEqual(t, mockChecker.callCount, 3)
}

func TestExecute_HealthPhase_NoChecker_SkipsHealthCheck(t *testing.T) {
	tmpDir := t.TempDir()

	// No health checker configured
	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		HealthChecker:   nil, // No health checker
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	var phases []ProvisioningPhase
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		phases = append(phases, phase)
	})

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      1,
		DataDir:            tmpDir,
		HealthCheckTimeout: 10 * time.Second,
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should still go through health checking phase (but skip quickly)
	assert.Contains(t, phases, PhaseHealthChecking)
	// Should reach Running
	assert.Equal(t, PhaseRunning, orch.CurrentPhase())
}

func TestExecute_HealthPhase_NegativeTimeout_SkipsHealthCheck(t *testing.T) {
	tmpDir := t.TempDir()

	mockChecker := newMockHealthChecker()

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		HealthChecker:   mockChecker,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	var phases []ProvisioningPhase
	orch.OnProgress(func(phase ProvisioningPhase, message string) {
		phases = append(phases, phase)
	})

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      1,
		DataDir:            tmpDir,
		HealthCheckTimeout: -1, // Explicit opt-out
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should NOT include HealthChecking phase when explicitly opted out
	assert.NotContains(t, phases, PhaseHealthChecking)
	// Should go directly to Running
	assert.Equal(t, PhaseRunning, orch.CurrentPhase())
	// Health checker should NOT have been called
	assert.Equal(t, 0, len(mockChecker.checkHealthCalls))
}

func TestExecute_HealthPhase_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()

	mockChecker := newMockHealthChecker()
	// Configure to never become healthy so we stay in the health check loop
	mockChecker.setHealthyForNode("test-devnet-validator-0", false, false)

	config := OrchestratorConfig{
		BinaryBuilder: &mockBinaryBuilder{
			buildResult: &builder.BuildResult{BinaryPath: "/path/to/binary"},
		},
		GenesisForker: &mockGenesisForker{
			forkResult: &ports.ForkResult{
				Genesis:    []byte(`{"chain_id": "test-chain"}`),
				NewChainID: "test-chain",
			},
		},
		NodeInitializer: &mockNodeInitializer{nodeIDResult: "node123"},
		NodeRuntime:     &mockNodeRuntime{},
		HealthChecker:   mockChecker,
		DataDir:         tmpDir,
		Logger:          slog.Default(),
	}

	orch := NewProvisioningOrchestrator(config)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay (simulating user interrupt)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      1,
		DataDir:            tmpDir,
		HealthCheckTimeout: 10 * time.Second, // Long timeout, but context will be cancelled
	}

	result, err := orch.Execute(ctx, opts)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context cancelled")
	assert.Equal(t, PhaseFailed, orch.CurrentPhase())
}

func TestExecuteHealthPhase_DefaultTimeout(t *testing.T) {
	// Test that zero timeout uses the default
	assert.Equal(t, 2*time.Minute, DefaultHealthCheckTimeout)
	assert.Equal(t, 5*time.Second, DefaultHealthCheckInterval)
}

// =============================================================================
// Post-Init Validator Injection & Peer Configuration Tests
// =============================================================================

// mockPluginGenesisTracker implements plugintypes.PluginGenesis with call tracking.
// Named differently from mockPluginGenesis in genesis_forker_test.go to avoid redeclaration.
type mockPluginGenesisTracker struct {
	patchGenesisCalls []struct {
		opts plugintypes.GenesisPatchOptions
	}
	patchGenesisResult []byte
	patchGenesisErr    error
}

func (m *mockPluginGenesisTracker) GetRPCEndpoint(networkType string) string  { return "" }
func (m *mockPluginGenesisTracker) GetSnapshotURL(networkType string) string  { return "" }
func (m *mockPluginGenesisTracker) ValidateGenesis(genesis []byte) error      { return nil }
func (m *mockPluginGenesisTracker) ExportCommandArgs(homeDir string) []string { return nil }
func (m *mockPluginGenesisTracker) BinaryName() string                        { return "testd" }

func (m *mockPluginGenesisTracker) PatchGenesis(genesis []byte, opts plugintypes.GenesisPatchOptions) ([]byte, error) {
	m.patchGenesisCalls = append(m.patchGenesisCalls, struct {
		opts plugintypes.GenesisPatchOptions
	}{opts})
	if m.patchGenesisResult != nil {
		return m.patchGenesisResult, m.patchGenesisErr
	}
	return genesis, m.patchGenesisErr
}

func TestReadValidatorKeys(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two validator node directories with mock priv_validator_key.json
	nodes := []*types.Node{
		{
			Metadata: types.ResourceMeta{Name: "test-validator-0"},
			Spec: types.NodeSpec{
				HomeDir: filepath.Join(tmpDir, "node0"),
				Role:    "validator",
			},
		},
		{
			Metadata: types.ResourceMeta{Name: "test-validator-1"},
			Spec: types.NodeSpec{
				HomeDir: filepath.Join(tmpDir, "node1"),
				Role:    "validator",
			},
		},
		{
			Metadata: types.ResourceMeta{Name: "test-fullnode-2"},
			Spec: types.NodeSpec{
				HomeDir: filepath.Join(tmpDir, "node2"),
				Role:    "fullnode",
			},
		},
	}

	// Write mock priv_validator_key.json files for validators
	for i, node := range nodes[:2] {
		configDir := filepath.Join(node.Spec.HomeDir, "config")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		// Use deterministic hex addresses (20 bytes = 40 hex chars)
		hexAddr := fmt.Sprintf("%040x", i+1)
		keyJSON := fmt.Sprintf(`{
			"address": "%s",
			"pub_key": {
				"type": "tendermint/PubKeyEd25519",
				"value": "dGVzdHB1YmtleSVk"
			}
		}`, hexAddr)
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "priv_validator_key.json"), []byte(keyJSON), 0644))
	}

	orch := NewProvisioningOrchestrator(OrchestratorConfig{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Bech32Prefix: "cosmos",
	})

	validators, err := orch.readValidatorKeys(nodes)
	require.NoError(t, err)

	// Should only read validators, not fullnodes
	require.Len(t, validators, 2)

	// Check first validator
	assert.Equal(t, "test-validator-0", validators[0].Moniker)
	assert.Equal(t, "dGVzdHB1YmtleSVk", validators[0].ConsPubKey)
	assert.Contains(t, validators[0].OperatorAddress, "cosmosvaloper")
	assert.Equal(t, "1000000000000000000000", validators[0].SelfDelegation)

	// Check second validator
	assert.Equal(t, "test-validator-1", validators[1].Moniker)
	assert.Contains(t, validators[1].OperatorAddress, "cosmosvaloper")
}

func TestReadValidatorKeys_SkipsFullnodes(t *testing.T) {
	tmpDir := t.TempDir()

	nodes := []*types.Node{
		{
			Metadata: types.ResourceMeta{Name: "test-fullnode-0"},
			Spec: types.NodeSpec{
				HomeDir: filepath.Join(tmpDir, "node0"),
				Role:    "fullnode",
			},
		},
	}

	orch := NewProvisioningOrchestrator(OrchestratorConfig{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Bech32Prefix: "cosmos",
	})

	validators, err := orch.readValidatorKeys(nodes)
	require.NoError(t, err)
	assert.Empty(t, validators)
}

func TestBuildPeersExcludingSelf(t *testing.T) {
	nodeIDs := []string{"aaa111", "bbb222", "ccc333"}

	t.Run("port-offset mode (no address)", func(t *testing.T) {
		nodes := []*types.Node{
			{Spec: types.NodeSpec{Index: 0}},
			{Spec: types.NodeSpec{Index: 1}},
			{Spec: types.NodeSpec{Index: 2}},
		}

		// Exclude node 0
		peers := buildPeersExcludingSelf(nodeIDs, nodes, 0)
		assert.Contains(t, peers, "bbb222@127.0.0.1:36656")
		assert.Contains(t, peers, "ccc333@127.0.0.1:46656")
		assert.NotContains(t, peers, "aaa111")

		// Exclude node 1
		peers = buildPeersExcludingSelf(nodeIDs, nodes, 1)
		assert.Contains(t, peers, "aaa111@127.0.0.1:26656")
		assert.Contains(t, peers, "ccc333@127.0.0.1:46656")
		assert.NotContains(t, peers, "bbb222")
	})

	t.Run("loopback subnet mode (with address)", func(t *testing.T) {
		nodes := []*types.Node{
			{Spec: types.NodeSpec{Index: 0, Address: "127.0.42.1"}},
			{Spec: types.NodeSpec{Index: 1, Address: "127.0.42.2"}},
			{Spec: types.NodeSpec{Index: 2, Address: "127.0.42.3"}},
		}

		peers := buildPeersExcludingSelf(nodeIDs, nodes, 0)
		assert.Contains(t, peers, "bbb222@127.0.42.2:26656")
		assert.Contains(t, peers, "ccc333@127.0.42.3:26656")
		assert.NotContains(t, peers, "aaa111")
	})

	t.Run("single node returns empty", func(t *testing.T) {
		nodes := []*types.Node{
			{Spec: types.NodeSpec{Index: 0}},
		}
		peers := buildPeersExcludingSelf([]string{"aaa111"}, nodes, 0)
		assert.Empty(t, peers)
	})
}

func TestPostInitValidatorInjection(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up mock genesis
	mockGenesis := []byte(`{"chain_id":"test-chain","app_state":{}}`)
	patchedGenesis := []byte(`{"chain_id":"test-chain","app_state":{"staking":{"validators":[]}}}`)

	pluginGenesis := &mockPluginGenesisTracker{
		patchGenesisResult: patchedGenesis,
	}

	nodeInit := &mockNodeInitializer{
		nodeIDMap: map[string]string{},
	}

	forker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:       mockGenesis,
			SourceChainID: "source-chain",
			NewChainID:    "test-chain",
		},
	}

	orch := NewProvisioningOrchestrator(OrchestratorConfig{
		BinaryBuilder:   &mockBinaryBuilder{},
		GenesisForker:   forker,
		NodeInitializer: nodeInit,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		PluginGenesis:   pluginGenesis,
		Bech32Prefix:    "cosmos",
	})

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      1,
		BinaryPath:         "/tmp/testd",
		DataDir:            tmpDir,
		HealthCheckTimeout: -1, // Skip health checks
		SkipStart:          true,
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify PatchGenesis was called by the post-init step
	require.Len(t, pluginGenesis.patchGenesisCalls, 1)
	call := pluginGenesis.patchGenesisCalls[0]
	assert.Equal(t, "test-chain", call.opts.ChainID)
	assert.Len(t, call.opts.Validators, 1)

	// Verify the patched genesis was written to the node's config dir
	nodeDir := filepath.Join(tmpDir, "nodes", "test-devnet-validator-0", "config", "genesis.json")
	writtenGenesis, err := os.ReadFile(nodeDir)
	require.NoError(t, err)
	assert.Equal(t, patchedGenesis, writtenGenesis)

	// Verify master genesis was updated
	masterGenesis, err := os.ReadFile(filepath.Join(tmpDir, "genesis.json"))
	require.NoError(t, err)
	assert.Equal(t, patchedGenesis, masterGenesis)
}

func TestPostInitSkippedWhenNoPluginGenesis(t *testing.T) {
	tmpDir := t.TempDir()

	mockGenesis := []byte(`{"chain_id":"test-chain","app_state":{}}`)

	nodeInit := &mockNodeInitializer{
		nodeIDResult: "abc123",
	}

	forker := &mockGenesisForker{
		forkResult: &ports.ForkResult{
			Genesis:       mockGenesis,
			SourceChainID: "source-chain",
			NewChainID:    "test-chain",
		},
	}

	// No PluginGenesis set — should skip validator injection
	orch := NewProvisioningOrchestrator(OrchestratorConfig{
		BinaryBuilder:   &mockBinaryBuilder{},
		GenesisForker:   forker,
		NodeInitializer: nodeInit,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	opts := ports.ProvisionOptions{
		DevnetName:         "test-devnet",
		ChainID:            "test-chain",
		NumValidators:      1,
		BinaryPath:         "/tmp/testd",
		DataDir:            tmpDir,
		HealthCheckTimeout: -1,
		SkipStart:          true,
	}

	result, err := orch.Execute(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify the original genesis (not re-patched) was written
	nodeDir := filepath.Join(tmpDir, "nodes", "test-devnet-validator-0", "config", "genesis.json")
	writtenGenesis, err := os.ReadFile(nodeDir)
	require.NoError(t, err)
	assert.Equal(t, mockGenesis, writtenGenesis)
}
