//go:build private

package ault

import (
	"fmt"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stablelabs/stable-devnet/internal/network"
)

// NewGenerator creates a new generator for the Ault network.
// TODO: Implement ault-specific generator when ault dependencies are available.
func (m *Module) NewGenerator(config *network.GeneratorConfig, logger log.Logger) (network.Generator, error) {
	return nil, fmt.Errorf("ault network generator not yet implemented - use stable network for now")
}

// DefaultGeneratorConfig returns the default generator configuration for Ault network.
func (m *Module) DefaultGeneratorConfig() *network.GeneratorConfig {
	// Default values for Ault network
	// TODO: Update with ault-specific denoms and amounts when available
	accountBalance, _ := math.NewIntFromString("1000000000000000000")
	validatorBalance, _ := math.NewIntFromString("100000000000000000000")
	validatorStake, _ := math.NewIntFromString("100000000000000000000")

	return &network.GeneratorConfig{
		NumValidators:    4,
		NumAccounts:      10,
		AccountBalance:   sdk.NewCoins(sdk.NewCoin("aault", accountBalance)),
		ValidatorBalance: sdk.NewCoins(sdk.NewCoin("aault", validatorBalance)),
		ValidatorStake:   validatorStake, // 100 consensus power
		OutputDir:        "./devnet",
		ChainID:          "aultdevnet_900-1",
	}
}
