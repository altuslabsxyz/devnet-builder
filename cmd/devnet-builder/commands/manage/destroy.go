package manage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/b-harvest/devnet-builder/cmd/devnet-builder/shared"
	"github.com/b-harvest/devnet-builder/internal/application"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	destroyForce bool
	destroyCache bool
)

// NewDestroyCmd creates the destroy command.
func NewDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Remove all devnet data",
		Long: `Remove all devnet data from the home directory.

This command removes the devnet directory and optionally the snapshot cache.
For Docker mode deployments, this also cleans up Docker networks and releases port allocations.
Use with caution as this is irreversible.

Examples:
  # Remove devnet data (keeps snapshot cache)
  devnet-builder destroy

  # Remove devnet data and snapshot cache
  devnet-builder destroy --cache

  # Skip confirmation prompt
  devnet-builder destroy --force`,
		RunE: runDestroy,
	}

	cmd.Flags().BoolVarP(&destroyForce, "force", "f", false,
		"Skip confirmation prompt")
	cmd.Flags().BoolVar(&destroyCache, "cache", false,
		"Also clean snapshot cache")

	return cmd
}

func runDestroy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	homeDir := shared.GetHomeDir()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return outputDestroyError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Check if devnet exists
	if !svc.DevnetExists() && !destroyCache {
		if jsonMode() {
			return outputDestroyError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Confirmation prompt (unless --force)
	if !destroyForce && !jsonMode() {
		msg := fmt.Sprintf("This will remove all devnet data at %s/devnet", homeDir)
		if destroyCache {
			msg += " and all cached snapshots"
		}
		fmt.Println(msg)

		confirmed, err := confirmPrompt("Continue?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Destroy cancelled.")
			return nil
		}
	}

	// Destroy devnet
	result, err := svc.Destroy(ctx, destroyCache)
	if err != nil {
		if jsonMode() {
			return outputDestroyError(err)
		}
		return err
	}

	if jsonMode() {
		return outputDestroyJSON(result.CacheCleared)
	}

	output.Success("Devnet destroyed successfully.")
	return nil
}

func outputDestroyJSON(cacheCleared bool) error {
	result := map[string]interface{}{
		"status":        "success",
		"message":       "Devnet destroyed successfully",
		"cache_cleared": cacheCleared,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func outputDestroyError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "DESTROY_FAILED",
		"message": err.Error(),
	}

	data, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		// Fallback to simple output if marshal fails
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	fmt.Println(string(data))
	return err
}
