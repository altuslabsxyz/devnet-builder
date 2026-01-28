// internal/daemon/server/wiring_test.go
package server

import (
	"context"
	"log/slog"
	"os"
	"testing"

	cosmoslog "cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
	pkgNetwork "github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mock NetworkModule for Testing
// =============================================================================

// mockNetworkModule implements network.NetworkModule for testing.
type mockNetworkModule struct {
	name             string
	binaryName       string
	defaultChainID   string
	defaultNodeHome  string
	dockerHomeDir    string
	initCommand      []string
	startCommand     []string
	exportCommand    []string
	rpcEndpoint      string
	snapshotURL      string
}

func newMockModule(name, binaryName string) *mockNetworkModule {
	return &mockNetworkModule{
		name:            name,
		binaryName:      binaryName,
		defaultChainID:  name + "-devnet-1",
		defaultNodeHome: "." + binaryName,
		dockerHomeDir:   "/home/" + binaryName,
		initCommand:     []string{"init", "--chain-id", name + "-1"},
		startCommand:    []string{"start"},
		exportCommand:   []string{"export"},
		rpcEndpoint:     "https://rpc." + name + ".network",
		snapshotURL:     "https://snapshots." + name + ".network",
	}
}

// NetworkIdentity
func (m *mockNetworkModule) Name() string        { return m.name }
func (m *mockNetworkModule) DisplayName() string { return m.name }
func (m *mockNetworkModule) Version() string     { return "1.0.0" }

// BinaryProvider
func (m *mockNetworkModule) BinaryName() string { return m.binaryName }
func (m *mockNetworkModule) BinarySource() network.BinarySource {
	return network.BinarySource{Type: network.BinarySourceGitHub, Owner: "test", Repo: m.name}
}
func (m *mockNetworkModule) DefaultBinaryVersion() string { return "v1.0.0" }
func (m *mockNetworkModule) GetBuildConfig(networkType string) (*pkgNetwork.BuildConfig, error) {
	return &pkgNetwork.BuildConfig{Tags: []string{"netgo"}}, nil
}

// ChainConfig
func (m *mockNetworkModule) Bech32Prefix() string             { return m.name[:3] }
func (m *mockNetworkModule) BaseDenom() string                { return "u" + m.name }
func (m *mockNetworkModule) GenesisConfig() network.GenesisConfig { return network.GenesisConfig{} }
func (m *mockNetworkModule) DefaultChainID() string           { return m.defaultChainID }

// DockerConfig
func (m *mockNetworkModule) DockerImage() string                  { return m.name + "/node" }
func (m *mockNetworkModule) DockerImageTag(version string) string { return version }
func (m *mockNetworkModule) DockerHomeDir() string                { return m.dockerHomeDir }

// CommandBuilder
func (m *mockNetworkModule) InitCommand(homeDir, chainID, moniker string) []string {
	return m.initCommand
}
func (m *mockNetworkModule) StartCommand(homeDir string) []string  { return m.startCommand }
func (m *mockNetworkModule) ExportCommand(homeDir string) []string { return m.exportCommand }
func (m *mockNetworkModule) DefaultMoniker(index int) string       { return "node" }

// ProcessConfig
func (m *mockNetworkModule) DefaultNodeHome() string       { return m.defaultNodeHome }
func (m *mockNetworkModule) PIDFileName() string           { return m.binaryName + ".pid" }
func (m *mockNetworkModule) LogFileName() string           { return m.binaryName + ".log" }
func (m *mockNetworkModule) ProcessPattern() string        { return m.binaryName }
func (m *mockNetworkModule) DefaultPorts() network.PortConfig { return network.DefaultPortConfig() }
func (m *mockNetworkModule) ConfigDir(homeDir string) string   { return homeDir + "/config" }
func (m *mockNetworkModule) DataDir(homeDir string) string     { return homeDir + "/data" }
func (m *mockNetworkModule) KeyringDir(homeDir string, backend string) string {
	return homeDir + "/keyring-" + backend
}

// GenesisModifier
func (m *mockNetworkModule) ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
	return genesis, nil
}

