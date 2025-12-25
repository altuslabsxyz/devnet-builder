package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/di"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/github"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Default upgrade constants
const (
	DefaultHeightBuffer  = 0 // 0 = auto-calculate based on block time
	DefaultVotingPeriod  = 60 * time.Second
)

// ExecutionMode for upgrade command
type UpgradeExecutionMode string

const (
	UpgradeModeDocker UpgradeExecutionMode = "docker"
	UpgradeModeLocal  UpgradeExecutionMode = "local"
)

// Upgrade command flags
var (
	upgradeName          string
	upgradeImage         string
	upgradeBinary        string
	upgradeMode          string
	votingPeriod         string
	heightBuffer         int
	upgradeHeight        int64
	exportGenesis        bool
	genesisDir           string
	upgradeNoInteractive bool
	upgradeVersion       string
)

// NewUpgradeCmd creates the upgrade command.
func NewUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Perform a software upgrade on the running devnet",
		Long: `Perform a software upgrade on the running devnet using Cosmos SDK governance.

This command automates the complete upgrade process:
  1. Submit an expedited upgrade proposal
  2. Vote YES from all validators
  3. Wait for the upgrade height
  4. Switch to the new binary
  5. Verify chain resumes

Examples:
  # Upgrade to a new Docker image
  devnet-builder upgrade --name v2.0.0-upgrade --image ghcr.io/stablelabs/stable:v2.0.0

  # Upgrade to a local binary
  devnet-builder upgrade --name v2.0.0-upgrade --binary /path/to/new/stabled

  # Upgrade with custom voting period
  devnet-builder upgrade --name v2.0.0-upgrade --image ghcr.io/stablelabs/stable:v2.0.0 --voting-period 120s

  # Upgrade and export genesis snapshots
  devnet-builder upgrade --name v2.0.0-upgrade --image ghcr.io/stablelabs/stable:v2.0.0 --export-genesis

  # Interactive mode (default) - select version interactively
  devnet-builder upgrade

  # Non-interactive mode with explicit version
  devnet-builder upgrade --no-interactive --name v2.0.0-upgrade --version v2.0.0`,
		RunE: runUpgrade,
	}

	// Version selection flags
	cmd.Flags().StringVarP(&upgradeName, "name", "n", "", "Upgrade handler name")
	cmd.Flags().StringVarP(&upgradeImage, "image", "i", "", "Target Docker image for upgrade")
	cmd.Flags().StringVarP(&upgradeBinary, "binary", "b", "", "Target local binary path for upgrade")
	cmd.Flags().StringVar(&upgradeVersion, "version", "", "Target version (tag or branch/commit for building)")
	cmd.Flags().StringVarP(&upgradeMode, "mode", "m", "", "Execution mode: docker or local (default: from devnet metadata)")

	// Interactive mode flags
	cmd.Flags().BoolVar(&upgradeNoInteractive, "no-interactive", false, "Disable interactive mode")

	// Optional flags
	cmd.Flags().StringVar(&votingPeriod, "voting-period", "60s", "Expedited voting period duration")
	cmd.Flags().IntVar(&heightBuffer, "height-buffer", DefaultHeightBuffer, "Blocks to add after voting period ends (0 = auto-calculate based on block time)")
	cmd.Flags().Int64Var(&upgradeHeight, "upgrade-height", 0, "Explicit upgrade height (0 = auto-calculate)")
	cmd.Flags().BoolVar(&exportGenesis, "export-genesis", false, "Export genesis before and after upgrade")
	cmd.Flags().StringVar(&genesisDir, "genesis-dir", "", "Directory for genesis exports (default: <home>/devnet/genesis-snapshots)")

	return cmd
}

