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
	cleanForce bool
	cleanCache bool
)

func NewCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "clean",
		Short:      "Remove all devnet data (deprecated: use 'destroy' instead)",
		Deprecated: "use 'destroy' instead",
		Long: `Remove all devnet data from the home directory.

DEPRECATED: This command is deprecated. Use 'devnet-builder destroy' instead.

This command removes the devnet directory and optionally the snapshot cache.
Use with caution as this is irreversible.

Examples:
  # Remove devnet data (keeps snapshot cache)
  devnet-builder destroy

  # Remove devnet data and snapshot cache
  devnet-builder destroy --cache

  # Skip confirmation prompt
  devnet-builder destroy --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintDeprecationWarning("clean", "destroy")
			return runClean(cmd, args)
		},
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
			// Try docker-based cleanup for root-owned files (created by docker containers)
			if !jsonMode {
				output.Info("Standard cleanup failed, trying docker-based cleanup for root-owned files...")
			}
			if dockerErr := cleanWithDocker(devnetDir); dockerErr != nil {
				if jsonMode {
					return outputCleanError(err)
				}
				return fmt.Errorf("failed to remove devnet: %w (docker cleanup also failed: %v)", err, dockerErr)
			}
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

// cleanWithDocker uses a docker container to remove root-owned files.
// This is needed because docker containers running as root create files
// owned by root, which the host user cannot delete without sudo.
func cleanWithDocker(dir string) error {
	// Use alpine image for minimal footprint
	// Remove contents of the directory, not the mount point itself
	cmd := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/cleanup", dir),
		"alpine:latest",
		"sh", "-c", "rm -rf /cleanup/*",
	)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cleanup failed: %s: %w", string(cmdOutput), err)
	}

	// Remove the now-empty directory
	return os.RemoveAll(dir)
}
