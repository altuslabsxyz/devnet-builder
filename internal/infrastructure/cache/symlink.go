package cache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/pkg/network"
)

// SymlinkManager manages the binary symlink.
type SymlinkManager struct {
	homeDir     string
	binaryName  string // Name of the binary (e.g., "stabled", "aultd")
	symlinkPath string
}

// NewSymlinkManager creates a new SymlinkManager.
// binaryName should be the network's binary name (e.g., "stabled", "aultd").
func NewSymlinkManager(homeDir, binaryName string) *SymlinkManager {
	if binaryName == "" {
		binaryName = DefaultBinaryName
	}
	return &SymlinkManager{
		homeDir:     homeDir,
		binaryName:  binaryName,
		symlinkPath: filepath.Join(homeDir, BinSubdir, binaryName),
	}
}

// BinaryName returns the configured binary name.
func (m *SymlinkManager) BinaryName() string {
	return m.binaryName
}

// SymlinkPath returns the full path to the symlink.
func (m *SymlinkManager) SymlinkPath() string {
	return m.symlinkPath
}

// GetCurrent returns information about the current symlink, or nil if not a symlink.
func (m *SymlinkManager) GetCurrent() (*ActiveSymlink, error) {
	info, err := os.Lstat(m.symlinkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat symlink: %w", err)
	}

	// Check if it's actually a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		return nil, nil // Not a symlink (might be a regular file)
	}

	// Read symlink target
	target, err := os.Readlink(m.symlinkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read symlink: %w", err)
	}

	// Extract commit hash from target path
	// Expected format: ../cache/binaries/{commit_hash}/{binaryName}
	commitHash := extractCommitHashFromPath(target)

	return &ActiveSymlink{
		Path:       m.symlinkPath,
		Target:     target,
		CommitHash: commitHash,
	}, nil
}

// Switch atomically switches the symlink to point to a new target.
// This uses the atomic rename pattern: create temp symlink, then rename.
func (m *SymlinkManager) Switch(targetPath string) error {
	// Ensure bin directory exists
	binDir := filepath.Dir(m.symlinkPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Create temporary symlink
	tempLink := m.symlinkPath + ".tmp"

	// Remove temp link if it exists (from failed previous attempt)
	os.Remove(tempLink)

	// Create symlink to target
	if err := os.Symlink(targetPath, tempLink); err != nil {
		return fmt.Errorf("failed to create temporary symlink: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempLink, m.symlinkPath); err != nil {
		os.Remove(tempLink) // Clean up temp link on failure
		return fmt.Errorf("failed to atomic rename symlink: %w", err)
	}

	return nil
}

// SwitchToCache switches the symlink to point to a cached binary by cache key.
func (m *SymlinkManager) SwitchToCache(cache *BinaryCache, cacheKey string) error {
	// Calculate relative path from bin dir to cache entry
	// From: ~/.devnet-builder/bin/{binaryName}
	// To:   ~/.devnet-builder/cache/binaries/{cacheKey}/{binaryName}
	// Relative: ../cache/binaries/{cacheKey}/{binaryName}

	relativePath := filepath.Join("..", CacheSubdir, cacheKey, m.binaryName)
	return m.Switch(relativePath)
}

// SwitchToCacheWithTags switches the symlink to point to a cached binary by commit hash and build tags.
// DEPRECATED: Uses legacy cache key format. New code should use SwitchToCacheWithConfig.
func (m *SymlinkManager) SwitchToCacheWithTags(cache *BinaryCache, commitHash string, buildTags []string) error {
	cacheKey := MakeCacheKeyLegacy(commitHash, buildTags)
	return m.SwitchToCache(cache, cacheKey)
}

// SwitchToCacheWithConfig switches the symlink to point to a cached binary by network type, commit hash, and build config.
// This is the preferred method for network-aware caching.
func (m *SymlinkManager) SwitchToCacheWithConfig(cache *BinaryCache, networkType, commitHash string, buildConfig *network.BuildConfig) error {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return m.SwitchToCache(cache, cacheKey)
}

// IsSymlink checks if the binary path is a symlink.
func (m *SymlinkManager) IsSymlink() bool {
	info, err := os.Lstat(m.symlinkPath)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// IsRegularFile checks if the binary path is a regular file (not a symlink).
func (m *SymlinkManager) IsRegularFile() bool {
	info, err := os.Lstat(m.symlinkPath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// MigrateToSymlink converts a regular binary file to a cached entry and creates a symlink.
// This is used for backward compatibility with devnets that have a direct binary.
func (m *SymlinkManager) MigrateToSymlink(cache *BinaryCache, commitHash, ref, network string, buildTags []string) error {
	if !m.IsRegularFile() {
		return fmt.Errorf("no regular file to migrate at %s", m.symlinkPath)
	}

	// Check if already cached with same build tags
	if cache.IsCachedWithTags(commitHash, buildTags) {
		// Just create symlink, binary already in cache
		return m.SwitchToCacheWithTags(cache, commitHash, buildTags)
	}

	// Store current binary in cache
	cached := &CachedBinary{
		CommitHash:  commitHash,
		Ref:         ref,
		NetworkType: network, // Use NetworkType instead of deprecated Network
		BuildTags:   buildTags,
	}

	if err := cache.Store(m.symlinkPath, cached); err != nil {
		return fmt.Errorf("failed to store binary in cache: %w", err)
	}

	// Remove original file
	if err := os.Remove(m.symlinkPath); err != nil {
		return fmt.Errorf("failed to remove original binary: %w", err)
	}

	// Create symlink
	return m.SwitchToCacheWithTags(cache, commitHash, buildTags)
}

// extractCommitHashFromPath extracts the commit hash from a cache path.
func extractCommitHashFromPath(path string) string {
	// Path format: ../cache/binaries/{cacheKey}/{binaryName}
	// or absolute: /home/user/.devnet-builder/cache/binaries/{cacheKey}/{binaryName}
	// cacheKey format: {commitHash}-{tagsHash} (49 chars) or just {commitHash} (40 chars, legacy)
	dir := filepath.Dir(path)      // ../cache/binaries/{cacheKey}
	cacheKey := filepath.Base(dir) // {cacheKey}

	// Handle new format: commitHash-tagsHash
	if isValidCacheKey(cacheKey) {
		return cacheKey[:40] // Extract just the commit hash part
	}

	// Handle legacy format: just commitHash
	if isValidCommitHash(cacheKey) {
		return cacheKey
	}
	return ""
}
