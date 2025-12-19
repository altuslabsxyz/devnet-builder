//go:build !private

package stable

import (
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stablelabs/stable-devnet/internal/network"
)

// NewGenerator creates a stub generator that returns an error.
// This is for non-private builds that don't have access to stablelabs packages.
func (m *Module) NewGenerator(config *network.GeneratorConfig, logger log.Logger) (network.Generator, error) {
	return nil, network.ErrPrivateBuildRequired
}

// DefaultGeneratorConfig returns the default generator configuration.
// This is a minimal stub for non-private builds.
func (m *Module) DefaultGeneratorConfig() *network.GeneratorConfig {
	return &network.GeneratorConfig{
		NumValidators:    4,
		NumAccounts:      10,
		AccountBalance:   sdk.NewCoins(),
		ValidatorBalance: sdk.NewCoins(),
		ValidatorStake:   math.ZeroInt(),
		OutputDir:        "./devnet",
		ChainID:          "stabledevnet_2200-1",
	}
}
