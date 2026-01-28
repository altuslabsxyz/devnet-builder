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
//	    "github.com/altuslabsxyz/devnet-builder/pkg/network"
//	    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
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

	// GetBuildConfig returns network-specific build configuration for binary compilation.
	// This method enables plugins to customize build tags, linker flags, and environment
	// variables for different network types (mainnet, testnet, devnet).
	//
	// Parameters:
	//   - networkType: Target network type ("mainnet", "testnet", "devnet", etc.)
	//
	// Returns:
	//   - BuildConfig with custom build configuration
	//   - error if network type is not supported
	//
	// Example:
	//   cfg, err := module.GetBuildConfig("testnet")
	//   if err != nil {
	//       return err
	//   }
	//   // cfg.LDFlags might contain: ["-X github.com/example/app.EVMChainID=2201"]
	//
	// If the plugin doesn't need network-specific build configuration, it should
	// return an empty BuildConfig{} (not nil) to indicate no custom configuration.
	//
	// This is called during binary building to inject network-specific values
	// (like chain IDs) at compile-time. The returned config is merged with
	// default build configurations before being passed to the build tool.
	GetBuildConfig(networkType string) (*BuildConfig, error)

	// ============================================
	// Chain Configuration
	// ============================================

	// DefaultChainID returns the default chain ID for the devnet.
	//
	// Deprecated: Chain ID is extracted from genesis files and stored in metadata.
	// This method will be removed in v2.0.0. Return empty string for new plugins.
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

	// ============================================
	// Node Configuration
	// ============================================

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
	ConsPubKey      string `json:"cons_pub_key"`     // Base64 encoded Ed25519 pubkey
	OperatorAddress string `json:"operator_address"` // Bech32 valoper address
	SelfDelegation  string `json:"self_delegation"`  // Amount of tokens delegated
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

// NodeConfigOptions contains options for node configuration.
// This is passed to ConfigureNode to customize config.toml and app.toml.
type NodeConfigOptions struct {
	// ChainID is the chain identifier
	ChainID string `json:"chain_id"`

	// Ports is the port configuration for this node
	Ports PortConfig `json:"ports"`

	// PersistentPeers is the persistent peers string (node_id@host:port,...)
	PersistentPeers string `json:"persistent_peers"`

	// NumValidators is the total number of validators
	NumValidators int `json:"num_validators"`

	// IsValidator indicates if this node is a validator
	IsValidator bool `json:"is_validator"`

	// Moniker is the node's moniker/name
	Moniker string `json:"moniker"`
}

// StateExporter is an optional interface that network modules can implement
// to provide state export functionality for snapshot-based genesis creation.
// If a module implements this interface, the devnet provisioning can use
// snapshots to create devnets with real on-chain state.
type StateExporter interface {
	// ExportCommandWithOptions returns arguments for exporting genesis/state
	// with the given export options.
	// Parameters: homeDir, opts
	// Returns: e.g., ["export", "--home", "/path"]
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
	ForZeroHeight bool `json:"for_zero_height"`

	// JailWhitelist is a list of validator operator addresses that should
	// not be jailed in the exported genesis.
	JailWhitelist []string `json:"jail_whitelist,omitempty"`

	// ModulesToSkip is a list of module names to skip during export.
	ModulesToSkip []string `json:"modules_to_skip,omitempty"`

	// Height specifies a specific height to export from.
	// If 0, exports from the latest height.
	Height int64 `json:"height"`

	// OutputPath is the path to write the exported genesis.
	// If empty, genesis is written to stdout.
	OutputPath string `json:"output_path,omitempty"`
}

// SnapshotFormat defines the archive format for snapshots.
type SnapshotFormat string

const (
	SnapshotFormatTarLz4 SnapshotFormat = "tar.lz4"
	SnapshotFormatTarZst SnapshotFormat = "tar.zst"
	SnapshotFormatTarGz  SnapshotFormat = "tar.gz"
)

// Extension returns the file extension for the snapshot format.
func (f SnapshotFormat) Extension() string {
	switch f {
	case SnapshotFormatTarLz4:
		return ".tar.lz4"
	case SnapshotFormatTarZst:
		return ".tar.zst"
	case SnapshotFormatTarGz:
		return ".tar.gz"
	default:
		return ".tar"
	}
}

// SnapshotInfo contains information about a snapshot.
type SnapshotInfo struct {
	// URL is the download URL for the snapshot archive.
	URL string `json:"url"`

	// Format is the archive format.
	Format SnapshotFormat `json:"format"`

	// Height is the block height of the snapshot.
	Height int64 `json:"height"`

	// Hash is the hash of the snapshot archive for verification.
	Hash string `json:"hash,omitempty"`

	// Size is the size of the snapshot in bytes.
	Size int64 `json:"size"`
}

// FileBasedGenesisModifier is an optional interface for handling large genesis files.
// When genesis files exceed gRPC message size limits (default 4MB), this interface
// allows passing file paths instead of raw bytes over gRPC.
//
// Implementations should:
//  1. Read genesis from inputPath
//  2. Apply modifications based on opts
//  3. Write modified genesis to outputPath
//
// This is particularly useful for fork-based devnets where exported genesis
// can be 50-100+ MB in size.
type FileBasedGenesisModifier interface {
	// ModifyGenesisFile modifies a genesis file at inputPath and writes to outputPath.
	// This is the file-based equivalent of Module.ModifyGenesis() for large genesis files.
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
