package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/builder"
	"github.com/stablelabs/stable-devnet/internal/cache"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/github"
	"github.com/stablelabs/stable-devnet/internal/interactive"
	"github.com/stablelabs/stable-devnet/internal/output"
	"github.com/stablelabs/stable-devnet/internal/upgrade"
)

// Upgrade command flags
var (
	upgradeName          string
	upgradeImage         string
	upgradeBinary        string
	upgradeMode          string // NEW: --mode flag for docker/local
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
	cmd.Flags().IntVar(&heightBuffer, "height-buffer", upgrade.DefaultHeightBuffer, "Blocks to add after voting period ends")
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
		if lastStage != "" {
			fmt.Printf("  Last stage: %s\n", lastStage)
		}
		output.Warn("The devnet may be in an intermediate state.")
		output.Info("Run 'devnet-builder status' to check chain health.")
		cancel()
	}()

	logger := output.DefaultLogger

	// Load devnet metadata using consolidated helper
	metadata, err := loadMetadataOrFail(logger)
	if err != nil {
		if jsonMode {
			return outputUpgradeError(err)
		}
		return err
	}

	if metadata.Status != devnet.StatusRunning {
		if jsonMode {
			return outputUpgradeError(fmt.Errorf("devnet is not running"))
		}
		return fmt.Errorf("devnet is not running\nStart it with 'devnet-builder start'")
	}

	// Resolve execution mode: flag > metadata default (T004)
	resolvedMode := metadata.ExecutionMode
	modeExplicitlySet := false
	if upgradeMode != "" {
		// Validate mode value
		switch devnet.ExecutionMode(upgradeMode) {
		case devnet.ModeDocker, devnet.ModeLocal:
			resolvedMode = devnet.ExecutionMode(upgradeMode)
			modeExplicitlySet = true
		default:
			return fmt.Errorf("invalid mode %q: must be 'docker' or 'local'", upgradeMode)
		}
	}

	// Mode validation against --image/--binary flags (T005, T006)
	if !jsonMode {
		// Warn if mode doesn't match the provided flags
		if resolvedMode == devnet.ModeDocker && upgradeBinary != "" && !modeExplicitlySet {
			output.Warn("Devnet was started in docker mode but --binary was provided.")
			output.Warn("Use --image for docker mode, or --mode local to switch modes.")
		}
		if resolvedMode == devnet.ModeLocal && upgradeImage != "" && !modeExplicitlySet {
			output.Warn("Devnet was started in local mode but --image was provided.")
			output.Warn("Use --binary for local mode, or --mode docker to switch modes.")
		}
		// Warn if explicitly switching modes (T010 - mode change warning)
		if modeExplicitlySet && resolvedMode != metadata.ExecutionMode {
			output.Warn("Switching execution mode from %s to %s.", metadata.ExecutionMode, resolvedMode)
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

	// Initialize binary cache
	binaryCache := cache.NewBinaryCache(homeDir, logger)
	if err := binaryCache.Initialize(); err != nil {
		logger.Warn("Failed to initialize binary cache: %v", err)
		// Continue without cache - will fall back to direct binary copy
	}

	// T011: Mode-aware version resolution
	// For docker mode with standard version tag, use docker image
	// For local mode or custom refs, build local binary
	var cachedBinary *cache.CachedBinary
	var customBinaryPath string
	var versionResolvedImage string

	if selectedVersion != "" && upgradeImage == "" && upgradeBinary == "" {
		networkModule, _ := metadata.GetNetworkModule()
		if resolvedMode == devnet.ModeDocker && isStandardVersionTag(selectedVersion) {
			// Docker mode with standard version tag: resolve to docker image
			dockerImage := "ghcr.io/stablelabs/stable" // fallback
			if networkModule != nil {
				dockerImage = networkModule.DockerImage()
			}
			versionResolvedImage = fmt.Sprintf("%s:%s", dockerImage, selectedVersion)
			logger.Info("Using docker image for version %s: %s", selectedVersion, versionResolvedImage)
		} else {
			// Local mode or custom ref: build local binary to cache
			b := builder.NewBuilder(homeDir, logger, networkModule)
			logger.Info("Pre-building upgrade binary (ref: %s)...", selectedVersion)

			// Build to cache
			cached, err := b.BuildToCache(ctx, builder.BuildOptions{
				Ref:     selectedVersion,
				Network: metadata.NetworkSource,
			}, binaryCache)
			if err != nil {
				return fmt.Errorf("failed to pre-build binary: %w", err)
			}
			cachedBinary = cached
			logger.Success("Binary pre-built and cached (commit: %s)", cached.CommitHash[:12])
		}
	}

	// Parse voting period
	vp, err := time.ParseDuration(votingPeriod)
	if err != nil {
		return fmt.Errorf("invalid voting period: %w", err)
	}

	// Determine target binary/image
	targetBinary := upgradeBinary
	targetImage := upgradeImage
	if customBinaryPath != "" {
		targetBinary = customBinaryPath
	}
	if versionResolvedImage != "" {
		targetImage = versionResolvedImage
	}

	// Build upgrade config
	cfg := &upgrade.UpgradeConfig{
		Name:          selectedName,
		Mode:          resolvedMode, // T008: Pass resolved mode to UpgradeConfig
		TargetImage:   targetImage,
		TargetBinary:  targetBinary,
		TargetVersion: selectedVersion,
		VotingPeriod:  vp,
		HeightBuffer:  heightBuffer,
		UpgradeHeight: upgradeHeight,
		ExportGenesis: exportGenesis,
		GenesisDir:    genesisDir,
	}

	// If we have a cached binary, use cache mode for atomic symlink switch
	if cachedBinary != nil {
		cfg.CachePath = cachedBinary.BinaryPath
		cfg.CommitHash = cachedBinary.CommitHash
		// Clear target binary since we're using cache
		cfg.TargetBinary = ""
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		if jsonMode {
			return outputUpgradeError(err)
		}
		return err
	}

	// Build execute options
	opts := &upgrade.ExecuteOptions{
		HomeDir:  homeDir,
		Metadata: metadata,
		Logger:   logger,
	}

	// Print upgrade plan (non-JSON mode)
	if !jsonMode {
		printUpgradePlan(cfg, metadata)
	}

	// Set up progress callback for non-JSON mode
	if !jsonMode {
		opts.ProgressCallback = func(p upgrade.UpgradeProgress) {
			printUpgradeProgress(p)
		}
	}

	// Execute the upgrade
	result, err := upgrade.ExecuteUpgrade(ctx, cfg, opts)
	if err != nil {
		if jsonMode {
			return outputUpgradeError(err)
		}
		return err
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

func printUpgradePlan(cfg *upgrade.UpgradeConfig, metadata *devnet.DevnetMetadata) {
	output.Bold("Upgrade Plan")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("Upgrade Name:     %s\n", cfg.Name)
	fmt.Printf("Mode:             %s\n", cfg.Mode) // T007: Display mode in upgrade plan
	if cfg.TargetImage != "" {
		fmt.Printf("Target Image:     %s\n", cfg.TargetImage)
	} else if cfg.TargetBinary != "" {
		fmt.Printf("Target Binary:    %s\n", cfg.TargetBinary)
	} else if cfg.CachePath != "" {
		fmt.Printf("Target Binary:    %s (cached)\n", cfg.CachePath)
	}
	fmt.Printf("Voting Period:    %s\n", cfg.VotingPeriod)
	fmt.Printf("Height Buffer:    %d blocks\n", cfg.HeightBuffer)
	if cfg.UpgradeHeight > 0 {
		fmt.Printf("Upgrade Height:   %d (explicit)\n", cfg.UpgradeHeight)
	} else {
		fmt.Printf("Upgrade Height:   auto-calculate\n")
	}
	fmt.Printf("Validators:       %d\n", metadata.NumValidators)
	fmt.Println()
}

var lastStage upgrade.UpgradeStage

func printUpgradeProgress(p upgrade.UpgradeProgress) {
	// Only print when stage changes, except for waiting stage which updates continuously
	if p.Stage == lastStage && p.Stage != upgrade.StageWaiting && p.Stage != upgrade.StageVoting {
		return
	}
	lastStage = p.Stage

	switch p.Stage {
	case upgrade.StageVerifying:
		fmt.Printf("[1/6] %s\n", color.CyanString("Verifying devnet status..."))
	case upgrade.StageSubmitting:
		fmt.Printf("[2/6] %s\n", color.CyanString("Submitting upgrade proposal..."))
	case upgrade.StageVoting:
		fmt.Printf("\r[3/6] %s Voted: %d/%d validators   ",
			color.CyanString("Voting from validators..."), p.VotesCast, p.TotalVoters)
		if p.VotesCast == p.TotalVoters {
			fmt.Println() // New line when done
		}
	case upgrade.StageWaiting:
		if p.TargetHeight > 0 {
			remaining := p.TargetHeight - p.CurrentHeight
			timeRemaining := time.Until(p.VotingEndTime)
			if timeRemaining < 0 {
				timeRemaining = 0
			}
			if remaining > 0 || timeRemaining > 0 {
				// Show time remaining if voting period not complete, otherwise show blocks
				if timeRemaining > 0 {
					fmt.Printf("\r[4/6] %s Block %d/%d (%s remaining)   ",
						color.CyanString("Waiting for voting period..."),
						p.CurrentHeight, p.TargetHeight, timeRemaining.Round(time.Second))
				} else {
					fmt.Printf("\r[4/6] %s Block %d/%d (%d blocks remaining)   ",
						color.CyanString("Waiting for upgrade height..."),
						p.CurrentHeight, p.TargetHeight, remaining)
				}
			}
		}
	case upgrade.StageSwitching:
		fmt.Println() // New line after waiting
		fmt.Printf("[5/6] %s\n", color.CyanString("Switching to new binary..."))
	case upgrade.StageVerifyingResume:
		fmt.Printf("[6/6] %s\n", color.CyanString("Verifying chain resumed..."))
	case upgrade.StageCompleted:
		fmt.Println()
		output.Success("Upgrade completed successfully!")
	case upgrade.StageFailed:
		fmt.Println()
		output.Error("Upgrade failed: %v", p.Error)
	}
}

func outputUpgradeText(result *upgrade.UpgradeResult) error {
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

func outputUpgradeJSON(result *upgrade.UpgradeResult) error {
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
	// Check if it's an UpgradeError for better error reporting
	var errCode string
	var suggestion string

	if ue, ok := err.(*upgrade.UpgradeError); ok {
		errCode = string(ue.Stage)
		suggestion = ue.Suggestion
	} else {
		errCode = "UPGRADE_FAILED"
	}

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