// UpgradeResultJSON represents the JSON output for the upgrade command.
type UpgradeResultJSON struct {
	Status            string `json:"status"`
	UpgradeName       string `json:"upgrade_name"`
	ProposalID        uint64 `json:"proposal_id"`
	UpgradeHeight     int64  `json:"upgrade_height"`
	PostUpgradeHeight int64  `json:"post_upgrade_height"`
	NewBinary         string `json:"new_binary"`
	Duration          string `json:"duration"`
	PreGenesisPath    string `json:"pre_genesis_path,omitempty"`
	PostGenesisPath   string `json:"post_genesis_path,omitempty"`
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	// Set up signal handling for graceful cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println()
		output.Warn("Upgrade interrupted. Current state:")
		if lastUpgradeStage != "" {
			fmt.Printf("  Last stage: %s\n", lastUpgradeStage)
		}
		output.Warn("The devnet may be in an intermediate state.")
		output.Info("Run 'devnet-builder status' to check chain health.")
		cancel()
	}()

	logger := output.DefaultLogger

	// Initialize CleanDevnetService for existence and status checks
	svc, err := getCleanService()
	if err != nil {
		return outputUpgradeError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Check if devnet exists using CleanDevnetService
	if !svc.DevnetExists() {
		err := fmt.Errorf("no devnet found at %s", homeDir)
		if jsonMode {
			return outputUpgradeError(err)
		}
		return err
	}

	// Load metadata via CleanDevnetService for status check
	cleanMetadata, err := svc.LoadMetadata(ctx)
	if err != nil {
		if jsonMode {
			return outputUpgradeError(err)
		}
		return err
	}

	if cleanMetadata.Status != ports.StateRunning {
		if jsonMode {
			return outputUpgradeError(fmt.Errorf("devnet is not running"))
		}
		return fmt.Errorf("devnet is not running\nStart it with 'devnet-builder start'")
	}

	// Get network module for binary name
	networkModule, err := network.Get(cleanMetadata.BlockchainNetwork)
	if err != nil {
		return outputUpgradeError(fmt.Errorf("failed to get network module: %w", err))
	}

	// Resolve execution mode: flag > metadata default
	resolvedMode := UpgradeExecutionMode(cleanMetadata.ExecutionMode)
	modeExplicitlySet := false
	if upgradeMode != "" {
		switch UpgradeExecutionMode(upgradeMode) {
		case UpgradeModeDocker, UpgradeModeLocal:
			resolvedMode = UpgradeExecutionMode(upgradeMode)
			modeExplicitlySet = true
		default:
			return fmt.Errorf("invalid mode %q: must be 'docker' or 'local'", upgradeMode)
		}
	}

	// Mode validation against --image/--binary flags
	if !jsonMode {
		if resolvedMode == UpgradeModeDocker && upgradeBinary != "" && !modeExplicitlySet {
			output.Warn("Devnet was started in docker mode but --binary was provided.")
			output.Warn("Use --image for docker mode, or --mode local to switch modes.")
		}
		if resolvedMode == UpgradeModeLocal && upgradeImage != "" && !modeExplicitlySet {
			output.Warn("Devnet was started in local mode but --image was provided.")
			output.Warn("Use --binary for local mode, or --mode docker to switch modes.")
		}
		if modeExplicitlySet && resolvedMode != UpgradeExecutionMode(cleanMetadata.ExecutionMode) {
			output.Warn("Switching execution mode from %s to %s.", cleanMetadata.ExecutionMode, resolvedMode)
			output.Warn("The devnet will continue in %s mode after this upgrade.", resolvedMode)
		}
	}

	// Track selected version and name
	var selectedVersion string
	var selectedName string

	// Interactive mode: run selection flow if not disabled
	if !upgradeNoInteractive && !jsonMode && upgradeImage == "" && upgradeBinary == "" {
		selection, err := runUpgradeInteractiveSelection(ctx, cmd)
		if err != nil {
			if interactive.IsCancellation(err) {
				fmt.Println("Operation cancelled.")
				return nil
			}
			return err
		}
		selectedName = selection.UpgradeName
		selectedVersion = selection.UpgradeVersion
	} else {
		// Non-interactive mode
		selectedName = upgradeName
		selectedVersion = upgradeVersion
	}

	// Validate that we have either image, binary, or version to build
	if upgradeImage == "" && upgradeBinary == "" && selectedVersion == "" {
		return fmt.Errorf("either --image, --binary, or --version must be provided (or use interactive mode)")
	}
	if upgradeImage != "" && upgradeBinary != "" {
		return fmt.Errorf("cannot specify both --image and --binary")
	}

	// Validate that name is provided
	if selectedName == "" {
		return fmt.Errorf("upgrade name is required (--name or interactive mode)")
	}

	// Mode-aware version resolution
	var cachedBuildResult *dto.BuildOutput
	var versionResolvedImage string

	if selectedVersion != "" && upgradeImage == "" && upgradeBinary == "" {
		if resolvedMode == UpgradeModeDocker && isStandardVersionTag(selectedVersion) {
			// Docker mode with standard version tag: resolve to docker image
			dockerImage := networkModule.DockerImage()
			versionResolvedImage = fmt.Sprintf("%s:%s", dockerImage, selectedVersion)
			logger.Info("Using docker image for version %s: %s", selectedVersion, versionResolvedImage)
		} else {
			// Local mode or custom ref: build local binary to cache using DI container
			buildResult, err := buildBinaryForUpgrade(ctx, cleanMetadata.BlockchainNetwork, selectedVersion, cleanMetadata.NetworkName, logger)
			if err != nil {
				return fmt.Errorf("failed to pre-build binary: %w", err)
			}
			cachedBuildResult = buildResult
			commitShort := buildResult.CommitHash
			if len(commitShort) > 12 {
				commitShort = commitShort[:12]
			}
			logger.Success("Binary pre-built and cached (commit: %s)", commitShort)
		}
	}

	// Get on-chain governance parameters
	logger.Info("Fetching governance parameters from chain...")
	rpcHost := "localhost"
	rpcPort := 26657
	tempFactory := di.NewInfrastructureFactory(homeDir, logger)
	rpcClient := tempFactory.CreateRPCClient(rpcHost, rpcPort)

	govParams, err := rpcClient.GetGovParams(ctx)
	if err != nil {
		logger.Debug("Failed to fetch gov params, using CLI flag value: %v", err)
		// Fallback to CLI flag if chain query fails
		vp, parseErr := time.ParseDuration(votingPeriod)
		if parseErr != nil {
			return fmt.Errorf("invalid voting period: %w", parseErr)
		}
		govParams = &ports.GovParams{
			ExpeditedVotingPeriod: vp,
		}
	}

	// Use expedited voting period from chain
	vp := govParams.ExpeditedVotingPeriod
	logger.Info("Using expedited voting period: %s", vp)

	// Determine target binary/image
	targetBinary := upgradeBinary
	targetImage := upgradeImage
	if versionResolvedImage != "" {
		targetImage = versionResolvedImage
	}

	// Print upgrade plan (non-JSON mode)
	if !jsonMode {
		printUpgradePlan(selectedName, string(resolvedMode), targetImage, targetBinary, cachedBuildResult, vp, cleanMetadata)
	}

	// Create DI container for upgrade
	factory := di.NewInfrastructureFactory(homeDir, logger).
		WithNetworkModule(networkModule).
		WithDockerMode(resolvedMode == UpgradeModeDocker)

	container, err := factory.WireContainer()
	if err != nil {
		return outputUpgradeError(fmt.Errorf("failed to initialize: %w", err))
	}

	// Build ExecuteUpgradeInput
	input := dto.ExecuteUpgradeInput{
		HomeDir:       homeDir,
		UpgradeName:   selectedName,
		TargetBinary:  targetBinary,
		TargetImage:   targetImage,
		TargetVersion: selectedVersion,
		VotingPeriod:  vp,
		HeightBuffer:  heightBuffer,
		UpgradeHeight: upgradeHeight,
		ExportGenesis: exportGenesis,
		GenesisDir:    genesisDir,
		Mode:          ports.ExecutionMode(resolvedMode),
	}

	// If we have a cached binary, use cache mode for atomic symlink switch
	if cachedBuildResult != nil {
		input.CachePath = cachedBuildResult.BinaryPath
		input.CommitHash = cachedBuildResult.CommitHash // Deprecated, kept for compatibility
		input.CacheRef = cachedBuildResult.CacheRef     // Use CacheRef for SetActive
		input.TargetBinary = "" // Clear since we're using cache
	}

	// Execute the upgrade using the UseCase
	if !jsonMode {
		fmt.Printf("[1/6] %s\n", color.CyanString("Verifying devnet status..."))
	}

	result, err := container.ExecuteUpgradeUseCase().Execute(ctx, input)
	if err != nil {
		if jsonMode {
			return outputUpgradeError(err)
		}
		return err
	}

	// Update metadata with new version if upgrade was successful
	if result.Success {
		cleanMetadata.CurrentVersion = selectedVersion
		cleanMetadata.ExecutionMode = ports.ExecutionMode(resolvedMode)
		if err := svc.SaveMetadata(ctx, cleanMetadata); err != nil {
			logger.Warn("Failed to update metadata: %v", err)
		}
	}

	// Output result
	if jsonMode {
		return outputUpgradeJSON(result)
	}
	return outputUpgradeText(result)
}

