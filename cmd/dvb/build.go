// cmd/dvb/build.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/builder"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/cosmos"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// simplePluginLoader is a basic plugin loader for CLI usage
type simplePluginLoader struct{}

// GetBuilder returns the appropriate plugin builder for the given plugin name
func (l *simplePluginLoader) GetBuilder(pluginName string) (plugintypes.PluginBuilder, error) {
	switch pluginName {
	case "stable":
		return cosmos.NewCosmosBuilder("stabled", "github.com/cosmosphere-labs/stable"), nil
	case "cosmos", "gaia":
		return cosmos.NewCosmosBuilder("gaiad", "github.com/cosmos/gaia"), nil
	default:
		return nil, fmt.Errorf("unknown plugin: %s (supported: stable, cosmos, gaia)", pluginName)
	}
}

func newBuildCmd() *cobra.Command {
	var (
		gitRepo    string
		gitRef     string
		network    string
		buildFlags map[string]string
		goVersion  string
		noCache    bool
		timeout    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a binary from source",
		Long: `Build a blockchain binary from a git repository.

The build command clones a git repository, checks out the specified ref,
and compiles the binary using the appropriate plugin builder.

Built binaries are cached based on git commit hash. Subsequent builds
of the same commit will use the cached binary.

Examples:
  # Build stable binary from default repo at a specific tag
  dvb build --network stable --git-ref v1.0.0

  # Build from a custom repository
  dvb build --network stable --git-repo github.com/myorg/mychain --git-ref main

  # Build with custom Go version
  dvb build --network cosmos --git-ref v15.0.0 --go-version 1.21

  # Build with custom build flags
  dvb build --network stable --git-ref v1.0.0 --build-flags ldflags="-s -w"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, gitRepo, gitRef, network, buildFlags, goVersion, noCache, timeout)
		},
	}

	// Flags
	cmd.Flags().StringVar(&gitRepo, "git-repo", "", "Git repository URL (uses plugin default if not specified)")
	cmd.Flags().StringVar(&gitRef, "git-ref", "", "Git ref (branch, tag, or commit) - required")
	cmd.Flags().StringVar(&network, "network", "", "Network/plugin name (e.g., stable, cosmos) - required")
	cmd.Flags().StringToStringVar(&buildFlags, "build-flags", nil, "Build flags as key=value pairs (e.g., --build-flags ldflags=\"-s -w\")")
	cmd.Flags().StringVar(&goVersion, "go-version", "", "Go version constraint (e.g., 1.21)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Skip cache and force rebuild")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Build timeout duration")

	// Required flags
	_ = cmd.MarkFlagRequired("git-ref")
	_ = cmd.MarkFlagRequired("network")

	return cmd
}

func runBuild(cmd *cobra.Command, gitRepo, gitRef, network string, buildFlags map[string]string, goVersion string, noCache bool, timeout time.Duration) error {
	// Get data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	dataDir := filepath.Join(homeDir, ".devnet-builder")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create plugin loader
	pluginLoader := &simplePluginLoader{}

	// Create builder
	b := builder.NewDefaultBuilder(dataDir, pluginLoader, logger)

	// Create build spec
	spec := builder.BuildSpec{
		GitRepo:    gitRepo,
		GitRef:     gitRef,
		PluginName: network,
		BuildFlags: buildFlags,
		GoVersion:  goVersion,
		NoCache:    noCache,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	// Print build info
	fmt.Printf("Building binary...\n")
	fmt.Printf("  Network:  %s\n", network)
	fmt.Printf("  Git Ref:  %s\n", gitRef)
	if gitRepo != "" {
		fmt.Printf("  Git Repo: %s\n", gitRepo)
	}
	if goVersion != "" {
		fmt.Printf("  Go:       %s\n", goVersion)
	}
	fmt.Printf("  Timeout:  %s\n", timeout)
	fmt.Println()

	// Run build
	startTime := time.Now()
	result, err := b.Build(ctx, spec)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	buildDuration := time.Since(startTime)

	// Print success output
	color.Green("Build successful!")
	fmt.Println()
	fmt.Printf("  Binary:     %s\n", result.BinaryPath)
	fmt.Printf("  Git Commit: %s\n", result.GitCommit)
	fmt.Printf("  Git Ref:    %s\n", result.GitRef)
	fmt.Printf("  Built At:   %s\n", result.BuiltAt.Format(time.RFC3339))
	fmt.Printf("  Build Time: %s\n", buildDuration.Round(time.Second))
	fmt.Printf("  Cache Key:  %s\n", result.CacheKey)

	return nil
}