// SnapshotProvider
func (m *mockNetworkModule) SnapshotURL(networkType string) string { return m.snapshotURL }
func (m *mockNetworkModule) RPCEndpoint(networkType string) string { return m.rpcEndpoint }
func (m *mockNetworkModule) AvailableNetworks() []string           { return []string{"mainnet", "testnet"} }

// DevnetGenerator
func (m *mockNetworkModule) NewGenerator(config *network.GeneratorConfig, logger cosmoslog.Logger) (network.Generator, error) {
	return nil, nil
}
func (m *mockNetworkModule) DefaultGeneratorConfig() *network.GeneratorConfig {
	return &network.GeneratorConfig{
		NumValidators:    1,
		NumAccounts:      0,
		AccountBalance:   sdk.NewCoins(),
		ValidatorBalance: sdk.NewCoins(),
		ValidatorStake:   math.NewInt(1000000),
		ChainID:          m.defaultChainID,
	}
}

// NodeConfigurator
func (m *mockNetworkModule) GetConfigOverrides(nodeIndex int, opts network.NodeConfigOptions) ([]byte, []byte, error) {
	return nil, nil, nil
}

// Validator
func (m *mockNetworkModule) Validate() error { return nil }

// Ensure mockNetworkModule implements network.NetworkModule
var _ network.NetworkModule = (*mockNetworkModule)(nil)

// =============================================================================
// OrchestratorFactory Tests
// =============================================================================

func TestNewOrchestratorFactory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	require.NotNil(t, f)
	assert.Equal(t, "/tmp/test-data", f.dataDir)
	assert.NotNil(t, f.logger)
}

func TestOrchestratorFactory_GetBuilder_UnknownNetwork(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	// Should fail for unknown network (no plugins loaded)
	builder, err := f.GetBuilder("nonexistent-network")
	assert.Error(t, err)
	assert.Nil(t, builder)
}

func TestOrchestratorFactory_GetPluginRuntime_UnknownNetwork(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	// Should fail for unknown network
	pr, err := f.GetPluginRuntime("nonexistent-network")
	assert.Error(t, err)
	assert.Nil(t, pr)
}

func TestOrchestratorFactory_CreateOrchestrator_UnknownNetwork(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	// Should fail for unknown network
	orch, err := f.CreateOrchestrator("nonexistent-network")
	assert.Error(t, err)
	assert.Nil(t, orch)
}

func TestOrchestratorFactory_ListAvailableNetworks_Empty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	// Should return empty list when no plugins are loaded
	networks := f.ListAvailableNetworks()
	// The global registry might have networks registered from other tests
	// So we just verify the function works
	assert.NotNil(t, networks)
}

// =============================================================================
// nodeInitializerAdapter Tests
// =============================================================================

func TestNodeInitializerAdapter_ImplementsInterface(t *testing.T) {
	// Verify the adapter implements ports.NodeInitializer
	var _ ports.NodeInitializer = (*nodeInitializerAdapter)(nil)
}

func TestNodeInitializerAdapter_ImplementsBinaryPathUpdater(t *testing.T) {
	// Verify the adapter implements provisioner.BinaryPathUpdater
	var _ provisioner.BinaryPathUpdater = (*nodeInitializerAdapter)(nil)
}

func TestNodeInitializerAdapter_SetBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Initially empty
	assert.Empty(t, adapter.getBinaryPath())

	// Set path
	adapter.SetBinaryPath("/usr/local/bin/testd")
	assert.Equal(t, "/usr/local/bin/testd", adapter.getBinaryPath())
}

