package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/cache"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	sdknetwork "github.com/altuslabsxyz/devnet-builder/pkg/network"
)

const (
	// DefaultBinaryName is the fallback binary name when no network module is available.
	DefaultBinaryName = "binary"
)

// GoreleaserConfig contains network-specific goreleaser configuration.
type GoreleaserConfig struct {
	ProjectName string
	BinaryName  string
	MainPath    string
	Toolchain   string
	LDFlags     []string
	Tags        []string
}

// DefaultGoreleaserConfig returns a default goreleaser configuration.
func DefaultGoreleaserConfig(binaryName, toolchain string) *GoreleaserConfig {
	return &GoreleaserConfig{
		ProjectName: binaryName,
		BinaryName:  binaryName,
		MainPath:    fmt.Sprintf("./cmd/%s", binaryName),
		Toolchain:   toolchain,
		LDFlags: []string{
			"-X github.com/cosmos/cosmos-sdk/version.Version={{ .Version }}",
			"-X github.com/cosmos/cosmos-sdk/version.Commit={{ .Commit }}",
			"-w -s",
		},
		Tags: []string{
			"netgo",
			"ledger",
		},
	}
}

// GenerateGoreleaserConfig generates a goreleaser config file content.
func GenerateGoreleaserConfig(cfg *GoreleaserConfig) string {
	ldflags := ""
	for _, ldf := range cfg.LDFlags {
		ldflags += fmt.Sprintf("\n      - %s", ldf)
	}

	tags := ""
	for _, tag := range cfg.Tags {
		tags += fmt.Sprintf("\n      - %s", tag)
	}

	return fmt.Sprintf(`# Devnet-builder goreleaser config (auto-generated)
version: 2

project_name: %s

env:
  - CGO_ENABLED=1
  - GOTOOLCHAIN=%s
  - GOPROXY=https://proxy.golang.org,direct
  - GOSUMDB=sum.golang.org

before:
  hooks:
    - go version
    - go mod download

builds:
  - id: %s-devnet
    main: %s
    binary: %s
    env:
      - CGO_ENABLED=1
      - CGO_CFLAGS=-O3 -g0 -DNDEBUG
      - CGO_LDFLAGS=-O3 -s
    flags:
      - -mod=readonly
      - -trimpath
    ldflags:%s
    tags:%s

archives:
  - id: devnet
    format: tar.gz
    name_template: "{{ .ProjectName }}-{{ .Version }}-{{ .Os }}-{{ .Arch }}-devnet"

checksum:
  name_template: sha256sum.txt
  algorithm: sha256

snapshot:
  version_template: "devnet-{{ .ShortCommit }}"

changelog:
  disable: true
`, cfg.ProjectName, cfg.Toolchain, cfg.BinaryName, cfg.MainPath, cfg.BinaryName, ldflags, tags)
}

// Builder handles building binaries from source.
type Builder struct {
	homeDir string
	logger  *output.Logger
	module  network.NetworkModule
}

// NewBuilder creates a new Builder with the specified NetworkModule.
// A NetworkModule is required for building - use network plugins to provide one.
func NewBuilder(homeDir string, logger *output.Logger, networkModule network.NetworkModule) *Builder {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &Builder{
		homeDir: homeDir,
		logger:  logger,
		module:  networkModule,
	}
}

// getRepoURL returns the repository URL for the current network module.
// If GITHUB_TOKEN is set in the environment, it will be included in the URL for authentication.
func (b *Builder) getRepoURL() (string, error) {
	if b.module == nil {
		return "", fmt.Errorf("no network module configured - use a network plugin")
	}
	src := b.module.BinarySource()
	if src.IsGitHub() {
		// Check for GITHUB_TOKEN environment variable
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			// Use authenticated URL with token
			return fmt.Sprintf("https://x:%s@github.com/%s/%s.git", token, src.Owner, src.Repo), nil
		}
		// No token, use unauthenticated URL
		return fmt.Sprintf("https://github.com/%s/%s.git", src.Owner, src.Repo), nil
	}
	if src.LocalPath != "" {
		return src.LocalPath, nil
	}
	return "", fmt.Errorf("network module %s has no valid binary source", b.module.Name())
}

// getBinaryName returns the binary name for the current network module.
func (b *Builder) getBinaryName() string {
	if b.module != nil {
		return b.module.BinaryName()
	}
	return DefaultBinaryName
}

