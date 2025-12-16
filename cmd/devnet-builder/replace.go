package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/builder"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
)

var (
	replaceVersion       string
	replaceHealthTimeout time.Duration
	replaceNoConfirm     bool
)

// ReplaceResult represents the result of a replace operation.
type ReplaceResult struct {
	Status         string `json:"status"`
	PreviousVersion string `json:"previous_version,omitempty"`
	NewVersion     string `json:"new_version"`
	CommitHash     string `json:"commit_hash,omitempty"`
	BinaryPath     string `json:"binary_path"`
	Error          string `json:"error,omitempty"`
}

func NewReplaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replace",
		Short: "Replace binary without governance upgrade",
		Long: `Replace the stabled binary without going through governance upgrade.

This command stops all nodes, builds a new binary using goreleaser,
replaces the existing binary, and restarts all nodes.

Unlike 'upgrade', this command does NOT submit a governance proposal.
Use this when you need to:
  - Test a new binary version quickly
  - Replace binary with a custom branch/commit
  - Debug issues with a specific version

WARNING: This is a hard replacement. The chain state must be compatible
with the new binary version.

Examples:
  # Replace with a specific version tag
  devnet-builder replace --version v1.2.0

  # Replace with a branch
  devnet-builder replace --version feat/my-feature

  # Replace with a commit hash
  devnet-builder replace --version abc1234

  # Skip confirmation prompt
  devnet-builder replace --version v1.2.0 -y`,
		RunE: runReplace,
	}

	cmd.Flags().StringVarP(&replaceVersion, "version", "V", "",
		"Version/ref to build (tag, branch, or commit)")
	cmd.Flags().DurationVar(&replaceHealthTimeout, "health-timeout", 5*time.Minute,
		"Timeout for node health check after restart")
	cmd.Flags().BoolVarP(&replaceNoConfirm, "yes", "y", false,
		"Skip confirmation prompt")

	cmd.MarkFlagRequired("version")

	return cmd
}

