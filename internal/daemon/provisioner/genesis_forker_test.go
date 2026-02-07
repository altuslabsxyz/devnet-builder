// internal/daemon/provisioner/genesis_forker_test.go
package provisioner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// mockPluginGenesis implements types.PluginGenesis for testing
type mockPluginGenesis struct {
	rpcEndpoint string
	snapshotURL string
	validateErr error
	patchErr    error
}

func (m *mockPluginGenesis) GetRPCEndpoint(networkType string) string {
	return m.rpcEndpoint
}

func (m *mockPluginGenesis) GetSnapshotURL(networkType string) string {
	return m.snapshotURL
}

func (m *mockPluginGenesis) ValidateGenesis(genesis []byte) error {
	return m.validateErr
}

func (m *mockPluginGenesis) PatchGenesis(genesis []byte, opts types.GenesisPatchOptions) ([]byte, error) {
	if m.patchErr != nil {
		return nil, m.patchErr
	}
	return genesis, nil
}

func (m *mockPluginGenesis) ExportCommandArgs(homeDir string) []string {
	return []string{"export", "--home", homeDir}
}

func (m *mockPluginGenesis) BinaryName() string {
	return "testd"
}

func TestGenesisForkerConfig(t *testing.T) {
	tempDir := t.TempDir()

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	if forker == nil {
		t.Fatal("NewGenesisForker returned nil")
	}
}

func TestGenesisForkerForkFromLocal(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test genesis file
	testGenesis := []byte(`{
		"chain_id": "test-chain",
		"app_state": {
			"auth": {},
			"bank": {},
			"staking": {"params": {"unbonding_time": "1814400s"}},
			"slashing": {},
			"gov": {"params": {"voting_period": "1209600s"}}
		}
	}`)

	genesisPath := filepath.Join(tempDir, "genesis.json")
	if err := os.WriteFile(genesisPath, testGenesis, 0644); err != nil {
		t.Fatalf("Failed to write test genesis: %v", err)
	}

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	// Fork from local file
	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:      types.GenesisModeLocal,
			LocalPath: genesisPath,
		},
		PatchOpts: types.GenesisPatchOptions{
			ChainID:      "devnet-1",
			VotingPeriod: 30 * time.Second,
		},
	}

	ctx := context.Background()
	result, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err != nil {
		t.Fatalf("Fork failed: %v", err)
	}

	if len(result.Genesis) == 0 {
		t.Error("Expected non-empty genesis result")
	}

	if result.SourceChainID != "test-chain" {
		t.Errorf("Expected source chain ID 'test-chain', got '%s'", result.SourceChainID)
	}

	if result.NewChainID != "devnet-1" {
		t.Errorf("Expected new chain ID 'devnet-1', got '%s'", result.NewChainID)
	}

	if result.SourceMode != types.GenesisModeLocal {
		t.Errorf("Expected source mode 'local', got '%s'", result.SourceMode)
	}
}

func TestGenesisForkerForkFromLocalMissingPath(t *testing.T) {
	tempDir := t.TempDir()

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	// Fork from local file without specifying path
	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode: types.GenesisModeLocal,
			// LocalPath intentionally empty
		},
	}

	ctx := context.Background()
	_, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err == nil {
		t.Fatal("Expected error for missing local path")
	}
}

func TestGenesisForkerForkFromLocalFileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	// Fork from local file that doesn't exist
	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:      types.GenesisModeLocal,
			LocalPath: filepath.Join(tempDir, "nonexistent.json"),
		},
	}

	ctx := context.Background()
	_, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

func TestGenesisForkerForkFromRPCNoURL(t *testing.T) {
	tempDir := t.TempDir()

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{}, // empty RPC endpoint
	}

	forker := NewGenesisForker(config)

	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode: types.GenesisModeRPC,
			// No RPCURL specified
		},
	}

	ctx := context.Background()
	_, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err == nil {
		t.Fatal("Expected error for missing RPC URL")
	}
}

func TestGenesisForkerForkFromSnapshotNoBinary(t *testing.T) {
	tempDir := t.TempDir()

	config := GenesisForkerConfig{
		DataDir: tempDir,
		PluginGenesis: &mockPluginGenesis{
			snapshotURL: "https://snapshot.example.com/snapshot.tar.lz4",
		},
	}

	forker := NewGenesisForker(config)

	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:        types.GenesisModeSnapshot,
			NetworkType: "mainnet",
		},
		// BinaryPath intentionally empty
	}

	ctx := context.Background()
	_, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err == nil {
		t.Fatal("Expected error for missing binary path")
	}
}

func TestGenesisForkerApplyChainIDPatch(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test genesis file
	testGenesis := []byte(`{
		"chain_id": "original-chain",
		"app_state": {}
	}`)

	genesisPath := filepath.Join(tempDir, "genesis.json")
	if err := os.WriteFile(genesisPath, testGenesis, 0644); err != nil {
		t.Fatalf("Failed to write test genesis: %v", err)
	}

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:      types.GenesisModeLocal,
			LocalPath: genesisPath,
		},
		PatchOpts: types.GenesisPatchOptions{
			ChainID: "new-devnet-chain",
		},
	}

	ctx := context.Background()
	result, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err != nil {
		t.Fatalf("Fork failed: %v", err)
	}

	// Verify the chain ID was patched
	if result.NewChainID != "new-devnet-chain" {
		t.Errorf("Expected new chain ID 'new-devnet-chain', got '%s'", result.NewChainID)
	}

	// Verify the original chain ID is preserved
	if result.SourceChainID != "original-chain" {
		t.Errorf("Expected source chain ID 'original-chain', got '%s'", result.SourceChainID)
	}
}

