package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/di"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/github"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	deployNetwork           string
	deployBlockchainNetwork string // Network module selection (stable, ault, etc.)
	deployValidators        int
	deployMode              string
	deployStableVersion     string
	deployNoCache           bool
	deployAccounts          int
	deployNoInteractive     bool
	deployExportVersion     string
	deployStartVersion      string
	deployImage             string
	deployFork              bool // Fork live network state via snapshot export
	deployTestMnemonic      bool // Use deterministic test mnemonics for validators
)

// DeployResult represents the JSON output for the deploy command.
type DeployResult struct {
	Status            string       `json:"status"`
	ChainID           string       `json:"chain_id"`
	Network           string       `json:"network"`            // Snapshot source: mainnet/testnet
	BlockchainNetwork string       `json:"blockchain_network"` // Network module: stable/ault
	Mode              string       `json:"mode"`
	DockerImage       string       `json:"docker_image,omitempty"`
	Validators        int          `json:"validators"`
	Nodes             []NodeResult `json:"nodes"`
}

func NewDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a local devnet (provision + start)",
		Long: `Deploy a local devnet with configurable validators and network source.

This command will:
1. Check prerequisites (Docker/local binary, curl, jq, zstd/lz4)
2. Download or use cached snapshot from the specified network
3. Export genesis state from snapshot
4. Generate devnet configuration for all validators
5. Start all validator nodes

Docker mode supports 1-100 validators with network isolation and automatic port management.
Local mode supports 1-4 validators for testing and development.

Examples:
  # Deploy with default settings (4 validators, mainnet, docker mode)
  devnet-builder deploy

  # Deploy with testnet data
  devnet-builder deploy --network testnet

  # Deploy with 10 validators in docker mode (isolated network, auto port allocation)
  devnet-builder deploy --validators 10

  # Deploy with 2 validators in local binary mode
  devnet-builder deploy --mode local --validators 2

  # Deploy with specific stable version
  devnet-builder deploy --stable-version v1.2.3

  # Large-scale deployment with 100 validators (docker mode only)
  devnet-builder deploy --validators 100`,
		RunE: runDeploy,
	}

	// Command flags
	cmd.Flags().StringVarP(&deployNetwork, "network", "n", "mainnet",
		"Network source (mainnet, testnet)")
	cmd.Flags().IntVar(&deployValidators, "validators", 4,
		"Number of validators (1-100 for docker mode, 1-4 for local mode)")
	cmd.Flags().StringVarP(&deployMode, "mode", "m", "docker",
		"Execution mode (docker, local)")
	cmd.Flags().StringVar(&deployStableVersion, "stable-version", "latest",
		"Stable repository version")
	cmd.Flags().BoolVar(&deployNoCache, "no-cache", false,
		"Skip snapshot cache")
	cmd.Flags().IntVar(&deployAccounts, "accounts", 4,
		"Additional funded accounts")
	cmd.Flags().BoolVar(&deployTestMnemonic, "test-mnemonic", true,
		"Use deterministic test mnemonics for validators (disable for production-like testing)")

	// Interactive mode flags (controls version/docker image selection prompts)
	// Note: Base config prompts (network, validators, mode) are handled by config.toml
	cmd.Flags().BoolVar(&deployNoInteractive, "no-interactive", false,
		"Disable version selection prompts (use --export-version, --start-version, --image instead)")
	cmd.Flags().StringVar(&deployExportVersion, "export-version", "",
		"Version for genesis export (non-interactive mode)")
	cmd.Flags().StringVar(&deployStartVersion, "start-version", "",
		"Version for node start (non-interactive mode)")

	// Docker image flag
	cmd.Flags().StringVar(&deployImage, "image", "",
		"Docker image for docker mode (e.g., v1.0.0 or ghcr.io/org/image:tag)")

	// Blockchain network module flag
	cmd.Flags().StringVar(&deployBlockchainNetwork, "blockchain", "stable",
		"Blockchain network module (stable, ault)")

	// Fork mode flag - exports genesis from snapshot state instead of RPC genesis
	cmd.Flags().BoolVar(&deployFork, "fork", true,
		"Fork live network state (export genesis from snapshot)")

	return cmd
}

