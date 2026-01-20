// Package manage provides devnet lifecycle management commands.
package manage

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/github"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/interactive"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

// GetLoadedFileConfig returns the loaded config.toml values via the shared accessor.
func GetLoadedFileConfig() *config.FileConfig {
	return shared.GetLoadedFileConfig()
}

// jsonMode returns the current JSON mode setting.
func jsonMode() bool {
	return shared.GetJSONMode()
}

// homeDir returns the configured home directory.
func homeDir() string {
	return shared.GetHomeDir()
}

// WrapInteractiveError wraps interactive errors with additional context.
func WrapInteractiveError(cmd *cobra.Command, err error, context string) error {
	if err == nil {
		return nil
	}

	// Check for cancellation errors
	if interactive.IsCancellation(err) {
		return err
	}

	return fmt.Errorf("%s: %w", context, err)
}

// SetupGitHubClient creates a GitHub client with optional caching and token.
func SetupGitHubClient(homeDir string, fileCfg *config.FileConfig) *github.Client {
	cacheTTL := github.DefaultCacheTTL
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		if ttl, err := time.ParseDuration(*fileCfg.CacheTTL); err == nil {
			cacheTTL = ttl
		}
	}

	cacheManager := github.NewCacheManager(homeDir, cacheTTL)

	clientOpts := []github.ClientOption{
		github.WithCache(cacheManager),
	}

	// Resolve GitHub token from keychain, environment, or config file
	if token, found := ResolveGitHubToken(fileCfg); found {
		clientOpts = append(clientOpts, github.WithToken(token))
	}

	return github.NewClient(clientOpts...)
}

// RunInteractiveVersionSelection runs the unified version selection flow.
func RunInteractiveVersionSelection(ctx context.Context, cmd *cobra.Command, includeNetworkSelection bool, network string) (*interactive.SelectionConfig, error) {
	fileCfg := GetLoadedFileConfig()

	client := SetupGitHubClient(shared.GetHomeDir(), fileCfg)

	selector := interactive.NewSelector(client)
	if includeNetworkSelection {
		return selector.RunSelectionFlow(ctx)
	}
	return selector.RunVersionSelectionFlow(ctx, network)
}

// RunInteractiveVersionSelectionWithMode runs version selection with upgrade mode support.
// When forUpgrade is true, returns a SelectionConfig derived from UpgradeSelectionConfig.
// When skipUpgradeName is true (e.g., --skip-gov mode), it skips the upgrade name prompt.
func RunInteractiveVersionSelectionWithMode(ctx context.Context, cmd *cobra.Command, includeNetworkSelection bool, forUpgrade bool, network string, skipUpgradeName bool) (*interactive.SelectionConfig, error) {
	fileCfg := GetLoadedFileConfig()

	client := SetupGitHubClient(shared.GetHomeDir(), fileCfg)

	selector := interactive.NewSelector(client)
	if forUpgrade {
		upgradeConfig, err := selector.RunUpgradeSelectionFlow(ctx, skipUpgradeName)
		if err != nil {
			return nil, err
		}
		// Convert UpgradeSelectionConfig to SelectionConfig
		return &interactive.SelectionConfig{
			StartVersion:     upgradeConfig.UpgradeVersion,
			StartIsCustomRef: upgradeConfig.IsCustomRef,
			UpgradeName:      upgradeConfig.UpgradeName,
		}, nil
	}
	if includeNetworkSelection {
		return selector.RunSelectionFlow(ctx)
	}
	return selector.RunVersionSelectionFlow(ctx, network)
}

// ResolveGitHubToken resolves the GitHub token from environment or config.
// Priority: environment variable > config file.
func ResolveGitHubToken(fileCfg *config.FileConfig) (string, bool) {
	// Priority 1: Environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, true
	}

	// Priority 2: Config file
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		return *fileCfg.GitHubToken, true
	}

	return "", false
}

// confirmPrompt shows a confirmation prompt.
func confirmPrompt(message string) (bool, error) {
	return output.ConfirmPrompt(message)
}

// formatBytes formats bytes as human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