// BuildOptions configures the build process.
type BuildOptions struct {
	Ref       string // Git ref (branch, tag, commit hash)
	OutputDir string // Directory to place the built binary
	Network   string // Network type (mainnet, testnet) - determines EVMChainID
}

// BuildResult contains the result of a build.
type BuildResult struct {
	BinaryPath string // Path to the built binary
	Ref        string // The ref that was built
	CommitHash string // The commit hash that was built
}

// Build builds a binary from the given ref, stores it in cache, and
// updates the symlink at ~/.devnet-builder/bin/{binaryName} to point to it.
//
// This is used by `start` command where the binary should be used immediately.
// For `upgrade` command, use BuildToCache() which only caches without symlink change.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	if opts.Ref == "" {
		return nil, fmt.Errorf("ref is required")
	}

	// Initialize cache and symlink manager with network's binary name
	binaryName := b.getBinaryName()
	binaryCache := cache.NewBinaryCache(b.homeDir, binaryName, b.logger)
	if err := binaryCache.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize binary cache: %w", err)
	}
	symlinkMgr := cache.NewSymlinkManager(b.homeDir, binaryName)

	// Get network-specific build configuration from plugin (T027)
	networkType := opts.Network
	if networkType == "" {
		networkType = "devnet" // Default to devnet if not specified
	}

	buildConfig, err := b.module.GetBuildConfig(networkType)
	if err != nil {
		b.logger.Warn("Failed to get build config for network %s: %v, using defaults", networkType, err)
		buildConfig = &sdknetwork.BuildConfig{}
	}
	if buildConfig == nil {
		buildConfig = &sdknetwork.BuildConfig{}
	}

	// Log build configuration for audit trail (FR-010)
	if !buildConfig.IsEmpty() {
		b.logger.Debug("Using build config for network %s: %s", networkType, buildConfig.String())
	}

	// Legacy: Also get build tags from BinarySource for backward compatibility
	src := b.module.BinarySource()
	buildTags := src.BuildTags

	// Try to resolve commit hash first to check cache
	commitHash, resolveErr := b.ResolveCommitHash(ctx, opts.Ref)
	if resolveErr == nil && commitHash != "" {
		// Check if already cached with matching build config (fast path - no build needed)
		if binaryCache.IsCachedWithConfig(networkType, commitHash, buildConfig) {
			b.logger.Info("Using cached binary for %s/%s (commit: %s)", networkType, opts.Ref, commitHash[:12])
			// Get the cached binary path
			cachedBinary := binaryCache.LookupWithConfig(networkType, commitHash, buildConfig)
			if cachedBinary == nil {
				b.logger.Warn("Cache check succeeded but lookup failed, rebuilding")
			} else {
				// Update symlink to point to cached binary
				if err := symlinkMgr.SwitchToCacheWithConfig(binaryCache, networkType, commitHash, buildConfig); err != nil {
					return nil, fmt.Errorf("failed to update symlink: %w", err)
				}
				b.logger.Success("Symlink updated: %s -> %s", symlinkMgr.SymlinkPath(), commitHash[:12])
				return &BuildResult{
					BinaryPath: symlinkMgr.SymlinkPath(),
					Ref:        opts.Ref,
					CommitHash: commitHash,
				}, nil
			}
		}
	}

	// Create build directory
	buildDir := filepath.Join(b.homeDir, "build", "source", opts.Ref)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create build directory: %w", err)
	}

	// Clone repository
	b.logger.Info("Cloning %s repository...", b.module.DisplayName())
	repoDir := filepath.Join(buildDir, b.module.Name())
	if err := b.cloneRepo(ctx, repoDir, opts.Ref); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get commit hash from cloned repo (more reliable)
	commitHash, err = b.getCommitHash(ctx, repoDir)
	if err != nil {
		b.logger.Warn("Failed to get commit hash: %v", err)
		commitHash = opts.Ref
	}

	// Double-check cache after clone with build config (in case ls-remote failed earlier)
	if binaryCache.IsCachedWithConfig(networkType, commitHash, buildConfig) {
		b.logger.Info("Using cached binary for %s/%s (commit: %s)", networkType, opts.Ref, commitHash[:12])
		if err := symlinkMgr.SwitchToCacheWithConfig(binaryCache, networkType, commitHash, buildConfig); err != nil {
			return nil, fmt.Errorf("failed to update symlink: %w", err)
		}
		b.logger.Success("Symlink updated: %s -> %s", symlinkMgr.SymlinkPath(), commitHash[:12])
		return &BuildResult{
			BinaryPath: symlinkMgr.SymlinkPath(),
			Ref:        opts.Ref,
			CommitHash: commitHash,
		}, nil
	}

	// Read toolchain version from go.mod and create devnet-specific goreleaser config
	toolchain := b.readToolchainFromGoMod(repoDir)
	cfg := DefaultGoreleaserConfig(binaryName, toolchain)

	// Merge network-specific build configuration (T028 - BuildConfig merge)
	// Plugin configuration overrides defaults
	if len(buildConfig.Tags) > 0 {
		cfg.Tags = append(cfg.Tags, buildConfig.Tags...)
	}
	if len(buildConfig.LDFlags) > 0 {
		cfg.LDFlags = append(cfg.LDFlags, buildConfig.LDFlags...)
	}

	// Legacy: Append build tags from BinarySource for backward compatibility
	if len(buildTags) > 0 {
		cfg.Tags = append(cfg.Tags, buildTags...)
	}

	b.logger.Debug("Final build config - Tags: %v, LDFlags: %v", cfg.Tags, cfg.LDFlags)

	configContent := GenerateGoreleaserConfig(cfg)
	configPath := filepath.Join(repoDir, ".goreleaser.devnet.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create goreleaser config: %w", err)
	}
	b.logger.Info("Building %s for network: %s", binaryName, b.module.DisplayName())

	// Build using embedded goreleaser
	b.logger.Info("Building binary with goreleaser (ref: %s)...", opts.Ref)
	if err := b.buildWithGoreleaser(ctx, repoDir, configPath); err != nil {
		return nil, fmt.Errorf("goreleaser build failed: %w", err)
	}

	// Find the built binary in goreleaser dist directory
	binaryPath, err := b.findBuiltBinary(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find built binary: %w", err)
	}

	// Store in cache with build configuration
	cached := &cache.CachedBinary{
		CommitHash:  commitHash,
		Ref:         opts.Ref,
		BuildTime:   time.Now(),
		NetworkType: networkType, // Network-aware caching
		BuildConfig: buildConfig, // Full build configuration
	}
	if err := binaryCache.Store(binaryPath, cached); err != nil {
		// Graceful degradation: log warning but continue (FR-008)
		b.logger.Warn("Failed to store binary in cache: %v (continuing with uncached binary)", err)
		// Binary is still usable at binaryPath, just won't be cached for future use
	} else {
		// Update symlink to point to newly cached binary
		if err := symlinkMgr.SwitchToCacheWithConfig(binaryCache, networkType, commitHash, buildConfig); err != nil {
			b.logger.Warn("Failed to update symlink: %v (binary still available at %s)", err, binaryPath)
		}
	}

	b.logger.Success("Binary built and active: %s (commit: %s)", symlinkMgr.SymlinkPath(), commitHash[:12])

	return &BuildResult{
		BinaryPath: symlinkMgr.SymlinkPath(),
		Ref:        opts.Ref,
		CommitHash: commitHash,
	}, nil
}

