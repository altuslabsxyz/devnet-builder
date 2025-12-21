// Package network provides the network module abstraction for supporting
// multiple Cosmos SDK networks in devnet-builder.
package network

import (
	"time"

	"cosmossdk.io/log"
)

// =============================================================================
// Interface Segregation: Split NetworkModule into focused interfaces
// =============================================================================

// NetworkIdentity provides basic identification for a network module.
type NetworkIdentity interface {
	// Name returns the unique identifier for this network module.
	// Example: "stable", "ault"
	// Constraints: lowercase, alphanumeric with hyphens, must be unique
	Name() string

	// DisplayName returns a human-readable name for the network.
	// Example: "Stable Network", "Ault Blockchain"
	DisplayName() string

	// Version returns the module version for compatibility checking.
	// Should follow semantic versioning (e.g., "1.0.0")
	Version() string
}

// BinaryProvider provides binary acquisition configuration.
type BinaryProvider interface {
	// BinaryName returns the name of the network's CLI binary.
	// Example: "stabled", "aultd"
	BinaryName() string

	// BinarySource returns the configuration for binary acquisition.
	// Used by the builder to download or locate the network binary.
	BinarySource() BinarySource

	// DefaultBinaryVersion returns the default version to use if not specified.
	// Example: "v1.1.3", "latest"
	DefaultBinaryVersion() string
}

// ChainConfig provides chain-specific configuration.
type ChainConfig interface {
	// DefaultChainID returns the default chain identifier for devnet.
	// Example: "stable_9000-1", "ault_20904-1"
	DefaultChainID() string

	// Bech32Prefix returns the address prefix for this network.
	// Example: "stable", "ault"
	// Used for account address generation and validation.
	Bech32Prefix() string

	// BaseDenom returns the base token denomination.
	// Example: "ustable", "aault"
	BaseDenom() string

	// GenesisConfig returns the default genesis configuration.
	// Contains staking, governance, and other chain parameters.
	GenesisConfig() GenesisConfig
}

// DockerConfig provides Docker-related configuration.
type DockerConfig interface {
	// DockerImage returns the Docker image name for this network.
	// Example: "ghcr.io/stablelabs/stabled"
	DockerImage() string

	// DockerImageTag returns the Docker tag for a given version.
	// Allows networks to customize version-to-tag mapping.
	// Example: version "v1.0.0" might map to tag "1.0.0" or "v1.0.0"
	DockerImageTag(version string) string

	// DockerHomeDir returns the home directory path inside Docker containers.
	// Example: "/home/stabled", "/home/aultd"
	// Used for Docker volume mounts and environment configuration.
	DockerHomeDir() string
}

// CommandBuilder provides command generation for node operations.
type CommandBuilder interface {
	// InitCommand returns the command arguments for initializing a node.
	// Parameters:
	//   - homeDir: Node home directory path
	//   - chainID: Chain identifier
	//   - moniker: Node moniker/name
	// Returns: Command arguments (e.g., ["init", "node0", "--chain-id", "..."])
	InitCommand(homeDir string, chainID string, moniker string) []string

	// StartCommand returns the command arguments for starting a node.
	// Parameters:
	//   - homeDir: Node home directory path
	// Returns: Command arguments (e.g., ["start", "--home", homeDir])
	StartCommand(homeDir string) []string

	// ExportCommand returns the command arguments for exporting genesis/state.
	// Parameters:
	//   - homeDir: Node home directory path
	// Returns: Command arguments (e.g., ["export", "--home", homeDir])
	ExportCommand(homeDir string) []string
}

// ProcessConfig provides process management configuration.
type ProcessConfig interface {
	// DefaultNodeHome returns the default node home directory path.
	// Example: "/root/.stabled", "/home/ault/.aultd"
	// Used for Docker containers and process management.
	DefaultNodeHome() string

	// PIDFileName returns the process ID file name for this network.
	// Example: "stabled.pid", "aultd.pid"
	// Used for local process management.
	PIDFileName() string

	// LogFileName returns the log file name for this network.
	// Example: "stabled.log", "aultd.log"
	// Used for local process logging.
	LogFileName() string

	// ProcessPattern returns the regex pattern to match running processes.
	// Example: "stabled.*start", "aultd.*start"
	// Used for process killing during upgrades.
	ProcessPattern() string

	// DefaultPorts returns the default port configuration for this network.
	// Used for node configuration and health checks.
	DefaultPorts() PortConfig
}

