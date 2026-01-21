package config

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/internal/paths"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

// NewShowCmd creates the config show subcommand.
func NewShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display current effective configuration",
		Long: `Display the current effective configuration with sources.

Shows all configuration values and where they came from:
  - default: Built-in default value
  - config.toml: Value from config file
  - environment: Value from environment variable
  - flag: Value from command-line flag

Examples:
  # Show current configuration
  devnet-builder config show

  # Show configuration with verbose output
  devnet-builder config show -v`,
		RunE: runShow,
	}

	return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
	ctxCfg := ctxconfig.FromContext(cmd.Context())
	cfg := buildEffectiveConfig(cmd, ctxCfg)

	if ctxCfg.JSONMode() {
		return outputShowJSON(cfg)
	}

	// Print table
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
	fmt.Fprintln(tw, "---\t-----\t------")
	fmt.Fprintf(tw, "home\t%s\t%s\n", cfg.Home.Value, cfg.Home.Source)
	fmt.Fprintf(tw, "verbose\t%t\t%s\n", cfg.Verbose.Value, cfg.Verbose.Source)
	fmt.Fprintf(tw, "json\t%t\t%s\n", cfg.JSON.Value, cfg.JSON.Source)
	fmt.Fprintf(tw, "no-color\t%t\t%s\n", cfg.NoColor.Value, cfg.NoColor.Source)
	fmt.Fprintf(tw, "network\t%s\t%s\n", cfg.Network.Value, cfg.Network.Source)
	fmt.Fprintf(tw, "validators\t%d\t%s\n", cfg.Validators.Value, cfg.Validators.Source)
	fmt.Fprintf(tw, "mode\t%s\t%s\n", cfg.Mode.Value, cfg.Mode.Source)
	fmt.Fprintf(tw, "network-version\t%s\t%s\n", cfg.NetworkVersion.Value, cfg.NetworkVersion.Source)
	fmt.Fprintf(tw, "no-cache\t%t\t%s\n", cfg.NoCache.Value, cfg.NoCache.Source)
	fmt.Fprintf(tw, "accounts\t%d\t%s\n", cfg.Accounts.Value, cfg.Accounts.Source)
	fmt.Fprintf(tw, "github-token\t%s\t%s\n", maskToken(cfg.GitHubToken.Value), cfg.GitHubToken.Source)
	fmt.Fprintf(tw, "cache-ttl\t%s\t%s\n", cfg.CacheTTL.Value, cfg.CacheTTL.Source)
	tw.Flush()

	// Print config file path if loaded
	if cfg.ConfigFilePath != "" {
		fmt.Printf("\nConfig file: %s\n", cfg.ConfigFilePath)
	} else {
		fmt.Println("\nNo config file loaded")
	}

	return nil
}

