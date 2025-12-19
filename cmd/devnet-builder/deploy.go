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
	"github.com/stablelabs/stable-devnet/internal/network"
	"github.com/stablelabs/stable-devnet/internal/output"
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

Examples:
  # Deploy with default settings (4 validators, mainnet, docker mode)
  devnet-builder deploy

  # Deploy with testnet data
  devnet-builder deploy --network testnet

  # Deploy with 2 validators
  devnet-builder deploy --validators 2

  # Deploy with local binary mode
  devnet-builder deploy --mode local

  # Deploy with specific stable version
  devnet-builder deploy --stable-version v1.2.3`,
		RunE: runDeploy,
	}

	// Command flags
	cmd.Flags().StringVarP(&deployNetwork, "network", "n", "mainnet",
		"Network source (mainnet, testnet)")
	cmd.Flags().IntVar(&deployValidators, "validators", 4,
		"Number of validators (1-4)")
	cmd.Flags().StringVarP(&deployMode, "mode", "m", "docker",
		"Execution mode (docker, local)")
	cmd.Flags().StringVar(&deployStableVersion, "stable-version", "latest",
		"Stable repository version")
	cmd.Flags().BoolVar(&deployNoCache, "no-cache", false,
		"Skip snapshot cache")
	cmd.Flags().IntVar(&deployAccounts, "accounts", 0,
		"Additional funded accounts")

	// Interactive mode flags
	cmd.Flags().BoolVar(&deployNoInteractive, "no-interactive", false,
		"Disable interactive mode (use flags instead)")
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

	return cmd
}

func runDeploy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Apply config.toml values (priority: default < config.toml < env < flag)
	fileCfg := GetLoadedFileConfig()
	if fileCfg != nil {
		// Apply config file values if flags not explicitly set
		if !cmd.Flags().Changed("network") && fileCfg.Network != nil {
			deployNetwork = *fileCfg.Network
		}
		if !cmd.Flags().Changed("blockchain") && fileCfg.BlockchainNetwork != nil {
			deployBlockchainNetwork = *fileCfg.BlockchainNetwork
		}
		if !cmd.Flags().Changed("validators") && fileCfg.Validators != nil {
			deployValidators = *fileCfg.Validators
		}
		if !cmd.Flags().Changed("mode") && fileCfg.Mode != nil {
			deployMode = *fileCfg.Mode
		}
		if !cmd.Flags().Changed("stable-version") && fileCfg.StableVersion != nil {
			deployStableVersion = *fileCfg.StableVersion
		}
		if !cmd.Flags().Changed("no-cache") && fileCfg.NoCache != nil {
			deployNoCache = *fileCfg.NoCache
		}
		if !cmd.Flags().Changed("accounts") && fileCfg.Accounts != nil {
			deployAccounts = *fileCfg.Accounts
		}
	}

	// Apply environment variable defaults (override config.toml, but not explicit flags)
	if network := os.Getenv("STABLE_DEVNET_NETWORK"); network != "" && !cmd.Flags().Changed("network") {
		deployNetwork = network
	}
	if mode := os.Getenv("STABLE_DEVNET_MODE"); mode != "" && !cmd.Flags().Changed("mode") {
		deployMode = mode
	}
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		deployStableVersion = version
	}

	// Track if versions are custom refs
	var exportIsCustomRef bool
	var exportVersion string
	var startVersion string
	var dockerImage string

	// Determine if running in interactive mode
	isInteractive := !deployNoInteractive && !jsonMode

	// Docker mode uses GHCR package versions, not GitHub releases
	if deployMode == "docker" {
		resolvedImage, err := resolveDeployDockerImage(ctx, cmd, isInteractive)
		if err != nil {
			if interactive.IsCancellation(err) {
				fmt.Println("Operation cancelled.")
				return nil
			}
			return err
		}
		dockerImage = resolvedImage
		exportVersion = deployStableVersion
		startVersion = deployStableVersion
	} else {
		// Local mode: run interactive selection flow for GitHub releases
		if isInteractive {
			selection, err := runDeployInteractiveSelection(ctx, cmd)
			if err != nil {
				if interactive.IsCancellation(err) {
					fmt.Println("Operation cancelled.")
					return nil
				}
				return err
			}
			deployNetwork = selection.Network
			exportVersion = selection.ExportVersion
			exportIsCustomRef = selection.ExportIsCustomRef
			startVersion = selection.StartVersion
			deployStableVersion = exportVersion
		} else {
			exportVersion = deployStableVersion
			startVersion = deployStableVersion
		}
	}

	// Validate inputs
	if deployNetwork != "mainnet" && deployNetwork != "testnet" {
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", deployNetwork)
	}
	if deployValidators < 1 || deployValidators > 4 {
		return fmt.Errorf("invalid validators: %d (must be 1-4)", deployValidators)
	}
	if deployMode != "docker" && deployMode != "local" {
		return fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", deployMode)
	}
	// Validate blockchain network module exists
	if !network.Has(deployBlockchainNetwork) {
		available := network.List()
		return fmt.Errorf("unknown blockchain network: %s (available: %v)", deployBlockchainNetwork, available)
	}

	// Check if devnet already exists
	if devnet.DevnetExists(homeDir) {
		return fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder destroy' to remove it first", homeDir)
	}

	// Build binary for local mode (all versions need to be built/cached)
	var customBinaryPath string
	if deployMode == "local" {
		networkModule, _ := network.Get(deployBlockchainNetwork)
		b := builder.NewBuilder(homeDir, logger, networkModule)
		logger.Info("Building binary from source (ref: %s)...", startVersion)
		result, err := b.Build(ctx, builder.BuildOptions{
			Ref:     startVersion,
			Network: deployNetwork,
		})
		if err != nil {
			return fmt.Errorf("failed to build from source: %w", err)
		}
		customBinaryPath = result.BinaryPath
		logger.Success("Binary built: %s (commit: %s)", result.BinaryPath, result.CommitHash)
	}

	// Store start version info in metadata for reference
	_ = startVersion
	_ = exportIsCustomRef

	// Phase 1: Provision (create config, generate validators)
	provisionOpts := devnet.ProvisionOptions{
		HomeDir:           homeDir,
		Network:           deployNetwork,
		BlockchainNetwork: deployBlockchainNetwork,
		NumValidators:     deployValidators,
		NumAccounts:       deployAccounts,
		Mode:              devnet.ExecutionMode(deployMode),
		StableVersion:     exportVersion,
		DockerImage:       dockerImage,
		NoCache:           deployNoCache,
		Logger:            logger,
	}

	_, err := devnet.Provision(ctx, provisionOpts)
	if err != nil {
		if jsonMode {
			return outputDeployError(err)
		}
		return err
	}

	// Phase 2: Run (start nodes)
	// For local mode, we always build/cache binary, so always use custom path
	useCustomBinary := deployMode == "local" && customBinaryPath != ""
	runOpts := devnet.RunOptions{
		HomeDir:          homeDir,
		Mode:             devnet.ExecutionMode(deployMode),
		StableVersion:    exportVersion,
		HealthTimeout:    devnet.HealthCheckTimeout,
		Logger:           logger,
		IsCustomRef:      useCustomBinary,
		CustomBinaryPath: customBinaryPath,
	}

	runResult, err := devnet.Run(ctx, runOpts)
	if err != nil {
		if runResult != nil && runResult.Devnet != nil {
			if jsonMode {
				return outputDeployJSONFromRunResult(runResult, err)
			}
		}
		if jsonMode {
			return outputDeployError(err)
		}
		return err
	}

	// Output result
	if jsonMode {
		return outputDeployJSON(runResult.Devnet)
	}
	return outputDeployText(runResult.Devnet)
}

func outputDeployJSONFromRunResult(result *devnet.RunResult, err error) error {
	jsonResult := DeployResult{
		Status:            "partial",
		ChainID:           result.Devnet.Metadata.ChainID,
		Network:           result.Devnet.Metadata.NetworkSource,
		BlockchainNetwork: result.Devnet.Metadata.BlockchainNetwork,
		Mode:              string(result.Devnet.Metadata.ExecutionMode),
		DockerImage:       result.Devnet.Metadata.DockerImage,
		Validators:        result.Devnet.Metadata.NumValidators,
		Nodes:             make([]NodeResult, len(result.Devnet.Nodes)),
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

func outputDeployText(d *devnet.Devnet) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", d.Metadata.ChainID)
	output.Info("Network:      %s", d.Metadata.NetworkSource)
	output.Info("Blockchain:   %s", d.Metadata.BlockchainNetwork)
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

func outputDeployJSON(d *devnet.Devnet) error {
	result := DeployResult{
		Status:            "success",
		ChainID:           d.Metadata.ChainID,
		Network:           d.Metadata.NetworkSource,
		BlockchainNetwork: d.Metadata.BlockchainNetwork,
		Mode:              string(d.Metadata.ExecutionMode),
		DockerImage:       d.Metadata.DockerImage,
		Validators:        d.Metadata.NumValidators,
		Nodes:             make([]NodeResult, len(d.Nodes)),
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

func outputDeployError(err error) error {
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
func runDeployInteractiveSelection(ctx context.Context, cmd *cobra.Command) (*interactive.SelectionConfig, error) {
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
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		clientOpts = append(clientOpts, github.WithToken(*fileCfg.GitHubToken))
	}
	client := github.NewClient(clientOpts...)

	selector := interactive.NewSelector(client)
	return selector.RunSelectionFlow(ctx)
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
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		clientOpts = append(clientOpts, github.WithToken(*fileCfg.GitHubToken))
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
