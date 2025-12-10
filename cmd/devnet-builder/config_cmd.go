package main

import "github.com/spf13/cobra"

// NewConfigCmd creates the config parent command.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: `Manage devnet-builder configuration.

The config command provides subcommands to initialize and view configuration settings.

Subcommands:
  init    Generate a sample config.toml file
  show    Display current effective configuration with sources

Examples:
  # Generate sample config file
  devnet-builder config init

  # Show current configuration
  devnet-builder config show`,
	}

	cmd.AddCommand(
		NewConfigInitCmd(),
		NewConfigShowCmd(),
		NewConfigSetCmd(),
	)

	return cmd
}