func outputShowJSON(cfg *config.EffectiveConfig) error {
	result := map[string]interface{}{
		"home":            cfg.Home.Value,
		"verbose":         cfg.Verbose.Value,
		"json":            cfg.JSON.Value,
		"no_color":        cfg.NoColor.Value,
		"network":         cfg.Network.Value,
		"validators":      cfg.Validators.Value,
		"mode":            cfg.Mode.Value,
		"network_version": cfg.NetworkVersion.Value,
		"no_cache":        cfg.NoCache.Value,
		"accounts":        cfg.Accounts.Value,
		"github_token":    maskToken(cfg.GitHubToken.Value),
		"cache_ttl":       cfg.CacheTTL.Value,
		"config_file":     cfg.ConfigFilePath,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

// defaultHomeDir returns the default home directory.
func defaultHomeDir() string {
	return paths.DefaultHomeDir()
}

func buildEffectiveConfig(cmd *cobra.Command, ctxCfg *ctxconfig.Config) *config.EffectiveConfig {
	homeDir := ctxCfg.HomeDir()
	configPath := ctxCfg.ConfigPath()
	fileCfg := ctxCfg.FileConfig()

	cfg := config.NewEffectiveConfig(defaultHomeDir())

	// Track config file path
	loader := config.NewConfigLoader(homeDir, configPath, nil)
	_, configFilePath, _ := loader.LoadFileConfig()
	cfg.ConfigFilePath = configFilePath

	// Home
	cfg.Home = config.StringValue{Value: homeDir, Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.Home != nil {
		cfg.Home = config.StringValue{Value: *fileCfg.Home, Source: config.SourceConfigFile}
	}
	if envHome := os.Getenv("DEVNET_HOME"); envHome != "" {
		cfg.Home = config.StringValue{Value: envHome, Source: config.SourceEnvironment}
	}
	if cmd.Flags().Changed("home") {
		cfg.Home = config.StringValue{Value: homeDir, Source: config.SourceFlag}
	}

	// Verbose
	cfg.Verbose = config.BoolValue{Value: ctxCfg.Verbose(), Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.Verbose != nil {
		cfg.Verbose = config.BoolValue{Value: *fileCfg.Verbose, Source: config.SourceConfigFile}
	}
	if cmd.Flags().Changed("verbose") {
		cfg.Verbose = config.BoolValue{Value: ctxCfg.Verbose(), Source: config.SourceFlag}
	}

	// JSON
	cfg.JSON = config.BoolValue{Value: ctxCfg.JSONMode(), Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.JSON != nil {
		cfg.JSON = config.BoolValue{Value: *fileCfg.JSON, Source: config.SourceConfigFile}
	}
	if cmd.Flags().Changed("json") {
		cfg.JSON = config.BoolValue{Value: ctxCfg.JSONMode(), Source: config.SourceFlag}
	}

	// NoColor
	cfg.NoColor = config.BoolValue{Value: ctxCfg.NoColor(), Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.NoColor != nil {
		cfg.NoColor = config.BoolValue{Value: *fileCfg.NoColor, Source: config.SourceConfigFile}
	}
	if os.Getenv("NO_COLOR") != "" {
		cfg.NoColor = config.BoolValue{Value: true, Source: config.SourceEnvironment}
	}
	if cmd.Flags().Changed("no-color") {
		cfg.NoColor = config.BoolValue{Value: ctxCfg.NoColor(), Source: config.SourceFlag}
	}

	// Network
	cfg.Network = config.StringValue{Value: "mainnet", Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.Network != nil {
		cfg.Network = config.StringValue{Value: *fileCfg.Network, Source: config.SourceConfigFile}
	}
	if envNetwork := os.Getenv("DEVNET_NETWORK"); envNetwork != "" {
		cfg.Network = config.StringValue{Value: envNetwork, Source: config.SourceEnvironment}
	}

	// Validators
	cfg.Validators = config.IntValue{Value: 4, Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.Validators != nil {
		cfg.Validators = config.IntValue{Value: *fileCfg.Validators, Source: config.SourceConfigFile}
	}

	// Mode
	cfg.Mode = config.StringValue{Value: "docker", Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.ExecutionMode != nil {
		cfg.Mode = config.StringValue{Value: string(*fileCfg.ExecutionMode), Source: config.SourceConfigFile}
	}
	if envMode := os.Getenv("DEVNET_MODE"); envMode != "" {
		cfg.Mode = config.StringValue{Value: envMode, Source: config.SourceEnvironment}
	}

	// NetworkVersion
	cfg.NetworkVersion = config.StringValue{Value: "latest", Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.NetworkVersion != nil {
		cfg.NetworkVersion = config.StringValue{Value: *fileCfg.NetworkVersion, Source: config.SourceConfigFile}
	}
	if envVersion := os.Getenv("NETWORK_VERSION"); envVersion != "" {
		cfg.NetworkVersion = config.StringValue{Value: envVersion, Source: config.SourceEnvironment}
	}

	// NoCache
	cfg.NoCache = config.BoolValue{Value: false, Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.NoCache != nil {
		cfg.NoCache = config.BoolValue{Value: *fileCfg.NoCache, Source: config.SourceConfigFile}
	}

	// Accounts
	cfg.Accounts = config.IntValue{Value: 0, Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.Accounts != nil {
		cfg.Accounts = config.IntValue{Value: *fileCfg.Accounts, Source: config.SourceConfigFile}
	}

	// GitHubToken
	cfg.GitHubToken = config.StringValue{Value: "", Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.GitHubToken != nil {
		cfg.GitHubToken = config.StringValue{Value: *fileCfg.GitHubToken, Source: config.SourceConfigFile}
	}
	if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		cfg.GitHubToken = config.StringValue{Value: envToken, Source: config.SourceEnvironment}
	}

	// CacheTTL
	cfg.CacheTTL = config.StringValue{Value: "1h", Source: config.SourceDefault}
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		cfg.CacheTTL = config.StringValue{Value: *fileCfg.CacheTTL, Source: config.SourceConfigFile}
	}

	return cfg
}

// maskToken masks a GitHub token for display.
func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
