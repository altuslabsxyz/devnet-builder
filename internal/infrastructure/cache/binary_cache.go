package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/pkg/network"
)

const (
	// CacheSubdir is the subdirectory name for binary cache.
	CacheSubdir = "cache/binaries"

	// DefaultBinaryName is the default name of the cached binary file.
	DefaultBinaryName = "binary"

	// MetadataFile is the name of the metadata JSON file.
	MetadataFile = "metadata.json"

	// BinSubdir is the subdirectory name for the binary symlink.
	BinSubdir = "bin"
)

// BinaryCache manages cached binary builds.
type BinaryCache struct {
	homeDir    string                   // Base directory (~/.devnet-builder)
	cacheDir   string                   // Cache directory path
	binaryName string                   // Name of the binary file (e.g., "stabled", "aultd")
	entries    map[string]*CachedBinary // In-memory index of cached binaries, keyed by cache key
	logger     *output.Logger
}

// NewBinaryCache creates a new BinaryCache manager.
// binaryName should be the network's binary name (e.g., "stabled", "aultd").
func NewBinaryCache(homeDir, binaryName string, logger *output.Logger) *BinaryCache {
	if logger == nil {
		logger = output.DefaultLogger
	}
	if binaryName == "" {
		binaryName = DefaultBinaryName
	}
	return &BinaryCache{
		homeDir:    homeDir,
		cacheDir:   filepath.Join(homeDir, CacheSubdir),
		binaryName: binaryName,
		entries:    make(map[string]*CachedBinary),
		logger:     logger,
	}
}

// BinaryName returns the configured binary name.
func (c *BinaryCache) BinaryName() string {
	return c.binaryName
}

// Initialize creates the cache directory structure if it doesn't exist.
func (c *BinaryCache) Initialize() error {
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	return c.loadEntries()
}

// loadEntries scans the cache directory and loads all cached binary metadata.
// Clears existing entries before loading to ensure consistency with filesystem.
func (c *BinaryCache) loadEntries() error {
	// Clear existing entries to reload from filesystem
	// This ensures consistency when called multiple times (e.g., after external changes)
	c.entries = make(map[string]*CachedBinary)

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cacheKey := entry.Name()
		// Accept both old format (commit hash only) and new format (commit-taghash)
		if !isValidCacheKey(cacheKey) && !isValidCommitHash(cacheKey) {
			continue
		}

		// Try to load metadata
		metadataPath := filepath.Join(c.cacheDir, cacheKey, MetadataFile)
		metadata, err := ReadMetadata(metadataPath)
		if err != nil {
			c.logger.Debug("Skipping cache entry %s: %v", cacheKey, err)
			continue
		}

		// Compute the correct cache key from metadata
		var actualCacheKey string
		if metadata.NetworkType != "" && metadata.BuildConfig != nil {
			// New format with network type
			actualCacheKey = MakeCacheKey(metadata.NetworkType, metadata.CommitHash, metadata.BuildConfig)
		} else if metadata.Network != "" && len(metadata.BuildTags) > 0 {
			// Legacy format with old Network field
			actualCacheKey = MakeCacheKeyLegacy(metadata.CommitHash, metadata.BuildTags)
		} else {
			// Fallback to legacy format
			actualCacheKey = MakeCacheKeyLegacy(metadata.CommitHash, metadata.BuildTags)
		}

		binaryPath := filepath.Join(c.cacheDir, cacheKey, c.binaryName)
		c.entries[actualCacheKey] = &CachedBinary{
			CommitHash:  metadata.CommitHash,
			Ref:         metadata.Ref,
			BuildTime:   metadata.BuildTime,
			Size:        metadata.Size,
			NetworkType: metadata.NetworkType,
			BuildConfig: metadata.BuildConfig,
			BuildTags:   metadata.BuildTags, // For backward compat
			BinaryPath:  binaryPath,
			DirKey:      cacheKey, // Store actual directory name for symlink creation
		}
	}

	return nil
}

