// Package network provides the network module abstraction for supporting
// multiple Cosmos SDK networks in devnet-builder.
package network

import (
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Generator is the interface for network-specific devnet generators.
// Each network module provides its own implementation to handle
// network-specific keyring options, denoms, and app configuration.
type Generator interface {
	// Build generates validators, modifies genesis, and saves to node directories.
	// Parameters:
	//   - genesisFile: Path to the exported genesis file
	// Returns: Error if generation fails
	Build(genesisFile string) error

	// GetValidators returns the generated validators info.
	GetValidators() []ValidatorInfo

	// GetAccounts returns the generated accounts info.
	GetAccounts() []AccountInfo
}

// GeneratorConfig holds the configuration for building a devnet.
// This is the network-agnostic configuration passed to generators.
type GeneratorConfig struct {
	// Number of validators to create
	NumValidators int

	// Number of dummy accounts to create
	NumAccounts int

	// Balance for each dummy account (supports multiple denoms)
	AccountBalance sdk.Coins

	// Balance for each validator account (supports multiple denoms)
	ValidatorBalance sdk.Coins

	// Validator stake amount (in base denom only)
	ValidatorStake math.Int

	// Output directory for devnet files
	OutputDir string

	// Chain ID
	ChainID string
}

// ValidatorInfo contains information about a generated validator.
type ValidatorInfo struct {
	Moniker        string
	AccountAddress sdk.AccAddress
	Tokens         math.Int
}

// AccountInfo contains information about a generated account.
type AccountInfo struct {
	Name    string
	Address sdk.AccAddress
}

// GeneratorFactory creates a Generator for a specific network.
// Networks implement this to provide their own generator.
type GeneratorFactory interface {
	// NewGenerator creates a new generator with the given configuration.
	// Parameters:
	//   - config: Generator configuration
	//   - logger: Logger for output
	// Returns: Generator instance, or error if creation fails
	NewGenerator(config *GeneratorConfig, logger log.Logger) (Generator, error)
}