func runReplace(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Check if devnet exists
	if !devnet.DevnetExists(homeDir) {
		return outputReplaceError(fmt.Errorf("no devnet found at %s", homeDir))
	}

	// Load devnet metadata
	metadata, err := devnet.LoadDevnetMetadata(homeDir)
	if err != nil {
		return outputReplaceError(fmt.Errorf("failed to load devnet metadata: %w", err))
	}

	// Show warning and get confirmation
	if !jsonMode && !replaceNoConfirm {
		output.Warn("This will replace the binary WITHOUT governance upgrade.")
		output.Warn("Chain state must be compatible with the new version.")
		fmt.Println()
		fmt.Printf("  Current version: %s\n", metadata.CurrentVersion)
		fmt.Printf("  Target version:  %s\n", replaceVersion)
		fmt.Printf("  Mode:            %s\n", metadata.ExecutionMode)
		fmt.Println()

		confirmed, err := confirmPrompt("Proceed with binary replacement?")
		if err != nil {
			return err
		}
		if !confirmed {
			output.Info("Operation cancelled.")
			return nil
		}
	}

	previousVersion := metadata.CurrentVersion

	// Step 1: Build new binary
	if !jsonMode {
		output.Info("[1/4] Building new binary (ref: %s)...", replaceVersion)
	}

	b := builder.NewBuilder(homeDir, logger)
	buildResult, err := b.Build(ctx, builder.BuildOptions{
		Ref:     replaceVersion,
		Network: metadata.NetworkSource,
	})
	if err != nil {
		return outputReplaceError(fmt.Errorf("failed to build binary: %w", err))
	}

	if !jsonMode {
		logger.Success("Binary built: %s (commit: %s)", buildResult.BinaryPath, buildResult.CommitHash)
	}

	// Step 2: Stop nodes
	if !jsonMode {
		output.Info("[2/4] Stopping nodes...")
	}

	d, err := devnet.LoadDevnetWithNodes(homeDir, logger)
	if err != nil {
		return outputReplaceError(fmt.Errorf("failed to load devnet: %w", err))
	}

	if d.Metadata.IsRunning() {
		if err := d.Stop(ctx, 30*time.Second); err != nil {
			return outputReplaceError(fmt.Errorf("failed to stop nodes: %w", err))
		}
	}

	if !jsonMode {
		logger.Success("Nodes stopped")
	}

	// Step 3: Replace binary
	if !jsonMode {
		output.Info("[3/4] Replacing binary...")
	}

	binDir := filepath.Join(homeDir, "bin")
	targetPath := filepath.Join(binDir, "stabled")

	// Ensure bin directory exists
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return outputReplaceError(fmt.Errorf("failed to create bin directory: %w", err))
	}

	// Copy built binary to target path
	if err := copyFile(buildResult.BinaryPath, targetPath); err != nil {
		return outputReplaceError(fmt.Errorf("failed to copy binary: %w", err))
	}

	// Make executable
	if err := os.Chmod(targetPath, 0755); err != nil {
		return outputReplaceError(fmt.Errorf("failed to make binary executable: %w", err))
	}

	if !jsonMode {
		logger.Success("Binary replaced: %s", targetPath)
	}

	// Update metadata with new version
	metadata.SetCurrentVersion(replaceVersion)
	if err := metadata.Save(); err != nil {
		logger.Warn("Failed to update metadata version: %v", err)
	}

	// Step 4: Restart nodes
	if !jsonMode {
		output.Info("[4/4] Restarting nodes...")
	}

	// Reload devnet for restart
	d, err = devnet.LoadDevnetWithNodes(homeDir, logger)
	if err != nil {
		return outputReplaceError(fmt.Errorf("failed to reload devnet: %w", err))
	}

	runOpts := devnet.RunOptions{
		HomeDir:       homeDir,
		Mode:          metadata.ExecutionMode,
		HealthTimeout: replaceHealthTimeout,
		Logger:        logger,
	}

	runResult, err := devnet.Run(ctx, runOpts)
	if err != nil {
		return outputReplaceError(fmt.Errorf("failed to restart nodes: %w", err))
	}

	// Output result
	result := ReplaceResult{
		Status:          "success",
		PreviousVersion: previousVersion,
		NewVersion:      replaceVersion,
		CommitHash:      buildResult.CommitHash,
		BinaryPath:      targetPath,
	}

	if !runResult.AllHealthy {
		result.Status = "partial"
	}

	if jsonMode {
		return outputReplaceJSON(result)
	}

	return outputReplaceText(result, runResult)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func outputReplaceText(result ReplaceResult, runResult *devnet.RunResult) error {
	fmt.Println()
	output.Success("Binary replacement completed!")
	fmt.Println()
	output.Bold("Replace Summary")
	fmt.Println("-------------------------------------------------------------")
	if result.PreviousVersion != "" {
		fmt.Printf("  Previous Version: %s\n", result.PreviousVersion)
	}
	fmt.Printf("  New Version:      %s\n", result.NewVersion)
	if result.CommitHash != "" {
		fmt.Printf("  Commit Hash:      %s\n", result.CommitHash)
	}
	fmt.Printf("  Binary Path:      %s\n", result.BinaryPath)
	fmt.Println("-------------------------------------------------------------")
	fmt.Println()

	if runResult != nil {
		if len(runResult.FailedNodes) > 0 {
			output.Warn("Some nodes failed to restart:")
			for _, fn := range runResult.FailedNodes {
				fmt.Printf("  Node %d: %s\n", fn.Index, fn.Error)
			}
			fmt.Println()
		}

		output.Info("Successful nodes: %v", runResult.SuccessfulNodes)
	}

	output.Info("Use 'devnet-builder status' to verify chain health")
	fmt.Println()

	return nil
}

func outputReplaceJSON(result ReplaceResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputReplaceError(err error) error {
	if jsonMode {
		result := ReplaceResult{
			Status: "error",
			Error:  err.Error(),
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	}
	return err
}
