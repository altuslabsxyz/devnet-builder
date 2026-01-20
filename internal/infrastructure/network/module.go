// Package network provides the network module abstraction for supporting
// multiple Cosmos SDK networks in devnet-builder.
package network

import (
	"time"

	"cosmossdk.io/log"

	sdknetwork "github.com/altuslabsxyz/devnet-builder/pkg/network"
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

	// GetBuildConfig returns network-specific build configuration.
	// Parameters:
	//   - networkType: Target network type ("mainnet", "testnet", "devnet", etc.)
	// Returns:
	//   - BuildConfig with custom build configuration
	//   - error if network type is not supported
	// If the module doesn't need custom build configuration, return empty BuildConfig{}, nil
	GetBuildConfig(networkType string) (*sdknetwork.BuildConfig, error)
}

// ChainConfig provides chain-specific configuration.
type ChainConfig interface {
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
	NodeConfigurator
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

	// Validators are the validators to add to genesis (with keys already generated)
	Validators []GenesisValidatorInfo

	// GovParams overrides governance parameters
	GovParams *GovParamsOverride

	// StakingParams overrides staking parameters
	StakingParams *StakingParamsOverride
}

// GenesisValidatorInfo contains validator info for genesis modification.
// This is used when validators are pre-generated and need to be injected into genesis.
type GenesisValidatorInfo struct {
	// Moniker is the validator's display name
	Moniker string

	// ConsPubKey is the base64-encoded Ed25519 consensus public key
	ConsPubKey string

	// OperatorAddress is the bech32-encoded valoper address
	OperatorAddress string

	// SelfDelegation is the amount of tokens to self-delegate
	SelfDelegation string
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

// =============================================================================
// Node Configuration Interface
// =============================================================================

// NodeConfigurator provides node configuration capabilities.
// This allows network modules to provide configuration overrides
// that will be merged with the default config.toml and app.toml.
type NodeConfigurator interface {
	// GetConfigOverrides returns TOML configuration overrides for a node.
	// The returned bytes are partial TOML that will be merged with the
	// default configs generated by node init.
	// Parameters:
	//   - nodeIndex: Index of the node (0, 1, 2, ...)
	//   - opts: Configuration options including ports and peers
	// Returns:
	//   - configToml: Partial config.toml overrides (can be nil if no overrides)
	//   - appToml: Partial app.toml overrides (can be nil if no overrides)
	//   - error: Error if generation fails
	GetConfigOverrides(nodeIndex int, opts NodeConfigOptions) (configToml []byte, appToml []byte, err error)
}

// NodeConfigOptions contains options for node configuration.
type NodeConfigOptions struct {
	// ChainID is the chain identifier
	ChainID string

	// Ports is the port configuration for this node
	Ports PortConfig

	// PersistentPeers is the persistent peers string (node_id@host:port,...)
	PersistentPeers string

	// NumValidators is the total number of validators
	NumValidators int

	// IsValidator indicates if this node is a validator
	IsValidator bool

	// Moniker is the node's moniker/name
	Moniker string
}

// =============================================================================
// State Export Interface (Optional)
// =============================================================================

// StateExporter is an optional interface that network modules can implement
// to provide state export functionality for snapshot-based genesis creation.
// If a module implements this interface, the devnet provisioning can use
// snapshots to create devnets with real on-chain state.
type StateExporter interface {
	// ExportCommandWithOptions returns arguments for exporting genesis/state
	// with the given export options.
	// Parameters: homeDir, opts
	// Returns: e.g., ["export", "--home", "/path", "--for-zero-height"]
	ExportCommandWithOptions(homeDir string, opts ExportOptions) []string

	// ValidateExportedGenesis validates the exported genesis for this network.
	// This performs network-specific validation (e.g., checking for EVM modules).
	ValidateExportedGenesis(genesis []byte) error

	// RequiredModules returns the list of modules that must be present
	// in an exported genesis for it to be valid.
	RequiredModules() []string

	// SnapshotFormat returns the expected snapshot archive format for this network.
	// Parameters: networkType ("mainnet", "testnet")
	// Returns: SnapshotFormat (e.g., SnapshotFormatTarLz4)
	SnapshotFormat(networkType string) SnapshotFormat
}

// ExportOptions defines options for genesis state export.
type ExportOptions struct {
	// ForZeroHeight resets the chain height to 0 for the exported genesis.
	ForZeroHeight bool

	// JailWhitelist is a list of validator operator addresses that should
	// not be jailed in the exported genesis.
	JailWhitelist []string

	// ModulesToSkip is a list of module names to skip during export.
	ModulesToSkip []string

	// Height specifies a specific height to export from.
	// If 0, exports from the latest height.
	Height int64

	// OutputPath is the path to write the exported genesis.
	// If empty, genesis is written to stdout.
	OutputPath string
}

// SnapshotFormat defines the archive format for snapshots.
type SnapshotFormat string

const (
	SnapshotFormatTarLz4 SnapshotFormat = "tar.lz4"
	SnapshotFormatTarZst SnapshotFormat = "tar.zst"
	SnapshotFormatTarGz  SnapshotFormat = "tar.gz"
)

// FileBasedGenesisModifier is an optional interface for handling large genesis files.
// When genesis files exceed gRPC message size limits (default 4MB), this interface
// allows passing file paths instead of raw bytes over gRPC.
//
// This is particularly useful for fork-based devnets where exported genesis
// can be 50-100+ MB in size.
type FileBasedGenesisModifier interface {
	// ModifyGenesisFile modifies a genesis file at inputPath and writes to outputPath.
	// This is the file-based equivalent of GenesisModifier.ModifyGenesis() for large files.
	//
	// Parameters:
	//   - inputPath: Path to the input genesis.json file
	//   - outputPath: Path where the modified genesis should be written
	//   - opts: Genesis modification options (chain_id, validators, etc.)
	//
	// Returns:
	//   - outputSize: Size of the output file in bytes
	//   - error: Any error that occurred during modification
	ModifyGenesisFile(inputPath, outputPath string, opts GenesisOptions) (outputSize int64, err error)
}
