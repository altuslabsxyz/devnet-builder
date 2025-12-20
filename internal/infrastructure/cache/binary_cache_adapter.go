// Package cache provides cache implementations.
package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	legacycache "github.com/b-harvest/devnet-builder/internal/cache"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// BinaryCacheAdapter adapts the legacy cache.BinaryCache to ports.BinaryCache.
type BinaryCacheAdapter struct {
	homeDir    string
	binaryName string
	cache      *legacycache.BinaryCache
	symlink    *legacycache.SymlinkManager
	activeRef  string // Currently active ref
}

// NewBinaryCacheAdapter creates a new BinaryCacheAdapter.
func NewBinaryCacheAdapter(homeDir, binaryName string, logger *output.Logger) (*BinaryCacheAdapter, error) {
	cache := legacycache.NewBinaryCache(homeDir, binaryName, logger)
	if err := cache.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	symlink := legacycache.NewSymlinkManager(homeDir, binaryName)

	return &BinaryCacheAdapter{
		homeDir:    homeDir,
		binaryName: binaryName,
		cache:      cache,
		symlink:    symlink,
	}, nil
}

// Store saves a binary to the cache.
func (a *BinaryCacheAdapter) Store(ctx context.Context, ref string, binaryPath string) (string, error) {
	cached := &legacycache.CachedBinary{
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
func (a *BinaryCacheAdapter) SetActive(ref string) error {
	if !a.cache.IsCached(ref) {
		return &CacheError{
			Operation: "set_active",
			Message:   fmt.Sprintf("ref %s not found in cache", ref),
		}
	}

	if err := a.symlink.SwitchToCache(a.cache, ref); err != nil {
		return fmt.Errorf("failed to switch symlink: %w", err)
	}

	a.activeRef = ref
	return nil
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

// Ensure BinaryCacheAdapter implements ports.BinaryCache.
var _ ports.BinaryCache = (*BinaryCacheAdapter)(nil)