// sanitizeURL removes sensitive tokens from URLs for safe logging.
func sanitizeURL(url string) string {
	// Match pattern: https://x:TOKEN@github.com/...
	// Replace TOKEN with ***
	if strings.Contains(url, "@github.com") {
		parts := strings.Split(url, "@")
		if len(parts) == 2 && strings.Contains(parts[0], ":") {
			// Format: https://x:TOKEN
			credParts := strings.Split(parts[0], ":")
			if len(credParts) >= 2 {
				return credParts[0] + ":***@" + parts[1]
			}
		}
	}
	return url
}

// prepareGitCommand configures a git command with proper authentication environment.
// This ensures git commands can authenticate even in test/CI environments where
// interactive prompts are disabled.
func prepareGitCommand(cmd *exec.Cmd) {
	// Start with current environment
	env := os.Environ()

	// Add/override git-specific authentication environment variables
	gitEnv := []string{
		"GIT_TERMINAL_PROMPT=0", // Disable interactive prompts
	}

	// Preserve authentication credentials if set
	if askpass := os.Getenv("GIT_ASKPASS"); askpass != "" {
		gitEnv = append(gitEnv, "GIT_ASKPASS="+askpass)
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		gitEnv = append(gitEnv, "GITHUB_TOKEN="+token)
	}
	if sshAskpass := os.Getenv("SSH_ASKPASS"); sshAskpass != "" {
		gitEnv = append(gitEnv, "SSH_ASKPASS="+sshAskpass)
	}

	env = append(env, gitEnv...)
	cmd.Env = env
}