// runUpgradeInteractiveSelection runs the interactive version selection flow for upgrade.
func runUpgradeInteractiveSelection(ctx context.Context, cmd *cobra.Command) (*interactive.UpgradeSelectionConfig, error) {
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
	return selector.RunUpgradeSelectionFlow(ctx)
}

// isStandardVersionTag checks if a string looks like a standard version tag (e.g., v1.2.3).
func isStandardVersionTag(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Standard version tags start with 'v' followed by a digit
	if s[0] == 'v' && len(s) > 1 && s[1] >= '0' && s[1] <= '9' {
		return true
	}
	return false
}

func printUpgradePlan(name, mode, targetImage, targetBinary string, cached *dto.BuildOutput, votingPeriod time.Duration, metadata *ports.DevnetMetadata) {
	output.Bold("Upgrade Plan")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("Upgrade Name:     %s\n", name)
	fmt.Printf("Mode:             %s\n", mode)
	if targetImage != "" {
		fmt.Printf("Target Image:     %s\n", targetImage)
	} else if targetBinary != "" {
		fmt.Printf("Target Binary:    %s\n", targetBinary)
	} else if cached != nil {
		fmt.Printf("Target Binary:    %s (cached)\n", cached.BinaryPath)
	}
	fmt.Printf("Voting Period:    %s\n", votingPeriod)
	if heightBuffer == 0 {
		fmt.Printf("Height Buffer:    auto-calculate (based on block time)\n")
	} else {
		fmt.Printf("Height Buffer:    %d blocks (manual)\n", heightBuffer)
	}
	if upgradeHeight > 0 {
		fmt.Printf("Upgrade Height:   %d (explicit)\n", upgradeHeight)
	} else {
		fmt.Printf("Upgrade Height:   auto-calculate\n")
	}
	fmt.Printf("Validators:       %d\n", metadata.NumValidators)
	fmt.Println()
}

