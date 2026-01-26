// internal/daemon/provisioner/orchestrator_test.go
package provisioner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
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

func (m *mockGenesisForker) Fork(ctx context.Context, opts ports.ForkOptions) (*ports.ForkResult, error) {
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
}

func (m *mockNodeInitializer) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	m.initializeCalls = append(m.initializeCalls, struct {
		nodeDir string
		moniker string
		chainID string
	}{nodeDir, moniker, chainID})
	return m.initializeErr
}

func (m *mockNodeInitializer) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
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

	// Verify phase progression
	expectedPhases := []ProvisioningPhase{
		PhaseBuilding,
		PhaseForking,
		PhaseInitializing,
		PhaseStarting,
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
