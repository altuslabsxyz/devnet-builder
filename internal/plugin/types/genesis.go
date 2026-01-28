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

// GenesisPatchOptions specifies modifications to apply to genesis
type GenesisPatchOptions struct {
	ChainID       string        // new chain ID for the forked network
	VotingPeriod  time.Duration // governance voting period (e.g., 30s for devnet)
	UnbondingTime time.Duration // staking unbonding time (e.g., 60s for devnet)
	InflationRate string        // inflation rate (e.g., "0.0" for no inflation)
	MinGasPrice   string        // minimum gas price
	BinaryVersion string        // binary version/ref used for genesis modification (e.g., "v1.0.0" or commit hash)
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
