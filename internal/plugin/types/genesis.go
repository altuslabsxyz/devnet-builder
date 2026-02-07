// internal/plugin/types/genesis.go
package types

import (
	"time"
)

// GenesisMode specifies how to obtain genesis
type GenesisMode string

const (
	// GenesisModeRPC fetches genesis directly from RPC endpoint
	GenesisModeRPC GenesisMode = "rpc"
	// GenesisModeSnapshot downloads snapshot and exports genesis from state
	GenesisModeSnapshot GenesisMode = "snapshot"
	// GenesisModeLocal uses a local genesis file
	GenesisModeLocal GenesisMode = "local"
	// GenesisModeFresh generates a fresh genesis (no forking)
	GenesisModeFresh GenesisMode = "fresh"
)

// GenesisSource specifies where to get genesis from
type GenesisSource struct {
	Mode        GenesisMode
	RPCURL      string // for RPC mode
	SnapshotURL string // for snapshot mode
	LocalPath   string // for local mode
	NetworkType string // e.g., "mainnet", "testnet"
}

// ValidatorInfo represents validator information for genesis injection.
type ValidatorInfo struct {
	Moniker         string // validator display name
	ConsPubKey      string // Base64 encoded Ed25519 consensus pubkey
	OperatorAddress string // Bech32 valoper address
	SelfDelegation  string // amount of tokens to self-delegate
}

// GenesisPatchOptions specifies modifications to apply to genesis
type GenesisPatchOptions struct {
	ChainID       string          // new chain ID for the forked network
	VotingPeriod  time.Duration   // governance voting period (e.g., 30s for devnet)
	UnbondingTime time.Duration   // staking unbonding time (e.g., 60s for devnet)
	InflationRate string // inflation rate (e.g., "0.0" for no inflation)
	// MinGasPrice is the minimum gas price for node configuration.
	// NOTE: This is applied via app.toml node configuration, not genesis patching.
	// It is preserved here for completeness but not consumed by PatchGenesis.
	MinGasPrice   string          // minimum gas price (applied via app.toml, not genesis)
	BinaryVersion string          // binary version/ref used for genesis modification (e.g., "v1.0.0" or commit hash)
	// Validators contains validator entries for genesis.
	// NOTE: Validator injection is handled by the provisioner/generator layer,
	// not by PatchGenesis. This field is passed through for reference.
	Validators []ValidatorInfo // validator entries (injected by provisioner, not PatchGenesis)
}

// DefaultDevnetPatchOptions returns patch options suitable for local devnets
func DefaultDevnetPatchOptions(chainID string) GenesisPatchOptions {
	return GenesisPatchOptions{
		ChainID:       chainID,
		VotingPeriod:  30 * time.Second,
		UnbondingTime: 60 * time.Second,
		InflationRate: "0.0",
		MinGasPrice:   "0",
	}
}

// PluginGenesis handles network-specific genesis operations
type PluginGenesis interface {
	// GetRPCEndpoint returns the default RPC endpoint for a network type
	GetRPCEndpoint(networkType string) string

	// GetSnapshotURL returns the snapshot URL for a network type
	GetSnapshotURL(networkType string) string

	// ValidateGenesis validates genesis for this network
	ValidateGenesis(genesis []byte) error

	// PatchGenesis applies network-specific modifications to genesis
	// This is called after the generic patches from GenesisPatchOptions
	PatchGenesis(genesis []byte, opts GenesisPatchOptions) ([]byte, error)

	// ExportCommandArgs returns the command args for exporting genesis from snapshot
	// The binary path will be prepended by the caller
	ExportCommandArgs(homeDir string) []string

	// BinaryName returns the binary name for this network
	BinaryName() string
}

// FileBasedPluginGenesis extends PluginGenesis with file-based operations
// for handling large genesis files that exceed gRPC message size limits.
type FileBasedPluginGenesis interface {
	PluginGenesis

	// PatchGenesisFile applies network-specific modifications to a genesis file.
	// This is used for large genesis files that exceed gRPC message size limits.
	// inputPath: path to the input genesis file
	// outputPath: path where the modified genesis should be written
	// opts: patch options to apply
	// Returns the size of the output file in bytes, or an error.
	PatchGenesisFile(inputPath, outputPath string, opts GenesisPatchOptions) (int64, error)
}
