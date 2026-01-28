// internal/daemon/builder/service.go
package builder

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	sdknetwork "github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// =============================================================================
// Build Options
// =============================================================================

// BuildOptions contains options for building a binary.
type BuildOptions struct {
	// NetworkType specifies the target network (e.g., "mainnet", "testnet", "devnet").
	// Used to get network-specific build configuration from the module.
	NetworkType string

	// GitRef is an optional git reference (branch, tag, or commit hash).
	// If empty, the module's DefaultBinaryVersion() is used.
	GitRef string

	// NoCache forces a rebuild even if a cached binary exists.
	NoCache bool

	// GoVersion is an optional Go version constraint for the build.
	GoVersion string

	// BuildFlagsOverride allows overriding the module's build flags.
	// These are merged with the module's GetBuildConfig() flags.
	BuildFlagsOverride map[string]string
}

// =============================================================================
// Binary Builder Service
// =============================================================================

// BinaryBuilderService builds binaries using NetworkModule for configuration.
// This service is GENERIC - it works with ANY network via the NetworkModule interface.
// The NetworkModule provides all chain-specific configuration (binary name, source, build config),
// while this service provides the actual build behavior.
type BinaryBuilderService struct {
	cache   *BinaryCache
	git     *GitOperations
	workDir string
	logger  *slog.Logger
}

// BinaryBuilderServiceConfig configures the BinaryBuilderService.
type BinaryBuilderServiceConfig struct {
	// WorkDir is the base directory for build artifacts and cache.
	WorkDir string

	// Logger for logging build progress.
	Logger *slog.Logger
}

// NewBinaryBuilderService creates a new BinaryBuilderService.
func NewBinaryBuilderService(config BinaryBuilderServiceConfig) *BinaryBuilderService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	cacheDir := filepath.Join(config.WorkDir, "binaries")
	return &BinaryBuilderService{
		cache:   NewBinaryCache(cacheDir),
		git:     &GitOperations{},
		workDir: config.WorkDir,
		logger:  logger,
	}
}

// Build builds a binary using configuration from the NetworkModule.
// The module provides:
//   - BinaryName() - name of the output binary
//   - BinarySource() - where to get the source (GitHub owner/repo)
//   - GetBuildConfig() - network-specific build configuration (tags, ldflags, env)
//
// Returns the path to the built binary, or an error.
func (s *BinaryBuilderService) Build(ctx context.Context, module network.NetworkModule, opts BuildOptions) (string, error) {
	binaryName := module.BinaryName()
	source := module.BinarySource()

	s.logger.Info("starting build",
		"network", module.Name(),
		"binaryName", binaryName,
		"networkType", opts.NetworkType,
	)

	// Determine git ref (use module default if not specified)
	gitRef := opts.GitRef
	if gitRef == "" {
		gitRef = module.DefaultBinaryVersion()
		s.logger.Debug("using default binary version", "version", gitRef)
	}

	// Build the git repo URL from source
	gitRepo := s.buildGitRepoURL(source)
	if gitRepo == "" {
		return "", fmt.Errorf("binary source does not specify a git repository")
	}

	// Get build configuration from module
	buildConfig, err := module.GetBuildConfig(opts.NetworkType)
	if err != nil {
		return "", fmt.Errorf("failed to get build config for network type %q: %w", opts.NetworkType, err)
	}

	// Create temp directory for git clone
	tempDir, err := os.MkdirTemp("", "dvb-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone repository
	s.logger.Debug("cloning repository", "repo", gitRepo, "dest", tempDir)
	if err := s.git.Clone(ctx, CloneOptions{
		Repo:    gitRepo,
		DestDir: tempDir,
		Depth:   1,
	}); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout the specified ref
	s.logger.Debug("checking out ref", "ref", gitRef)
	resolvedCommit, err := s.git.Checkout(ctx, CheckoutOptions{
		RepoDir: tempDir,
		Ref:     gitRef,
	})
	if err != nil {
		return "", fmt.Errorf("failed to checkout ref %q: %w", gitRef, err)
	}
	s.logger.Debug("resolved commit", "commit", resolvedCommit)

	// Build cache key
	cacheKey := s.buildCacheKey(module.Name(), resolvedCommit, buildConfig)

	// Check cache (unless NoCache is set)
	if !opts.NoCache {
		if cachedResult, found := s.cache.Get(cacheKey); found {
			s.logger.Info("cache hit", "cacheKey", cacheKey, "binaryPath", cachedResult.BinaryPath)
			return cachedResult.BinaryPath, nil
		}
	} else {
		s.logger.Info("skipping cache lookup (NoCache=true)")
	}

	// Create output directory
	outputDir := s.cache.CachePath(cacheKey)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Clean up output directory on build failure
	buildSuccess := false
	defer func() {
		if !buildSuccess {
			os.RemoveAll(outputDir)
		}
	}()

	// Build the binary
	s.logger.Info("building binary", "outputDir", outputDir)
	binaryPath := filepath.Join(outputDir, binaryName)

	if err := s.executeBuild(ctx, tempDir, binaryPath, buildConfig, resolvedCommit, gitRef, opts); err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}

	// Validate the built binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		return "", fmt.Errorf("binary validation failed: built binary not found at %s", binaryPath)
	}

	// Store in cache
	result := &BuildResult{
		BinaryPath: binaryPath,
		GitCommit:  resolvedCommit,
		GitRef:     gitRef,
		BuiltAt:    time.Now(),
		CacheKey:   cacheKey,
	}
	if err := s.cache.Store(result); err != nil {
		s.logger.Warn("failed to store result in cache", "error", err)
	}

	buildSuccess = true
	s.logger.Info("build completed successfully",
		"binaryPath", binaryPath,
		"commit", resolvedCommit,
		"cacheKey", cacheKey,
	)

	return binaryPath, nil
}

