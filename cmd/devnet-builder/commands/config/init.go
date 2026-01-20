package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

// Sample config.toml template
const sampleConfigTemplate = `# devnet-builder configuration file
# Priority: default < config.toml < environment < CLI flag
#
# This file is located at: ~/.devnet-builder/config.toml
# Use --config /path/to/config.toml to specify an alternative location

# =============================================================================
# Global Settings (apply to all commands)
# =============================================================================

# Base directory for devnet data
# home = "~/.devnet-builder"

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
	initOutput   string
	initForce    bool
	initTemplate bool
)

// NewInitCmd creates the config init subcommand.
func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize or reconfigure config.toml interactively",
		Long: `Initialize or reconfigure the config.toml file interactively.

By default, this runs an interactive setup where you can select your preferred
settings. If a config already exists, current values are shown as defaults.

Use --template to generate a sample config file with all available options
instead of running the interactive setup.

Examples:
  # Interactive configuration (creates/updates ~/.devnet-builder/config.toml)
  devnet-builder config init

  # Generate a template config file
  devnet-builder config init --template

  # Generate template at custom location
  devnet-builder config init --template --output /path/to/config.toml

  # Overwrite existing config without prompting (uses defaults)
  devnet-builder config init --force`,
		RunE: runInit,
	}

	cmd.Flags().StringVarP(&initOutput, "output", "o", "",
		"Output path for config file (default: ~/.devnet-builder/config.toml)")
	cmd.Flags().BoolVarP(&initForce, "force", "f", false,
		"Overwrite existing config with defaults without prompting")
	cmd.Flags().BoolVarP(&initTemplate, "template", "t", false,
		"Generate a template config file instead of interactive setup")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	if initTemplate {
		return runInitTemplate()
	}
	return runInitInteractive()
}

func runInitTemplate() error {
	homeDir := shared.GetHomeDir()
	outputPath := initOutput
	if outputPath == "" {
		outputPath = filepath.Join(homeDir, "config.toml")
	}

	// Expand ~ to home directory
	if len(outputPath) > 0 && outputPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		outputPath = filepath.Join(home, outputPath[1:])
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil && !initForce {
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

	output.Success("Created config template: %s", outputPath)
	output.Info("Edit the file to customize your settings.")
	return nil
}

func runInitInteractive() error {
	homeDir := shared.GetHomeDir()
	logger := output.DefaultLogger

	// Check if terminal is interactive
	if !config.IsInteractive() {
		if initForce {
			// Non-interactive with --force: use defaults
			setup := config.NewInteractiveSetup(homeDir)
			cfg := setup.RunWithDefaults()
			if err := setup.WriteConfig(cfg); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}
			logger.Success("Configuration saved to %s", filepath.Join(homeDir, "config.toml"))
			return nil
		}
		return fmt.Errorf("interactive mode requires a terminal\nUse --template to generate a sample config file, or --force to use defaults")
	}

	setup := config.NewInteractiveSetup(homeDir)

	// Check if config exists and warn user
	if setup.ConfigExists() && !initForce {
		logger.Info("Existing configuration found. Current values will be shown as defaults.")
	}

	// Run interactive setup
	cfg, err := setup.Run()
	if err != nil {
		return err
	}

	// Write config
	if err := setup.WriteConfig(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	logger.Success("Configuration saved to %s", filepath.Join(homeDir, "config.toml"))
	return nil
}
