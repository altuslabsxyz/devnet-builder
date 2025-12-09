package main

import (
	"fmt"
	"os"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/stablelabs/stable-devnet/internal/generator"
)

var (
	cfg    *generator.Config
	logger log.Logger
)

func NewBuildCmd() *cobra.Command {
	cfg = generator.DefaultConfig()
	logger = log.NewLogger(os.Stdout)

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
		RunE: runBuild,
	}

	// Flags for configuration
	cmd.Flags().IntVar(&cfg.NumValidators, "validators", cfg.NumValidators, "Number of validators to create")
	cmd.Flags().IntVar(&cfg.NumAccounts, "accounts", cfg.NumAccounts, "Number of dummy accounts to create")

	var accountBalanceStr string
	var validatorBalanceStr string
	var validatorStakeStr string

	cmd.Flags().StringVar(&accountBalanceStr, "account-balance", "1000000000000000000000astable,500000000000000000000agusdt", "Balance for each account (supports multiple denoms, e.g. \"1000astable,500agusdt\")")
	cmd.Flags().StringVar(&validatorBalanceStr, "validator-balance", "1000000000000000000000astable,500000000000000000000agusdt", "Balance for each validator (supports multiple denoms, e.g. \"1000astable,500agusdt\")")
	cmd.Flags().StringVar(&validatorStakeStr, "validator-stake", "100000000000000000000", "Stake amount for each validator (in astable)")
	cmd.Flags().StringVar(&cfg.OutputDir, "output", cfg.OutputDir, "Output directory for devnet files")
	cmd.Flags().StringVar(&cfg.ChainID, "chain-id", "", "Chain ID (defaults to from genesis)")

	// Parse balance strings in PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		var err error

		// Parse account balance (supports multiple denoms)
		cfg.AccountBalance, err = sdk.ParseCoinsNormalized(accountBalanceStr)
		if err != nil {
			return fmt.Errorf("invalid account-balance: %w (expected format: \"1000astable,500agusdt\")", err)
		}

		// Parse validator balance (supports multiple denoms)
		cfg.ValidatorBalance, err = sdk.ParseCoinsNormalized(validatorBalanceStr)
		if err != nil {
			return fmt.Errorf("invalid validator-balance: %w (expected format: \"1000astable,500agusdt\")", err)
		}

		// Parse validator stake (single denom only)
		var ok bool
		cfg.ValidatorStake, ok = math.NewIntFromString(validatorStakeStr)
		if !ok {
			return fmt.Errorf("invalid validator-stake: %s", validatorStakeStr)
		}

		return nil
	}

	return cmd
}

func runBuild(cmd *cobra.Command, args []string) error {
	genesisFile := args[0]

	// Check if genesis file exists
	if _, err := os.Stat(genesisFile); os.IsNotExist(err) {
		return fmt.Errorf("genesis file not found: %s", genesisFile)
	}

	// Log configuration parameters
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

	// Build the devnet
	gen := generator.NewDevnetGenerator(cfg, logger)
	if err := gen.Build(genesisFile); err != nil {
		return fmt.Errorf("failed to build devnet: %w", err)
	}

	logger.Info("Devnet created", "output", cfg.OutputDir)

	return nil
}
