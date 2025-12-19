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

	"github.com/stablelabs/stable-devnet/internal/cache"
	"github.com/stablelabs/stable-devnet/internal/network"
	"github.com/stablelabs/stable-devnet/internal/output"
)

const (
	// DefaultRepo is the default stable repository (kept for backward compatibility).
	// Prefer using NetworkModule.BinarySource() for network-specific repos.
	DefaultRepo = "https://github.com/stablelabs/stable.git"

	// BinaryName is the default binary name (kept for backward compatibility).
	// Prefer using NetworkModule.BinaryName() for network-specific binaries.
	BinaryName = "stabled"

	// EVMChainID constants for different networks (kept for backward compatibility).
	// Prefer using NetworkModule.GenesisConfig().EVMChainID for network-specific values.
	MainnetEVMChainID = "988"
	TestnetEVMChainID = "2201"

	// DevnetGoreleaserConfigTemplate is a goreleaser config template for devnet builds.
	// This config mirrors the production build flags from stable's .goreleaser.yml
	// but targets only the current OS/arch for local development.
	// Format args: %s = GOTOOLCHAIN version, %s = EVMChainID
	DevnetGoreleaserConfigTemplate = `# Devnet-builder goreleaser config (auto-generated)
# Mirrors production build flags for local development builds
version: 2

project_name: stabled

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
  - id: stabled-devnet
    main: ./cmd/stabled
    binary: stabled
    env:
      - CGO_ENABLED=1
      - CGO_CFLAGS=-O3 -g0 -DNDEBUG
      - CGO_LDFLAGS=-O3 -s
    flags:
      - -mod=readonly
      - -trimpath
    ldflags:
      - -X github.com/cosmos/cosmos-sdk/version.Name=stable
      - -X github.com/cosmos/cosmos-sdk/version.AppName=stabled
      - -X github.com/cosmos/cosmos-sdk/version.Version={{ .Version }}
      - -X github.com/cosmos/cosmos-sdk/version.Commit={{ .Commit }}
      - -X github.com/cosmos/cosmos-sdk/version.BuildTags=netgo,ledger,osusergo,no_dynamic_precompiles
      - -X github.com/stablelabs/stable/app/config.EVMChainID=%s
      - -w -s
    tags:
      - netgo
      - ledger
      - osusergo
      - no_dynamic_precompiles

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
`
)

// Builder handles building binaries from source.
type Builder struct {
	homeDir string
	logger  *output.Logger
	module  network.NetworkModule
}

// NewBuilder creates a new Builder with optional NetworkModule.
// If networkModule is nil, defaults to stable network for backward compatibility.
func NewBuilder(homeDir string, logger *output.Logger, networkModule ...network.NetworkModule) *Builder {
	if logger == nil {
		logger = output.DefaultLogger
	}
	b := &Builder{
		homeDir: homeDir,
		logger:  logger,
	}
	// Use provided network module or default to stable
	if len(networkModule) > 0 && networkModule[0] != nil {
		b.module = networkModule[0]
	} else {
		// Default to stable for backward compatibility
		mod, _ := network.Get("stable")
		b.module = mod
	}
	return b
}

// getRepoURL returns the repository URL for the current network module.
func (b *Builder) getRepoURL() string {
	if b.module != nil {
		src := b.module.BinarySource()
		if src.IsGitHub() {
			return fmt.Sprintf("https://github.com/%s/%s.git", src.Owner, src.Repo)
		}
	}
	return DefaultRepo
}

