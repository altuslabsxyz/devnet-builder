// internal/plugin/cosmos/genesis_test.go
package cosmos

import (
	"encoding/json"
	"strings"
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
	if !strings.Contains(patchedStr, `"chain_id": "devnet-1"`) {
		t.Error("Patched genesis should contain new chain_id")
	}

	// Verify voting period was patched (nanosecond format for SDK v0.50+)
	if !strings.Contains(patchedStr, `"voting_period": "30000000000ns"`) {
		t.Errorf("Patched genesis should contain new voting_period in nanosecond format, got: %s", patchedStr)
	}

	// Verify unbonding time was patched (nanosecond format for SDK v0.50+)
	if !strings.Contains(patchedStr, `"unbonding_time": "60000000000ns"`) {
		t.Errorf("Patched genesis should contain new unbonding_time in nanosecond format, got: %s", patchedStr)
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
	if !strings.Contains(patchedStr, `"voting_period": "30000000000ns"`) {
		t.Errorf("Patched genesis should contain new voting_period in legacy format, got: %s", patchedStr)
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
	if !strings.Contains(patchedStr, `"chain_id": "cosmoshub-4"`) {
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

func TestCosmosGenesisPatchGenesisWithBinaryVersion(t *testing.T) {
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
		BinaryVersion: "v1.2.3",
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis with BinaryVersion failed: %v", err)
	}

	patchedStr := string(patched)

	// Verify devnet_builder metadata is present
	if !strings.Contains(patchedStr, `"devnet_builder"`) {
		t.Error("Patched genesis should contain devnet_builder metadata")
	}

	// Verify binary_version is present
	if !strings.Contains(patchedStr, `"binary_version": "v1.2.3"`) {
		t.Error("Patched genesis should contain binary_version")
	}

	// Verify binary_name is present
	if !strings.Contains(patchedStr, `"binary_name": "stabled"`) {
		t.Error("Patched genesis should contain binary_name")
	}

	// Verify patched_at is present (just check the key exists)
	if !strings.Contains(patchedStr, `"patched_at"`) {
		t.Error("Patched genesis should contain patched_at timestamp")
	}
}

func TestCosmosGenesisPatchGenesisWithoutBinaryVersion(t *testing.T) {
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

	// No BinaryVersion specified - should not add devnet_builder metadata
	opts := types.GenesisPatchOptions{
		ChainID: "devnet-1",
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis without BinaryVersion failed: %v", err)
	}

	patchedStr := string(patched)

	// Verify devnet_builder metadata is NOT present when BinaryVersion is empty
	if strings.Contains(patchedStr, `"devnet_builder"`) {
		t.Error("Patched genesis should NOT contain devnet_builder metadata when BinaryVersion is empty")
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
	for i, arg := range args {
		if arg == "export" {
			hasExport = true
		}
		if arg == "--home" && i+1 < len(args) && args[i+1] == "/home/node" {
			hasHome = true
		}
	}

	if !hasExport {
		t.Error("Export command should include 'export'")
	}
	if !hasHome {
		t.Error("Export command should include '--home' with path")
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

func TestCosmosGenesisPatchGenesisWarnsOnMissingGov(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Genesis without gov module - PatchGenesis should warn but not error
	genesis := []byte(`{
		"chain_id": "test-1",
		"app_state": {
			"auth": {},
			"bank": {},
			"staking": {"params": {"unbonding_time": "1814400s"}}
		}
	}`)

	opts := types.GenesisPatchOptions{
		VotingPeriod: 30 * time.Second,
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis should not error on missing gov module: %v", err)
	}

	// Verify we got valid JSON back
	var js json.RawMessage
	if err := json.Unmarshal(patched, &js); err != nil {
		t.Fatalf("Patched genesis should be valid JSON: %v", err)
	}
}

func TestCosmosGenesisPatchGenesisWarnsOnMissingStaking(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Genesis without staking module - PatchGenesis should warn but not error
	genesis := []byte(`{
		"chain_id": "test-1",
		"app_state": {
			"auth": {},
			"bank": {},
			"gov": {"params": {"voting_period": "1209600s"}}
		}
	}`)

	opts := types.GenesisPatchOptions{
		UnbondingTime: 60 * time.Second,
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis should not error on missing staking module: %v", err)
	}

	// Verify we got valid JSON back
	var js json.RawMessage
	if err := json.Unmarshal(patched, &js); err != nil {
		t.Fatalf("Patched genesis should be valid JSON: %v", err)
	}
}

func TestCosmosGenesisPatchGenesisInflationRate(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	genesis := []byte(`{
		"chain_id": "test-1",
		"app_state": {
			"auth": {},
			"bank": {},
			"staking": {"params": {"unbonding_time": "1814400s"}},
			"slashing": {},
			"gov": {"params": {"voting_period": "1209600s"}},
			"mint": {
				"minter": {"inflation": "0.13"},
				"params": {"inflation_rate_change": "0.13", "inflation_max": "0.20", "inflation_min": "0.07"}
			}
		}
	}`)

	opts := types.GenesisPatchOptions{
		InflationRate: "0.0",
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis with InflationRate failed: %v", err)
	}

	patchedStr := string(patched)
	if !strings.Contains(patchedStr, `"inflation": "0.0"`) {
		t.Errorf("Patched genesis should contain inflation 0.0, got: %s", patchedStr)
	}
	if !strings.Contains(patchedStr, `"inflation_max": "0.0"`) {
		t.Errorf("Patched genesis should contain inflation_max 0.0, got: %s", patchedStr)
	}
	if !strings.Contains(patchedStr, `"inflation_min": "0.0"`) {
		t.Errorf("Patched genesis should contain inflation_min 0.0, got: %s", patchedStr)
	}
}

func TestCosmosGenesisPatchGenesisInflationRateNoMint(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Genesis without mint module - should warn but not error
	genesis := []byte(`{
		"chain_id": "test-1",
		"app_state": {
			"auth": {},
			"bank": {},
			"staking": {"params": {"unbonding_time": "1814400s"}},
			"slashing": {},
			"gov": {"params": {"voting_period": "1209600s"}}
		}
	}`)

	opts := types.GenesisPatchOptions{
		InflationRate: "0.0",
	}

	patched, err := g.PatchGenesis(genesis, opts)
	if err != nil {
		t.Fatalf("PatchGenesis should not error on missing mint module: %v", err)
	}

	// Verify we got valid JSON back
	var js json.RawMessage
	if err := json.Unmarshal(patched, &js); err != nil {
		t.Fatalf("Patched genesis should be valid JSON: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0ns"},
		{"30 seconds", 30 * time.Second, "30000000000ns"},
		{"60 seconds", 60 * time.Second, "60000000000ns"},
		{"1 nanosecond", time.Nanosecond, "1ns"},
		{"500 milliseconds", 500 * time.Millisecond, "500000000ns"},
		{"14 days", 14 * 24 * time.Hour, "1209600000000000ns"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestCosmosGenesisWithLogger(t *testing.T) {
	g := NewCosmosGenesis("stabled")

	// Verify WithLogger returns the same instance for chaining
	result := g.WithLogger(nil)
	if result != g {
		t.Error("WithLogger should return same instance for chaining")
	}
}
