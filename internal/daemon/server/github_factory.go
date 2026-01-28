package server

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/github"
)

// GitHubClientFactory creates GitHubClient instances for specific networks.
type GitHubClientFactory interface {
	// CreateClient creates a GitHubClient for the given owner/repo.
	// networkName is used for cache isolation.
	CreateClient(networkName, owner, repo string) ports.GitHubClient
}

// DefaultGitHubClientFactory is the default implementation of GitHubClientFactory.
type DefaultGitHubClientFactory struct {
	cacheDir string
	logger   *slog.Logger
}

// NewDefaultGitHubClientFactory creates a new DefaultGitHubClientFactory.
func NewDefaultGitHubClientFactory(dataDir string, logger *slog.Logger) *DefaultGitHubClientFactory {
	return &DefaultGitHubClientFactory{
		cacheDir: filepath.Join(dataDir, "cache"),
		logger:   logger,
	}
}

// CreateClient creates a GitHubClient for the given owner/repo.
// Each network gets its own cache subdirectory to avoid mixing data.
func (f *DefaultGitHubClientFactory) CreateClient(networkName, owner, repo string) ports.GitHubClient {
	// Create network-specific cache directory
	networkCacheDir := filepath.Join(f.cacheDir, networkName)

	// Get token from environment (standard GITHUB_TOKEN)
	token := os.Getenv("GITHUB_TOKEN")

	f.logger.Debug("creating GitHub client",
		"network", networkName,
		"owner", owner,
		"repo", repo,
		"cacheDir", networkCacheDir,
		"hasToken", token != "")

	return github.NewAdapter(token, owner, repo, networkCacheDir)
}
