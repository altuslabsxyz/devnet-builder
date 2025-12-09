package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// Global configuration variables
var (
	homeDir   string
	jsonMode  bool
	noColor   bool
	verbose   bool
)

// DefaultHomeDir returns the default home directory for devnet data.
func DefaultHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".stable-devnet"
	}
	return filepath.Join(home, ".stable-devnet")
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devnet-builder",
		Short: "CLI tool for managing local Stable blockchain development networks",
		Long: `devnet-builder is a CLI tool for managing local Stable blockchain development networks.

It consolidates multiple shell scripts into a single binary for easier devnet management:
  - Start a fully functional multi-validator devnet with a single command
  - Manage devnet lifecycle (stop, restart, reset, clean)
  - Monitor devnet status and view node logs
  - Export validator and account keys
  - Build with specific stable repository versions

Examples:
  # Start a 4-validator devnet using mainnet data
  devnet-builder start

  # Start with testnet data and 2 validators
  devnet-builder start --network testnet --validators 2

  # Check devnet status
  devnet-builder status

  # View node logs
  devnet-builder logs -f

  # Stop the devnet
  devnet-builder stop`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Apply global configuration
			output.DefaultLogger.SetNoColor(noColor)
			output.DefaultLogger.SetVerbose(verbose)
			output.DefaultLogger.SetJSONMode(jsonMode)
		},
	}

	// Global flags available on all commands
	cmd.PersistentFlags().StringVarP(&homeDir, "home", "H", DefaultHomeDir(),
		"Base directory for devnet data")
	cmd.PersistentFlags().BoolVar(&jsonMode, "json", false,
		"Output in JSON format")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false,
		"Disable colored output")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose logging")

	// Check environment variables for defaults
	if envHome := os.Getenv("STABLE_DEVNET_HOME"); envHome != "" {
		homeDir = envHome
	}
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
	}

	// Add subcommands
	cmd.AddCommand(
		NewBuildCmd(),
		NewStartCmd(),
		NewStopCmd(),
		NewRestartCmd(),
		NewResetCmd(),
		NewCleanCmd(),
		NewStatusCmd(),
		NewLogsCmd(),
		NewExportKeysCmd(),
		NewVersionCmd(),
		NewCompletionCmd(),
	)

	return cmd
}

// GetHomeDir returns the configured home directory.
func GetHomeDir() string {
	return homeDir
}

// IsJSONMode returns true if JSON output is enabled.
func IsJSONMode() bool {
	return jsonMode
}

// IsVerbose returns true if verbose logging is enabled.
func IsVerbose() bool {
	return verbose
}

// confirmPrompt is a helper function for confirmation prompts.
func confirmPrompt(message string) (bool, error) {
	return output.ConfirmPrompt(message)
}