// getBinaryName returns the binary name for the current network module.
func (b *Builder) getBinaryName() string {
	if b.module != nil {
		return b.module.BinaryName()
	}
	return BinaryName
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

// Build builds a stable binary from the given ref, stores it in cache, and
// updates the symlink at ~/.stable-devnet/bin/stabled to point to it.
//
// This is used by `start` command where the binary should be used immediately.
// For `upgrade` command, use BuildToCache() which only caches without symlink change.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	if opts.Ref == "" {
		return nil, fmt.Errorf("ref is required")
	}

	// Initialize cache and symlink manager
	binaryCache := cache.NewBinaryCache(b.homeDir, b.logger)
	if err := binaryCache.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize binary cache: %w", err)
	}
	symlinkMgr := cache.NewSymlinkManager(b.homeDir)

	// Try to resolve commit hash first to check cache
	commitHash, resolveErr := b.ResolveCommitHash(ctx, opts.Ref)
	if resolveErr == nil && commitHash != "" {
		// Check if already cached (fast path - no build needed)
		if binaryCache.IsCached(commitHash) {
			b.logger.Info("Using cached binary for %s (commit: %s)", opts.Ref, commitHash[:12])
			// Update symlink to point to cached binary
			if err := symlinkMgr.SwitchToCache(binaryCache, commitHash); err != nil {
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
	commitHash, err := b.getCommitHash(ctx, repoDir)
	if err != nil {
		b.logger.Warn("Failed to get commit hash: %v", err)
		commitHash = opts.Ref
	}

	// Double-check cache after clone (in case ls-remote failed earlier)
	if binaryCache.IsCached(commitHash) {
		b.logger.Info("Using cached binary for %s (commit: %s)", opts.Ref, commitHash[:12])
		if err := symlinkMgr.SwitchToCache(binaryCache, commitHash); err != nil {
			return nil, fmt.Errorf("failed to update symlink: %w", err)
		}
		b.logger.Success("Symlink updated: %s -> %s", symlinkMgr.SymlinkPath(), commitHash[:12])
		return &BuildResult{
			BinaryPath: symlinkMgr.SymlinkPath(),
			Ref:        opts.Ref,
			CommitHash: commitHash,
		}, nil
	}

	// Determine EVMChainID based on network
	evmChainID := MainnetEVMChainID
	if opts.Network == "testnet" {
		evmChainID = TestnetEVMChainID
	}

	// Read toolchain version from go.mod and create devnet-specific goreleaser config
	toolchain := b.readToolchainFromGoMod(repoDir)
	configContent := fmt.Sprintf(DevnetGoreleaserConfigTemplate, toolchain, evmChainID)
	configPath := filepath.Join(repoDir, ".goreleaser.devnet.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create goreleaser config: %w", err)
	}
	b.logger.Info("Using EVMChainID: %s (network: %s)", evmChainID, opts.Network)

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

	// Store in cache
	cached := &cache.CachedBinary{
		CommitHash: commitHash,
		Ref:        opts.Ref,
		BuildTime:  time.Now(),
		Network:    opts.Network,
	}
	if err := binaryCache.Store(binaryPath, cached); err != nil {
		return nil, fmt.Errorf("failed to store binary in cache: %w", err)
	}

	// Update symlink to point to newly cached binary
	if err := symlinkMgr.SwitchToCache(binaryCache, commitHash); err != nil {
		return nil, fmt.Errorf("failed to update symlink: %w", err)
	}

	b.logger.Success("Binary built and active: %s (commit: %s)", symlinkMgr.SymlinkPath(), commitHash[:12])

	return &BuildResult{
		BinaryPath: symlinkMgr.SymlinkPath(),
		Ref:        opts.Ref,
		CommitHash: commitHash,
	}, nil
}

// cloneRepo clones the repository and checks out the given ref.
func (b *Builder) cloneRepo(ctx context.Context, repoDir, ref string) error {
	// Remove existing directory if exists
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}

	repoURL := b.getRepoURL()

	// Clone with depth 1 for efficiency (if it's a branch/tag)
	// For commit hashes, we need full history
	args := []string{"clone"}
	if !isCommitHash(ref) {
		args = append(args, "--depth", "1", "--branch", ref)
	}
	args = append(args, repoURL, repoDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = b.logger.Writer()
	cmd.Stderr = b.logger.Writer()
	if err := cmd.Run(); err != nil {
		// If shallow clone failed (maybe branch doesn't exist), try full clone
		if !isCommitHash(ref) {
			b.logger.Debug("Shallow clone failed, trying full clone...")
			args = []string{"clone", repoURL, repoDir}
			cmd = exec.CommandContext(ctx, "git", args...)
			cmd.Stdout = b.logger.Writer()
			cmd.Stderr = b.logger.Writer()
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("git clone failed: %w", err)
			}
		} else {
			return fmt.Errorf("git clone failed: %w", err)
		}
	}

	// Checkout the specific ref if it's a commit hash or if shallow clone didn't specify branch
	if isCommitHash(ref) {
		cmd = exec.CommandContext(ctx, "git", "checkout", ref)
		cmd.Dir = repoDir
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

	// Try to resolve commit hash first to check cache without cloning
	commitHash, resolveErr := b.ResolveCommitHash(ctx, opts.Ref)
	if resolveErr == nil && commitHash != "" {
		// Check if already cached (fast path - no clone needed)
		if binaryCache.IsCached(commitHash) {
			b.logger.Success("Using cached binary for commit %s", commitHash[:12])
			return binaryCache.Lookup(commitHash), nil
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

	// Check if already cached (double-check after clone in case ls-remote failed)
	if binaryCache.IsCached(commitHash) {
		b.logger.Success("Using cached binary for commit %s", commitHash[:12])
		return binaryCache.Lookup(commitHash), nil
	}

	// Determine EVMChainID based on network
	evmChainID := MainnetEVMChainID
	if opts.Network == "testnet" {
		evmChainID = TestnetEVMChainID
	}

	// Read toolchain version from go.mod and create devnet-specific goreleaser config
	toolchain := b.readToolchainFromGoMod(repoDir)
	configContent := fmt.Sprintf(DevnetGoreleaserConfigTemplate, toolchain, evmChainID)
	configPath := filepath.Join(repoDir, ".goreleaser.devnet.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to create goreleaser config: %w", err)
	}
	b.logger.Info("Using EVMChainID: %s (network: %s)", evmChainID, opts.Network)

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

	// Store in cache
	cached := &cache.CachedBinary{
		CommitHash: commitHash,
		Ref:        opts.Ref,
		BuildTime:  time.Now(),
		Network:    opts.Network,
	}

	if err := binaryCache.Store(binaryPath, cached); err != nil {
		return nil, fmt.Errorf("failed to store binary in cache: %w", err)
	}

	b.logger.Success("Binary built and cached: %s (commit: %s)", binaryCache.GetBinaryPath(commitHash), commitHash[:12])

	return cached, nil
}

// ResolveCommitHash resolves a ref to a commit hash by cloning the repo.
// This is useful for checking the cache before building.
func (b *Builder) ResolveCommitHash(ctx context.Context, ref string) (string, error) {
	// If it's already a 40-char commit hash, return as-is
	if len(ref) == 40 && isCommitHash(ref) {
		return ref, nil
	}

	// Create temp directory for ls-remote
	repoURL := b.getRepoURL()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repoURL, ref)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve ref: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("ref not found: %s", ref)
	}

	// Output format: "hash\trefs/heads/branch" or "hash\trefs/tags/tag"
	parts := strings.Fields(lines[0])
	if len(parts) < 1 {
		return "", fmt.Errorf("unexpected git ls-remote output")
	}

	return parts[0], nil
}
