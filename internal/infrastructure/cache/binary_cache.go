package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/paths"
	"github.com/b-harvest/devnet-builder/pkg/network"
)

type BinaryCache struct {
	homeDir    string
	cacheDir   string
	binaryName string
	entries    map[string]*CachedBinary
	logger     *output.Logger
}

func NewBinaryCache(homeDir, binaryName string, logger *output.Logger) *BinaryCache {
	if logger == nil {
		logger = output.DefaultLogger
	}
	if binaryName == "" {
		binaryName = paths.DefaultBinaryName
	}
	return &BinaryCache{
		homeDir:    homeDir,
		cacheDir:   paths.BinaryCachePath(homeDir),
		binaryName: binaryName,
		entries:    make(map[string]*CachedBinary),
		logger:     logger,
	}
}

func (c *BinaryCache) BinaryName() string {
	return c.binaryName
}

func (c *BinaryCache) Initialize() error {
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	return c.loadEntries()
}

// loadEntries scans cache directory and loads all cached binary metadata.
// Format: binaries/{networkType}/{commitHash}-{configHash}/
func (c *BinaryCache) loadEntries() error {
	c.entries = make(map[string]*CachedBinary)

	networkDirs, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, networkDir := range networkDirs {
		if !networkDir.IsDir() {
			continue
		}

		networkType := networkDir.Name()
		networkPath := filepath.Join(c.cacheDir, networkType)

		binaryDirs, err := os.ReadDir(networkPath)
		if err != nil {
			c.logger.Debug("Skipping network directory %s: %v", networkType, err)
			continue
		}

		for _, binaryDir := range binaryDirs {
			if !binaryDir.IsDir() {
				continue
			}

			binaryDirName := binaryDir.Name()
			if !isValidBinaryDirName(binaryDirName) {
				c.logger.Debug("Skipping invalid binary directory: %s/%s", networkType, binaryDirName)
				continue
			}

			cacheKeyPath := filepath.Join(networkType, binaryDirName)
			if err := c.loadCacheEntry(cacheKeyPath, networkType, binaryDirName); err != nil {
				c.logger.Debug("Skipping cache entry %s: %v", cacheKeyPath, err)
				continue
			}
		}
	}

	return nil
}

func (c *BinaryCache) loadCacheEntry(cacheKeyPath, networkType, binaryDirName string) error {
	metadataPath := filepath.Join(c.cacheDir, cacheKeyPath, paths.MetadataFile)
	metadata, err := ReadMetadata(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	if metadata.CommitHash == "" {
		return fmt.Errorf("metadata missing commit_hash")
	}
	if metadata.NetworkType == "" {
		return fmt.Errorf("metadata missing network_type")
	}
	if metadata.BuildConfig == nil {
		return fmt.Errorf("metadata missing build_config")
	}

	actualCacheKey := MakeCacheKey(metadata.NetworkType, metadata.CommitHash, metadata.BuildConfig)
	binaryPath := filepath.Join(c.cacheDir, cacheKeyPath, c.binaryName)

	c.entries[actualCacheKey] = &CachedBinary{
		CommitHash:  metadata.CommitHash,
		Ref:         metadata.Ref,
		BuildTime:   metadata.BuildTime,
		Size:        metadata.Size,
		NetworkType: metadata.NetworkType,
		BuildConfig: metadata.BuildConfig,
		BinaryPath:  binaryPath,
		DirKey:      cacheKeyPath,
	}

	return nil
}

func (c *BinaryCache) CacheDir() string {
	return c.cacheDir
}

func (c *BinaryCache) GetEntryDir(cacheKey string) string {
	return filepath.Join(c.cacheDir, cacheKey)
}

func (c *BinaryCache) GetBinaryPath(cacheKey string) string {
	return filepath.Join(c.cacheDir, cacheKey, c.binaryName)
}

func (c *BinaryCache) GetBinaryPathWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) string {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.GetBinaryPath(cacheKey)
}

func (c *BinaryCache) Lookup(cacheKey string) *CachedBinary {
	return c.entries[cacheKey]
}

func (c *BinaryCache) LookupWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) *CachedBinary {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.entries[cacheKey]
}

func (c *BinaryCache) IsCached(cacheKey string) bool {
	entry := c.entries[cacheKey]
	if entry == nil {
		return false
	}
	info, err := os.Stat(entry.BinaryPath)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}

func (c *BinaryCache) IsCachedWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) bool {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.IsCached(cacheKey)
}

func (c *BinaryCache) Store(sourcePath string, cached *CachedBinary) error {
	if cached.CommitHash == "" {
		return fmt.Errorf("commit hash is required")
	}

	cacheKey := cached.CacheKey()
	entryDir := c.GetEntryDir(cacheKey)
	if err := os.MkdirAll(entryDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache entry directory: %w", err)
	}

	destPath := filepath.Join(entryDir, c.binaryName)
	if err := copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy binary to cache: %w", err)
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("failed to stat cached binary: %w", err)
	}
	cached.Size = info.Size()
	cached.BinaryPath = destPath
	cached.DirKey = cacheKey

	metadataPath := filepath.Join(entryDir, paths.MetadataFile)
	if err := WriteMetadata(metadataPath, cached.ToMetadata()); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	c.entries[cacheKey] = cached
	return nil
}

func (c *BinaryCache) ValidateKey(cacheKey string) error {
	binaryPath := c.GetBinaryPath(cacheKey)

	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cached binary not found: %s", binaryPath)
		}
		return fmt.Errorf("failed to stat binary: %w", err)
	}

	if info.Mode()&0111 == 0 {
		return fmt.Errorf("cached binary is not executable: %s", binaryPath)
	}

	return nil
}

func (c *BinaryCache) ValidateWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) error {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.ValidateKey(cacheKey)
}

func (c *BinaryCache) Remove(cacheKey string) error {
	entryDir := c.GetEntryDir(cacheKey)
	if err := os.RemoveAll(entryDir); err != nil {
		return fmt.Errorf("failed to remove cache entry: %w", err)
	}
	delete(c.entries, cacheKey)
	return nil
}

func (c *BinaryCache) RemoveWithConfig(networkType, commitHash string, buildConfig *network.BuildConfig) error {
	cacheKey := MakeCacheKey(networkType, commitHash, buildConfig)
	return c.Remove(cacheKey)
}

func (c *BinaryCache) List() []*CachedBinary {
	result := make([]*CachedBinary, 0, len(c.entries))
	for _, entry := range c.entries {
		result = append(result, entry)
	}
	return result
}

func (c *BinaryCache) Stats() *CacheStats {
	stats := &CacheStats{
		TotalEntries: len(c.entries),
	}
	for _, entry := range c.entries {
		stats.TotalSize += entry.Size
	}
	return stats
}

func (c *BinaryCache) Clean() error {
	if err := os.RemoveAll(c.cacheDir); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	c.entries = make(map[string]*CachedBinary)
	return c.Initialize()
}

// isValidBinaryDirName validates format: {commitHash(40hex)}-{configHash(8hex)}
func isValidBinaryDirName(s string) bool {
	if len(s) != 49 {
		return false
	}
	if s[40] != '-' {
		return false
	}
	commitHash := s[:40]
	for _, c := range commitHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	configHash := s[41:]
	if len(configHash) != 8 {
		return false
	}
	for _, c := range configHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func ReadMetadata(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	if metadata.CommitHash == "" {
		return nil, fmt.Errorf("metadata missing commit_hash")
	}

	return &metadata, nil
}

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
