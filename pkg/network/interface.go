// Package network provides the public SDK for developing devnet-builder network plugins.
//
// This package allows third-party developers to create custom network modules
// for their own Cosmos SDK-based blockchains.
//
// Example usage:
//
//	package main
//
//	import (
//	    "github.com/b-harvest/devnet-builder/pkg/network"
//	    "github.com/b-harvest/devnet-builder/pkg/network/plugin"
//	)
//
//	type MyNetwork struct{}
//
//	func (m *MyNetwork) Name() string { return "mychain" }
//	// ... implement other methods
//
//	func main() {
//	    plugin.Serve(&MyNetwork{})
//	}
package network

import (
	"context"
	"time"
)

// Module is the interface that all network plugins must implement.
// This is the core abstraction for supporting multiple Cosmos SDK-based networks.
type Module interface {
	// ============================================
	// Identity Methods
	// ============================================

	// Name returns the unique identifier for this network.
	// This should be lowercase, alphanumeric with hyphens only.
	// Examples: "stable", "ault", "osmosis", "cosmos-hub"
	Name() string

	// DisplayName returns a human-readable name for the network.
	// Examples: "Stable Network", "Osmosis DEX", "Cosmos Hub"
	DisplayName() string

	// Version returns the module version (semantic versioning recommended).
	// Example: "1.0.0"
	Version() string

	// ============================================
	// Binary Configuration
	// ============================================

	// BinaryName returns the name of the network's CLI binary.
	// Examples: "stabled", "osmosisd", "gaiad"
	BinaryName() string

	// BinarySource returns configuration for acquiring the binary.
	BinarySource() BinarySource

	// DefaultBinaryVersion returns the default version to build/use.
	// Examples: "v1.1.3", "latest", "main"
	DefaultBinaryVersion() string

	// ============================================
	// Chain Configuration
	// ============================================

	// DefaultChainID returns the default chain ID for the devnet.
	// Examples: "stabledevnet_2200-1", "osmosis-devnet-1"
	DefaultChainID() string

	// Bech32Prefix returns the address prefix for this network.
	// Examples: "stable", "osmo", "cosmos"
	Bech32Prefix() string

	// BaseDenom returns the base token denomination.
	// Examples: "ustable", "uosmo", "uatom"
	BaseDenom() string

	// GenesisConfig returns default genesis parameters.
	GenesisConfig() GenesisConfig

	// DefaultPorts returns the default port configuration.
	DefaultPorts() PortConfig

	// ============================================
	// Docker Configuration
	// ============================================

	// DockerImage returns the Docker image name for this network.
	// Example: "ghcr.io/stablelabs/stable"
	DockerImage() string

	// DockerImageTag returns the Docker tag for a given version.
	// Allows networks to customize version-to-tag mapping.
	DockerImageTag(version string) string

	// DockerHomeDir returns the home directory inside Docker containers.
	// Example: "/home/stabled"
	DockerHomeDir() string

	// ============================================
	// Path Configuration
	// ============================================

	// DefaultNodeHome returns the default node home directory path.
	// Example: "/root/.stabled"
	DefaultNodeHome() string

	// PIDFileName returns the process ID file name.
	// Example: "stabled.pid"
	PIDFileName() string

	// LogFileName returns the log file name.
	// Example: "stabled.log"
	LogFileName() string

	// ProcessPattern returns the regex pattern to match running processes.
	// Example: "stabled.*start"
	ProcessPattern() string

	// ============================================
	// Command Generation
	// ============================================

	// InitCommand returns arguments for initializing a node.
	// Parameters: homeDir, chainID, moniker
	// Returns: e.g., ["init", "node0", "--chain-id", "mychain-1", "--home", "/path"]
	InitCommand(homeDir, chainID, moniker string) []string

	// StartCommand returns arguments for starting a node.
	// Parameters: homeDir
	// Returns: e.g., ["start", "--home", "/path"]
	StartCommand(homeDir string) []string

	// ExportCommand returns arguments for exporting genesis/state.
	// Parameters: homeDir
	// Returns: e.g., ["export", "--home", "/path"]
	ExportCommand(homeDir string) []string

	// ============================================
	// Devnet Operations
	// ============================================

	// ModifyGenesis applies network-specific modifications to a genesis file.
	// Parameters:
	//   - genesis: Raw genesis JSON bytes
	//   - opts: User-provided customization options
	// Returns: Modified genesis JSON bytes, or error
	ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error)

	// GenerateDevnet generates validators, accounts, and modifies genesis.
	// This is the main entry point for devnet creation.
	GenerateDevnet(ctx context.Context, config GeneratorConfig, genesisFile string) error

	// DefaultGeneratorConfig returns the default configuration for devnet generation.
	DefaultGeneratorConfig() GeneratorConfig

	// ============================================
	// Codec and Keyring
	// ============================================

	// GetCodec returns the network-specific codec configuration.
	// This is used for serialization/deserialization of chain-specific types.
	// Returns encoded codec configuration or error.
	GetCodec() ([]byte, error)

	// ============================================
	// Validation
	// ============================================

	// Validate checks if the module configuration is valid.
	// Called during registration and before use.
	Validate() error

	// ============================================
	// Snapshot Configuration
	// ============================================

	// SnapshotURL returns the snapshot download URL for the given network type.
	// Parameters:
	//   - networkType: Type of network ("mainnet", "testnet", etc.)
	// Returns: Full URL to the snapshot archive, or empty string if not available
	// Example: "https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst"
	SnapshotURL(networkType string) string

	// RPCEndpoint returns the RPC endpoint for the given network type.
	// Parameters:
	//   - networkType: Type of network ("mainnet", "testnet", etc.)
	// Returns: Full URL to the RPC endpoint, or empty string if not available
	// Example: "https://cosmos-rpc.stable.xyz"
	RPCEndpoint(networkType string) string

	// AvailableNetworks returns a list of supported network types.
	// Returns: Slice of network type strings (e.g., ["mainnet", "testnet"])
	AvailableNetworks() []string
}

