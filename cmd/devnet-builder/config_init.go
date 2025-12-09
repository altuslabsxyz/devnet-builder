package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// Sample config.toml template
const sampleConfigTemplate = `# devnet-builder configuration file
# Priority: default < config.toml < environment < CLI flag
#
# Place this file at:
#   - ./config.toml (current directory - highest priority)
#   - ~/.stable-devnet/config.toml (home directory - fallback)
# Or specify with: --config /path/to/config.toml

# =============================================================================
# Global Settings (apply to all commands)
# =============================================================================

# Base directory for devnet data
# home = "~/.stable-devnet"

# Enable verbose logging
# verbose = false

# Output in JSON format
# json = false

# Disable colored output
# no_color = false

# =============================================================================
# Start Command Settings
# =============================================================================

# Network source for snapshot data
# Valid values: "mainnet", "testnet"
# network = "mainnet"

# Number of validators to create
# Valid values: 1-4
# validators = 4

# Execution mode
# Valid values: "docker", "local"
# mode = "docker"

# Stable repository version for building
# stable_version = "latest"

# Skip snapshot cache (force re-download)
# no_cache = false

# Additional funded accounts to create
# Valid values: 0-100
# accounts = 0
`

var (
	configInitOutput string
	configInitForce  bool
)

// NewConfigInitCmd creates the config init subcommand.
func NewConfigInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a sample config.toml file",
		Long: `Generate a sample config.toml file with all available options.

The config file will be created in the current directory by default.
Use --output to specify a different location.

Examples:
  # Generate config.toml in current directory
  devnet-builder config init

  # Generate in home directory
  devnet-builder config init --output ~/.stable-devnet/config.toml

  # Overwrite existing file
  devnet-builder config init --force`,
		RunE: runConfigInit,
	}

	cmd.Flags().StringVarP(&configInitOutput, "output", "o", "./config.toml",
		"Output path for config file")
	cmd.Flags().BoolVarP(&configInitForce, "force", "f", false,
		"Overwrite existing file")

	return cmd
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	outputPath := configInitOutput

	// Expand ~ to home directory
	if len(outputPath) > 0 && outputPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		outputPath = filepath.Join(home, outputPath[1:])
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil && !configInitForce {
		return fmt.Errorf("config file already exists: %s\nUse --force to overwrite", outputPath)
	}

	// Create parent directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write config file
	if err := os.WriteFile(outputPath, []byte(sampleConfigTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	output.Success("Created config file: %s", outputPath)
	output.Info("Edit the file to customize your settings.")
	return nil
}
