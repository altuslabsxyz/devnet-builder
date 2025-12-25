// Example plugin for devnet-builder demonstrating how to create
// a custom network module for your own Cosmos SDK-based blockchain.
//
// To build this plugin:
//
//	go build -o devnet-cosmos ./examples/cosmos-plugin
//
// To use with devnet-builder:
//
//  1. Copy the binary to ~/.devnet-builder/plugins/
//  2. Run: devnet-builder networks  (should show "cosmos" in the list)
//  3. Run: devnet-builder --network cosmos generate
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/pkg/network"
	"github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

func main() {
	plugin.Serve(&CosmosNetwork{})
}

// CosmosNetwork implements network.Module for the Cosmos Hub.
type CosmosNetwork struct{}

// Ensure CosmosNetwork implements network.Module
var _ network.Module = (*CosmosNetwork)(nil)

// ============================================
// Identity Methods
// ============================================

func (n *CosmosNetwork) Name() string {
	return "cosmos"
}

func (n *CosmosNetwork) DisplayName() string {
	return "Cosmos Hub"
}

func (n *CosmosNetwork) Version() string {
	return "1.0.0"
}

// ============================================
// Binary Configuration
// ============================================

func (n *CosmosNetwork) BinaryName() string {
	return "gaiad"
}

func (n *CosmosNetwork) BinarySource() network.BinarySource {
	return network.BinarySource{
		Type:      "github",
		Owner:     "cosmos",
		Repo:      "gaia",
		AssetName: "gaiad-*-linux-amd64",
	}
}

func (n *CosmosNetwork) DefaultBinaryVersion() string {
	return "v18.1.0"
}

func (n *CosmosNetwork) GetBuildConfig(networkType string) (*network.BuildConfig, error) {
	// Example: Return empty config for all networks (no custom build configuration)
	// Plugins can customize this based on networkType
	return &network.BuildConfig{}, nil
}

// ============================================
// Chain Configuration
// ============================================

func (n *CosmosNetwork) DefaultChainID() string {
	return "cosmosdevnet-1"
}

func (n *CosmosNetwork) Bech32Prefix() string {
	return "cosmos"
}

func (n *CosmosNetwork) BaseDenom() string {
	return "uatom"
}

func (n *CosmosNetwork) GenesisConfig() network.GenesisConfig {
	return network.GenesisConfig{
		ChainIDPattern:    "cosmosdevnet-{num}",
		EVMChainID:        0, // Cosmos Hub doesn't have EVM
		BaseDenom:         "uatom",
		DenomExponent:     6,
		DisplayDenom:      "ATOM",
		BondDenom:         "uatom",
		MinSelfDelegation: "1",
		UnbondingTime:     120 * time.Second, // Short for devnet
		MaxValidators:     100,
		MinDeposit:        "10000000uatom",
		VotingPeriod:      60 * time.Second, // Short for devnet
		MaxDepositPeriod:  120 * time.Second,
		CommunityTax:      "0.020000000000000000",
	}
}

func (n *CosmosNetwork) DefaultPorts() network.PortConfig {
	return network.PortConfig{
		RPC:       26657,
		P2P:       26656,
		GRPC:      9090,
		GRPCWeb:   9091,
		API:       1317,
		EVMRPC:    0, // No EVM
		EVMSocket: 0, // No EVM
	}
}

// ============================================
// Docker Configuration
// ============================================

func (n *CosmosNetwork) DockerImage() string {
	return "ghcr.io/cosmos/gaia"
}

func (n *CosmosNetwork) DockerImageTag(version string) string {
	return version
}

func (n *CosmosNetwork) DockerHomeDir() string {
	return "/home/gaia"
}

// ============================================
// Path Configuration
// ============================================

func (n *CosmosNetwork) DefaultNodeHome() string {
	return "/root/.gaia"
}

func (n *CosmosNetwork) PIDFileName() string {
	return "gaiad.pid"
}

func (n *CosmosNetwork) LogFileName() string {
	return "gaiad.log"
}

func (n *CosmosNetwork) ProcessPattern() string {
	return "gaiad.*start"
}

// ============================================
// Command Generation
// ============================================

func (n *CosmosNetwork) InitCommand(homeDir, chainID, moniker string) []string {
	return []string{
		"init", moniker,
		"--chain-id", chainID,
		"--home", homeDir,
	}
}

func (n *CosmosNetwork) StartCommand(homeDir string) []string {
	return []string{
		"start",
		"--home", homeDir,
	}
}

func (n *CosmosNetwork) ExportCommand(homeDir string) []string {
	return []string{
		"export",
		"--home", homeDir,
	}
}

// ============================================
// Devnet Operations
// ============================================

func (n *CosmosNetwork) ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
	// This is a simplified example. In a real implementation,
	// you would parse the genesis JSON, modify parameters, and return.
	//
	// For example:
	// - Reduce unbonding time for faster testing
	// - Set governance parameters for quick proposals
	// - Configure staking parameters
	//
	// The genesis file is JSON bytes that can be parsed with encoding/json.
	return genesis, nil
}