func TestNodeInitializerAdapter_InitializeWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Should fail without binary path set
	err := adapter.Initialize(context.Background(), "/tmp/node", "node0", "test-chain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_GetNodeIDWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Should fail without binary path set
	_, err := adapter.GetNodeID(context.Background(), "/tmp/node")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_CreateAccountKeyWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Should fail without binary path set
	_, err := adapter.CreateAccountKey(context.Background(), "/tmp/keyring", "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_CreateAccountKeyFromMnemonicWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Should fail without binary path set
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	_, err := adapter.CreateAccountKeyFromMnemonic(context.Background(), "/tmp/keyring", "test-key", mnemonic)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_GetAccountKeyWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Should fail without binary path set
	_, err := adapter.GetAccountKey(context.Background(), "/tmp/keyring", "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_GetTestMnemonic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	tests := []struct {
		index    int
		wantWord string // First word of the mnemonic
	}{
		{0, "abandon"},
		{1, "zoo"},
		{2, "vessel"},
		{3, "range"},
		{4, "abandon"}, // wraps around
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.index)), func(t *testing.T) {
			mnemonic := adapter.GetTestMnemonic(tt.index)
			assert.NotEmpty(t, mnemonic)
			// Check first word
			words := splitWords(mnemonic)
			assert.True(t, len(words) >= 12, "mnemonic should have at least 12 words")
			assert.Equal(t, tt.wantWord, words[0])
		})
	}
}

func TestNodeInitializerAdapter_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Test concurrent read/write to binary path
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			adapter.SetBinaryPath("/path/" + string(rune('a'+i%26)))
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = adapter.getBinaryPath()
	}

	<-done
	// Test passes if no race condition occurs
}

// =============================================================================
// parseKeyOutput Tests
// =============================================================================

func TestParseKeyOutput_V2Format(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Cosmos SDK v0.46+ format
	output := `{"name":"validator0","address":"cosmos1abc123def456","pubkey":"cosmospub1abc","mnemonic":"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about","type":"local"}`

	result, err := adapter.parseKeyOutput([]byte(output))
	require.NoError(t, err)
	assert.Equal(t, "validator0", result.Name)
	assert.Equal(t, "cosmos1abc123def456", result.Address)
	assert.Equal(t, "cosmospub1abc", result.PubKey)
	assert.Contains(t, result.Mnemonic, "abandon")
}

func TestParseKeyOutput_V1Format(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Cosmos SDK v0.45 and earlier format
	output := `{"name":"validator0","address":"cosmos1abc123def456","pubkey":"cosmospub1abc"}`

	result, err := adapter.parseKeyOutput([]byte(output))
	require.NoError(t, err)
	assert.Equal(t, "validator0", result.Name)
	assert.Equal(t, "cosmos1abc123def456", result.Address)
	assert.Equal(t, "cosmospub1abc", result.PubKey)
	assert.Empty(t, result.Mnemonic) // No mnemonic in show output
}

func TestParseKeyOutput_WithWhitespace(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Output with leading/trailing whitespace
	output := `
  {"name":"validator0","address":"cosmos1abc123def456","pubkey":"cosmospub1abc"}
  `

	result, err := adapter.parseKeyOutput([]byte(output))
	require.NoError(t, err)
	assert.Equal(t, "cosmos1abc123def456", result.Address)
}

func TestParseKeyOutput_EmptyOutput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	_, err := adapter.parseKeyOutput([]byte(""))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty key output")
}

func TestParseKeyOutput_InvalidJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	_, err := adapter.parseKeyOutput([]byte("not valid json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse key output")
}

func TestParseKeyOutput_MissingAddress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockModule("test", "testd")
	adapter := newNodeInitializerAdapter(mock, logger)

	// Valid JSON but no address field
	output := `{"name":"validator0","pubkey":"cosmospub1abc"}`

	_, err := adapter.parseKeyOutput([]byte(output))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized format")
}

// =============================================================================
// Adapter Interface Compliance Tests
// =============================================================================

func TestModuleBuilderAdapter_ImplementsInterface(t *testing.T) {
	var _ plugintypes.PluginBuilder = (*moduleBuilderAdapter)(nil)
}

func TestModuleGenesisAdapter_ImplementsInterface(t *testing.T) {
	var _ plugintypes.PluginGenesis = (*moduleGenesisAdapter)(nil)
}

func TestModuleInitializerAdapter_ImplementsInterface(t *testing.T) {
	var _ plugintypes.PluginInitializer = (*moduleInitializerAdapter)(nil)
}

// =============================================================================
// Helper Functions
// =============================================================================

// splitWords splits a string into words by spaces.
func splitWords(s string) []string {
	var words []string
	word := ""
	for _, c := range s {
		if c == ' ' {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		} else {
			word += string(c)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}
