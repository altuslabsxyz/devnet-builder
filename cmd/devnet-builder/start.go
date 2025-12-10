package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/builder"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/github"
	"github.com/stablelabs/stable-devnet/internal/interactive"
	"github.com/stablelabs/stable-devnet/internal/output"
)

var (
	startNetwork       string
	startValidators    int
	startMode          string
	startStableVersion string
	startNoCache       bool
	startAccounts      int
	startNoInteractive bool
	startExportVersion string
	startStartVersion  string
)

// StartResult represents the JSON output for the start command.
type StartResult struct {
	Status     string       `json:"status"`
	ChainID    string       `json:"chain_id"`
	Network    string       `json:"network"`
	Mode       string       `json:"mode"`
	Validators int          `json:"validators"`
	Nodes      []NodeResult `json:"nodes"`
}

// NodeResult represents a node in the JSON output.
type NodeResult struct {
	Index  int    `json:"index"`
	RPC    string `json:"rpc"`
	EVMRPC string `json:"evm_rpc"`
	Status string `json:"status"`
}

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a local devnet",
		Long: `Start a local devnet with configurable validators and network source.

This command will:
1. Check prerequisites (Docker/local binary, curl, jq, zstd/lz4)
2. Download or use cached snapshot from the specified network
3. Export genesis state from snapshot
4. Generate devnet configuration for all validators
5. Start all validator nodes

Examples:
  # Start with default settings (4 validators, mainnet, docker mode)
  devnet-builder start

  # Start with testnet data
  devnet-builder start --network testnet

  # Start with 2 validators
  devnet-builder start --validators 2

  # Start with local binary mode
  devnet-builder start --mode local

  # Start with specific stable version
  devnet-builder start --stable-version v1.2.3`,
		RunE: runStart,
	}

	// Command flags
	cmd.Flags().StringVarP(&startNetwork, "network", "n", "mainnet",
		"Network source (mainnet, testnet)")
	cmd.Flags().IntVar(&startValidators, "validators", 4,
		"Number of validators (1-4)")
	cmd.Flags().StringVarP(&startMode, "mode", "m", "docker",
		"Execution mode (docker, local)")
	cmd.Flags().StringVar(&startStableVersion, "stable-version", "latest",
		"Stable repository version")
	cmd.Flags().BoolVar(&startNoCache, "no-cache", false,
		"Skip snapshot cache")
	cmd.Flags().IntVar(&startAccounts, "accounts", 0,
		"Additional funded accounts")

	// Interactive mode flags
	cmd.Flags().BoolVar(&startNoInteractive, "no-interactive", false,
		"Disable interactive mode (use flags instead)")
	cmd.Flags().StringVar(&startExportVersion, "export-version", "",
		"Version for genesis export (non-interactive mode)")
	cmd.Flags().StringVar(&startStartVersion, "start-version", "",
		"Version for node start (non-interactive mode)")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Apply config.toml values (priority: default < config.toml < env < flag)
	fileCfg := GetLoadedFileConfig()
	if fileCfg != nil {
		// Apply config file values if flags not explicitly set
		if !cmd.Flags().Changed("network") && fileCfg.Network != nil {
			startNetwork = *fileCfg.Network
		}
		if !cmd.Flags().Changed("validators") && fileCfg.Validators != nil {
			startValidators = *fileCfg.Validators
		}
		if !cmd.Flags().Changed("mode") && fileCfg.Mode != nil {
			startMode = *fileCfg.Mode
		}
		if !cmd.Flags().Changed("stable-version") && fileCfg.StableVersion != nil {
			startStableVersion = *fileCfg.StableVersion
		}
		if !cmd.Flags().Changed("no-cache") && fileCfg.NoCache != nil {
			startNoCache = *fileCfg.NoCache
		}
		if !cmd.Flags().Changed("accounts") && fileCfg.Accounts != nil {
			startAccounts = *fileCfg.Accounts
		}
	}

	// Apply environment variable defaults (override config.toml, but not explicit flags)
	if network := os.Getenv("STABLE_DEVNET_NETWORK"); network != "" && !cmd.Flags().Changed("network") {
		startNetwork = network
	}
	if mode := os.Getenv("STABLE_DEVNET_MODE"); mode != "" && !cmd.Flags().Changed("mode") {
		startMode = mode
	}
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		startStableVersion = version
	}

	// Track if versions are custom refs
	var exportIsCustomRef bool
	var startIsCustomRef bool
	var exportVersion string
	var startVersion string

	// Interactive mode: run selection flow if not disabled
	if !startNoInteractive && !jsonMode {
		selection, err := runInteractiveSelection(ctx, cmd)
		if err != nil {
			if interactive.IsCancellation(err) {
				fmt.Println("Operation cancelled.")
				return nil
			}
			return err
		}
		// Apply selections
		startNetwork = selection.Network
		exportVersion = selection.ExportVersion
		exportIsCustomRef = selection.ExportIsCustomRef
		startVersion = selection.StartVersion
		startIsCustomRef = selection.StartIsCustomRef
		// Use export version for provisioning (stableVersion flag)
		startStableVersion = exportVersion
	} else {
		// Non-interactive mode: both versions are the same
		exportVersion = startStableVersion
		startVersion = startStableVersion
	}

	// Validate inputs
	if startNetwork != "mainnet" && startNetwork != "testnet" {
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", startNetwork)
	}
	if startValidators < 1 || startValidators > 4 {
		return fmt.Errorf("invalid validators: %d (must be 1-4)", startValidators)
	}
	if startMode != "docker" && startMode != "local" {
		return fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", startMode)
	}

	// Check if devnet already exists
	if devnet.DevnetExists(homeDir) {
		return fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder clean' to remove it first", homeDir)
	}

	// Build from source if start version is a custom ref
	var customBinaryPath string
	if startIsCustomRef {
		b := builder.NewBuilder(homeDir, logger)
		logger.Info("Building binary from source (ref: %s)...", startVersion)
		result, err := b.Build(ctx, builder.BuildOptions{
			Ref:     startVersion,
			Network: startNetwork,
		})
		if err != nil {
			return fmt.Errorf("failed to build from source: %w", err)
		}
		customBinaryPath = result.BinaryPath
		logger.Success("Binary built: %s (commit: %s)", result.BinaryPath, result.CommitHash)
	}

	// Start devnet
	// Note: StableVersion is used for provisioning (export), CustomBinaryPath for node start
	opts := devnet.StartOptions{
		HomeDir:          homeDir,
		Network:          startNetwork,
		NumValidators:    startValidators,
		NumAccounts:      startAccounts,
		Mode:             devnet.ExecutionMode(startMode),
		StableVersion:    exportVersion,
		NoCache:          startNoCache,
		Logger:           logger,
		IsCustomRef:      startIsCustomRef,
		CustomBinaryPath: customBinaryPath,
	}

	// Store start version info in metadata for reference
	_ = startVersion       // Used for building, stored via CustomBinaryPath
	_ = exportIsCustomRef  // Export custom ref not yet supported (would need separate build)

	d, err := devnet.Start(ctx, opts)
	if err != nil {
		if jsonMode {
			return outputStartError(err)
		}
		return err
	}

	// Output result
	if jsonMode {
		return outputStartJSON(d)
	}
	return outputStartText(d)
}