// GenesisModifier provides genesis file modification capabilities.
type GenesisModifier interface {
	// ModifyGenesis applies network-specific modifications to a genesis file.
	// Parameters:
	//   - genesis: Raw genesis JSON bytes
	//   - opts: User-provided customization options
	// Returns: Modified genesis JSON bytes, or error
	ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error)
}

// DevnetGenerator provides devnet generation capabilities.
type DevnetGenerator interface {
	// NewGenerator creates a new devnet generator for this network.
	// Parameters:
	//   - config: Generator configuration with validator/account counts, balances, etc.
	//   - logger: Logger for output
	// Returns: Generator instance, or error if creation fails
	NewGenerator(config *GeneratorConfig, logger log.Logger) (Generator, error)

	// DefaultGeneratorConfig returns the default generator configuration for this network.
	// This includes network-specific defaults for denoms, balances, and stake amounts.
	DefaultGeneratorConfig() *GeneratorConfig
}

// Validator provides validation capabilities.
type Validator interface {
	// Validate checks if the module configuration is valid.
	// Called during registration and before use.
	// Returns error describing any configuration issues.
	Validate() error
}

// SnapshotProvider provides network-specific snapshot and RPC configuration.
// This allows each network module to define its own snapshot URLs and RPC endpoints.
type SnapshotProvider interface {
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

// =============================================================================
// NetworkModule: Composite interface for full network module functionality
// =============================================================================

// NetworkModule defines the complete interface that all network implementations must satisfy.
// It composes all the segregated interfaces for full network support.
//
// For consumers that only need partial functionality, prefer using the specific
// interfaces (NetworkIdentity, BinaryProvider, etc.) to reduce coupling.
type NetworkModule interface {
	NetworkIdentity
	BinaryProvider
	ChainConfig
	DockerConfig
	CommandBuilder
	ProcessConfig
	GenesisModifier
	DevnetGenerator
	Validator
	SnapshotProvider
}

// =============================================================================
// Configuration Types
// =============================================================================

// GenesisConfig contains default genesis parameters for a network.
type GenesisConfig struct {
	// Chain identity
	ChainIDPattern string // e.g., "stable_{evmid}-1"
	EVMChainID     int64  // EVM chain ID (e.g., 9000 for stable devnet)

	// Token configuration
	BaseDenom     string // e.g., "ustable", "aault"
	DenomExponent int    // Decimal places (typically 18)
	DisplayDenom  string // Human-readable denom (e.g., "STABLE", "AULT")

	// Staking parameters
	BondDenom         string        // Staking token denom
	MinSelfDelegation string        // Minimum self-delegation amount
	UnbondingTime     time.Duration // Unbonding period
	MaxValidators     uint32        // Maximum active validators

	// Governance parameters
	MinDeposit       string        // Minimum proposal deposit
	VotingPeriod     time.Duration // Voting duration
	MaxDepositPeriod time.Duration // Deposit window

	// Bank/distribution
	CommunityTax string // Community tax rate (e.g., "0.02")
}

// GenesisOptions contains user-provided overrides for genesis configuration.
type GenesisOptions struct {
	// ChainID overrides the default chain ID
	ChainID string

	// Accounts are additional genesis accounts to create
	Accounts []GenesisAccount

	// NumValidators is the number of validators to create (default: 1)
	NumValidators int

	// GovParams overrides governance parameters
	GovParams *GovParamsOverride

	// StakingParams overrides staking parameters
	StakingParams *StakingParamsOverride
}

// GenesisAccount represents an account to add to genesis.
type GenesisAccount struct {
	// Address is the bech32 account address
	Address string

	// Coins are the initial balances
	Coins []Coin

	// Mnemonic is an optional BIP39 mnemonic for deterministic key derivation
	Mnemonic string

	// Index is the HD derivation path index (default: 0)
	Index int
}

// Coin represents a token amount.
type Coin struct {
	Denom  string
	Amount string
}

// GovParamsOverride contains governance parameter overrides.
type GovParamsOverride struct {
	VotingPeriod     *time.Duration
	MinDeposit       *string
	MaxDepositPeriod *time.Duration
}

// StakingParamsOverride contains staking parameter overrides.
type StakingParamsOverride struct {
	UnbondingTime     *time.Duration
	MaxValidators     *uint32
	MinSelfDelegation *string
}
