// Package cache provides cache implementations.
package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// BinaryCacheAdapter implements ports.BinaryCache.
type BinaryCacheAdapter struct {
	homeDir    string
	binaryName string
	cache      *BinaryCache
	symlink    *SymlinkManager
	activeRef  string // Currently active ref
}

// NewBinaryCacheAdapter creates a new BinaryCacheAdapter.
func NewBinaryCacheAdapter(homeDir, binaryName string, logger *output.Logger) (*BinaryCacheAdapter, error) {
	cache := NewBinaryCache(homeDir, binaryName, logger)
	if err := cache.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	symlink := NewSymlinkManager(homeDir, binaryName)

	return &BinaryCacheAdapter{
		homeDir:    homeDir,
		binaryName: binaryName,
		cache:      cache,
		symlink:    symlink,
	}, nil
}

// Store saves a binary to the cache.
func (a *BinaryCacheAdapter) Store(ctx context.Context, ref string, binaryPath string) (string, error) {
	cached := &CachedBinary{
		CommitHash: ref,
		Ref:        ref,
	}

	if err := a.cache.Store(binaryPath, cached); err != nil {
		return "", fmt.Errorf("failed to store binary: %w", err)
	}

	return a.cache.GetBinaryPath(ref), nil
}

// Get retrieves a cached binary path by ref.
func (a *BinaryCacheAdapter) Get(ref string) (string, bool) {
	if !a.cache.IsCached(ref) {
		return "", false
	}
	return a.cache.GetBinaryPath(ref), true
}

// Has checks if a binary is cached.
func (a *BinaryCacheAdapter) Has(ref string) bool {
	return a.cache.IsCached(ref)
}

// List returns all cached binary refs.
func (a *BinaryCacheAdapter) List() []string {
	entries := a.cache.List()
	refs := make([]string, len(entries))
	for i, entry := range entries {
		refs[i] = entry.CommitHash
	}
	return refs
}

// Remove deletes a cached binary.
func (a *BinaryCacheAdapter) Remove(ref string) error {
	return a.cache.Remove(ref)
}

// SetActive sets the active binary version.
// The ref can be:
// - A full cache key (49 chars: {commitHash}-{tagsHash})
// - A commit hash (40 chars) - will search for matching cache entry
// - A directory name on disk (for entries created by other processes)
func (a *BinaryCacheAdapter) SetActive(ref string) error {
	// Determine the cache key to use
	var cacheKey string

	// First, try as a full cache key
	if a.cache.IsCached(ref) {
		cacheKey = ref
	} else {
		// Search for a cache entry matching the commit hash prefix
		cacheKey = a.findCacheKeyByCommit(ref)
		if cacheKey == "" {
			// Cache might be stale - reload from filesystem
			// This handles cases where another process/instance added entries
			if err := a.cache.Initialize(); err != nil {
				return fmt.Errorf("failed to reload cache: %w", err)
			}

			// Try again after reload
			if a.cache.IsCached(ref) {
				cacheKey = ref
			} else {
				cacheKey = a.findCacheKeyByCommit(ref)
				if cacheKey == "" {
					return &CacheError{
						Operation: "set_active",
						Message:   fmt.Sprintf("ref %s not found in cache", ref),
					}
				}
			}
		}
	}

	if err := a.symlink.SwitchToCache(a.cache, cacheKey); err != nil {
		return fmt.Errorf("failed to switch symlink: %w", err)
	}

	a.activeRef = cacheKey
	return nil
}

// findCacheKeyByCommit finds a cache key that matches the given commit hash.
// Returns the actual directory name on disk (which may differ from the computed cache key
// for legacy entries that don't have tag hashes in the directory name).
func (a *BinaryCacheAdapter) findCacheKeyByCommit(commitHash string) string {
	entries := a.cache.List()
	for _, entry := range entries {
		if entry.CommitHash == commitHash {
			return entry.ActualDirKey()
		}
	}
	return ""
}

// GetActive returns the currently active binary path.
func (a *BinaryCacheAdapter) GetActive() (string, error) {
	binPath := filepath.Join(a.homeDir, "bin", a.binaryName)

	// Check if symlink exists and resolve it
	target, err := os.Readlink(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &NotActiveError{BinaryName: a.binaryName}
		}
		// Not a symlink, check if it's a regular file
		if _, statErr := os.Stat(binPath); statErr == nil {
			return binPath, nil
		}
		return "", fmt.Errorf("failed to resolve binary: %w", err)
	}

	// Resolve relative symlink to absolute path
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(binPath), target)
	}

	return target, nil
}

// ListDetailed returns detailed information about all cached binaries.
func (a *BinaryCacheAdapter) ListDetailed() []ports.CachedBinaryInfo {
	entries := a.cache.List()
	result := make([]ports.CachedBinaryInfo, len(entries))
	for i, entry := range entries {
		result[i] = ports.CachedBinaryInfo{
			Ref:        entry.Ref,
			CommitHash: entry.CommitHash,
			Path:       entry.BinaryPath,
			Size:       entry.Size,
			BuildTime:  entry.BuildTime,
			Network:    entry.NetworkType, // Use NetworkType instead of deprecated Network
		}
	}
	return result
}

// Stats returns cache statistics.
func (a *BinaryCacheAdapter) Stats() ports.CacheStats {
	cacheStats := a.cache.Stats()
	return ports.CacheStats{
		TotalEntries: cacheStats.TotalEntries,
		TotalSize:    cacheStats.TotalSize,
	}
}

// Clean removes all cached binaries.
func (a *BinaryCacheAdapter) Clean() error {
	return a.cache.Clean()
}

// CacheDir returns the cache directory path.
func (a *BinaryCacheAdapter) CacheDir() string {
	return a.cache.CacheDir()
}

// SymlinkPath returns the symlink path.
func (a *BinaryCacheAdapter) SymlinkPath() string {
	return filepath.Join(a.homeDir, "bin", a.binaryName)
}

// SymlinkInfo returns information about the current symlink.
func (a *BinaryCacheAdapter) SymlinkInfo() (*ports.SymlinkInfo, error) {
	binPath := a.SymlinkPath()
	info := &ports.SymlinkInfo{
		Path: binPath,
	}

	// Check if path exists
	fileInfo, err := os.Lstat(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			info.Exists = false
			return info, nil
		}
		return nil, fmt.Errorf("failed to stat symlink: %w", err)
	}

	info.Exists = true

	// Check if it's a symlink or regular file
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(binPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read symlink: %w", err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(binPath), target)
		}
		info.Target = target
		// Extract commit hash from path (cache/binaries/<hash>/binary)
		dir := filepath.Dir(target)
		info.CommitHash = filepath.Base(dir)
	} else {
		info.IsRegular = true
	}

	return info, nil
}

// Ensure BinaryCacheAdapter implements ports.BinaryCache.
var _ ports.BinaryCache = (*BinaryCacheAdapter)(nil)
