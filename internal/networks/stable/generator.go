//go:build private

package stable

import (
	"cosmossdk.io/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmostypes "github.com/cosmos/evm/types"

	appcfg "github.com/stablelabs/stable/app/config"

	"github.com/stablelabs/stable-devnet/internal/generator"
	"github.com/stablelabs/stable-devnet/internal/network"
)

// stableGenerator wraps the DevnetGenerator to implement the network.Generator interface.
type stableGenerator struct {
	gen *generator.DevnetGenerator
}

// NewGenerator creates a new generator for the Stable network.
func (m *Module) NewGenerator(config *network.GeneratorConfig, logger log.Logger) (network.Generator, error) {
	// Convert network.GeneratorConfig to generator.Config
	genConfig := &generator.Config{
		NumValidators:    config.NumValidators,
		NumAccounts:      config.NumAccounts,
		AccountBalance:   config.AccountBalance,
		ValidatorBalance: config.ValidatorBalance,
		ValidatorStake:   config.ValidatorStake,
		OutputDir:        config.OutputDir,
		ChainID:          config.ChainID,
	}

	gen := generator.NewDevnetGenerator(genConfig, logger)
	return &stableGenerator{gen: gen}, nil
}

// DefaultGeneratorConfig returns the default generator configuration for Stable network.
func (m *Module) DefaultGeneratorConfig() *network.GeneratorConfig {
	// Validator balance: 100000 STABLE (enough for expedited proposal deposit of 50001)
	defaultAmount := sdk.TokensFromConsensusPower(100000, evmostypes.AttoPowerReduction)
	// Gas token (agusdt) needed for transaction fees
	// 10000 * 10^18 agusdt = 10000 gusdt (plenty for tx fees)
	defaultGasAmount := sdk.TokensFromConsensusPower(10000, evmostypes.AttoPowerReduction)
	// Account balance: 10000 STABLE
	accountAmount := sdk.TokensFromConsensusPower(10000, evmostypes.AttoPowerReduction)

	return &network.GeneratorConfig{
		NumValidators: 4,
		NumAccounts:   10,
		AccountBalance: sdk.NewCoins(
			sdk.NewCoin(appcfg.GovAttoDenom, accountAmount),
			sdk.NewCoin(appcfg.GasAttoDenom, defaultGasAmount),
		),
		ValidatorBalance: sdk.NewCoins(
			sdk.NewCoin(appcfg.GovAttoDenom, defaultAmount),
			sdk.NewCoin(appcfg.GasAttoDenom, defaultGasAmount),
		),
		ValidatorStake: sdk.TokensFromConsensusPower(100, evmostypes.AttoPowerReduction), // 100 consensus power
		OutputDir:      "./devnet",
		ChainID:        "stabledevnet_2200-1",
	}
}

// Build generates validators, modifies genesis, and saves to node directories.
func (g *stableGenerator) Build(genesisFile string) error {
	return g.gen.Build(genesisFile)
}

// GetValidators returns the generated validators info.
func (g *stableGenerator) GetValidators() []network.ValidatorInfo {
	validators := g.gen.GetValidators()
	result := make([]network.ValidatorInfo, len(validators))
	for i, v := range validators {
		result[i] = network.ValidatorInfo{
			Moniker:        v.Moniker,
			AccountAddress: v.AccountAddress,
			Tokens:         v.Tokens,
		}
	}
	return result
}

// GetAccounts returns the generated accounts info.
func (g *stableGenerator) GetAccounts() []network.AccountInfo {
	accounts := g.gen.GetAccounts()
	result := make([]network.AccountInfo, len(accounts))
	for i, a := range accounts {
		result[i] = network.AccountInfo{
			Name:    a.Name,
			Address: a.Address,
		}
	}
	return result
}
