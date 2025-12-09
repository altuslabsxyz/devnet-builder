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
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
	"github.com/stablelabs/stable-devnet/internal/upgrade"
)

// Upgrade command flags
var (
	upgradeName      string
	upgradeImage     string
	upgradeBinary    string
	votingPeriod     string
	heightBuffer     int
	upgradeHeight    int64
	exportGenesis    bool
	genesisDir       string
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
  devnet-builder upgrade --name v2.0.0-upgrade --image ghcr.io/stablelabs/stable:v2.0.0 --export-genesis`,
		RunE: runUpgrade,
	}

	// Required flags
	cmd.Flags().StringVarP(&upgradeName, "name", "n", "", "Upgrade handler name (required)")
	cmd.Flags().StringVarP(&upgradeImage, "image", "i", "", "Target Docker image for upgrade")
	cmd.Flags().StringVarP(&upgradeBinary, "binary", "b", "", "Target local binary path for upgrade")

	// Optional flags
	cmd.Flags().StringVar(&votingPeriod, "voting-period", "60s", "Expedited voting period duration")
	cmd.Flags().IntVar(&heightBuffer, "height-buffer", upgrade.DefaultHeightBuffer, "Blocks to add after voting period ends")
	cmd.Flags().Int64Var(&upgradeHeight, "upgrade-height", 0, "Explicit upgrade height (0 = auto-calculate)")
	cmd.Flags().BoolVar(&exportGenesis, "export-genesis", false, "Export genesis before and after upgrade")
	cmd.Flags().StringVar(&genesisDir, "genesis-dir", "", "Directory for genesis exports (default: <home>/devnet/genesis-snapshots)")

	// Mark name as required
	cmd.MarkFlagRequired("name")

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

	// Validate that either image or binary is provided
	if upgradeImage == "" && upgradeBinary == "" {
		return fmt.Errorf("either --image or --binary must be provided")
	}
	if upgradeImage != "" && upgradeBinary != "" {
		return fmt.Errorf("cannot specify both --image and --binary")
	}

	// Check if devnet exists and is running
	if !devnet.DevnetExists(homeDir) {
		if jsonMode {
			return outputUpgradeError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s\nStart a devnet first with 'devnet-builder start'", homeDir)
	}

	// Load devnet metadata
	metadata, err := devnet.LoadDevnetMetadata(homeDir)
	if err != nil {
		if jsonMode {
			return outputUpgradeError(err)
		}
		return fmt.Errorf("failed to load devnet metadata: %w", err)
	}

	if metadata.Status != devnet.StatusRunning {
		if jsonMode {
			return outputUpgradeError(fmt.Errorf("devnet is not running"))
		}
		return fmt.Errorf("devnet is not running\nStart it with 'devnet-builder start'")
	}

	// Parse voting period
	vp, err := time.ParseDuration(votingPeriod)
	if err != nil {
		return fmt.Errorf("invalid voting period: %w", err)
	}

	// Build upgrade config
	cfg := &upgrade.UpgradeConfig{
		Name:          upgradeName,
		TargetImage:   upgradeImage,
		TargetBinary:  upgradeBinary,
		VotingPeriod:  vp,
		HeightBuffer:  heightBuffer,
		UpgradeHeight: upgradeHeight,
		ExportGenesis: exportGenesis,
		GenesisDir:    genesisDir,
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

func printUpgradePlan(cfg *upgrade.UpgradeConfig, metadata *devnet.DevnetMetadata) {
	output.Bold("Upgrade Plan")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("Upgrade Name:     %s\n", cfg.Name)
	if cfg.TargetImage != "" {
		fmt.Printf("Target Image:     %s\n", cfg.TargetImage)
	} else {
		fmt.Printf("Target Binary:    %s\n", cfg.TargetBinary)
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
			if remaining > 0 {
				fmt.Printf("\r[4/6] %s Block %d/%d (%d remaining)   ",
					color.CyanString("Waiting for upgrade height..."),
					p.CurrentHeight, p.TargetHeight, remaining)
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
