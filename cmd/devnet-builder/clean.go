package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
	"github.com/stablelabs/stable-devnet/internal/snapshot"
)

var (
	cleanForce bool
	cleanCache bool
)

func NewCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove all devnet data",
		Long: `Remove all devnet data from the home directory.

This command removes the devnet directory and optionally the snapshot cache.
Use with caution as this is irreversible.

Examples:
  # Remove devnet data (keeps snapshot cache)
  devnet-builder clean

  # Remove devnet data and snapshot cache
  devnet-builder clean --cache

  # Skip confirmation prompt
  devnet-builder clean --force`,
		RunE: runClean,
	}

	cmd.Flags().BoolVarP(&cleanForce, "force", "f", false,
		"Skip confirmation prompt")
	cmd.Flags().BoolVar(&cleanCache, "cache", false,
		"Also clean snapshot cache")

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	logger := output.DefaultLogger

	devnetDir := filepath.Join(homeDir, "devnet")
	devnetExists := devnet.DevnetExists(homeDir)

	if !devnetExists && !cleanCache {
		if jsonMode {
			return outputCleanError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Confirmation prompt (unless --force)
	if !cleanForce && !jsonMode {
		msg := fmt.Sprintf("This will remove all devnet data at %s", devnetDir)
		if cleanCache {
			msg += " and all cached snapshots"
		}
		fmt.Println(msg)

		confirmed, err := confirmPrompt("Continue?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Clean cancelled.")
			return nil
		}
	}

	// Clean devnet
	if !jsonMode {
		output.Info("Cleaning devnet...")
	}

	if devnetExists {
		if err := os.RemoveAll(devnetDir); err != nil {
			if jsonMode {
				return outputCleanError(err)
			}
			return fmt.Errorf("failed to remove devnet: %w", err)
		}
	}

	// Clean cache if requested
	if cleanCache {
		if !jsonMode {
			output.Info("Cleaning snapshot cache...")
		}

		if err := snapshot.ClearAllCaches(homeDir); err != nil {
			logger.Warn("Failed to clear cache: %v", err)
		}
	}

	if jsonMode {
		return outputCleanJSON(cleanCache)
	}

	output.Success("Devnet removed successfully.")
	return nil
}

func outputCleanJSON(cacheCleared bool) error {
	result := map[string]interface{}{
		"status":        "success",
		"message":       "Devnet removed successfully",
		"cache_cleared": cacheCleared,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputCleanError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "CLEAN_FAILED",
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}