// CacheDir returns the cache directory path.
func (c *BinaryCache) CacheDir() string {
	return c.cacheDir
}

// GetEntryDir returns the directory path for a specific cache key.
func (c *BinaryCache) GetEntryDir(cacheKey string) string {
	return filepath.Join(c.cacheDir, cacheKey)
}

// GetBinaryPath returns the full path to a cached binary by cache key.
func (c *BinaryCache) GetBinaryPath(cacheKey string) string {
	return filepath.Join(c.cacheDir, cacheKey, c.binaryName)
}

// GetBinaryPathWithTags returns the full path to a cached binary by commit hash and build tags.
// DEPRECATED: Use GetBinaryPathWithConfig instead. This method uses legacy cache key format.
func (c *BinaryCache) GetBinaryPathWithTags(commitHash string, buildTags []string) string {
	cacheKey := MakeCacheKeyLegacy(commitHash, buildTags)
	return c.GetBinaryPath(cacheKey)
}

// GetBinaryPathWithConfig returns the full path to a cached binary by network type, commit hash, and build config.
func (c *BinaryCache) GetBinaryPathWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) string {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.GetBinaryPath(cacheKey)
}

// Lookup returns the cached binary for the given cache key, or nil if not cached.
func (c *BinaryCache) Lookup(cacheKey string) *CachedBinary {
	return c.entries[cacheKey]
}

// LookupWithTags returns the cached binary for the given commit hash and build tags.
// DEPRECATED: Use LookupWithConfig instead. This method uses legacy cache key format.
func (c *BinaryCache) LookupWithTags(commitHash string, buildTags []string) *CachedBinary {
	cacheKey := MakeCacheKeyLegacy(commitHash, buildTags)
	return c.entries[cacheKey]
}

// LookupWithConfig returns the cached binary for the given network type, commit hash, and build config.
func (c *BinaryCache) LookupWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) *CachedBinary {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.entries[cacheKey]
}

// IsCached checks if a binary for the given cache key exists in cache.
func (c *BinaryCache) IsCached(cacheKey string) bool {
	entry := c.entries[cacheKey]
	if entry == nil {
		return false
	}
	// Also verify the binary file actually exists
	return c.ValidateKey(cacheKey) == nil
}

// IsCachedWithTags checks if a binary for the given commit hash and build tags exists in cache.
// DEPRECATED: Use IsCachedWithConfig instead. This method uses legacy cache key format.
func (c *BinaryCache) IsCachedWithTags(commitHash string, buildTags []string) bool {
	cacheKey := MakeCacheKeyLegacy(commitHash, buildTags)
	return c.IsCached(cacheKey)
}

// IsCachedWithConfig checks if a binary for the given network type, commit hash, and build config exists in cache.
func (c *BinaryCache) IsCachedWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) bool {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.IsCached(cacheKey)
}

// Store saves a binary to the cache with its metadata.
// The cache key is computed from CommitHash and BuildTags.
func (c *BinaryCache) Store(sourcePath string, cached *CachedBinary) error {
	if cached.CommitHash == "" {
		return fmt.Errorf("commit hash is required")
	}

	cacheKey := cached.CacheKey()
	entryDir := c.GetEntryDir(cacheKey)
	if err := os.MkdirAll(entryDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache entry directory: %w", err)
	}

	// Copy binary to cache
	destPath := filepath.Join(entryDir, c.binaryName)
	if err := copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy binary to cache: %w", err)
	}

	// Make binary executable
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}

	// Get file size
	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("failed to stat cached binary: %w", err)
	}
	cached.Size = info.Size()
	cached.BinaryPath = destPath
	cached.DirKey = cacheKey // For newly created entries, DirKey matches CacheKey

	// Write metadata
	metadataPath := filepath.Join(entryDir, MetadataFile)
	if err := WriteMetadata(metadataPath, cached.ToMetadata()); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Update in-memory index using cache key
	c.entries[cacheKey] = cached

	return nil
}

