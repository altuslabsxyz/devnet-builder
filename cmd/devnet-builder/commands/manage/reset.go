package manage

import (
	"encoding/json"
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

var (
	resetHard  bool
	resetForce bool
)

// NewResetCmd creates the reset command.
func NewResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset devnet chain data",
		Long: `Reset the devnet chain data.

By default, this performs a soft reset which clears chain data but preserves
genesis and configuration. Use --hard to remove all data (requires re-provisioning).

Examples:
  # Soft reset (clear chain data, keep genesis)
  devnet-builder reset

  # Hard reset (clear everything)
  devnet-builder reset --hard

  # Skip confirmation prompt
  devnet-builder reset --force`,
		RunE: runReset,
	}

	cmd.Flags().BoolVar(&resetHard, "hard", false,
		"Remove all data (requires re-provisioning)")
	cmd.Flags().BoolVarP(&resetForce, "force", "f", false,
		"Skip confirmation prompt")

	return cmd
}

func runReset(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()
	jsonMode := cfg.JSONMode()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return outputResetError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		if jsonMode {
			return outputResetError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Confirmation prompt (unless --force)
	if !resetForce && !jsonMode {
		if resetHard {
			fmt.Println("This will remove all devnet data. You will need to re-provision.")
		} else {
			fmt.Println("This will clear all chain data. Genesis and configuration will be preserved.")
		}

		confirmed, err := confirmPrompt("Continue?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	// Perform reset
	if !jsonMode {
		output.Info("Resetting devnet...")
	}

	result, err := svc.Reset(ctx, resetHard)
	if err != nil {
		if jsonMode {
			return outputResetError(err)
		}
		return err
	}

	if jsonMode {
		return outputResetJSON(result.Type == "hard")
	}

	// Display warnings for failed deletions
	if len(result.Failed) > 0 {
		fmt.Println("\n⚠️  Warning: Some directories failed to delete:")
		for path, err := range result.Failed {
			fmt.Printf("  - %s: %v\n", path, err)
		}
		fmt.Println("\nYou may need to fix permissions and run reset again.")
	}

	output.Success("Devnet reset successfully.")
	if resetHard {
		fmt.Println("Run 'devnet-builder deploy' to provision a new devnet.")
	} else {
		fmt.Println("Run 'devnet-builder start' to restart the devnet with fresh chain data.")
	}

	return nil
}

func outputResetJSON(hard bool) error {
	resetType := "soft"
	if hard {
		resetType = "hard"
	}

	result := map[string]interface{}{
		"status":     "success",
		"reset_type": resetType,
		"message":    "Devnet reset successfully",
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputResetError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "RESET_FAILED",
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}
