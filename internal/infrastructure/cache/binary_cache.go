package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/output"
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
	homeDir    string                   // Base directory (~/.stable-devnet)
	cacheDir   string                   // Cache directory path
	binaryName string                   // Name of the binary file (e.g., "stabled", "aultd")
	entries    map[string]*CachedBinary // In-memory index of cached binaries
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
func (c *BinaryCache) loadEntries() error {
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

		commitHash := entry.Name()
		if !isValidCommitHash(commitHash) {
			continue
		}

		// Try to load metadata
		metadataPath := filepath.Join(c.cacheDir, commitHash, MetadataFile)
		metadata, err := ReadMetadata(metadataPath)
		if err != nil {
			c.logger.Debug("Skipping cache entry %s: %v", commitHash, err)
			continue
		}

		binaryPath := filepath.Join(c.cacheDir, commitHash, c.binaryName)
		c.entries[commitHash] = &CachedBinary{
			CommitHash: metadata.CommitHash,
			Ref:        metadata.Ref,
			BuildTime:  metadata.BuildTime,
			Size:       metadata.Size,
			Network:    metadata.Network,
			BinaryPath: binaryPath,
		}
	}

	return nil
}

// CacheDir returns the cache directory path.
func (c *BinaryCache) CacheDir() string {
	return c.cacheDir
}

// GetEntryDir returns the directory path for a specific commit hash.
func (c *BinaryCache) GetEntryDir(commitHash string) string {
	return filepath.Join(c.cacheDir, commitHash)
}

// GetBinaryPath returns the full path to a cached binary.
func (c *BinaryCache) GetBinaryPath(commitHash string) string {
	return filepath.Join(c.cacheDir, commitHash, c.binaryName)
}

// Lookup returns the cached binary for the given commit hash, or nil if not cached.
func (c *BinaryCache) Lookup(commitHash string) *CachedBinary {
	return c.entries[commitHash]
}

// IsCached checks if a binary for the given commit hash exists in cache.
func (c *BinaryCache) IsCached(commitHash string) bool {
	entry := c.entries[commitHash]
	if entry == nil {
		return false
	}
	// Also verify the binary file actually exists
	return c.Validate(commitHash) == nil
}

// Store saves a binary to the cache with its metadata.
func (c *BinaryCache) Store(sourcePath string, cached *CachedBinary) error {
	if cached.CommitHash == "" {
		return fmt.Errorf("commit hash is required")
	}

	entryDir := c.GetEntryDir(cached.CommitHash)
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

	// Write metadata
	metadataPath := filepath.Join(entryDir, MetadataFile)
	if err := WriteMetadata(metadataPath, cached.ToMetadata()); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Update in-memory index
	c.entries[cached.CommitHash] = cached

	return nil
}

// Validate checks if a cached binary exists and is executable.
func (c *BinaryCache) Validate(commitHash string) error {
	binaryPath := c.GetBinaryPath(commitHash)

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

// Remove deletes a cached binary entry.
func (c *BinaryCache) Remove(commitHash string) error {
	entryDir := c.GetEntryDir(commitHash)
	if err := os.RemoveAll(entryDir); err != nil {
		return fmt.Errorf("failed to remove cache entry: %w", err)
	}
	delete(c.entries, commitHash)
	return nil
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
