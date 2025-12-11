package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
	"github.com/stablelabs/stable-devnet/internal/snapshot"
)

var (
	destroyForce bool
	destroyCache bool
)

func NewDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Remove all devnet data",
		Long: `Remove all devnet data from the home directory.

This command removes the devnet directory and optionally the snapshot cache.
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
	logger := output.DefaultLogger

	devnetDir := filepath.Join(homeDir, "devnet")
	devnetExists := devnet.DevnetExists(homeDir)

	if !devnetExists && !destroyCache {
		if jsonMode {
			return outputDestroyError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Confirmation prompt (unless --force)
	if !destroyForce && !jsonMode {
		msg := fmt.Sprintf("This will remove all devnet data at %s", devnetDir)
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

	// Clean devnet
	if !jsonMode {
		output.Info("Destroying devnet...")
	}

	if devnetExists {
		if err := os.RemoveAll(devnetDir); err != nil {
			// Try docker-based cleanup for root-owned files
			if !jsonMode {
				output.Info("Standard cleanup failed, trying docker-based cleanup for root-owned files...")
			}
			if dockerErr := destroyWithDocker(devnetDir); dockerErr != nil {
				if jsonMode {
					return outputDestroyError(err)
				}
				return fmt.Errorf("failed to remove devnet: %w (docker cleanup also failed: %v)", err, dockerErr)
			}
		}
	}

	// Clean cache if requested
	if destroyCache {
		if !jsonMode {
			output.Info("Cleaning snapshot cache...")
		}

		if err := snapshot.ClearAllCaches(homeDir); err != nil {
			logger.Warn("Failed to clear cache: %v", err)
		}
	}

	if jsonMode {
		return outputDestroyJSON(destroyCache)
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

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputDestroyError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "DESTROY_FAILED",
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}

// destroyWithDocker uses a docker container to remove root-owned files.
func destroyWithDocker(dir string) error {
	cmd := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/cleanup", dir),
		"alpine:latest",
		"sh", "-c", "rm -rf /cleanup/*",
	)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cleanup failed: %s: %w", string(cmdOutput), err)
	}

	return os.RemoveAll(dir)
}