func runDeploy(cmd *cobra.Command, args []string) error {
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
		fileCfg.Network = &deployNetwork
	}
	if cmd.Flags().Changed("blockchain") {
		fileCfg.BlockchainNetwork = &deployBlockchainNetwork
	}
	if cmd.Flags().Changed("validators") {
		fileCfg.Validators = &deployValidators
	}
	if cmd.Flags().Changed("mode") {
		fileCfg.Mode = &deployMode
	}
	if cmd.Flags().Changed("stable-version") {
		fileCfg.StableVersion = &deployStableVersion
	}
	if cmd.Flags().Changed("no-cache") {
		fileCfg.NoCache = &deployNoCache
	}
	if cmd.Flags().Changed("accounts") {
		fileCfg.Accounts = &deployAccounts
	}

	// Apply environment variables (env overrides config.toml but not flags)
	if networkEnv := os.Getenv("STABLE_DEVNET_NETWORK"); networkEnv != "" && !cmd.Flags().Changed("network") {
		fileCfg.Network = &networkEnv
	}
	if modeEnv := os.Getenv("STABLE_DEVNET_MODE"); modeEnv != "" && !cmd.Flags().Changed("mode") {
		fileCfg.Mode = &modeEnv
	}
	if versionEnv := os.Getenv("STABLE_VERSION"); versionEnv != "" && !cmd.Flags().Changed("stable-version") {
		fileCfg.StableVersion = &versionEnv
	}

	// Run partial interactive setup for missing base config values
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
	deployNetwork = *effectiveCfg.Network
	deployBlockchainNetwork = *effectiveCfg.BlockchainNetwork
	deployValidators = *effectiveCfg.Validators
	deployMode = *effectiveCfg.Mode
	if effectiveCfg.StableVersion != nil {
		deployStableVersion = *effectiveCfg.StableVersion
	}
	if effectiveCfg.NoCache != nil {
		deployNoCache = *effectiveCfg.NoCache
	}
	if effectiveCfg.Accounts != nil {
		deployAccounts = *effectiveCfg.Accounts
	}

	// Track if versions are custom refs
	var exportVersion string
	var startVersion string
	var dockerImage string

	// Determine if running in interactive mode for version selection
	// Note: Base config interactive prompts are handled above via RunPartial
	isInteractive := !deployNoInteractive && !jsonMode

	// Docker mode uses GHCR package versions, not GitHub releases
	if deployMode == "docker" {
		resolvedImage, err := resolveDeployDockerImage(ctx, cmd, isInteractive)
		if err != nil {
			return wrapInteractiveError(cmd, err, "failed to resolve docker image")
		}
		dockerImage = resolvedImage
		exportVersion = deployStableVersion
		startVersion = deployStableVersion
	} else {
		// Local mode: run interactive selection flow for GitHub releases
		if isInteractive {
			selection, err := runDeployInteractiveSelection(ctx, cmd, deployNetwork)
			if err != nil {
				return wrapInteractiveError(cmd, err, "failed to fetch versions")
			}
			exportVersion = selection.ExportVersion
			startVersion = selection.StartVersion
			deployStableVersion = exportVersion
		} else {
			// Non-interactive: use explicit flags if provided, otherwise fall back to --stable-version
			if deployExportVersion != "" {
				exportVersion = deployExportVersion
			} else {
				exportVersion = deployStableVersion
			}
			if deployStartVersion != "" {
				startVersion = deployStartVersion
			} else {
				startVersion = deployStableVersion
			}
		}
	}

	// Validate inputs
	if deployNetwork != "mainnet" && deployNetwork != "testnet" {
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", deployNetwork)
	}
	// Validate validator count based on mode
	if deployMode == "docker" {
		if deployValidators < 1 || deployValidators > 100 {
			return fmt.Errorf("invalid validators: %d (must be 1-100 for docker mode)", deployValidators)
		}
	} else if deployMode == "local" {
		if deployValidators < 1 || deployValidators > 4 {
			return fmt.Errorf("invalid validators: %d (must be 1-4 for local mode)", deployValidators)
		}
	} else {
		return fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", deployMode)
	}
	// Validate blockchain network module exists
	if !network.Has(deployBlockchainNetwork) {
		available := network.List()
		return fmt.Errorf("unknown blockchain network: %s (available: %v)", deployBlockchainNetwork, available)
	}

	// Get network module for DI container
	networkModule, err := network.Get(deployBlockchainNetwork)
	if err != nil {
		return fmt.Errorf("failed to get network module: %w", err)
	}

	// Check if devnet already exists
	svc, err := getCleanServiceWithConfig(CleanServiceConfig{
		NetworkModule: networkModule,
		DockerMode:    deployMode == "docker",
	})
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}
	if svc.DevnetExists() {
		return fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder destroy' to remove it first", homeDir)
	}

	// Build binary for local mode (all versions need to be built/cached)
	var customBinaryPath string
	if deployMode == "local" {
		buildResult, err := buildBinaryForDeploy(ctx, deployBlockchainNetwork, startVersion, deployNetwork, logger)
		if err != nil {
			return fmt.Errorf("failed to build from source: %w", err)
		}
		customBinaryPath = buildResult.BinaryPath
		logger.Success("Binary built: %s (commit: %s)", buildResult.BinaryPath, buildResult.CommitHash)
	}

	// Phase 1: Provision using CleanDevnetService
	provisionInput := dto.ProvisionInput{
		HomeDir:           homeDir,
		Network:           deployNetwork,
		BlockchainNetwork: deployBlockchainNetwork,
		NumValidators:     deployValidators,
		NumAccounts:       deployAccounts,
		Mode:              deployMode,
		StableVersion:     exportVersion,
		DockerImage:       dockerImage,
		NoCache:           deployNoCache,
		CustomBinaryPath:  customBinaryPath,
		UseSnapshot:       deployFork,
		BinaryPath:        customBinaryPath,
		UseTestMnemonic:   deployTestMnemonic,
	}

	_, err = svc.Provision(ctx, provisionInput)
	if err != nil {
		if jsonMode {
			return outputDeployErrorClean(err)
		}
		return err
	}

	// Phase 2: Run using CleanDevnetService
	runResult, err := svc.Start(ctx, 5*time.Minute)
	if err != nil {
		if jsonMode {
			return outputDeployErrorClean(err)
		}
		return err
	}

	// Get devnet info for output
	devnetInfo, _ := svc.LoadDevnetInfo(ctx)

	// Output result
	if jsonMode {
		return outputDeployJSONClean(runResult, devnetInfo)
	}
	return outputDeployTextClean(runResult, devnetInfo)
}