// ValidateKey checks if a cached binary exists and is executable by cache key.
func (c *BinaryCache) ValidateKey(cacheKey string) error {
	binaryPath := c.GetBinaryPath(cacheKey)

	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cached binary not found: %s", binaryPath)
		}
		return fmt.Errorf("failed to stat binary: %w", err)
	}

	// Check if executable
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("cached binary is not executable: %s", binaryPath)
	}

	return nil
}

// ValidateWithTags checks if a cached binary exists and is executable by commit hash and build tags.
// DEPRECATED: Use ValidateWithConfig instead. This method uses legacy cache key format.
func (c *BinaryCache) ValidateWithTags(commitHash string, buildTags []string) error {
	cacheKey := MakeCacheKeyLegacy(commitHash, buildTags)
	return c.ValidateKey(cacheKey)
}

// ValidateWithConfig checks if a cached binary exists and is executable by network type, commit hash, and build config.
func (c *BinaryCache) ValidateWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) error {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.ValidateKey(cacheKey)
}

// Remove deletes a cached binary entry by cache key.
func (c *BinaryCache) Remove(cacheKey string) error {
	entryDir := c.GetEntryDir(cacheKey)
	if err := os.RemoveAll(entryDir); err != nil {
		return fmt.Errorf("failed to remove cache entry: %w", err)
	}
	delete(c.entries, cacheKey)
	return nil
}

// RemoveWithTags deletes a cached binary entry by commit hash and build tags.
// DEPRECATED: Use RemoveWithConfig instead. This method uses legacy cache key format.
func (c *BinaryCache) RemoveWithTags(commitHash string, buildTags []string) error {
	cacheKey := MakeCacheKeyLegacy(commitHash, buildTags)
	return c.Remove(cacheKey)
}

// RemoveWithConfig deletes a cached binary entry by network type, commit hash, and build config.
func (c *BinaryCache) RemoveWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) error {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.Remove(cacheKey)
}

// List returns all cached binaries.
func (c *BinaryCache) List() []*CachedBinary {
	result := make([]*CachedBinary, 0, len(c.entries))
	for _, entry := range c.entries {
		result = append(result, entry)
	}
	return result
}

// Stats returns cache statistics.
func (c *BinaryCache) Stats() *CacheStats {
	stats := &CacheStats{
		TotalEntries: len(c.entries),
	}
	for _, entry := range c.entries {
		stats.TotalSize += entry.Size
	}
	return stats
}

// Clean removes all cached binaries.
func (c *BinaryCache) Clean() error {
	if err := os.RemoveAll(c.cacheDir); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	c.entries = make(map[string]*CachedBinary)
	return c.Initialize()
}

// isValidCommitHash checks if a string is a valid 40-character hex commit hash.
func isValidCommitHash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isValidCacheKey checks if a string is a valid cache key format.
// Format: {commitHash}-{tagsHash} where commitHash is 40 hex chars and tagsHash is 8 hex chars.
func isValidCacheKey(s string) bool {
	// Must be 49 characters: 40 (commit) + 1 (-) + 8 (tags hash)
	if len(s) != 49 {
		return false
	}
	// Check for dash separator at position 40
	if s[40] != '-' {
		return false
	}
	// Validate commit hash part
	if !isValidCommitHash(s[:40]) {
		return false
	}
	// Validate tags hash part (8 hex chars)
	tagsHash := s[41:]
	for _, c := range tagsHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

// ReadMetadata reads cache metadata from a JSON file.
func ReadMetadata(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Validate required fields
	if metadata.CommitHash == "" {
		return nil, fmt.Errorf("metadata missing commit_hash")
	}

	return &metadata, nil
}

// WriteMetadata writes cache metadata to a JSON file.
func WriteMetadata(path string, metadata *Metadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}