func outputStartText(d *devnet.Devnet) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", d.Metadata.ChainID)
	output.Info("Network:      %s", d.Metadata.NetworkSource)
	output.Info("Mode:         %s", d.Metadata.ExecutionMode)
	output.Info("Validators:   %d", d.Metadata.NumValidators)
	fmt.Println()
	output.Bold("Endpoints:")

	for _, n := range d.Nodes {
		fmt.Printf("  Node %d: %s (RPC) | %s (EVM)\n",
			n.Index, n.RPCURL(), n.EVMRPCURL())
	}

	return nil
}

func outputStartJSON(d *devnet.Devnet) error {
	result := StartResult{
		Status:     "success",
		ChainID:    d.Metadata.ChainID,
		Network:    d.Metadata.NetworkSource,
		Mode:       string(d.Metadata.ExecutionMode),
		Validators: d.Metadata.NumValidators,
		Nodes:      make([]NodeResult, len(d.Nodes)),
	}

	for i, n := range d.Nodes {
		result.Nodes[i] = NodeResult{
			Index:  n.Index,
			RPC:    n.RPCURL(),
			EVMRPC: n.EVMRPCURL(),
			Status: string(n.Status),
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputStartError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    getErrorCode(err),
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}

func getErrorCode(err error) string {
	errStr := err.Error()
	switch {
	case contains(errStr, "prerequisite"):
		return "PREREQUISITE_MISSING"
	case contains(errStr, "already exists"):
		return "DEVNET_ALREADY_RUNNING"
	case contains(errStr, "snapshot"):
		return "SNAPSHOT_DOWNLOAD_FAILED"
	case contains(errStr, "start"):
		return "NODE_START_FAILED"
	case contains(errStr, "port"):
		return "PORT_CONFLICT"
	default:
		return "GENERAL_ERROR"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// runInteractiveSelection runs the interactive version selection flow.
func runInteractiveSelection(ctx context.Context, cmd *cobra.Command) (*interactive.SelectionConfig, error) {
	// Get config for cache settings
	fileCfg := GetLoadedFileConfig()

	// Set up cache manager
	cacheTTL := github.DefaultCacheTTL
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		if ttl, err := time.ParseDuration(*fileCfg.CacheTTL); err == nil {
			cacheTTL = ttl
		}
	}
	cacheManager := github.NewCacheManager(homeDir, cacheTTL)

	// Set up GitHub client with cache and optional token
	clientOpts := []github.ClientOption{
		github.WithCache(cacheManager),
	}
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		clientOpts = append(clientOpts, github.WithToken(*fileCfg.GitHubToken))
	}
	client := github.NewClient(clientOpts...)

	// Run selection flow
	selector := interactive.NewSelector(client)
	return selector.RunSelectionFlow(ctx)
}
