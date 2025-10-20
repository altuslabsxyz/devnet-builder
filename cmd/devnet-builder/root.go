package main

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devnet-builder",
		Short: "Build a local devnet with multiple validators and accounts",
		Long: `devnet-builder creates a complete local development network with:
  - Multiple validators (configurable)
  - Dummy accounts with balances (configurable)
  - Keyring-backend test for easy development
  - Ready-to-use directory structure for each validator node

Example:
  devnet-builder genesis_export.json --validators 4 --accounts 10 --balance 1000000000000`,
	}

	cmd.AddCommand(
		NewBuildCmd(),
	)

	return cmd
}
