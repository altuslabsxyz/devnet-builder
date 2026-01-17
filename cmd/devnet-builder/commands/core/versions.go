package core

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/cmd/devnet-builder/shared"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/github"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	versionsList       bool
	versionsRefresh    bool
	versionsClearCache bool
	versionsCacheInfo  bool
)

// NewVersionsCmd creates the versions command.
func NewVersionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Manage version cache and list available versions",
		Long: `Manage the version cache and list available versions from GitHub.

Examples:
  # List available versions
  devnet-builder versions --list

  # Force refresh from GitHub
  devnet-builder versions --refresh

  # Clear the version cache
  devnet-builder versions --clear-cache

  # Show cache status
  devnet-builder versions --cache-info`,
		RunE: runVersions,
	}

	cmd.Flags().BoolVar(&versionsList, "list", false,
		"List all available versions")
	cmd.Flags().BoolVar(&versionsRefresh, "refresh", false,
		"Force refresh from GitHub API")
	cmd.Flags().BoolVar(&versionsClearCache, "clear-cache", false,
		"Delete cached version data")
	cmd.Flags().BoolVar(&versionsCacheInfo, "cache-info", false,
		"Show cache status (age, expiry)")

	return cmd
}

func runVersions(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger
	homeDir := shared.GetHomeDir()

	// Get config for cache settings
	fileCfg := shared.GetLoadedFileConfig()

	// Set up cache manager
	cacheTTL := github.DefaultCacheTTL
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		if ttl, err := time.ParseDuration(*fileCfg.CacheTTL); err == nil {
			cacheTTL = ttl
		}
	}
	cacheManager := github.NewCacheManager(homeDir, cacheTTL)

	// Handle cache info
	if versionsCacheInfo {
		return showVersionsCacheInfo(cacheManager)
	}

	// Handle cache clear
	if versionsClearCache {
		if err := cacheManager.Clear(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Println("Version cache cleared.")
		return nil
	}

	// Set up GitHub client
	clientOpts := []github.ClientOption{}
	if !versionsRefresh {
		clientOpts = append(clientOpts, github.WithCache(cacheManager))
	}
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		clientOpts = append(clientOpts, github.WithToken(*fileCfg.GitHubToken))
	}
	client := github.NewClient(clientOpts...)

	// Handle list or refresh
	if versionsList || versionsRefresh {
		return listVersionsFromGitHub(ctx, client, cacheManager, logger)
	}

	// Default: show help
	return cmd.Help()
}

func listVersionsFromGitHub(ctx context.Context, client *github.Client, cacheManager *github.CacheManager, logger *output.Logger) error {
	// Fetch versions
	releases, fromCache, err := client.FetchReleasesWithCache(ctx)
	if err != nil {
		// Check if it's a warning (stale data)
		if warning, ok := err.(*github.StaleDataWarning); ok {
			fmt.Printf("Warning: %s\n\n", warning.Message)
		} else {
			return fmt.Errorf("failed to fetch versions: %w", err)
		}
	}

	if len(releases) == 0 {
		fmt.Println("No versions found.")
		return nil
	}

	// Print header
	fmt.Printf("\nAvailable versions for %s/%s:\n\n", github.DefaultOwner, github.DefaultRepo)

	// Print versions
	for i, r := range releases {
		suffix := ""
		if i == 0 {
			suffix = " (latest)"
		} else if r.Prerelease {
			suffix = " (pre-release)"
		}
		fmt.Printf("  %-12s %s%s\n", r.TagName, r.PublishedAt.Format("2006-01-02"), suffix)
	}

	fmt.Printf("\nTotal: %d versions\n", len(releases))

	// Show data source
	// Note: fromCache is only true when API failed and we fell back to cache
	// In normal operation, we always fetch fresh data from GitHub API
	if fromCache {
		info, _ := cacheManager.Info()
		if info != nil {
			age := time.Since(info.FetchedAt).Round(time.Minute)
			fmt.Printf("Source: Cache fallback (fetched %s ago, API unavailable)\n", age)
		}
	} else {
		fmt.Println("Source: GitHub API (fresh)")
	}

	return nil
}

func showVersionsCacheInfo(cacheManager *github.CacheManager) error {
	info, err := cacheManager.Info()
	if err != nil {
		return fmt.Errorf("failed to get cache info: %w", err)
	}

	if info == nil {
		fmt.Println("No cache file exists.")
		return nil
	}

	fmt.Println("\nVersion Cache Status:")
	fmt.Printf("  Location: %s\n", info.Path)
	fmt.Printf("  Fetched:  %s\n", info.FetchedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Expires:  %s", info.ExpiresAt.Format("2006-01-02 15:04:05"))

	if info.IsExpired {
		fmt.Println(" (expired)")
	} else {
		remaining := time.Until(info.ExpiresAt).Round(time.Minute)
		fmt.Printf(" (in %s)\n", remaining)
	}

	fmt.Printf("  Versions: %d\n", info.VersionCount)

	return nil
}