// cloneRepo clones the repository and checks out the given ref.
func (b *Builder) cloneRepo(ctx context.Context, repoDir, ref string) error {
	// Remove existing directory if exists
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}

	repoURL, err := b.getRepoURL()
	if err != nil {
		return fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Clone with depth 1 for efficiency (if it's a branch/tag)
	// For commit hashes, we need full history
	args := []string{"clone"}
	if !isCommitHash(ref) {
		args = append(args, "--depth", "1", "--branch", ref)
	}
	args = append(args, repoURL, repoDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	prepareGitCommand(cmd)

	// Capture stderr for detailed error reporting
	var stderrBuf strings.Builder
	cmd.Stdout = b.logger.Writer()
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		// Log git stderr for debugging
		if stderrBuf.Len() > 0 {
			b.logger.Debug("Git clone stderr: %s", stderrBuf.String())
		}

		// If shallow clone failed (maybe branch doesn't exist), try full clone
		if !isCommitHash(ref) {
			b.logger.Debug("Shallow clone failed, trying full clone...")
			args = []string{"clone", repoURL, repoDir}
			cmd = exec.CommandContext(ctx, "git", args...)
			prepareGitCommand(cmd)

			stderrBuf.Reset()
			cmd.Stdout = b.logger.Writer()
			cmd.Stderr = &stderrBuf

			if err := cmd.Run(); err != nil {
				if stderrBuf.Len() > 0 {
					return fmt.Errorf("git clone failed: %w (stderr: %s)", err, stderrBuf.String())
				}
				return fmt.Errorf("git clone failed: %w", err)
			}
		} else {
			if stderrBuf.Len() > 0 {
				return fmt.Errorf("git clone failed: %w (stderr: %s)", err, stderrBuf.String())
			}
			return fmt.Errorf("git clone failed: %w", err)
		}
	}

	// Checkout the specific ref if it's a commit hash or if shallow clone didn't specify branch
	if isCommitHash(ref) {
		cmd = exec.CommandContext(ctx, "git", "checkout", ref)
		cmd.Dir = repoDir
		prepareGitCommand(cmd)
		cmd.Stdout = b.logger.Writer()
		cmd.Stderr = b.logger.Writer()
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git checkout failed: %w", err)
		}
	}

	return nil
}

// getCommitHash returns the current commit hash.
func (b *Builder) getCommitHash(ctx context.Context, repoDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	prepareGitCommand(cmd)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// buildWithGoreleaser builds the binary using goreleaser as a subprocess.
func (b *Builder) buildWithGoreleaser(ctx context.Context, repoDir, configPath string) error {
	// Build goreleaser arguments
	args := []string{
		"build",
		"-f", configPath,
		"--single-target",
		"--snapshot",
		"--clean",
	}

	// Execute goreleaser as subprocess
	cmd := exec.CommandContext(ctx, "goreleaser", args...)
	cmd.Dir = repoDir
	cmd.Stdout = b.logger.Writer()
	cmd.Stderr = b.logger.Writer()
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goreleaser build failed: %w", err)
	}

	return nil
}

// readToolchainFromGoMod reads the toolchain directive from go.mod.
// Returns the toolchain version (e.g., "go1.23.11") or "auto" if not found.
func (b *Builder) readToolchainFromGoMod(repoDir string) string {
	goModPath := filepath.Join(repoDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		b.logger.Debug("Failed to read go.mod: %v", err)
		return "auto"
	}

	// Parse go.mod for toolchain directive
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "toolchain ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return "auto"
}

