// internal/plugin/cosmos/genesis_test.go
package cosmos

import (
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

func TestCosmosGenesisGetRPCEndpoint(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	tests := []struct {
		networkType string
		wantContain string
	}{
		{"mainnet", "rpc"},
		{"testnet", "rpc"},
	}

	for _, tt := range tests {
		t.Run(tt.networkType, func(t *testing.T) {
			endpoint := g.GetRPCEndpoint(tt.networkType)
			if endpoint == "" {
				t.Error("Expected non-empty RPC endpoint")
			}
		})
	}
}

func TestCosmosGenesisGetRPCEndpointFallback(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Unknown network type should fall back to mainnet
	endpoint := g.GetRPCEndpoint("unknown")
	if endpoint == "" {
		t.Error("Expected fallback to mainnet RPC endpoint")
	}
}

func TestCosmosGenesisWithRPCEndpoint(t *testing.T) {
	g := NewCosmosGenesis("stabled")
	customEndpoint := "https://custom-rpc.example.com"

	g.WithRPCEndpoint("custom", customEndpoint)

	endpoint := g.GetRPCEndpoint("custom")
	if endpoint != customEndpoint {
		t.Errorf("Expected custom endpoint %s, got %s", customEndpoint, endpoint)
	}
}

func TestCosmosGenesisGetSnapshotURL(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Default snapshot URLs are empty
	url := g.GetSnapshotURL("mainnet")
	if url != "" {
		t.Errorf("Expected empty default snapshot URL, got %s", url)
	}

	// Configure a snapshot URL
	customURL := "https://snapshots.example.com/snapshot.tar.gz"
	g.WithSnapshotURL("mainnet", customURL)

	url = g.GetSnapshotURL("mainnet")
	if url != customURL {
		t.Errorf("Expected snapshot URL %s, got %s", customURL, url)
	}
}

func TestCosmosGenesisValidateGenesis(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Valid genesis
	validGenesis := []byte(`{"chain_id": "test-1", "app_state": {"auth": {}, "bank": {}, "staking": {}, "slashing": {}, "gov": {}}}`)
	if err := g.ValidateGenesis(validGenesis); err != nil {
		t.Errorf("Valid genesis should pass validation: %v", err)
	}

	// Invalid genesis - missing chain_id
	invalidGenesis := []byte(`{"app_state": {"auth": {}, "bank": {}, "staking": {}, "slashing": {}, "gov": {}}}`)
	if err := g.ValidateGenesis(invalidGenesis); err == nil {
		t.Error("Invalid genesis (missing chain_id) should fail validation")
	}

	// Invalid genesis - missing app_state
	invalidGenesis2 := []byte(`{"chain_id": "test-1"}`)
	if err := g.ValidateGenesis(invalidGenesis2); err == nil {
		t.Error("Invalid genesis (missing app_state) should fail validation")
	}

	// Invalid genesis - missing required module
	invalidGenesis3 := []byte(`{"chain_id": "test-1", "app_state": {"auth": {}, "bank": {}}}`)
	if err := g.ValidateGenesis(invalidGenesis3); err == nil {
		t.Error("Invalid genesis (missing required modules) should fail validation")
	}

	// Invalid JSON
	invalidJSON := []byte(`{invalid json}`)
	if err := g.ValidateGenesis(invalidJSON); err == nil {
		t.Error("Invalid JSON should fail validation")
	}
}

func TestCosmosGenesisPatchGenesis(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	genesis := []byte(`{
		"chain_id": "cosmoshub-4",
		"app_state": {
			"auth": {},
			"bank": {},
			"staking": {"params": {"unbonding_time": "1814400s"}},
			"slashing": {},
			"gov": {"params": {"voting_period": "1209600s"}}
		}
	}`)

	opts := types.GenesisPatchOptions{
		ChainID:       "devnet-1",
		VotingPeriod:  30 * time.Second,
		UnbondingTime: 60 * time.Second,
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis failed: %v", err)
	}

	// Verify chain_id was changed
	if string(patched) == string(genesis) {
		t.Error("Patched genesis should be different from original")
	}

	// Verify patched content contains new chain_id
	patchedStr := string(patched)
	if !contains(patchedStr, `"chain_id": "devnet-1"`) {
		t.Error("Patched genesis should contain new chain_id")
	}

	// Verify voting period was patched
	if !contains(patchedStr, `"voting_period": "30s"`) {
		t.Error("Patched genesis should contain new voting_period")
	}

	// Verify unbonding time was patched
	if !contains(patchedStr, `"unbonding_time": "60s"`) {
		t.Error("Patched genesis should contain new unbonding_time")
	}
}

func TestCosmosGenesisPatchGenesisLegacyGovFormat(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Legacy gov format uses voting_params instead of params
	genesis := []byte(`{
		"chain_id": "cosmoshub-4",
		"app_state": {
			"auth": {},
			"bank": {},
			"staking": {"params": {"unbonding_time": "1814400s"}},
			"slashing": {},
			"gov": {"voting_params": {"voting_period": "1209600s"}}
		}
	}`)

	opts := types.GenesisPatchOptions{
		VotingPeriod: 30 * time.Second,
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis with legacy gov format failed: %v", err)
	}

	patchedStr := string(patched)
	if !contains(patchedStr, `"voting_period": "30s"`) {
		t.Error("Patched genesis should contain new voting_period in legacy format")
	}
}

func TestCosmosGenesisPatchGenesisNoOpts(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	genesis := []byte(`{
		"chain_id": "cosmoshub-4",
		"app_state": {
			"auth": {},
			"bank": {},
			"staking": {"params": {"unbonding_time": "1814400s"}},
			"slashing": {},
			"gov": {"params": {"voting_period": "1209600s"}}
		}
	}`)

	// Empty options - should not change chain_id or other params
	opts := types.GenesisPatchOptions{}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis with empty opts failed: %v", err)
	}

	patchedStr := string(patched)
	// Chain ID should remain unchanged
	if !contains(patchedStr, `"chain_id": "cosmoshub-4"`) {
		t.Error("Chain ID should remain unchanged when not specified in opts")
	}
}