var lastUpgradeStage string

func outputUpgradeText(result *dto.ExecuteUpgradeOutput) error {
	if result.Error != nil {
		output.Error("Upgrade failed: %v", result.Error)
		return result.Error
	}

	fmt.Println()
	output.Success("Upgrade completed successfully!")
	fmt.Println()
	output.Bold("Upgrade Summary")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("  Status:           %s\n", color.GreenString("SUCCESS"))
	fmt.Printf("  Proposal ID:      %d\n", result.ProposalID)
	fmt.Printf("  Upgrade Height:   %d\n", result.UpgradeHeight)
	fmt.Printf("  Post-Upgrade:     %d (chain resumed)\n", result.PostUpgradeHeight)
	fmt.Printf("  New Binary:       %s\n", result.NewBinary)
	fmt.Printf("  Total Duration:   %s\n", result.Duration.Round(time.Second))

	// Show genesis export paths if available
	if result.PreGenesisPath != "" || result.PostGenesisPath != "" {
		fmt.Println()
		output.Bold("Genesis Snapshots:")
		if result.PreGenesisPath != "" {
			fmt.Printf("  Pre-Upgrade:      %s\n", result.PreGenesisPath)
		}
		if result.PostGenesisPath != "" {
			fmt.Printf("  Post-Upgrade:     %s\n", result.PostGenesisPath)
		}
	}

	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println()
	output.Info("Use 'devnet-builder status' to verify chain health")
	fmt.Println()
	return nil
}