// findBuiltBinary finds the built binary in the goreleaser dist directory.
func (b *Builder) findBuiltBinary(repoDir string) (string, error) {
	distDir := filepath.Join(repoDir, "dist")
	binaryName := b.getBinaryName()

	// Get current OS and arch
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Try common patterns for goreleaser output
	patterns := []string{
		// Format: dist/<binary>-devnet_<os>_<arch>/<binary>
		filepath.Join(distDir, fmt.Sprintf("%s-devnet_%s_%s*", binaryName, goos, goarch), binaryName),
		filepath.Join(distDir, fmt.Sprintf("%s_%s_%s*", binaryName, goos, goarch), binaryName),
		filepath.Join(distDir, fmt.Sprintf("*_%s_%s*", goos, goarch), binaryName),
		// Direct binary output (format: binary)
		filepath.Join(distDir, binaryName),
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			return matches[0], nil
		}
	}

	// Fallback: search recursively for the binary
	var binaryPath string
	err := filepath.Walk(distDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == binaryName && !info.IsDir() {
			binaryPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err == nil && binaryPath != "" {
		return binaryPath, nil
	}

	return "", fmt.Errorf("binary not found in %s", distDir)
}

// isCommitHash checks if the string looks like a git commit hash.
func isCommitHash(ref string) bool {
	if len(ref) < 7 || len(ref) > 40 {
		return false
	}
	for _, c := range ref {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// copyBinary copies a binary file and preserves executable permissions.
// It ALWAYS removes any existing file first to ensure a clean replacement.
// This handles:
// - "text file busy" errors (running binary)
// - Stale binaries from previous builds
// - Symlinks that need to be replaced with regular files
func copyBinary(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Always remove existing file/symlink first to ensure clean replacement
	// This is critical: without this, a stale binary from a different version
	// could persist and cause upgrade handler mismatches (BINARY UPDATED BEFORE TRIGGER)
	if _, statErr := os.Lstat(dst); statErr == nil {
		if removeErr := os.Remove(dst); removeErr != nil {
			return fmt.Errorf("failed to remove existing binary at %s: %w", dst, removeErr)
		}
	}

	// Write new binary
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return fmt.Errorf("failed to write binary to %s: %w", dst, err)
	}

	return nil
}

// isTextFileBusy checks if the error is a "text file busy" error.
func isTextFileBusy(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "text file busy")
}

// IsBinaryBuilt checks if a binary exists for the given ref.
func (b *Builder) IsBinaryBuilt(ref string) (string, bool) {
	binaryPath := filepath.Join(b.homeDir, "bin", b.getBinaryName())
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, true
	}
	return "", false
}

// GetBinaryPath returns the path where the binary would be for a given ref.
func (b *Builder) GetBinaryPath() string {
	return filepath.Join(b.homeDir, "bin", b.getBinaryName())
}

// BuildToCache builds a binary and stores it in the cache.
// Returns the cached binary entry if successful.
// This method should be called BEFORE chain halt to pre-build the upgrade binary.
func (b *Builder) BuildToCache(ctx context.Context, opts BuildOptions, binaryCache *cache.BinaryCache) (*cache.CachedBinary, error) {
	if opts.Ref == "" {
		return nil, fmt.Errorf("ref is required")
	}

	// Get build tags from plugin for cache key calculation
	src := b.module.BinarySource()
	buildTags := src.BuildTags

	// Determine network type
	networkType := opts.Network
	if networkType == "" {
		networkType = "default"
	}

	// Create build configuration for cache key
	buildConfig := &sdknetwork.BuildConfig{
		Tags: buildTags,
	}

	// Try to resolve commit hash first to check cache without cloning
	commitHash, resolveErr := b.ResolveCommitHash(ctx, opts.Ref)
	if resolveErr == nil && commitHash != "" {
		// Check if already cached with matching build config (fast path - no clone needed)
		if binaryCache.IsCachedWithConfig(networkType, commitHash, buildConfig) {
			b.logger.Success("Using cached binary for commit %s (network: %s)", commitHash[:12], networkType)
			return binaryCache.LookupWithConfig(networkType, commitHash, buildConfig), nil
		}
	}

	// Create build directory
	buildDir := filepath.Join(b.homeDir, "build", "source", opts.Ref)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create build directory: %w", err)
	}

	// Clone repository
	b.logger.Info("Cloning %s repository...", b.module.DisplayName())
	repoDir := filepath.Join(buildDir, b.module.Name())
	if err := b.cloneRepo(ctx, repoDir, opts.Ref); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get commit hash from cloned repo (more reliable than ls-remote)
	commitHash, err := b.getCommitHash(ctx, repoDir)
	if err != nil {
		b.logger.Warn("Failed to get commit hash: %v", err)
		commitHash = opts.Ref
	}

	// Check if already cached with build config (double-check after clone in case ls-remote failed)
	if binaryCache.IsCachedWithConfig(networkType, commitHash, buildConfig) {
		b.logger.Success("Using cached binary for commit %s (network: %s)", commitHash[:12], networkType)
		return binaryCache.LookupWithConfig(networkType, commitHash, buildConfig), nil
	}

	// Read toolchain version from go.mod and create devnet-specific goreleaser config
	toolchain := b.readToolchainFromGoMod(repoDir)
	binaryName := b.getBinaryName()
	cfg := DefaultGoreleaserConfig(binaryName, toolchain)

	// Append network-specific build tags from the plugin
	if len(buildTags) > 0 {
		cfg.Tags = append(cfg.Tags, buildTags...)
		b.logger.Debug("Using build tags: %v", cfg.Tags)
	}

	configContent := GenerateGoreleaserConfig(cfg)
	configPath := filepath.Join(repoDir, ".goreleaser.devnet.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create goreleaser config: %w", err)
	}
	b.logger.Info("Building %s for network: %s", binaryName, b.module.DisplayName())

	// Build using embedded goreleaser
	b.logger.Info("Building binary with goreleaser (ref: %s)...", opts.Ref)
	if err := b.buildWithGoreleaser(ctx, repoDir, configPath); err != nil {
		return nil, fmt.Errorf("goreleaser build failed: %w", err)
	}

	// Find the built binary in goreleaser dist directory
	binaryPath, err := b.findBuiltBinary(repoDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find built binary: %w", err)
	}

	// Store in cache with build config
	cached := &cache.CachedBinary{
		CommitHash:  commitHash,
		Ref:         opts.Ref,
		BuildTime:   time.Now(),
		NetworkType: networkType,
		BuildConfig: buildConfig,
	}

	if err := binaryCache.Store(binaryPath, cached); err != nil {
		// Graceful degradation: log warning but return binary info (FR-008)
		b.logger.Warn("Failed to store binary in cache: %v (binary still available)", err)
		// Return cached entry with path for upgrade flow to use
		cached.BinaryPath = binaryPath
		return cached, nil
	}

	cacheKey := cache.MakeCacheKey(networkType, commitHash, buildConfig)
	b.logger.Success("Binary built and cached: %s (commit: %s)", binaryCache.GetBinaryPath(cacheKey), commitHash[:12])

	return cached, nil
}

