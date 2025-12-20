package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/builder"
	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/github"
	"github.com/b-harvest/devnet-builder/internal/interactive"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
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
	startImage         string // Docker image for docker mode
)

// StartResult represents the JSON output for the start command.
type StartResult struct {
	Status      string       `json:"status"`
	ChainID     string       `json:"chain_id"`
	Network     string       `json:"network"`
	Mode        string       `json:"mode"`
	DockerImage string       `json:"docker_image,omitempty"`
	Validators  int          `json:"validators"`
	Nodes       []NodeResult `json:"nodes"`
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
		Use:        "start",
		Short:      "Start a local devnet (deprecated: use 'deploy' instead)",
		Deprecated: "use 'deploy' instead",
		Long: `Start a local devnet with configurable validators and network source.

DEPRECATED: This command is deprecated. Use 'devnet-builder deploy' instead.

This command will:
1. Check prerequisites (Docker/local binary, curl, jq, zstd/lz4)
2. Download or use cached snapshot from the specified network
3. Export genesis state from snapshot
4. Generate devnet configuration for all validators
5. Start all validator nodes

Examples:
  # Start with default settings (4 validators, mainnet, docker mode)
  devnet-builder deploy

  # Start with testnet data
  devnet-builder deploy --network testnet

  # Start with 2 validators
  devnet-builder deploy --validators 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintDeprecationWarning("start", "deploy")
			return runStart(cmd, args)
		},
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

	// Interactive mode flags (controls version/docker image selection prompts)
	// Note: Base config prompts (network, validators, mode) are handled by config.toml
	cmd.Flags().BoolVar(&startNoInteractive, "no-interactive", false,
		"Disable version selection prompts (use --export-version, --start-version, --image instead)")
	cmd.Flags().StringVar(&startExportVersion, "export-version", "",
		"Version for genesis export (non-interactive mode)")
	cmd.Flags().StringVar(&startStartVersion, "start-version", "",
		"Version for node start (non-interactive mode)")

	// Docker image flag
	cmd.Flags().StringVar(&startImage, "image", "",
		"Docker image for docker mode (e.g., v1.0.0 or ghcr.io/org/image:tag)")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Build effective config from: default < config.toml < env < flag
	// Start with loaded config.toml values
	fileCfg := GetLoadedFileConfig()
	if fileCfg == nil {
		fileCfg = &config.FileConfig{}
	}

	// Apply flag values (flags override config.toml)
	if cmd.Flags().Changed("network") {
		fileCfg.Network = &startNetwork
	}
	if cmd.Flags().Changed("validators") {
		fileCfg.Validators = &startValidators
	}
	if cmd.Flags().Changed("mode") {
		fileCfg.Mode = &startMode
	}
	if cmd.Flags().Changed("stable-version") {
		fileCfg.StableVersion = &startStableVersion
	}
	if cmd.Flags().Changed("no-cache") {
		fileCfg.NoCache = &startNoCache
	}
	if cmd.Flags().Changed("accounts") {
		fileCfg.Accounts = &startAccounts
	}

	// Apply environment variables (env overrides config.toml but not flags)
	if network := os.Getenv("STABLE_DEVNET_NETWORK"); network != "" && !cmd.Flags().Changed("network") {
		fileCfg.Network = &network
	}
	if mode := os.Getenv("STABLE_DEVNET_MODE"); mode != "" && !cmd.Flags().Changed("mode") {
		fileCfg.Mode = &mode
	}
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		fileCfg.StableVersion = &version
	}

	// For deprecated start command, default blockchain to "stable" if not set
	if fileCfg.BlockchainNetwork == nil {
		defaultBlockchain := "stable"
		fileCfg.BlockchainNetwork = &defaultBlockchain
	}

	// Run partial interactive setup for missing values
	setup := config.NewInteractiveSetup(homeDir)
	effectiveCfg, err := setup.RunPartial(fileCfg)
	if err != nil {
		// Check if it's a missing fields error for better messaging
		if mfErr, ok := err.(*config.MissingFieldsError); ok {
			return fmt.Errorf("missing required configuration: %v\nRun 'devnet-builder config init' to create a configuration file", mfErr.Fields)
		}
		return err
	}

	// Extract values from effective config
	startNetwork = *effectiveCfg.Network
	startValidators = *effectiveCfg.Validators
	startMode = *effectiveCfg.Mode
	if effectiveCfg.StableVersion != nil {
		startStableVersion = *effectiveCfg.StableVersion
	}
	if effectiveCfg.NoCache != nil {
		startNoCache = *effectiveCfg.NoCache
	}
	if effectiveCfg.Accounts != nil {
		startAccounts = *effectiveCfg.Accounts
	}

	// Track if versions are custom refs
	var exportIsCustomRef bool
	var startIsCustomRef bool
	var exportVersion string
	var startVersion string
	var dockerImage string // Selected docker image (only for docker mode)

	// Determine if running in interactive mode
	isInteractive := !startNoInteractive && !jsonMode

	// Docker mode uses GHCR package versions, not GitHub releases
	if startMode == "docker" {
		// For docker mode, resolve docker image (handles --image flag, interactive selection, defaults)
		resolvedImage, err := resolveDockerImage(ctx, cmd, isInteractive)
		if err != nil {
			if interactive.IsCancellation(err) {
				fmt.Println("Operation cancelled.")
				return nil
			}
			return err
		}
		dockerImage = resolvedImage
		// Docker mode doesn't need export/start version selection - use defaults
		exportVersion = startStableVersion
		startVersion = startStableVersion
	} else {
		// Local mode: run interactive selection flow for GitHub releases
		if isInteractive {
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

	// Build from source for local mode
	// In local mode, we always need a binary at ~/.stable-devnet/bin/{binaryName}
	var customBinaryPath string
	if startMode == "local" {
		// Get network module for building
		netModule, err := network.Default()
		if err != nil {
			return fmt.Errorf("failed to get network module: %w", err)
		}

		b := builder.NewBuilder(homeDir, logger, netModule)
		ref := startVersion
		if ref == "" || ref == "latest" {
			ref = "main" // Default to main branch for local builds
		}
		logger.Info("Building binary from source (ref: %s)...", ref)
		result, err := b.Build(ctx, builder.BuildOptions{
			Ref:     ref,
			Network: startNetwork,
		})
		if err != nil {
			return fmt.Errorf("failed to build from source: %w", err)
		}
		customBinaryPath = result.BinaryPath
		startIsCustomRef = true // Mark as custom ref since we built a binary
		logger.Success("Binary built: %s (commit: %s)", result.BinaryPath, result.CommitHash)
	}

	// Store start version info in metadata for reference
	_ = startVersion      // Used for building, stored via CustomBinaryPath
	_ = exportIsCustomRef // Export custom ref not yet supported (would need separate build)

	// Phase 1: Provision (create config, generate validators)
	provisionOpts := devnet.ProvisionOptions{
		HomeDir:       homeDir,
		Network:       startNetwork,
		NumValidators: startValidators,
		NumAccounts:   startAccounts,
		Mode:          devnet.ExecutionMode(startMode),
		StableVersion: exportVersion,
		DockerImage:   dockerImage,
		NoCache:       startNoCache,
		Logger:        logger,
	}

	_, err = devnet.Provision(ctx, provisionOpts)
	if err != nil {
		if jsonMode {
			return outputStartError(err)
		}
		return err
	}

	// Phase 2: Run (start nodes)
	runOpts := devnet.RunOptions{
		HomeDir:          homeDir,
		Mode:             devnet.ExecutionMode(startMode),
		StableVersion:    exportVersion,
		HealthTimeout:    devnet.HealthCheckTimeout,
		Logger:           logger,
		IsCustomRef:      startIsCustomRef,
		CustomBinaryPath: customBinaryPath,
	}

	runResult, err := devnet.Run(ctx, runOpts)
	if err != nil {
		// Still output partial results if available
		if runResult != nil && runResult.Devnet != nil {
			if jsonMode {
				return outputStartJSONFromRunResult(runResult, err)
			}
		}
		if jsonMode {
			return outputStartError(err)
		}
		return err
	}

	// Output result
	if jsonMode {
		return outputStartJSON(runResult.Devnet)
	}
	return outputStartText(runResult.Devnet)
}

func outputStartJSONFromRunResult(result *devnet.RunResult, err error) error {
	jsonResult := StartResult{
		Status:      "partial",
		ChainID:     result.Devnet.Metadata.ChainID,
		Network:     result.Devnet.Metadata.NetworkSource,
		Mode:        string(result.Devnet.Metadata.ExecutionMode),
		DockerImage: result.Devnet.Metadata.DockerImage,
		Validators:  result.Devnet.Metadata.NumValidators,
		Nodes:       make([]NodeResult, len(result.Devnet.Nodes)),
	}

	for i, n := range result.Devnet.Nodes {
		status := "running"
		for _, fn := range result.FailedNodes {
			if fn.Index == n.Index {
				status = "failed"
				break
			}
		}
		jsonResult.Nodes[i] = NodeResult{
			Index:  n.Index,
			RPC:    n.RPCURL(),
			EVMRPC: n.EVMRPCURL(),
			Status: status,
		}
	}

	data, jsonErr := json.MarshalIndent(jsonResult, "", "  ")
	if jsonErr != nil {
		return jsonErr
	}

	fmt.Println(string(data))
	return err
}

func outputStartText(d *devnet.Devnet) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", d.Metadata.ChainID)
	output.Info("Network:      %s", d.Metadata.NetworkSource)
	output.Info("Mode:         %s", d.Metadata.ExecutionMode)
	if d.Metadata.DockerImage != "" {
		output.Info("Docker Image: %s", d.Metadata.DockerImage)
	}
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
		Status:      "success",
		ChainID:     d.Metadata.ChainID,
		Network:     d.Metadata.NetworkSource,
		Mode:        string(d.Metadata.ExecutionMode),
		DockerImage: d.Metadata.DockerImage,
		Validators:  d.Metadata.NumValidators,
		Nodes:       make([]NodeResult, len(d.Nodes)),
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

// DockerImageSelectionResult holds the result of docker image selection.
type DockerImageSelectionResult struct {
	ImageTag  string // Selected image tag or full custom URL
	IsCustom  bool   // True if user entered a custom image URL
	FromCache bool   // True if versions were loaded from cache
}

// DefaultGHCRImage is the default GHCR image for stable.
const DefaultGHCRImage = "ghcr.io/stablelabs/stable"

// normalizeImageURL converts a tag-only input to a full GHCR URL.
// If the input already contains a registry (contains "/" or ":"), it returns as-is.
// Otherwise, it constructs: ghcr.io/stablelabs/stable:{tag}
func normalizeImageURL(image string) string {
	if image == "" {
		return ""
	}
	// If it looks like a full URL (contains "/" indicating a registry path), return as-is
	if strings.Contains(image, "/") {
		return image
	}
	// Otherwise, treat it as a tag and construct GHCR URL
	return fmt.Sprintf("%s:%s", DefaultGHCRImage, image)
}

// resolveDockerImage determines the docker image to use based on priority:
// 1. --image flag (highest priority)
// 2. Interactive selection (if in interactive mode and docker mode)
// 3. Default latest tag (for non-interactive docker mode)
// Returns the resolved image URL and any error.
func resolveDockerImage(ctx context.Context, cmd *cobra.Command, isInteractive bool) (string, error) {
	// Priority 1: --image flag was explicitly provided
	if cmd.Flags().Changed("image") && startImage != "" {
		return normalizeImageURL(startImage), nil
	}

	// Priority 2: Interactive mode - prompt user to select
	if isInteractive && startMode == "docker" {
		imageSelection, err := runDockerImageSelection(ctx)
		if err != nil {
			return "", err
		}
		// If custom image, user provided full URL; otherwise construct GHCR URL
		if imageSelection.IsCustom {
			return imageSelection.ImageTag, nil
		}
		return fmt.Sprintf("%s:%s", DefaultGHCRImage, imageSelection.ImageTag), nil
	}

	// Priority 3: Non-interactive mode with --image flag (but not explicitly changed)
	// Use the provided value or fall back to empty (will use default in devnet package)
	if startImage != "" {
		return normalizeImageURL(startImage), nil
	}

	// No image specified - return empty to use default behavior
	return "", nil
}

// DefaultDockerPackage is the default container package name for docker images.
const DefaultDockerPackage = "stable"

// runDockerImageSelection prompts the user to select a docker image version.
func runDockerImageSelection(ctx context.Context) (*DockerImageSelectionResult, error) {
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

	// Fetch available docker image versions
	versions, fromCache, err := client.GetImageVersionsWithCache(ctx, DefaultDockerPackage)
	if err != nil {
		// Check if it's a warning (stale data)
		if warning, ok := err.(*github.StaleDataWarning); ok {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning.Message)
		} else {
			return nil, fmt.Errorf("failed to fetch docker image versions: %w", err)
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no docker image versions available. Check your network connection or GitHub token")
	}

	if fromCache {
		fmt.Println("(Using cached docker image data)")
	}

	// Run the interactive selection
	imageTag, isCustom, err := interactive.SelectDockerImage(versions)
	if err != nil {
		return nil, err
	}

	return &DockerImageSelectionResult{
		ImageTag:  imageTag,
		IsCustom:  isCustom,
		FromCache: fromCache,
	}, nil
}
