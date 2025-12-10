package main

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/config"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// Global configuration variables
var (
	homeDir    string
	jsonMode   bool
	noColor    bool
	verbose    bool
	configPath string // Path to config.toml file (--config flag)

	// loadedFileConfig holds the parsed config.toml values (nil if no config file)
	loadedFileConfig *config.FileConfig
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Load config file
			loader := config.NewConfigLoader(homeDir, configPath, output.DefaultLogger)
			fileCfg, configFilePath, err := loader.LoadFileConfig()
			if err != nil {
				return err
			}
			loadedFileConfig = fileCfg

			// Apply config file values to global flags (if not explicitly set)
			// Priority: default < config.toml < env < flag

			// Apply home from config.toml
			if !cmd.Flags().Changed("home") && fileCfg.Home != nil {
				homeDir = *fileCfg.Home
			}

			// Apply verbose from config.toml (before env check)
			if !cmd.Flags().Changed("verbose") && fileCfg.Verbose != nil {
				verbose = *fileCfg.Verbose
			}

			// Apply json from config.toml
			if !cmd.Flags().Changed("json") && fileCfg.JSON != nil {
				jsonMode = *fileCfg.JSON
			}

			// Apply no_color from config.toml (before env check)
			if !cmd.Flags().Changed("no-color") && fileCfg.NoColor != nil {
				noColor = *fileCfg.NoColor
			}

			// Environment variables override config.toml (but not explicit flags)
			if envHome := os.Getenv("STABLE_DEVNET_HOME"); envHome != "" && !cmd.Flags().Changed("home") {
				homeDir = envHome
			}
			if os.Getenv("NO_COLOR") != "" && !cmd.Flags().Changed("no-color") {
				noColor = true
			}

			// Log which config file was loaded (if verbose)
			if configFilePath != "" && verbose {
				output.DefaultLogger.Debug("Using config file: %s", configFilePath)
			}

			// Apply global configuration to logger
			output.DefaultLogger.SetNoColor(noColor)
			output.DefaultLogger.SetVerbose(verbose)
			output.DefaultLogger.SetJSONMode(jsonMode)

			return nil
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
	cmd.PersistentFlags().StringVar(&configPath, "config", "",
		"Path to config.toml file")

	// Add subcommands
	cmd.AddCommand(
		NewBuildCmd(),
		NewProvisionCmd(),
		NewRunCmd(),
		NewStartCmd(),
		NewStopCmd(),
		NewRestartCmd(),
		NewResetCmd(),
		NewCleanCmd(),
		NewStatusCmd(),
		NewLogsCmd(),
		NewNodeCmd(),
		NewExportKeysCmd(),
		NewVersionCmd(),
		NewCompletionCmd(),
		NewConfigCmd(),
		NewUpgradeCmd(),
		NewVersionsCmd(),
		NewCacheCmd(),
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

// GetLoadedFileConfig returns the loaded config.toml values.
// Returns nil if no config file was loaded.
func GetLoadedFileConfig() *config.FileConfig {
	return loadedFileConfig
}