// ResolveCommitHash resolves a ref to a commit hash without cloning.
// This is useful for checking the cache before building.
//
// Resolution priority:
// 1. Annotated tag's actual commit (refs/tags/ref^{})
// 2. Lightweight tag (refs/tags/ref)
// 3. Branch (refs/heads/ref)
//
// This ensures tags are preferred over branches when both exist with similar names
// (e.g., v1.1.4 tag vs benchmark/v1.1.4 branch).
func (b *Builder) ResolveCommitHash(ctx context.Context, ref string) (string, error) {
	// If it's already a 40-char commit hash, return as-is
	if len(ref) == 40 && isCommitHash(ref) {
		return ref, nil
	}

	repoURL, err := b.getRepoURL()
	if err != nil {
		return "", fmt.Errorf("failed to get repository URL: %w", err)
	}

	// Query for all refs matching the pattern
	// Use wildcards to catch both exact matches and annotated tag dereferences
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repoURL)
	prepareGitCommand(cmd)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve ref: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("ref not found: %s", ref)
	}

	// Parse all refs and find the best match
	// Priority: annotated tag commit (^{}) > lightweight tag > branch
	var (
		annotatedTagCommit string // refs/tags/ref^{} - highest priority
		tagCommit          string // refs/tags/ref
		branchCommit       string // refs/heads/*/ref or refs/heads/ref
	)

	exactTagRef := fmt.Sprintf("refs/tags/%s", ref)
	annotatedTagRef := fmt.Sprintf("refs/tags/%s^{}", ref)

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		refPath := parts[1]

		// Check for annotated tag dereference (highest priority)
		if refPath == annotatedTagRef {
			annotatedTagCommit = hash
			continue
		}

		// Check for exact tag match
		if refPath == exactTagRef {
			tagCommit = hash
			continue
		}

		// Check for branch ending with our ref (e.g., refs/heads/main or refs/heads/feature/v1.1.4)
		if strings.HasPrefix(refPath, "refs/heads/") && strings.HasSuffix(refPath, "/"+ref) {
			branchCommit = hash
			continue
		}

		// Check for exact branch match (refs/heads/ref)
		if refPath == fmt.Sprintf("refs/heads/%s", ref) {
			branchCommit = hash
			continue
		}
	}

	// Return based on priority
	if annotatedTagCommit != "" {
		b.logger.Debug("Resolved %s to annotated tag commit: %s", ref, annotatedTagCommit[:12])
		return annotatedTagCommit, nil
	}
	if tagCommit != "" {
		b.logger.Debug("Resolved %s to tag: %s", ref, tagCommit[:12])
		return tagCommit, nil
	}
	if branchCommit != "" {
		b.logger.Debug("Resolved %s to branch: %s", ref, branchCommit[:12])
		return branchCommit, nil
	}

	return "", fmt.Errorf("ref not found: %s", ref)
}
