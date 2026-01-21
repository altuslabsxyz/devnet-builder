// Package commands provides the CLI command implementations for devnet-builder.
// This file defines the root command and registers all subcommands.
package commands

import (
	"context"
	"os"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/commands/cache"
	configcmd "github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/commands/config"
	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/commands/core"
	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/commands/export"
	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/commands/manage"
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/internal/paths"
	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

// Command group IDs for organized help output.
const (
	GroupMain       = "main"
	GroupMonitoring = "monitoring"
	GroupAdvanced   = "advanced"
)

// Local variables for flag binding (Cobra requires pointers to local vars)
var (
	homeDir    string
	jsonMode   bool
	noColor    bool
	verbose    bool
	configPath string
)

// DefaultHomeDir returns the default home directory for devnet data.
func DefaultHomeDir() string {
	return paths.DefaultHomeDir()
}

// NewRootCmd creates the root command with all subcommands registered.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devnet-builder",
		Short: "CLI tool for managing local Stable blockchain development networks",
		Long: `devnet-builder is a CLI tool for managing local Stable blockchain development networks.

It consolidates multiple shell scripts into a single binary for easier devnet management:
  - Start a fully functional multi-validator devnet with a single command
  - Manage devnet lifecycle (start, stop, destroy)
  - Monitor devnet status and view node logs
  - Export validator and account keys
  - Build with specific stable repository versions

Examples:
  # Deploy a 4-validator devnet using mainnet data
  devnet-builder deploy

  # Deploy with testnet data and 2 validators
  devnet-builder deploy --network testnet --validators 2

  # Check devnet status
  devnet-builder status

  # View node logs
  devnet-builder logs -f

  # Stop the devnet
  devnet-builder stop`,
		PersistentPreRunE: persistentPreRunE,
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

	// Define command groups for organized help output
	cmd.AddGroup(&cobra.Group{ID: GroupMain, Title: "Main Commands:"})
	cmd.AddGroup(&cobra.Group{ID: GroupMonitoring, Title: "Monitoring Commands:"})
	cmd.AddGroup(&cobra.Group{ID: GroupAdvanced, Title: "Advanced Commands:"})

	// Register all commands
	registerCommands(cmd)

	return cmd
}

// persistentPreRunE handles configuration loading and global state setup.
func persistentPreRunE(cmd *cobra.Command, args []string) error {
	// Load config file
	loader := config.NewConfigLoader(homeDir, configPath, output.DefaultLogger)
	fileCfg, configFilePath, err := loader.LoadFileConfig()
	if err != nil {
		return err
	}

	// Apply config file values to global flags (if not explicitly set)
	// Priority: default < config.toml < env < flag
	applyConfigDefaults(cmd, fileCfg)

	// Environment variables override config.toml (but not explicit flags)
	applyEnvironmentOverrides(cmd)

	// Build context-based config from FileConfig and CLI overrides
	cfg := ctxconfig.New(
		ctxconfig.FromFileConfig(fileCfg),
		ctxconfig.WithHomeDir(homeDir),
		ctxconfig.WithConfigPath(configPath),
		ctxconfig.WithJSONMode(jsonMode),
		ctxconfig.WithNoColor(noColor),
		ctxconfig.WithVerbose(verbose),
		ctxconfig.WithFileConfig(fileCfg), // Store original FileConfig for commands that need it
	)

	// Set config in context (new pattern)
	ctx := cmd.Context()
	ctx = ctxconfig.WithConfig(ctx, cfg)

	// Deprecated: Keep old context value for backward compatibility during migration
	ctx = context.WithValue(ctx, types.ExecutionModeCtxKey, fileCfg.ExecutionMode)

	// Log which config file was loaded (if verbose)
	if configFilePath != "" && verbose {
		output.DefaultLogger.Debug("Using config file: %s", configFilePath)
	}

	// Apply global configuration to logger
	output.DefaultLogger.SetNoColor(noColor)
	output.DefaultLogger.SetVerbose(verbose)
	output.DefaultLogger.SetJSONMode(jsonMode)

	types.SetHomeDir(homeDir)

	// Store context
	cmd.SetContext(ctx)

	return nil
}

