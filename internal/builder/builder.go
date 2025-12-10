package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/stablelabs/stable-devnet/internal/output"
)

const (
	// DefaultRepo is the default stable repository.
	DefaultRepo = "https://github.com/stablelabs/stable.git"

	// BinaryName is the name of the built binary.
	BinaryName = "stabled"

	// EVMChainID constants for different networks
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

// Builder handles building stable binaries from source.
type Builder struct {
	homeDir string
	logger  *output.Logger
}

// NewBuilder creates a new Builder.
func NewBuilder(homeDir string, logger *output.Logger) *Builder {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &Builder{
		homeDir: homeDir,
		logger:  logger,
	}
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

// Build builds a stable binary from the given ref using embedded goreleaser.
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	if opts.Ref == "" {
		return nil, fmt.Errorf("ref is required")
	}

	// Create build directory
	buildDir := filepath.Join(b.homeDir, "build", "source", opts.Ref)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create build directory: %w", err)
	}

	// Clone repository
	b.logger.Info("Cloning stable repository...")
	repoDir := filepath.Join(buildDir, "stable")
	if err := b.cloneRepo(ctx, repoDir, opts.Ref); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get commit hash
	commitHash, err := b.getCommitHash(ctx, repoDir)
	if err != nil {
		b.logger.Warn("Failed to get commit hash: %v", err)
		commitHash = opts.Ref
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

	// Copy binary to output directory
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(b.homeDir, "bin")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	destPath := filepath.Join(outputDir, BinaryName)
	if err := copyBinary(binaryPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy binary: %w", err)
	}

	b.logger.Success("Binary built successfully: %s", destPath)

	return &BuildResult{
		BinaryPath: destPath,
		Ref:        opts.Ref,
		CommitHash: commitHash,
	}, nil
}

// cloneRepo clones the stable repository and checks out the given ref.
func (b *Builder) cloneRepo(ctx context.Context, repoDir, ref string) error {
	// Remove existing directory if exists
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}

	// Clone with depth 1 for efficiency (if it's a branch/tag)
	// For commit hashes, we need full history
	args := []string{"clone"}
	if !isCommitHash(ref) {
		args = append(args, "--depth", "1", "--branch", ref)
	}
	args = append(args, DefaultRepo, repoDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = b.logger.Writer()
	cmd.Stderr = b.logger.Writer()
	if err := cmd.Run(); err != nil {
		// If shallow clone failed (maybe branch doesn't exist), try full clone
		if !isCommitHash(ref) {
			b.logger.Debug("Shallow clone failed, trying full clone...")
			args = []string{"clone", DefaultRepo, repoDir}
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

	// Get current OS and arch
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Try common patterns for goreleaser output
	patterns := []string{
		// Format: dist/stabled-devnet_<os>_<arch>/stabled
		filepath.Join(distDir, fmt.Sprintf("stabled-devnet_%s_%s*", goos, goarch), BinaryName),
		filepath.Join(distDir, fmt.Sprintf("stabled_%s_%s*", goos, goarch), BinaryName),
		filepath.Join(distDir, fmt.Sprintf("*_%s_%s*", goos, goarch), BinaryName),
		// Direct binary output (format: binary)
		filepath.Join(distDir, BinaryName),
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
		if info.Name() == BinaryName && !info.IsDir() {
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
func copyBinary(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

// IsBinaryBuilt checks if a binary exists for the given ref.
func (b *Builder) IsBinaryBuilt(ref string) (string, bool) {
	binaryPath := filepath.Join(b.homeDir, "bin", BinaryName)
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, true
	}
	return "", false
}

// GetBinaryPath returns the path where the binary would be for a given ref.
func (b *Builder) GetBinaryPath() string {
	return filepath.Join(b.homeDir, "bin", BinaryName)
}
