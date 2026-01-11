package main

import (
	"time"

	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/github"
)

// setupGitHubClient consolidates duplicate GitHub client setup logic that was
// previously duplicated between runDeployInteractiveSelection and runUpgradeInteractiveSelection.
//
// This function addresses User Story 2 (US2) by eliminating ~80% code duplication.
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
//
// Design Decision: Returns client directly (not error) since setup cannot fail.
// Token resolution failure is non-fatal - client works without token (rate-limited).
func setupGitHubClient(homeDir string, fileCfg *config.FileConfig) *github.Client {
	// Parse cache TTL from config, fall back to default if not set or invalid
	cacheTTL := github.DefaultCacheTTL
	if fileCfg != nil && fileCfg.CacheTTL != nil {
		if ttl, err := time.ParseDuration(*fileCfg.CacheTTL); err == nil {
			cacheTTL = ttl
		}
		// Note: If parsing fails, silently use default (non-fatal error)
	}

	// Create cache manager with resolved TTL
	cacheManager := github.NewCacheManager(homeDir, cacheTTL)

	// Build client options with cache
	clientOpts := []github.ClientOption{
		github.WithCache(cacheManager),
	}

	// Resolve GitHub token from keychain, environment, or config file
	// resolveGitHubToken checks multiple sources in priority order
	if token, found := resolveGitHubToken(fileCfg); found {
		clientOpts = append(clientOpts, github.WithToken(token))
	}
	// Note: If token not found, client will work without authentication (rate-limited)

	// Create and return configured client
	return github.NewClient(clientOpts...)
}
