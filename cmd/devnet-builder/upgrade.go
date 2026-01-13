package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/di"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/binary"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/cache"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/executor"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/filesystem"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
	infrarpc "github.com/b-harvest/devnet-builder/internal/infrastructure/rpc"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Default upgrade constants
const (
	DefaultHeightBuffer = 0 // 0 = auto-calculate based on block time
	DefaultVotingPeriod = 60 * time.Second
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
	forceVotingPeriod    bool
	heightBuffer         int
	withExport           bool
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

  # Upgrade to a local binary (interactive selection)
  devnet-builder upgrade --name v2.0.0-upgrade --version v2.0.0
  → Then select "Use local binary" and browse to your binary

  # Upgrade with custom voting period (fallback if chain query fails)
  devnet-builder upgrade --name v2.0.0-upgrade --image ghcr.io/stablelabs/stable:v2.0.0 --voting-period 120s

  # Force custom voting period (override chain/plugin parameters)
  devnet-builder upgrade --name v2.0.0-upgrade --image ghcr.io/stablelabs/stable:v2.0.0 --voting-period 30s --force-voting-period

  # Upgrade and export state snapshots
  devnet-builder upgrade --name v2.0.0-upgrade --image ghcr.io/stablelabs/stable:v2.0.0 --with-export

  # Interactive mode (default) - select version interactively
  devnet-builder upgrade

  # Non-interactive mode with explicit version
  devnet-builder upgrade --no-interactive --name v2.0.0-upgrade --version v2.0.0`,
		RunE: runUpgrade,
	}

	// Version selection flags
	cmd.Flags().StringVarP(&upgradeName, "name", "n", "", "Upgrade handler name")
	cmd.Flags().StringVarP(&upgradeImage, "image", "i", "", "Target Docker image for upgrade")
	// T047: Removed --binary flag (replaced by interactive selection)
	// cmd.Flags().StringVarP(&upgradeBinary, "binary", "b", "", "Target local binary path for upgrade")
	cmd.Flags().StringVar(&upgradeVersion, "version", "", "Target version (tag or branch/commit for building)")
	cmd.Flags().StringVarP(&upgradeMode, "mode", "m", "", "Execution mode: docker or local (default: from devnet metadata)")

	// Interactive mode flags
	cmd.Flags().BoolVar(&upgradeNoInteractive, "no-interactive", false, "Disable interactive mode")

	// Optional flags
	cmd.Flags().StringVar(&votingPeriod, "voting-period", "60s", "Expedited voting period duration")
	cmd.Flags().BoolVar(&forceVotingPeriod, "force-voting-period", false, "Force use of --voting-period value, ignoring on-chain parameters")
	cmd.Flags().IntVar(&heightBuffer, "height-buffer", DefaultHeightBuffer, "Blocks to add after voting period ends (0 = auto-calculate based on block time)")
	cmd.Flags().BoolVar(&withExport, "with-export", false, "Export state before and after upgrade")
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
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		fmt.Println()
		output.Warn("Upgrade interrupted. Current state:")
		lastUpgradeStageMu.RLock()
		stage := lastUpgradeStage
		lastUpgradeStageMu.RUnlock()
		if stage != "" {
			fmt.Printf("  Last stage: %s\n", stage)
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

	// Variable to store custom binary path (set by unified selection or selectBinaryForUpgrade)
	var customBinarySymlinkPath string

	// Interactive mode: run selection flow if not disabled
	// Skip interactive selection if --image or --binary flags are provided
	if !upgradeNoInteractive && !jsonMode && upgradeImage == "" && upgradeBinary == "" {
		// T049: Use unified selection function for upgrade command
		// forUpgrade = true: collects only upgrade target version (no export/start distinction)
		// includeNetworkSelection = false: network is already determined from running devnet
		// network = "": not used for upgrade flow (forUpgrade=true exits early in handleGitHubReleaseSelection)
		selection, err := runInteractiveVersionSelectionWithMode(ctx, cmd, false, true, "")
		if err != nil {
			if interactive.IsCancellation(err) {
				fmt.Println("Operation cancelled.")
				return nil
			}
			return err
		}

		// Extract version information
		// For upgrade, we use the start version (same as export version in unified selection)
		selectedVersion = selection.StartVersion

		// Determine upgrade name with priority:
		// 1. CLI flag (--name) if provided
		// 2. User input from interactive prompt (selection.UpgradeName)
		// 3. Auto-generate from version as fallback
		if upgradeName != "" {
			selectedName = upgradeName
		} else if selection.UpgradeName != "" {
			// Use the upgrade name entered by user in interactive prompt
			selectedName = selection.UpgradeName
		} else {
			// Auto-generate upgrade name from version
			// For custom refs (branches), extract just the last part after '/'
			// e.g., "feat/gas-waiver" -> "gas-waiver-upgrade"
			// For tags, use as-is: "v2.0.0" -> "v2.0.0-upgrade"
			versionForName := selection.StartVersion
			if selection.StartIsCustomRef && strings.Contains(versionForName, "/") {
				parts := strings.Split(versionForName, "/")
				versionForName = parts[len(parts)-1]
			}
			selectedName = versionForName + "-upgrade"
		}

		// T049: If user selected a local binary, store it for later use
		// This prevents the need to call selectBinaryForUpgrade() again
		if selection.BinarySource.IsLocal() && selection.BinarySource.SelectedPath != "" {
			customBinarySymlinkPath = selection.BinarySource.SelectedPath
		}
	} else {
		// Non-interactive mode: use explicit flags
		selectedName = upgradeName
		selectedVersion = upgradeVersion
	}

	// T049: Check for deprecated --binary flag usage
	if upgradeBinary != "" {
		return fmt.Errorf(`the --binary flag has been removed in favor of interactive binary selection

When you run 'devnet-builder upgrade' in interactive mode, you will be prompted to:
1. Choose between using a local binary or downloading from GitHub releases
2. If you select "local binary", you can browse your filesystem with Tab autocomplete

Migration guide:
  • Interactive mode (recommended):
      devnet-builder upgrade
      → Select "Use local binary (browse filesystem)"
      → Navigate to your binary using Tab autocomplete

  • Non-interactive mode with environment variable:
      export DEVNET_BINARY_PATH=/path/to/your/binary
      devnet-builder upgrade --no-interactive --name upgrade-name --version v1.2.3

  • Docker mode (unchanged):
      devnet-builder upgrade --mode docker --image your-image:tag --name upgrade-name

For more information, see: https://github.com/b-harvest/devnet-builder/blob/main/docs/MIGRATION.md`)
	}

	// Validate that we have either image or version to build (binary flag removed)
	if upgradeImage == "" && selectedVersion == "" {
		return fmt.Errorf("either --image or --version must be provided (or use interactive mode)")
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

	// Get governance parameters (either from chain or forced CLI value)
	var govParams *ports.GovParams
	var vp time.Duration

	if forceVotingPeriod {
		// User explicitly wants to override with CLI value
		logger.Info("Using forced voting period from --voting-period flag...")
		parsedVP, parseErr := time.ParseDuration(votingPeriod)
		if parseErr != nil {
			return fmt.Errorf("invalid voting period: %w", parseErr)
		}
		vp = parsedVP
		logger.Info("Forced expedited voting period: %s", vp)
	} else {
		// Query from chain (plugin or REST)
		logger.Info("Fetching governance parameters from chain...")
		rpcHost := "localhost"
		rpcPort := 26657
		tempFactory := di.NewInfrastructureFactory(homeDir, logger).
			WithNetworkModule(networkModule)
		rpcClient := tempFactory.CreateRPCClient(rpcHost, rpcPort)

		// Configure plugin delegation for governance parameter queries
		// Type assert to check if network module supports governance parameter queries
		if cosmosClient, ok := rpcClient.(*infrarpc.CosmosRPCClient); ok {
			// Check if network module implements GetGovernanceParams (optional interface)
			if pluginModule, ok := networkModule.(infrarpc.NetworkPluginModule); ok {
				rpcClient = cosmosClient.WithPlugin(pluginModule, cleanMetadata.NetworkName)
			}
			// If plugin doesn't implement GetGovernanceParams, will fall back to REST API
		}

		var err error
		govParams, err = rpcClient.GetGovParams(ctx)
		if err != nil {
			logger.Debug("Failed to fetch gov params, using CLI flag value: %v", err)
			// Fallback to CLI flag if chain query fails
			parsedVP, parseErr := time.ParseDuration(votingPeriod)
			if parseErr != nil {
				return fmt.Errorf("invalid voting period: %w", parseErr)
			}
			govParams = &ports.GovParams{
				ExpeditedVotingPeriod: parsedVP,
			}
		}

		// Use expedited voting period from chain
		vp = govParams.ExpeditedVotingPeriod
		logger.Info("Using expedited voting period: %s", vp)
	}

	// Binary resolution for local mode upgrades (T049: --binary flag removed)
	// Priority: pre-built binary (cachedBuildResult) > unified selection > cached binary selection > error
	if resolvedMode == UpgradeModeLocal {
		// T049: Check if binary was already built from custom ref
		// If cachedBuildResult is set, the binary was just pre-built - skip selection
		if cachedBuildResult != nil {
			// Binary was pre-built from custom ref (e.g., feat/gas-waiver) - use it directly
			logger.Debug("Using pre-built binary from custom ref: %s", cachedBuildResult.BinaryPath)
		} else if customBinarySymlinkPath == "" {
			// No binary selected yet - fall back to cache selection (for non-interactive mode or GitHub release flow)
			// Priority 2: Interactive/Auto selection from cache (US1)
			// This is only for local mode; docker mode uses images
			selectedPath, err := selectBinaryForUpgrade(ctx, cleanMetadata.NetworkName, cleanMetadata.BlockchainNetwork, homeDir, logger)
			if err != nil {
				// Error contains detailed validation failure info
				return err
			}

			if selectedPath == "" {
				// No cached binaries available at all
				cacheDir := filepath.Join(homeDir, "cache", "binaries")
				return fmt.Errorf("no cached binaries found for upgrade\nCache directory: %s\nUse --binary flag to specify a binary, or deploy/build a binary first", cacheDir)
			}

			customBinarySymlinkPath = selectedPath
		} else {
			// customBinarySymlinkPath already set from unified selection - use it directly
			logger.Success("Using selected binary: %s", customBinarySymlinkPath)
		}
	}

	// Determine target binary/image
	targetBinary := customBinarySymlinkPath // Use selected/imported binary if available
	if targetBinary == "" && upgradeBinary != "" {
		targetBinary = upgradeBinary // Fallback to raw path (should not happen with import)
	}
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
		UpgradeHeight: 0, // Always auto-calculate
		WithExport:    withExport,
		GenesisDir:    genesisDir,
		Mode:          ports.ExecutionMode(resolvedMode),
	}

	// If we have a cached binary, use cache mode for atomic symlink switch
	if cachedBuildResult != nil {
		input.CachePath = cachedBuildResult.BinaryPath
		input.CommitHash = cachedBuildResult.CommitHash // Deprecated, kept for compatibility
		input.CacheRef = cachedBuildResult.CacheRef     // Use CacheRef for SetActive
		input.TargetBinary = ""                         // Clear since we're using cache
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
	fileCfg := GetLoadedFileConfig()

	// Use unified GitHub client setup (US2: eliminates code duplication)
	client := setupGitHubClient(homeDir, fileCfg)

	// Run selection flow
	selector := interactive.NewSelector(client)
	return selector.RunUpgradeSelectionFlow(ctx)
}

