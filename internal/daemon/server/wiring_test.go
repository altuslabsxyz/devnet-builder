// internal/daemon/server/wiring_test.go
package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NetworkRegistry Tests
// =============================================================================

func TestNewNetworkRegistry(t *testing.T) {
	r := NewNetworkRegistry()
	require.NotNil(t, r)
	require.NotNil(t, r.networks)
}

func TestNetworkRegistry_Get_SupportedNetworks(t *testing.T) {
	r := NewNetworkRegistry()

	tests := []struct {
		name       string
		binaryName string
	}{
		{"stable", "stabled"},
		{"cosmos", "gaiad"},
		{"gaia", "gaiad"}, // alias for cosmos
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := r.Get(tt.name)
			require.NoError(t, err)
			require.NotNil(t, plugin)

			assert.Equal(t, tt.binaryName, plugin.BinaryName)
			assert.NotNil(t, plugin.Builder)
			assert.NotNil(t, plugin.Genesis)
			assert.NotNil(t, plugin.Initializer)
			assert.NotNil(t, plugin.Runtime)
		})
	}
}

func TestNetworkRegistry_Get_UnknownNetwork(t *testing.T) {
	r := NewNetworkRegistry()

	plugin, err := r.Get("unknown-network")
	assert.Error(t, err)
	assert.Nil(t, plugin)
	assert.Contains(t, err.Error(), "unknown network")
	assert.Contains(t, err.Error(), "unknown-network")
}

func TestNetworkRegistry_GetPluginRuntime(t *testing.T) {
	r := NewNetworkRegistry()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"stable", false},
		{"cosmos", false},
		{"gaia", false},
		{"nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr, err := r.GetPluginRuntime(tt.name)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pr)
				// Verify it implements PluginRuntime
				var _ runtime.PluginRuntime = pr
			}
		})
	}
}

func TestNetworkPlugin_StableConfig(t *testing.T) {
	r := NewNetworkRegistry()
	plugin, err := r.Get("stable")
	require.NoError(t, err)

	assert.Equal(t, "stable", plugin.Name)
	assert.Equal(t, "stabled", plugin.BinaryName)
	assert.Equal(t, "github.com/stablelabs/stable", plugin.DefaultRepo)
}

func TestNetworkPlugin_CosmosConfig(t *testing.T) {
	r := NewNetworkRegistry()
	plugin, err := r.Get("cosmos")
	require.NoError(t, err)

	assert.Equal(t, "cosmos", plugin.Name)
	assert.Equal(t, "gaiad", plugin.BinaryName)
	assert.Equal(t, "github.com/cosmos/gaia", plugin.DefaultRepo)
}

func TestNetworkPlugin_GaiaAlias(t *testing.T) {
	r := NewNetworkRegistry()

	cosmosPlugin, _ := r.Get("cosmos")
	gaiaPlugin, _ := r.Get("gaia")

	// Both should reference the same plugin
	assert.Equal(t, cosmosPlugin, gaiaPlugin)
}

// =============================================================================
// OrchestratorFactory Tests
// =============================================================================

func TestNewOrchestratorFactory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	require.NotNil(t, f)
	assert.Equal(t, "/tmp/test-data", f.dataDir)
	assert.NotNil(t, f.registry)
	assert.NotNil(t, f.logger)
}

func TestOrchestratorFactory_GetBuilder(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	tests := []struct {
		pluginName string
		wantErr    bool
	}{
		{"stable", false},
		{"cosmos", false},
		{"gaia", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.pluginName, func(t *testing.T) {
			builder, err := f.GetBuilder(tt.pluginName)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, builder)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, builder)
			}
		})
	}
}

func TestOrchestratorFactory_GetPluginRuntime(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	pr, err := f.GetPluginRuntime("stable")
	require.NoError(t, err)
	require.NotNil(t, pr)

	// Verify it's a proper PluginRuntime
	var _ runtime.PluginRuntime = pr
}

func TestOrchestratorFactory_CreateOrchestrator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	f := NewOrchestratorFactory("/tmp/test-data", logger)

	tests := []struct {
		network string
		wantErr bool
	}{
		{"stable", false},
		{"cosmos", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.network, func(t *testing.T) {
			orch, err := f.CreateOrchestrator(tt.network)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, orch)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, orch)
			}
		})
	}
}

// =============================================================================
// nodeInitializerAdapter Tests
// =============================================================================

// mockPluginInitializer implements plugintypes.PluginInitializer for testing.
type mockPluginInitializer struct {
	binaryName string
	chainID    string
	initArgs   []string
}

func (m *mockPluginInitializer) BinaryName() string {
	if m.binaryName != "" {
		return m.binaryName
	}
	return "mockd"
}

func (m *mockPluginInitializer) DefaultChainID() string {
	if m.chainID != "" {
		return m.chainID
	}
	return "mock-chain-1"
}

func (m *mockPluginInitializer) DefaultMoniker(index int) string {
	return fmt.Sprintf("validator-%d", index)
}

func (m *mockPluginInitializer) InitCommandArgs(homeDir, moniker, chainID string) []string {
	if m.initArgs != nil {
		return m.initArgs
	}
	return []string{"init", moniker, "--chain-id", chainID, "--home", homeDir}
}

func (m *mockPluginInitializer) ConfigDir(homeDir string) string {
	return homeDir + "/config"
}

func (m *mockPluginInitializer) DataDir(homeDir string) string {
	return homeDir + "/data"
}

func (m *mockPluginInitializer) KeyringDir(homeDir string) string {
	return homeDir
}

func TestNodeInitializerAdapter_ImplementsInterface(t *testing.T) {
	// Verify the adapter implements ports.NodeInitializer
	var _ ports.NodeInitializer = (*nodeInitializerAdapter)(nil)
}

func TestNodeInitializerAdapter_SetBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	// Initially empty
	assert.Empty(t, adapter.getBinaryPath())

	// Set path
	adapter.SetBinaryPath("/usr/local/bin/stabled")
	assert.Equal(t, "/usr/local/bin/stabled", adapter.getBinaryPath())
}

func TestNodeInitializerAdapter_InitializeWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	// Should fail without binary path set
	err := adapter.Initialize(context.Background(), "/tmp/node", "node0", "test-chain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_GetNodeIDWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	// Should fail without binary path set
	_, err := adapter.GetNodeID(context.Background(), "/tmp/node")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_CreateAccountKeyWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	// Should fail without binary path set
	_, err := adapter.CreateAccountKey(context.Background(), "/tmp/keyring", "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_CreateAccountKeyFromMnemonicWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	// Should fail without binary path set
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	_, err := adapter.CreateAccountKeyFromMnemonic(context.Background(), "/tmp/keyring", "test-key", mnemonic)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_GetAccountKeyWithoutBinaryPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	// Should fail without binary path set
	_, err := adapter.GetAccountKey(context.Background(), "/tmp/keyring", "test-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "binary path not set")
}

func TestNodeInitializerAdapter_GetTestMnemonic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

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
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

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
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

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
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

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
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

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
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	_, err := adapter.parseKeyOutput([]byte(""))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty key output")
}

func TestParseKeyOutput_InvalidJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	_, err := adapter.parseKeyOutput([]byte("not valid json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse key output")
}

func TestParseKeyOutput_MissingAddress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := &mockPluginInitializer{}
	adapter := newNodeInitializerAdapter(mock, "stabled", logger)

	// Valid JSON but no address field
	output := `{"name":"validator0","pubkey":"cosmospub1abc"}`

	_, err := adapter.parseKeyOutput([]byte(output))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized format")
}

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