func outputUpgradeJSON(result *dto.ExecuteUpgradeOutput) error {
	if result.Error != nil {
		return outputUpgradeError(result.Error)
	}

	jsonResult := UpgradeResultJSON{
		Status:            "success",
		UpgradeName:       result.NewBinary,
		ProposalID:        result.ProposalID,
		UpgradeHeight:     result.UpgradeHeight,
		PostUpgradeHeight: result.PostUpgradeHeight,
		NewBinary:         result.NewBinary,
		Duration:          result.Duration.String(),
		PreGenesisPath:    result.PreGenesisPath,
		PostGenesisPath:   result.PostGenesisPath,
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputUpgradeError(err error) error {
	errCode := "UPGRADE_FAILED"
	suggestion := ""

	result := map[string]interface{}{
		"error":   true,
		"code":    errCode,
		"message": err.Error(),
	}
	if suggestion != "" {
		result["suggestion"] = suggestion
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}

// buildBinaryForUpgrade builds a binary using DI container and BuildUseCase.
func buildBinaryForUpgrade(ctx context.Context, blockchainNetwork, ref, networkType string, logger *output.Logger) (*dto.BuildOutput, error) {
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

	logger.Info("Pre-building upgrade binary (ref: %s)...", ref)

	// Execute BuildUseCase
	return container.BuildUseCase().Execute(ctx, dto.BuildInput{
		Ref:      ref,
		Network:  networkType,
		UseCache: true,
		ToCache:  true,
	})
}

// PortsMetadataAdapter wraps *ports.DevnetMetadata for compatibility.
type PortsMetadataAdapter struct {
	metadata      *ports.DevnetMetadata
	svc           *CleanDevnetService
	networkModule network.NetworkModule
}

// NewPortsMetadataAdapter creates a new adapter wrapping the given ports metadata.
func NewPortsMetadataAdapter(m *ports.DevnetMetadata, svc *CleanDevnetService, nm network.NetworkModule) *PortsMetadataAdapter {
	return &PortsMetadataAdapter{
		metadata:      m,
		svc:           svc,
		networkModule: nm,
	}
}

func (a *PortsMetadataAdapter) GetChainID() string {
	return a.metadata.ChainID
}

func (a *PortsMetadataAdapter) GetExecutionMode() ports.ExecutionMode {
	return a.metadata.ExecutionMode
}

func (a *PortsMetadataAdapter) SetExecutionMode(mode ports.ExecutionMode) {
	a.metadata.ExecutionMode = mode
}

func (a *PortsMetadataAdapter) GetVersion() string {
	return a.metadata.CurrentVersion
}

func (a *PortsMetadataAdapter) SetVersion(version string) {
	a.metadata.CurrentVersion = version
}

func (a *PortsMetadataAdapter) GetNumValidators() int {
	return a.metadata.NumValidators
}

func (a *PortsMetadataAdapter) GetBinaryName() string {
	return a.networkModule.BinaryName()
}

func (a *PortsMetadataAdapter) GetHomeDir() string {
	return a.metadata.HomeDir
}

func (a *PortsMetadataAdapter) Save() error {
	ctx := context.Background()
	return a.svc.SaveMetadata(ctx, a.metadata)
}
