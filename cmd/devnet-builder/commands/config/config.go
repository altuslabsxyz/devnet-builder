// Package config provides configuration management commands for devnet-builder.
package config

import "github.com/spf13/cobra"

// NewConfigCmd creates the config parent command with all subcommands.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: `Manage devnet-builder configuration.

The config command provides subcommands to initialize and view configuration settings.

Subcommands:
  init    Generate a sample config.toml file
  show    Display current effective configuration with sources
  set     Set a configuration value

Examples:
  # Generate sample config file
  devnet-builder config init

  # Show current configuration
  devnet-builder config show

  # Set a configuration value
  devnet-builder config set network testnet`,
	}

	cmd.AddCommand(
		NewInitCmd(),
		NewShowCmd(),
		NewSetCmd(),
	)

	return cmd
}
