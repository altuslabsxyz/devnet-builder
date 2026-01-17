package core

import (
	"fmt"
	"os"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
)

// buildOpts holds the configuration for the build command.
type buildOpts struct {
	numValidators       int
	numAccounts         int
	accountBalanceStr   string
	validatorBalanceStr string
	validatorStakeStr   string
	outputDir           string
	chainID             string
	networkName         string
}

// NewBuildCmd creates the build command.
func NewBuildCmd() *cobra.Command {
	opts := &buildOpts{
		numValidators:       4,
		numAccounts:         10,
		accountBalanceStr:   "1000000000000000000000astable,500000000000000000000agusdt",
		validatorBalanceStr: "1000000000000000000000astable,500000000000000000000agusdt",
		validatorStakeStr:   "100000000000000000000",
		outputDir:           "./devnet",
		networkName:         "stable",
	}

	cmd := &cobra.Command{
		Use:   "build [genesis-export.json]",
		Short: "Build a devnet from an exported genesis file",
		Long: `Build creates a complete local development network by:
  1. Loading the exported genesis file
  2. Replacing validators with new local validators
  3. Creating dummy accounts with specified balances
  4. Setting up directory structure for each validator node
  5. Creating keyring-backend test for validators and accounts

The output structure will be:
  devnet/
    node0/
      config/genesis.json
      config/priv_validator_key.json
      data/priv_validator_state.json
      keyring-test/  (validator wallet)
    node1/
      ...
    accounts/
      keyring-test/  (all account keys)
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(opts, args)
		},
	}

	// Flags for configuration
	cmd.Flags().IntVar(&opts.numValidators, "validators", opts.numValidators, "Number of validators to create")
	cmd.Flags().IntVar(&opts.numAccounts, "accounts", opts.numAccounts, "Number of dummy accounts to create")
	cmd.Flags().StringVar(&opts.accountBalanceStr, "account-balance", opts.accountBalanceStr, "Balance for each account (supports multiple denoms, e.g. \"1000astable,500agusdt\")")
	cmd.Flags().StringVar(&opts.validatorBalanceStr, "validator-balance", opts.validatorBalanceStr, "Balance for each validator (supports multiple denoms, e.g. \"1000astable,500agusdt\")")
	cmd.Flags().StringVar(&opts.validatorStakeStr, "validator-stake", opts.validatorStakeStr, "Stake amount for each validator (in astable)")
	cmd.Flags().StringVar(&opts.outputDir, "output", opts.outputDir, "Output directory for devnet files")
	cmd.Flags().StringVar(&opts.chainID, "chain-id", "", "Chain ID (defaults to from genesis)")
	cmd.Flags().StringVar(&opts.networkName, "network", opts.networkName, "Network module to use (e.g., stable, ault)")

	return cmd
}

func runBuild(opts *buildOpts, args []string) error {
	genesisFile := args[0]
	logger := log.NewLogger(os.Stdout)

	// Check if genesis file exists
	if _, err := os.Stat(genesisFile); os.IsNotExist(err) {
		return fmt.Errorf("genesis file not found: %s", genesisFile)
	}

	// Get the network module
	netModule, err := network.Get(opts.networkName)
	if err != nil {
		return fmt.Errorf("failed to get network module '%s': %w", opts.networkName, err)
	}

	// Get default config from network module and apply overrides
	cfg := netModule.DefaultGeneratorConfig()
	cfg.NumValidators = opts.numValidators
	cfg.NumAccounts = opts.numAccounts
	cfg.OutputDir = opts.outputDir
	if opts.chainID != "" {
		cfg.ChainID = opts.chainID
	}

	// Parse account balance (supports multiple denoms)
	cfg.AccountBalance, err = sdk.ParseCoinsNormalized(opts.accountBalanceStr)
	if err != nil {
		return fmt.Errorf("invalid account-balance: %w (expected format: \"1000astable,500agusdt\")", err)
	}

	// Parse validator balance (supports multiple denoms)
	cfg.ValidatorBalance, err = sdk.ParseCoinsNormalized(opts.validatorBalanceStr)
	if err != nil {
		return fmt.Errorf("invalid validator-balance: %w (expected format: \"1000astable,500agusdt\")", err)
	}

	// Parse validator stake (single denom only)
	var ok bool
	cfg.ValidatorStake, ok = math.NewIntFromString(opts.validatorStakeStr)
	if !ok {
		return fmt.Errorf("invalid validator-stake: %s", opts.validatorStakeStr)
	}

	// Log configuration parameters
	logger.Info("Network", "module", opts.networkName)
	logger.Info("Genesis file", "path", genesisFile)
	logger.Info("Validators", "count", cfg.NumValidators)
	logger.Info("Accounts", "count", cfg.NumAccounts)
	logger.Info("Account balance", "balance", cfg.AccountBalance.String())
	logger.Info("Validator balance", "balance", cfg.ValidatorBalance.String())
	logger.Info("Validator stake", "stake", cfg.ValidatorStake.String())
	logger.Info("Output directory", "path", cfg.OutputDir)
	if cfg.ChainID != "" {
		logger.Info("Chain ID", "id", cfg.ChainID)
	}

	// Build the devnet using network module's generator
	gen, err := netModule.NewGenerator(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	if err := gen.Build(genesisFile); err != nil {
		return fmt.Errorf("failed to build devnet: %w", err)
	}

	logger.Info("Devnet created", "output", cfg.OutputDir)

	return nil
}
