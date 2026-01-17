package core

import (
	"os"
	"time"

	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/github"
)

// SetupGitHubClient creates a GitHub client with optional caching and token.
//
// This function consolidates duplicate GitHub client setup logic that was
// previously duplicated between runDeployInteractiveSelection and runUpgradeInteractiveSelection.
//
// Functionality:
//   - Parses cache TTL from config with fallback to default
//   - Creates cache manager with appropriate TTL
//   - Resolves GitHub token from keychain, environment, or config file
//   - Builds GitHub client with cache and optional token
//
// Parameters:
//   - homeDir: Home directory path for cache storage (e.g., ~/.devnet-builder)
//   - fileCfg: Loaded file configuration (may be nil if not loaded)
//
// Returns:
//   - Configured GitHub client ready for use with interactive.Selector
func SetupGitHubClient(homeDir string, fileCfg *config.FileConfig) *github.Client {
	// Parse cache TTL from config, fall back to default if not set or invalid
	cacheTTL := github.DefaultCacheTTL
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		if ttl, err := time.ParseDuration(*fileCfg.CacheTTL); err == nil {
			cacheTTL = ttl
		}
	}

	// Create cache manager with resolved TTL
	cacheManager := github.NewCacheManager(homeDir, cacheTTL)

	// Build client options with cache
	clientOpts := []github.ClientOption{
		github.WithCache(cacheManager),
	}

	// Resolve GitHub token from keychain, environment, or config file
	if token, found := ResolveGitHubToken(fileCfg); found {
		clientOpts = append(clientOpts, github.WithToken(token))
	}

	return github.NewClient(clientOpts...)
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