// GetCached returns a cached binary path if available.
// Returns the binary path and true if found, empty string and false otherwise.
func (s *BinaryBuilderService) GetCached(ctx context.Context, module network.NetworkModule, opts BuildOptions) (string, bool) {
	// We need to resolve the git ref to check cache, which requires network access.
	// For simplicity, we return false here - the Build method handles caching internally.
	return "", false
}

// Clean removes cached builds older than maxAge.
func (s *BinaryBuilderService) Clean(ctx context.Context, maxAge time.Duration) error {
	return s.cache.Clean(maxAge)
}

// =============================================================================
// Internal Methods
// =============================================================================

// buildGitRepoURL constructs a git repo URL from the BinarySource.
func (s *BinaryBuilderService) buildGitRepoURL(source network.BinarySource) string {
	if source.IsLocal() {
		return ""
	}

	if source.Owner == "" || source.Repo == "" {
		return ""
	}

	return fmt.Sprintf("https://github.com/%s/%s", source.Owner, source.Repo)
}

// buildCacheKey constructs a cache key from network name, commit, and build config.
func (s *BinaryBuilderService) buildCacheKey(networkName, commit string, buildConfig *sdknetwork.BuildConfig) string {
	// Include build config hash in cache key to separate builds with different configs
	configHash := "default"
	if buildConfig != nil && !buildConfig.IsEmpty() {
		configHash = buildConfig.Hash()
	}

	// Use first 12 chars of commit for readability
	shortCommit := commit
	if len(commit) > 12 {
		shortCommit = commit[:12]
	}

	return fmt.Sprintf("%s-%s-%s", networkName, shortCommit, configHash)
}

// executeBuild runs the actual build process.
func (s *BinaryBuilderService) executeBuild(ctx context.Context, sourceDir, outputPath string, buildConfig *sdknetwork.BuildConfig, commit, ref string, opts BuildOptions) error {
	// Build go build command with appropriate flags
	args := []string{"build"}

	// Add build tags
	var tags []string
	if buildConfig != nil && len(buildConfig.Tags) > 0 {
		tags = append(tags, buildConfig.Tags...)
	}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, " "))
	}

	// Add ldflags
	var ldflags []string
	if buildConfig != nil && len(buildConfig.LDFlags) > 0 {
		ldflags = append(ldflags, buildConfig.LDFlags...)
	}
	// Add version info
	ldflags = append(ldflags,
		fmt.Sprintf("-X main.Version=%s", ref),
		fmt.Sprintf("-X main.Commit=%s", commit),
	)
	if len(ldflags) > 0 {
		args = append(args, "-ldflags", strings.Join(ldflags, " "))
	}

	// Add output path
	args = append(args, "-o", outputPath)

	// Add main package (assume ./cmd/<binaryname> or .)
	mainPkg := s.findMainPackage(sourceDir, filepath.Base(outputPath))
	args = append(args, mainPkg)

	// Create command
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = sourceDir

	// Set environment
	env := os.Environ()
	if buildConfig != nil {
		for k, v := range buildConfig.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	cmd.Env = env

	s.logger.Debug("executing build",
		"args", args,
		"sourceDir", sourceDir,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		s.logger.Error("build command failed",
			"stdout", stdout.String(),
			"stderr", stderr.String(),
			"error", err,
		)
		return fmt.Errorf("go build failed: %w: %s", err, stderr.String())
	}

	return nil
}

// findMainPackage attempts to find the main package path for a binary.
func (s *BinaryBuilderService) findMainPackage(sourceDir, binaryName string) string {
	// Try common patterns
	patterns := []string{
		fmt.Sprintf("./cmd/%s", binaryName),
		"./cmd/...",
		".",
	}

	for _, pattern := range patterns {
		// Handle patterns with wildcards differently
		if strings.Contains(pattern, "...") {
			return pattern
		}

		path := filepath.Join(sourceDir, pattern)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			// Check if it contains Go files
			entries, err := os.ReadDir(path)
			if err == nil {
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".go") {
						return pattern
					}
				}
			}
		}
	}

	// Default to current directory
	return "."
}
