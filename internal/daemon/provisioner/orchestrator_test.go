// internal/daemon/provisioner/orchestrator_test.go
package provisioner

import (
	"context"
	"errors"
	"fmt"
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