// selectBinaryForUpgrade orchestrates binary selection from cache for upgrade command.
// This is simpler than selectBinaryForDeployment because upgrade doesn't build from source.
//
// Priority Order:
//  1. Interactive/auto selection from cache (NEW)
//  2. Return empty if no cache (caller will error)
//
// Parameters:
//   - ctx: Context for cancellation
//   - network: Network type ("mainnet", "testnet")
//   - blockchain: Blockchain name ("stable", "ault")
//   - homeDir: Home directory path
//   - logger: Logger for user feedback
//
// Returns:
//   - Binary path to use (symlink in ~/.devnet-builder/bin/)
//   - Error if selection fails or user cancels
//
// Note: Unlike deploy, upgrade MUST have a binary (either --binary flag or cached).
// If no binaries found, returns empty string and caller will error appropriately.
func selectBinaryForUpgrade(
	ctx context.Context,
	network string,
	blockchain string,
	homeDir string,
	logger *output.Logger,
) (string, error) {
	// Setup components (same as deploy)
	fs := filesystem.NewOSFileSystem()
	scanner := cache.NewBinaryScanner(fs)

	executor := executor.NewOSCommandExecutor()
	detector := binary.NewVersionDetectorAdapter(executor, 5*time.Second)
	validator := binary.NewBinaryValidator(detector)

	prompter := interactive.NewPrompterAdapter()
	selector := interactive.NewBinarySelector(prompter)

	// Scan cache for binaries
	binaryName := blockchain + "d" // e.g., "stable" → "stabled"
	cacheDir := filepath.Join(homeDir, "cache", "binaries")

	// Debug: log what we're searching for
	logger.Debug("Scanning cache for binaries: network=%q, blockchain=%q, binaryName=%q, cacheDir=%q",
		network, blockchain, binaryName, cacheDir)

	var scannedBinaries []cache.CachedBinaryMetadata
	var err error

	if network != "" {
		// Scan specific network directory
		scannedBinaries, err = scanner.ScanCachedBinaries(ctx, cacheDir, network, binaryName)
		if err != nil {
			return "", fmt.Errorf("failed to scan cache: %w", err)
		}
		logger.Debug("Found %d binaries in network %q", len(scannedBinaries), network)
	}

	// Fallback: if no binaries found with specific network, scan ALL networks
	// This handles edge cases where NetworkName in metadata is empty or mismatched
	if len(scannedBinaries) == 0 {
		if network != "" {
			logger.Debug("No binaries found in network %q, scanning all networks...", network)
		} else {
			logger.Debug("Network not specified, scanning all networks...")
		}

		scannedBinaries, err = scanner.ScanAllNetworks(ctx, cacheDir, binaryName)
		if err != nil {
			return "", fmt.Errorf("failed to scan all networks: %w", err)
		}
		logger.Debug("Found %d binaries across all networks", len(scannedBinaries))
	}

	// Validate binaries concurrently
	validBinaries, invalidBinaries := filterValidBinariesForUpgradeWithDiagnostics(ctx, scannedBinaries, validator, logger)

	// If no valid binaries found, provide detailed diagnostics
	if len(validBinaries) == 0 {
		if len(scannedBinaries) > 0 {
			// Binaries were found but all failed validation - show WHY
			logger.Warn("Found %d cached binaries but all failed validation:", len(scannedBinaries))
			for _, inv := range invalidBinaries {
				logger.Warn("  - %s: %s", inv.binary.Path, inv.reason)
			}
			// Offer to clean up invalid cache entries
			logger.Warn("Run 'devnet-builder cache clean' to remove invalid entries")
			return "", fmt.Errorf("all %d cached binaries failed validation (see warnings above)", len(scannedBinaries))
		}
		logger.Debug("No cached binaries found for %q in cache directory: %s", binaryName, cacheDir)
		return "", nil // Empty result indicates no cache
	}

	// Run selection with appropriate options
	// Note: Upgrade doesn't have "Build from source" option
	opts := interactive.BinarySelectionOptions{
		AllowBuildFromSource: false,                               // Upgrade must use existing binary
		AutoSelectSingle:     true,                                // CLARIFICATION 1: Auto-select single binary
		IsInteractive:        interactive.IsTerminalInteractive(), // EC-004: TTY detection
	}

	result, err := selector.RunBinarySelectionFlow(ctx, validBinaries, opts)
	if err != nil {
		return "", fmt.Errorf("binary selection failed: %w", err)
	}

	// User cancelled
	if result.WasCancelled {
		return "", fmt.Errorf("selection cancelled by user")
	}

	// No binary selected (shouldn't happen since AllowBuildFromSource=false)
	if result.SelectedBinary == nil {
		return "", fmt.Errorf("no binary selected")
	}

	// Binary selected from cache
	// EC-002: Single binary was auto-selected (log for transparency)
	if len(validBinaries) == 1 {
		logger.Info("Using cached binary: %s %s (%s)",
			result.SelectedBinary.Name,
			result.SelectedBinary.Version,
			result.SelectedBinary.CommitHashShort)
	} else {
		logger.Success("Selected binary: %s %s (%s)",
			result.SelectedBinary.Name,
			result.SelectedBinary.Version,
			result.SelectedBinary.CommitHashShort)
	}

	return result.BinaryPath, nil
}