// BinarySource defines how to acquire the network binary.
type BinarySource struct {
	// Type specifies the source type: "github", "local", "docker"
	Type string `json:"type"`

	// GitHub source configuration
	Owner     string `json:"owner,omitempty"`      // GitHub owner/org
	Repo      string `json:"repo,omitempty"`       // GitHub repository
	AssetName string `json:"asset_name,omitempty"` // Release asset name pattern

	// Local source configuration
	LocalPath string `json:"local_path,omitempty"` // Path to local binary

	// Build configuration
	BuildTags []string `json:"build_tags,omitempty"` // Go build tags (e.g., ["no_dynamic_precompiles"])
}

// IsGitHub returns true if this is a GitHub source.
func (s BinarySource) IsGitHub() bool {
	return s.Type == "github" || (s.Owner != "" && s.Repo != "")
}

// PortConfig contains network port configuration.
type PortConfig struct {
	RPC       int `json:"rpc"`        // Tendermint RPC (default: 26657)
	P2P       int `json:"p2p"`        // P2P networking (default: 26656)
	GRPC      int `json:"grpc"`       // gRPC server (default: 9090)
	GRPCWeb   int `json:"grpc_web"`   // gRPC-Web (default: 9091)
	API       int `json:"api"`        // REST API (default: 1317)
	EVMRPC    int `json:"evm_rpc"`    // EVM JSON-RPC (default: 8545)
	EVMSocket int `json:"evm_socket"` // EVM WebSocket (default: 8546)
}

// GenesisConfig contains genesis parameters for a network.
type GenesisConfig struct {
	// Chain identity
	ChainIDPattern string `json:"chain_id_pattern"` // e.g., "stable_{evmid}-1"
	EVMChainID     int64  `json:"evm_chain_id"`     // EVM chain ID

	// Token configuration
	BaseDenom     string `json:"base_denom"`     // e.g., "ustable"
	DenomExponent int    `json:"denom_exponent"` // Decimal places (typically 18)
	DisplayDenom  string `json:"display_denom"`  // Human-readable (e.g., "STABLE")

	// Staking parameters
	BondDenom         string        `json:"bond_denom"`
	MinSelfDelegation string        `json:"min_self_delegation"`
	UnbondingTime     time.Duration `json:"unbonding_time"`
	MaxValidators     uint32        `json:"max_validators"`

	// Governance parameters
	MinDeposit       string        `json:"min_deposit"`
	VotingPeriod     time.Duration `json:"voting_period"`
	MaxDepositPeriod time.Duration `json:"max_deposit_period"`

	// Distribution
	CommunityTax string `json:"community_tax"`
}

// ValidatorInfo represents validator information for genesis injection.
type ValidatorInfo struct {
	Moniker         string `json:"moniker"`
	ConsPubKey      string `json:"cons_pub_key"`      // Base64 encoded Ed25519 pubkey
	OperatorAddress string `json:"operator_address"`  // Bech32 valoper address
	SelfDelegation  string `json:"self_delegation"`   // Amount of tokens delegated
}

// GenesisOptions contains user-provided overrides for genesis modification.
type GenesisOptions struct {
	ChainID       string          `json:"chain_id,omitempty"`
	NumValidators int             `json:"num_validators,omitempty"`
	Validators    []ValidatorInfo `json:"validators,omitempty"`
}

// GeneratorConfig contains configuration for devnet generation.
type GeneratorConfig struct {
	NumValidators    int    `json:"num_validators"`
	NumAccounts      int    `json:"num_accounts"`
	AccountBalance   string `json:"account_balance"`   // JSON encoded coins
	ValidatorBalance string `json:"validator_balance"` // JSON encoded coins
	ValidatorStake   string `json:"validator_stake"`   // JSON encoded amount
	OutputDir        string `json:"output_dir"`
	ChainID          string `json:"chain_id"`
}
