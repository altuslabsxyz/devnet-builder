// internal/daemon/builder/builder.go
package builder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// PluginLoader loads plugin builders by name
type PluginLoader interface {
	GetBuilder(pluginName string) (plugintypes.PluginBuilder, error)
}

// DefaultBuilder implements BinaryBuilder using git, cache, and plugins
type DefaultBuilder struct {
	cache        *BinaryCache
	git          *GitOperations
	pluginLoader PluginLoader
	dataDir      string
	logger       *slog.Logger
}

// NewDefaultBuilder creates a new DefaultBuilder
func NewDefaultBuilder(dataDir string, pluginLoader PluginLoader, logger *slog.Logger) *DefaultBuilder {
	if logger == nil {
		logger = slog.Default()
	}

	cacheDir := filepath.Join(dataDir, "binaries")
	return &DefaultBuilder{
		cache:        NewBinaryCache(cacheDir),
		git:          &GitOperations{},
		pluginLoader: pluginLoader,
		dataDir:      dataDir,
		logger:       logger,
	}
}

// Build builds a binary from source and returns the path to the built binary
func (b *DefaultBuilder) Build(ctx context.Context, spec BuildSpec) (*BuildResult, error) {
	b.logger.Info("starting build",
		"plugin", spec.PluginName,
		"repo", spec.GitRepo,
		"ref", spec.GitRef,
	)

	// Get plugin builder
	pluginBuilder, err := b.pluginLoader.GetBuilder(spec.PluginName)
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin builder for %q: %w", spec.PluginName, err)
	}

	// Use default repo if not specified
	gitRepo := spec.GitRepo
	if gitRepo == "" {
		gitRepo = pluginBuilder.DefaultGitRepo()
		b.logger.Debug("using default git repo", "repo", gitRepo)
	}

	// Ensure repo is a valid URL
	repoURL := gitRepo
	if !isURL(gitRepo) {
		repoURL = "https://" + gitRepo
	}

	// Create temp directory for git clone
	tempDir, err := os.MkdirTemp("", "dvb-build-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up on any exit

	// Clone repository (shallow clone with depth=1 for speed)
	b.logger.Debug("cloning repository", "repo", repoURL, "dest", tempDir)
	if err := b.git.Clone(ctx, CloneOptions{
		Repo:    repoURL,
		DestDir: tempDir,
		Depth:   1,
	}); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Fetch and checkout the specified ref
	gitRef := spec.GitRef
	if gitRef == "" {
		gitRef, err = b.git.GetRemoteDefaultBranch(ctx, tempDir)
		if err != nil {
			gitRef = "main" // fallback
		}
		b.logger.Debug("using default git ref", "ref", gitRef)
	}

	b.logger.Debug("checking out ref", "ref", gitRef)
	resolvedCommit, err := b.git.Checkout(ctx, CheckoutOptions{
		RepoDir: tempDir,
		Ref:     gitRef,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to checkout ref %q: %w", gitRef, err)
	}
	b.logger.Debug("resolved commit", "commit", resolvedCommit)

	// Check cache with resolved commit (unless NoCache is set)
	cacheKey := b.cache.CacheKey(spec, resolvedCommit)
	if !spec.NoCache {
		if cachedResult, found := b.cache.Get(cacheKey); found {
			b.logger.Info("cache hit", "cacheKey", cacheKey, "binaryPath", cachedResult.BinaryPath)
			return cachedResult, nil
		}
	} else {
		b.logger.Info("skipping cache lookup (--no-cache)")
	}

	// Create output directory via cache path
	outputDir := b.cache.CachePath(cacheKey)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Clean up output directory on build failure
	buildSuccess := false
	defer func() {
		if !buildSuccess {
			os.RemoveAll(outputDir)
		}
	}()

	// Merge build flags (plugin defaults + spec overrides)
	mergedFlags := mergeBuildFlags(pluginBuilder.DefaultBuildFlags(), spec.BuildFlags)

	// Build the binary
	b.logger.Info("building binary", "outputDir", outputDir)
	buildOpts := plugintypes.BuildOptions{
		SourceDir: tempDir,
		OutputDir: outputDir,
		Flags:     mergedFlags,
		GoVersion: spec.GoVersion,
		GitCommit: resolvedCommit,
		GitRef:    gitRef,
		Logger:    b.logger,
	}

	if err := pluginBuilder.BuildBinary(ctx, buildOpts); err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	// Get binary path
	binaryName := pluginBuilder.BinaryName()
	binaryPath := filepath.Join(outputDir, binaryName)

	// Validate the built binary
	b.logger.Debug("validating binary", "path", binaryPath)
	if err := pluginBuilder.ValidateBinary(ctx, binaryPath); err != nil {
		return nil, fmt.Errorf("binary validation failed: %w", err)
	}

	// Create build result
	result := &BuildResult{
		BinaryPath: binaryPath,
		GitCommit:  resolvedCommit,
		GitRef:     gitRef,
		BuiltAt:    time.Now(),
		CacheKey:   cacheKey,
	}

	// Store in cache
	if err := b.cache.Store(result); err != nil {
		b.logger.Warn("failed to store result in cache", "error", err)
		// Don't fail the build if caching fails
	}

	buildSuccess = true
	b.logger.Info("build completed successfully",
		"binaryPath", binaryPath,
		"commit", resolvedCommit,
		"cacheKey", cacheKey,
	)

	return result, nil
}

// GetCached returns a cached build if available and valid
// Note: Returns nil, false because we need to resolve the git ref to a commit
// before we can check the cache. The cache key depends on the resolved commit.
func (b *DefaultBuilder) GetCached(ctx context.Context, spec BuildSpec) (*BuildResult, bool) {
	// We can't check cache without first resolving the git ref to a commit,
	// which requires cloning the repo. The Build method handles this.
	return nil, false
}

// Clean removes cached builds older than maxAge
func (b *DefaultBuilder) Clean(ctx context.Context, maxAge time.Duration) error {
	return b.cache.Clean(maxAge)
}

// isURL checks if a string looks like a URL (has protocol prefix or is a git@ URL)
func isURL(s string) bool {
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "git@")
}

// mergeBuildFlags merges default flags with override flags
// Overrides take precedence over defaults
func mergeBuildFlags(defaults, overrides map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy defaults
	for k, v := range defaults {
		result[k] = v
	}

	// Apply overrides
	for k, v := range overrides {
		result[k] = v
	}

	return result
}