func TestGenesisForkerUnsupportedMode(t *testing.T) {
	tempDir := t.TempDir()

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode: "invalid-mode",
		},
	}

	ctx := context.Background()
	_, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err == nil {
		t.Fatal("Expected error for unsupported mode")
	}
}

func TestGenesisForkerWithNilPluginGenesis(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test genesis file
	testGenesis := []byte(`{
		"chain_id": "test-chain",
		"app_state": {}
	}`)

	genesisPath := filepath.Join(tempDir, "genesis.json")
	if err := os.WriteFile(genesisPath, testGenesis, 0644); err != nil {
		t.Fatalf("Failed to write test genesis: %v", err)
	}

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: nil, // No plugin genesis
	}

	forker := NewGenesisForker(config)

	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:      types.GenesisModeLocal,
			LocalPath: genesisPath,
		},
		PatchOpts: types.GenesisPatchOptions{
			ChainID: "devnet-1",
		},
	}

	ctx := context.Background()
	result, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err != nil {
		t.Fatalf("Fork failed: %v", err)
	}

	if len(result.Genesis) == 0 {
		t.Error("Expected non-empty genesis result")
	}
}

func TestGenesisForkerNoPatchOptions(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test genesis file
	testGenesis := []byte(`{
		"chain_id": "test-chain",
		"app_state": {}
	}`)

	genesisPath := filepath.Join(tempDir, "genesis.json")
	if err := os.WriteFile(genesisPath, testGenesis, 0644); err != nil {
		t.Fatalf("Failed to write test genesis: %v", err)
	}

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:      types.GenesisModeLocal,
			LocalPath: genesisPath,
		},
		// No PatchOpts - should not modify anything
	}

	ctx := context.Background()
	result, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err != nil {
		t.Fatalf("Fork failed: %v", err)
	}

	// With no patch options, chain ID should remain original
	if result.SourceChainID != "test-chain" {
		t.Errorf("Expected source chain ID 'test-chain', got '%s'", result.SourceChainID)
	}
}

func TestGenesisForkerNilPluginWithPatchOptions(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test genesis file with gov and staking modules
	testGenesis := []byte(`{
		"chain_id": "test-chain",
		"app_state": {
			"gov": {"params": {"voting_period": "1209600s"}},
			"staking": {"params": {"unbonding_time": "1814400s"}}
		}
	}`)

	genesisPath := filepath.Join(tempDir, "genesis.json")
	if err := os.WriteFile(genesisPath, testGenesis, 0644); err != nil {
		t.Fatalf("Failed to write test genesis: %v", err)
	}

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: nil, // No plugin - VotingPeriod/UnbondingTime won't be applied
	}

	forker := NewGenesisForker(config)

	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:      types.GenesisModeLocal,
			LocalPath: genesisPath,
		},
		PatchOpts: types.GenesisPatchOptions{
			ChainID:       "devnet-1",
			VotingPeriod:  30 * time.Second,
			UnbondingTime: 60 * time.Second,
			InflationRate: "0.0",
		},
	}

	ctx := context.Background()
	result, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err != nil {
		t.Fatalf("Fork should succeed even without plugin, got: %v", err)
	}

	// Chain ID should still be patched (handled by applyPatches, not plugin)
	if result.NewChainID != "devnet-1" {
		t.Errorf("Expected new chain ID 'devnet-1', got '%s'", result.NewChainID)
	}

	// Genesis should contain the new chain_id
	if !strings.Contains(string(result.Genesis), `"devnet-1"`) {
		t.Error("Expected genesis to contain patched chain_id 'devnet-1'")
	}

	// The original voting_period should remain unchanged (plugin wasn't available to patch it)
	if !strings.Contains(string(result.Genesis), "1209600s") {
		t.Error("Expected original voting_period to remain unchanged when no plugin is configured")
	}
}

func TestGenesisForkerForkFromLocalRelativePathRejected(t *testing.T) {
	tempDir := t.TempDir()

	config := GenesisForkerConfig{
		DataDir:       tempDir,
		PluginGenesis: &mockPluginGenesis{},
	}

	forker := NewGenesisForker(config)

	// Fork from local file with relative path - should be rejected for security
	opts := ports.ForkOptions{
		Source: types.GenesisSource{
			Mode:      types.GenesisModeLocal,
			LocalPath: "relative/path/genesis.json", // Not absolute
		},
	}

	ctx := context.Background()
	_, err := forker.Fork(ctx, opts, ports.NilProgressReporter)
	if err == nil {
		t.Fatal("Expected error for relative path")
	}
	if !strings.Contains(err.Error(), "must be absolute") {
		t.Errorf("Expected 'must be absolute' error, got: %v", err)
	}
}