func (n *CosmosNetwork) GenerateDevnet(ctx context.Context, config network.GeneratorConfig, genesisFile string) error {
	// This is where you implement the devnet generation logic.
	// Typically this involves:
	// 1. Creating validator directories
	// 2. Generating keys for validators
	// 3. Creating genesis transactions
	// 4. Collecting genesis transactions
	// 5. Writing final genesis.json
	//
	// For a real implementation, you would use the Cosmos SDK's
	// key generation and genesis utilities.
	return fmt.Errorf("GenerateDevnet not implemented for example plugin")
}

func (n *CosmosNetwork) DefaultGeneratorConfig() network.GeneratorConfig {
	return network.GeneratorConfig{
		NumValidators:    4,
		NumAccounts:      10,
		AccountBalance:   "100000000000uatom",
		ValidatorBalance: "1000000000000uatom",
		ValidatorStake:   "100000000uatom",
		OutputDir:        "./devnet",
		ChainID:          "cosmosdevnet-1",
	}
}

// ============================================
// Codec and Keyring
// ============================================

func (n *CosmosNetwork) GetCodec() ([]byte, error) {
	// Return network-specific codec configuration.
	// This is used for serialization of chain-specific types.
	// For most networks, you can return nil if not using custom types.
	return nil, nil
}

// ============================================
// Validation
// ============================================

func (n *CosmosNetwork) Validate() error {
	// Validate the module configuration.
	// Check that all required values are set correctly.
	if n.Name() == "" {
		return fmt.Errorf("network name is required")
	}
	if n.BinaryName() == "" {
		return fmt.Errorf("binary name is required")
	}
	return nil
}

// ============================================
// Snapshot Configuration
// ============================================

func (n *CosmosNetwork) SnapshotURL(networkType string) string {
	// Return snapshot URLs for your network.
	// These are typically hosted on S3, GCS, or other CDN.
	switch networkType {
	case "mainnet":
		return "https://snapshots.cosmos.directory/cosmoshub-4/latest.tar.lz4"
	case "testnet":
		return "https://snapshots.cosmos.directory/theta-testnet-001/latest.tar.lz4"
	default:
		return ""
	}
}

func (n *CosmosNetwork) RPCEndpoint(networkType string) string {
	// Return RPC endpoints for your network.
	switch networkType {
	case "mainnet":
		return "https://cosmos-rpc.polkachu.com"
	case "testnet":
		return "https://rpc.sentry-01.theta-testnet.polypore.xyz"
	default:
		return ""
	}
}

func (n *CosmosNetwork) AvailableNetworks() []string {
	return []string{"mainnet", "testnet"}
}

// ============================================
// Node Configuration
// ============================================

func (n *CosmosNetwork) GetConfigOverrides(nodeIndex int, opts network.NodeConfigOptions) ([]byte, []byte, error) {
	// Cosmos Hub doesn't require special configuration beyond defaults.
	// Return nil to use the default configs generated by init.
	// For networks with EVM or other special features, return TOML overrides here.
	return nil, nil, nil
}

// ============================================
// Governance Parameters (Plugin-based Query)
// ============================================

// GetGovernanceParams queries governance parameters from the blockchain via RPC/REST.
// This method is called by the devnet-builder during upgrade workflows to determine
// voting periods and deposit requirements dynamically from the running chain.
//
// Implementation approach:
// 1. Query Cosmos SDK governance module via REST API (cosmos/gov/v1/params/*)
// 2. Parse and validate the response
// 3. Convert to standardized response format
// 4. Return error field populated if query fails (network error, parsing error, etc.)
//
// This allows each network plugin to:
// - Use chain-specific RPC/REST endpoints
// - Handle different governance module versions (v1, v1beta1)
// - Apply network-specific parameter transformations
// - Support custom governance modules
func (n *CosmosNetwork) GetGovernanceParams(rpcEndpoint, networkType string) (*plugin.GovernanceParamsResponse, error) {
	// In a real implementation, you would:
	// 1. Make HTTP requests to rpcEndpoint + "/cosmos/gov/v1/params/voting"
	//    and rpcEndpoint + "/cosmos/gov/v1/params/deposit"
	// 2. Parse the JSON responses
	// 3. Extract voting_period, expedited_voting_period, min_deposit, expedited_min_deposit
	// 4. Convert time.Duration to nanoseconds (int64)
	// 5. Return the response

	// For this example, return sensible devnet defaults:
	return &plugin.GovernanceParamsResponse{
		VotingPeriodNs:           int64(60 * time.Second),  // 60 seconds for devnet
		ExpeditedVotingPeriodNs:  int64(30 * time.Second),  // 30 seconds for expedited
		MinDeposit:               "10000000uatom",          // 10 ATOM
		ExpeditedMinDeposit:      "50000000uatom",          // 50 ATOM
		Error:                    "",                       // Empty = success
	}, nil

	// Example error handling:
	// If network is unreachable, return error in response field:
	// return &plugin.GovernanceParamsResponse{
	//     Error: fmt.Sprintf("failed to connect to %s: connection refused", rpcEndpoint),
	// }, nil
}