// applyConfigDefaults applies config file values to global flags if not explicitly set.
func applyConfigDefaults(cmd *cobra.Command, fileCfg *config.FileConfig) {
	// Apply home from config.toml
	if !cmd.Flags().Changed("home") && fileCfg.Home != nil {
		homeDir = *fileCfg.Home
	}

	// Apply verbose from config.toml
	if !cmd.Flags().Changed("verbose") && fileCfg.Verbose != nil {
		verbose = *fileCfg.Verbose
	}

	// Apply json from config.toml
	if !cmd.Flags().Changed("json") && fileCfg.JSON != nil {
		jsonMode = *fileCfg.JSON
	}

	// Apply no_color from config.toml
	if !cmd.Flags().Changed("no-color") && fileCfg.NoColor != nil {
		noColor = *fileCfg.NoColor
	}
}

// applyEnvironmentOverrides applies environment variable overrides.
func applyEnvironmentOverrides(cmd *cobra.Command) {
	if envHome := os.Getenv("DEVNET_HOME"); envHome != "" && !cmd.Flags().Changed("home") {
		homeDir = envHome
	}
	if os.Getenv("NO_COLOR") != "" && !cmd.Flags().Changed("no-color") {
		noColor = true
	}
}

// registerCommands registers all subcommands with appropriate group assignments.
func registerCommands(rootCmd *cobra.Command) {
	// Main commands
	deployCmd := manage.NewDeployCmd()
	deployCmd.GroupID = GroupMain
	startCmd := manage.NewStartCmd()
	startCmd.GroupID = GroupMain
	stopCmd := manage.NewStopCmd()
	stopCmd.GroupID = GroupMain
	initCmd := core.NewInitCmd()
	initCmd.GroupID = GroupMain
	destroyCmd := manage.NewDestroyCmd()
	destroyCmd.GroupID = GroupMain

	// Monitoring commands
	statusCmd := core.NewStatusCmd()
	statusCmd.GroupID = GroupMonitoring
	logsCmd := core.NewLogsCmd()
	logsCmd.GroupID = GroupMonitoring
	nodeCmd := manage.NewNodeCmd()
	nodeCmd.GroupID = GroupMonitoring

	// Advanced commands
	buildCmd := core.NewBuildCmd()
	buildCmd.GroupID = GroupAdvanced
	resetCmd := manage.NewResetCmd()
	resetCmd.GroupID = GroupAdvanced
	exportKeysCmd := core.NewExportKeysCmd()
	exportKeysCmd.GroupID = GroupAdvanced
	exportCmd := export.NewExportCmd()
	exportCmd.GroupID = GroupAdvanced
	upgradeCmd := manage.NewUpgradeCmd()
	upgradeCmd.GroupID = GroupAdvanced
	versionsCmd := core.NewVersionsCmd()
	versionsCmd.GroupID = GroupAdvanced
	cacheCmd := cache.NewCacheCmd()
	cacheCmd.GroupID = GroupAdvanced
	configCmd := configcmd.NewConfigCmd()
	configCmd.GroupID = GroupAdvanced
	networksCmd := core.NewNetworksCmd()
	networksCmd.GroupID = GroupAdvanced

	// Utility commands (no group - shown separately)
	versionCmd := core.NewVersionCmd()
	completionCmd := core.NewCompletionCmd()

	// Add all subcommands
	rootCmd.AddCommand(
		// Main commands
		deployCmd,
		startCmd,
		stopCmd,
		initCmd,
		destroyCmd,

		// Monitoring commands
		statusCmd,
		logsCmd,
		nodeCmd,

		// Advanced commands
		buildCmd,
		resetCmd,
		exportKeysCmd,
		exportCmd,
		upgradeCmd,
		versionsCmd,
		cacheCmd,
		configCmd,
		networksCmd,

		// Utility commands
		versionCmd,
		completionCmd,
	)
}