// invalidBinaryInfo holds information about a binary that failed validation.
type invalidBinaryInfo struct {
	binary cache.CachedBinaryMetadata
	reason string
}

// filterValidBinariesForUpgradeWithDiagnostics validates binaries and returns both valid and invalid lists.
// This provides better diagnostics for debugging cache issues.
func filterValidBinariesForUpgradeWithDiagnostics(
	ctx context.Context,
	binaries []cache.CachedBinaryMetadata,
	validator *binary.BinaryValidator,
	logger *output.Logger,
) ([]cache.CachedBinaryMetadata, []invalidBinaryInfo) {
	if len(binaries) == 0 {
		return []cache.CachedBinaryMetadata{}, []invalidBinaryInfo{}
	}

	// Validate concurrently for performance
	type validationResult struct {
		binary cache.CachedBinaryMetadata
		err    error
	}

	results := make(chan validationResult, len(binaries))
	var wg sync.WaitGroup

	for i := range binaries {
		wg.Add(1)
		go func(b cache.CachedBinaryMetadata) {
			defer wg.Done()

			// Validate and enrich with version info
			enriched, err := validator.ValidateAndEnrichMetadata(ctx, &b)
			results <- validationResult{
				binary: *enriched,
				err:    err,
			}
		}(binaries[i])
	}

	// Wait for all validations to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect valid and invalid binaries
	var validBinaries []cache.CachedBinaryMetadata
	var invalidBinaries []invalidBinaryInfo
	for result := range results {
		if result.err != nil {
			// EC-007: Corrupted/invalid binary - collect for diagnostics
			invalidBinaries = append(invalidBinaries, invalidBinaryInfo{
				binary: result.binary,
				reason: result.err.Error(),
			})
			continue
		}
		if result.binary.IsValid {
			validBinaries = append(validBinaries, result.binary)
		} else {
			invalidBinaries = append(invalidBinaries, invalidBinaryInfo{
				binary: result.binary,
				reason: result.binary.ValidationError,
			})
		}
	}

	// Sort by modification time descending (most recent first)
	sort.Slice(validBinaries, func(i, j int) bool {
		return validBinaries[i].ModTime.After(validBinaries[j].ModTime)
	})

	return validBinaries, invalidBinaries
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
	fmt.Printf("Upgrade Height:   auto-calculate\n")
	fmt.Printf("Validators:       %d\n", metadata.NumValidators)
	fmt.Println()
}

var (
	lastUpgradeStage   string
	lastUpgradeStageMu sync.RWMutex
)

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