func outputDeployTextClean(result *dto.RunOutput, devnetInfo *dto.DevnetInfo) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", devnetInfo.ChainID)
	output.Info("Network:      %s", devnetInfo.NetworkSource)
	output.Info("Blockchain:   %s", devnetInfo.BlockchainNetwork)
	output.Info("Mode:         %s", devnetInfo.ExecutionMode)
	if devnetInfo.DockerImage != "" {
		output.Info("Docker Image: %s", devnetInfo.DockerImage)
	}
	output.Info("Validators:   %d", devnetInfo.NumValidators)
	fmt.Println()
	output.Bold("Endpoints:")

	for _, n := range devnetInfo.Nodes {
		status := "running"
		for _, ns := range result.Nodes {
			if ns.Index == n.Index && !ns.IsRunning {
				status = "failed"
				break
			}
		}
		if status == "running" {
			fmt.Printf("  Node %d: %s (RPC) | %s (EVM)\n",
				n.Index, n.RPCURL, n.EVMURL)
		} else {
			fmt.Printf("  Node %d: [FAILED]\n", n.Index)
		}
	}

	return nil
}

func outputDeployJSONClean(result *dto.RunOutput, devnetInfo *dto.DevnetInfo) error {
	jsonResult := DeployResult{
		Status:            "success",
		ChainID:           devnetInfo.ChainID,
		Network:           devnetInfo.NetworkSource,
		BlockchainNetwork: devnetInfo.BlockchainNetwork,
		Mode:              devnetInfo.ExecutionMode,
		DockerImage:       devnetInfo.DockerImage,
		Validators:        devnetInfo.NumValidators,
		Nodes:             make([]NodeResult, len(devnetInfo.Nodes)),
	}

	if !result.AllRunning {
		jsonResult.Status = "partial"
	}

	for i, n := range devnetInfo.Nodes {
		status := "running"
		for _, ns := range result.Nodes {
			if ns.Index == n.Index && !ns.IsRunning {
				status = "failed"
				break
			}
		}
		jsonResult.Nodes[i] = NodeResult{
			Index:  n.Index,
			RPC:    n.RPCURL,
			EVMRPC: n.EVMURL,
			Status: status,
		}
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputDeployErrorClean(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    getErrorCode(err),
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}

// runDeployInteractiveSelection runs the interactive version selection flow.
// Network is already determined from config, so only versions are selected interactively.
func runDeployInteractiveSelection(ctx context.Context, cmd *cobra.Command, network string) (*interactive.SelectionConfig, error) {
	fileCfg := GetLoadedFileConfig()

	cacheTTL := github.DefaultCacheTTL
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		if ttl, err := time.ParseDuration(*fileCfg.CacheTTL); err == nil {
			cacheTTL = ttl
		}
	}
	cacheManager := github.NewCacheManager(homeDir, cacheTTL)

	clientOpts := []github.ClientOption{
		github.WithCache(cacheManager),
	}

	// Resolve GitHub token from keychain, environment, or config file
	if token, found := resolveGitHubToken(fileCfg); found {
		clientOpts = append(clientOpts, github.WithToken(token))
	}

	client := github.NewClient(clientOpts...)

	selector := interactive.NewSelector(client)
	// Use RunVersionSelectionFlow to skip network selection (network already determined)
	return selector.RunVersionSelectionFlow(ctx, network)
}

// resolveDeployDockerImage determines the docker image to use.
func resolveDeployDockerImage(ctx context.Context, cmd *cobra.Command, isInteractive bool) (string, error) {
	// Priority 1: --image flag was explicitly provided
	if cmd.Flags().Changed("image") && deployImage != "" {
		return normalizeImageURL(deployImage), nil
	}

	// Priority 2: Interactive mode - prompt user to select
	if isInteractive && deployMode == "docker" {
		imageSelection, err := runDeployDockerImageSelection(ctx)
		if err != nil {
			return "", err
		}
		if imageSelection.IsCustom {
			return imageSelection.ImageTag, nil
		}
		return fmt.Sprintf("%s:%s", DefaultGHCRImage, imageSelection.ImageTag), nil
	}

	// Priority 3: Non-interactive mode with --image flag
	if deployImage != "" {
		return normalizeImageURL(deployImage), nil
	}

	return "", nil
}

// runDeployDockerImageSelection prompts the user to select a docker image version.
func runDeployDockerImageSelection(ctx context.Context) (*DockerImageSelectionResult, error) {
	fileCfg := GetLoadedFileConfig()

	cacheTTL := github.DefaultCacheTTL
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		if ttl, err := time.ParseDuration(*fileCfg.CacheTTL); err == nil {
			cacheTTL = ttl
		}
	}
	cacheManager := github.NewCacheManager(homeDir, cacheTTL)

	clientOpts := []github.ClientOption{
		github.WithCache(cacheManager),
	}

	// Resolve GitHub token from keychain, environment, or config file
	if token, found := resolveGitHubToken(fileCfg); found {
		clientOpts = append(clientOpts, github.WithToken(token))
	}

	client := github.NewClient(clientOpts...)

	versions, fromCache, err := client.GetImageVersionsWithCache(ctx, DefaultDockerPackage)
	if err != nil {
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

// buildBinaryForDeploy builds a binary using DI container and BuildUseCase.
// This replaces direct usage of the legacy builder package.
func buildBinaryForDeploy(ctx context.Context, blockchainNetwork, ref, networkType string, logger *output.Logger) (*dto.BuildOutput, error) {
	// Get network module
	networkModule, err := network.Get(blockchainNetwork)
	if err != nil {
		return nil, fmt.Errorf("failed to get network module: %w", err)
	}

	// Create DI factory with network module
	factory := di.NewInfrastructureFactory(homeDir, logger).
		WithNetworkModule(networkModule)

	// Wire container
	container, err := factory.WireContainer()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	logger.Info("Building binary from source (ref: %s)...", ref)

	// Execute BuildUseCase
	// Note: ToCache=false means Build() is called which creates the symlink
	// at ~/.devnet-builder/bin/stabled, required for key creation and node init
	return container.BuildUseCase().Execute(ctx, dto.BuildInput{
		Ref:      ref,
		Network:  networkType,
		UseCache: true,  // Check cache first
		ToCache:  false, // Use Build() to create symlink at bin/stabled
	})
}