func TestCosmosGenesisPatchGenesisInvalidJSON(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	invalidJSON := []byte(`{invalid json}`)
	opts := types.GenesisPatchOptions{ChainID: "devnet-1"}

	_, err := g.PatchGenesis(invalidJSON, opts)
	if err == nil {
		t.Error("PatchGenesis should fail with invalid JSON")
	}
}

func TestCosmosGenesisPatchGenesisInvalidAppState(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// app_state is not a map
	genesis := []byte(`{"chain_id": "test-1", "app_state": "invalid"}`)
	opts := types.GenesisPatchOptions{ChainID: "devnet-1"}

	_, err := g.PatchGenesis(genesis, opts)
	if err == nil {
		t.Error("PatchGenesis should fail with invalid app_state format")
	}
}

func TestCosmosGenesisExportCommandArgs(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	args := g.ExportCommandArgs("/home/node")

	if len(args) == 0 {
		t.Error("Expected non-empty export command args")
	}

	// Should include export and --home
	hasExport := false
	hasHome := false
	hasForZeroHeight := false
	for i, arg := range args {
		if arg == "export" {
			hasExport = true
		}
		if arg == "--home" && i+1 < len(args) && args[i+1] == "/home/node" {
			hasHome = true
		}
		if arg == "--for-zero-height" {
			hasForZeroHeight = true
		}
	}

	if !hasExport {
		t.Error("Export command should include 'export'")
	}
	if !hasHome {
		t.Error("Export command should include '--home' with path")
	}
	if !hasForZeroHeight {
		t.Error("Export command should include '--for-zero-height'")
	}
}

func TestCosmosGenesisBinaryName(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	if g.BinaryName() != "stabled" {
		t.Errorf("Expected binary name 'stabled', got '%s'", g.BinaryName())
	}
}

func TestCosmosGenesisImplementsInterface(t *testing.T) {
	// This test ensures CosmosGenesis implements PluginGenesis interface
	var _ types.PluginGenesis = (*CosmosGenesis)(nil)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
