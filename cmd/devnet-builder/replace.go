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
	"github.com/stablelabs/stable-devnet/internal/github"
	"github.com/stablelabs/stable-devnet/internal/interactive"
	"github.com/stablelabs/stable-devnet/internal/output"
)

var (
	replaceVersion       string
	replaceHealthTimeout time.Duration
	replaceNoConfirm     bool
	replaceNoInteractive bool
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
  devnet-builder replace --version v1.2.0 -y

  # Non-interactive mode (requires --version)
  devnet-builder replace --version v1.2.0 --no-interactive`,
		RunE: runReplace,
	}

	cmd.Flags().StringVarP(&replaceVersion, "version", "V", "",
		"Version/ref to build (tag, branch, or commit)")
	cmd.Flags().DurationVar(&replaceHealthTimeout, "health-timeout", 5*time.Minute,
		"Timeout for node health check after restart")
	cmd.Flags().BoolVarP(&replaceNoConfirm, "yes", "y", false,
		"Skip confirmation prompt")
	cmd.Flags().BoolVar(&replaceNoInteractive, "no-interactive", false,
		"Disable interactive prompts (requires --version)")

	return cmd
}

func runReplace(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Load devnet metadata using consolidated helper
	metadata, err := loadMetadataOrFail(logger)
	if err != nil {
		return outputReplaceError(err)
	}

	// Interactive mode: if no version specified, show interactive selector
	if replaceVersion == "" {
		if replaceNoInteractive || jsonMode {
			return outputReplaceError(fmt.Errorf("--version is required in non-interactive mode"))
		}

		// Run interactive version selection
		selectedVersion, err := runReplaceInteractiveSelection(ctx, metadata.CurrentVersion)
		if err != nil {
			if interactive.IsCancellation(err) {
				output.Info("Operation cancelled.")
				return nil
			}
			return outputReplaceError(err)
		}
		replaceVersion = selectedVersion
	}

	// Show warning and get confirmation (skip if already confirmed in interactive mode or -y flag)
	if !jsonMode && !replaceNoConfirm && !replaceNoInteractive {
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

	networkModule, _ := metadata.GetNetworkModule()
	b := builder.NewBuilder(homeDir, logger, networkModule)
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

// runReplaceInteractiveSelection runs the interactive version selection for replace command.
func runReplaceInteractiveSelection(ctx context.Context, currentVersion string) (string, error) {
	// Initialize GitHub client with token from config
	clientOpts := []github.ClientOption{}
	fileCfg := GetLoadedFileConfig()
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		clientOpts = append(clientOpts, github.WithToken(*fileCfg.GitHubToken))
	}
	client := github.NewClient(clientOpts...)

	// Fetch available versions
	releases, fromCache, err := client.FetchReleasesWithCache(ctx)
	if err != nil {
		// Check if it's a warning (stale data)
		if warning, ok := err.(*github.StaleDataWarning); ok {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning.Message)
		} else {
			return "", fmt.Errorf("failed to fetch versions: %w", err)
		}
	}

	if len(releases) == 0 {
		return "", fmt.Errorf("no versions available. Check your network connection or GitHub token")
	}

	if fromCache {
		fmt.Println("(Using cached version data)")
	}

	// Show current version info
	fmt.Printf("\nCurrent version: %s\n", currentVersion)
	fmt.Println("Note: Even if you select the same version, the binary will be rebuilt (new commits may exist).")
	fmt.Println()

	// Select replacement version
	version, _, err := interactive.SelectVersion("Select replacement binary version", releases, currentVersion)
	if err != nil {
		return "", err
	}

	// Confirm selection
	fmt.Printf("\nReplace binary configuration:\n")
	fmt.Printf("  Current version: %s\n", currentVersion)
	fmt.Printf("  Target version:  %s\n", version)
	fmt.Println()

	confirmed, err := interactive.ConfirmReplaceSelection(version)
	if err != nil {
		return "", err
	}
	if !confirmed {
		return "", &interactive.CancellationError{Message: "Operation cancelled by user"}
	}

	return version, nil
}
