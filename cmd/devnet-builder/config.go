package main

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmostypes "github.com/cosmos/evm/types"

	appcfg "github.com/stablelabs/stable/app/config"
)

// DevnetConfig holds the configuration for building a devnet
type DevnetConfig struct {
	// Number of validators to create
	NumValidators int

	// Number of dummy accounts to create
	NumAccounts int

	// Balance for each dummy account (supports multiple denoms)
	// Format: "1000000000000000000000astable,500000000000000000000agusdt"
	AccountBalance sdk.Coins

	// Balance for each validator account (supports multiple denoms)
	// Format: "1000000000000000000000astable,500000000000000000000agusdt"
	ValidatorBalance sdk.Coins

	// Validator stake amount (in base denom only)
	ValidatorStake math.Int

	// Output directory for devnet files
	OutputDir string

	// Chain ID
	ChainID string
}

// DefaultDevnetConfig returns default configuration
func DefaultDevnetConfig() *DevnetConfig {
	defaultAmount := sdk.TokensFromConsensusPower(5000, evmostypes.AttoPowerReduction) // 5000 consensus power
	return &DevnetConfig{
		NumValidators:    4,
		NumAccounts:      10,
		AccountBalance:   sdk.NewCoins(sdk.NewCoin(appcfg.GovAttoDenom, defaultAmount)),    // Default: only astable
		ValidatorBalance: sdk.NewCoins(sdk.NewCoin(appcfg.GovAttoDenom, defaultAmount)),    // Default: only astable
		ValidatorStake:   sdk.TokensFromConsensusPower(100, evmostypes.AttoPowerReduction), // 100 consensus power
		OutputDir:        "./devnet",
		ChainID:          "stabledevnet_2200-1",
	}
}
