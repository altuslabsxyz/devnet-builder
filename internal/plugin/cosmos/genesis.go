// internal/plugin/cosmos/genesis.go
package cosmos

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// CosmosGenesis handles genesis operations for Cosmos SDK chains
type CosmosGenesis struct {
	binaryName string
	// Network-specific RPC endpoints
	rpcEndpoints map[string]string
	// Network-specific snapshot URLs
	snapshotURLs map[string]string
}

// NewCosmosGenesis creates a new Cosmos genesis handler
func NewCosmosGenesis(binaryName string) *CosmosGenesis {
	return &CosmosGenesis{
		binaryName: binaryName,
		rpcEndpoints: map[string]string{
			"mainnet": "https://rpc.cosmos.network",
			"testnet": "https://rpc.sentry-01.theta-testnet.polypore.xyz",
		},
		snapshotURLs: map[string]string{
			"mainnet": "", // To be configured per network
			"testnet": "",
		},
	}
}

// WithRPCEndpoint configures a custom RPC endpoint for a network type
func (g *CosmosGenesis) WithRPCEndpoint(networkType, endpoint string) *CosmosGenesis {
	g.rpcEndpoints[networkType] = endpoint
	return g
}

// WithSnapshotURL configures a snapshot URL for a network type
func (g *CosmosGenesis) WithSnapshotURL(networkType, url string) *CosmosGenesis {
	g.snapshotURLs[networkType] = url
	return g
}

// GetRPCEndpoint returns the RPC endpoint for a network type
func (g *CosmosGenesis) GetRPCEndpoint(networkType string) string {
	if endpoint, ok := g.rpcEndpoints[networkType]; ok {
		return endpoint
	}
	return g.rpcEndpoints["mainnet"]
}

// GetSnapshotURL returns the snapshot URL for a network type
func (g *CosmosGenesis) GetSnapshotURL(networkType string) string {
	if url, ok := g.snapshotURLs[networkType]; ok {
		return url
	}
	return ""
}

// ValidateGenesis validates genesis for Cosmos SDK chains
func (g *CosmosGenesis) ValidateGenesis(genesis []byte) error {
	var gen struct {
		ChainID  string                     `json:"chain_id"`
		AppState map[string]json.RawMessage `json:"app_state"`
	}

	if err := json.Unmarshal(genesis, &gen); err != nil {
		return fmt.Errorf("failed to parse genesis: %w", err)
	}

	if gen.ChainID == "" {
		return fmt.Errorf("chain_id is empty")
	}

	if gen.AppState == nil {
		return fmt.Errorf("missing app_state")
	}

	// Check for required Cosmos SDK modules
	requiredModules := []string{"auth", "bank", "staking", "slashing", "gov"}
	for _, mod := range requiredModules {
		if _, ok := gen.AppState[mod]; !ok {
			return fmt.Errorf("missing required module: %s", mod)
		}
	}

	return nil
}

// PatchGenesis applies modifications to the genesis
func (g *CosmosGenesis) PatchGenesis(genesis []byte, opts types.GenesisPatchOptions) ([]byte, error) {
	var gen map[string]interface{}
	if err := json.Unmarshal(genesis, &gen); err != nil {
		return nil, fmt.Errorf("failed to parse genesis: %w", err)
	}

	// Patch chain_id
	if opts.ChainID != "" {
		gen["chain_id"] = opts.ChainID
	}

	// Get app_state
	appState, ok := gen["app_state"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid app_state format")
	}

	// Patch governance voting period
	if opts.VotingPeriod > 0 {
		if err := g.patchGovParams(appState, opts.VotingPeriod); err != nil {
			// Non-fatal: some chains may not have gov module
		}
	}

	// Patch staking unbonding time
	if opts.UnbondingTime > 0 {
		if err := g.patchStakingParams(appState, opts.UnbondingTime); err != nil {
			// Non-fatal: some chains may not have staking module
		}
	}

	return json.MarshalIndent(gen, "", "  ")
}

// patchGovParams patches governance parameters
func (g *CosmosGenesis) patchGovParams(appState map[string]interface{}, votingPeriod time.Duration) error {
	gov, ok := appState["gov"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("gov module not found or invalid format")
	}

	// Try to find params (different SDK versions have different structures)
	params, ok := gov["params"].(map[string]interface{})
	if !ok {
		// Try legacy format
		params, ok = gov["voting_params"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("gov params not found")
		}
	}

	// Format as duration string (e.g., "30s")
	params["voting_period"] = formatDuration(votingPeriod)
	return nil
}

// patchStakingParams patches staking parameters
func (g *CosmosGenesis) patchStakingParams(appState map[string]interface{}, unbondingTime time.Duration) error {
	staking, ok := appState["staking"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("staking module not found or invalid format")
	}

	params, ok := staking["params"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("staking params not found")
	}

	params["unbonding_time"] = formatDuration(unbondingTime)
	return nil
}

// formatDuration formats a duration as a Cosmos SDK duration string (e.g., "30s")
func formatDuration(d time.Duration) string {
	// Cosmos SDK uses format like "1209600s" (seconds as string)
	return fmt.Sprintf("%ds", int64(d.Seconds()))
}

// ExportCommandArgs returns the export command arguments
func (g *CosmosGenesis) ExportCommandArgs(homeDir string) []string {
	return []string{
		"export",
		"--home", homeDir,
		"--for-zero-height",
	}
}

// BinaryName returns the binary name
func (g *CosmosGenesis) BinaryName() string {
	return g.binaryName
}

// Ensure CosmosGenesis implements PluginGenesis
var _ types.PluginGenesis = (*CosmosGenesis)(nil)
