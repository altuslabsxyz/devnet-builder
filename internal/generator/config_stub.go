//go:build !private

package generator

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Config holds the configuration for building a devnet.
// This is a stub for non-private builds.
type Config struct {
	NumValidators    int
	NumAccounts      int
	AccountBalance   sdk.Coins
	ValidatorBalance sdk.Coins
	ValidatorStake   math.Int
	OutputDir        string
	ChainID          string
}

// DefaultConfig returns default configuration.
// This is a stub that returns minimal defaults for non-private builds.
func DefaultConfig() *Config {
	return &Config{
		NumValidators:    4,
		NumAccounts:      10,
		AccountBalance:   sdk.NewCoins(),
		ValidatorBalance: sdk.NewCoins(),
		ValidatorStake:   math.ZeroInt(),
		OutputDir:        "./devnet",
		ChainID:          "devnet-1",
	}
}
