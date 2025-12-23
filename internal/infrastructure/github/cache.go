package github

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheManager handles version cache operations.
type CacheManager struct {
	cacheDir  string
	cacheFile string
	ttl       time.Duration
}

// NewCacheManager creates a new cache manager.
func NewCacheManager(homeDir string, ttl time.Duration) *CacheManager {
	if ttl == 0 {
		ttl = DefaultCacheTTL
	}

	cacheDir := filepath.Join(homeDir, "cache")
	return &CacheManager{
		cacheDir:  cacheDir,
		cacheFile: filepath.Join(cacheDir, "versions.json"),
		ttl:       ttl,
	}
}

// TTL returns the cache time-to-live duration.
func (m *CacheManager) TTL() time.Duration {
	return m.ttl
}

// CachePath returns the path to the cache file.
func (m *CacheManager) CachePath() string {
	return m.cacheFile
}

// Load loads the cache from disk.
func (m *CacheManager) Load() (*VersionCache, error) {
	data, err := os.ReadFile(m.cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache file exists
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache VersionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Check schema version
	if cache.Version != CacheSchemaVersion {
		return nil, nil // Schema mismatch, treat as no cache
	}

	return &cache, nil
}

// Save saves the cache to disk atomically.
func (m *CacheManager) Save(cache *VersionCache) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	// Write atomically
	return writeFileAtomic(m.cacheFile, data)
}

// writeFileAtomic writes data to a file atomically by writing to a temp file first.
func writeFileAtomic(filename string, data []byte) error {
	dir := filepath.Dir(filename)

	// Create temp file in the same directory
	tmpFile, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	// Cleanup on error
	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpName)
		}
	}()

	// Write data
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	tmpFile = nil // Prevent deferred cleanup

	// Atomic rename
	if err := os.Rename(tmpName, filename); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// IsExpired returns true if the cache has expired.
func (m *CacheManager) IsExpired(cache *VersionCache) bool {
	if cache == nil {
		return true
	}
	return time.Now().After(cache.ExpiresAt)
}

// Clear deletes the cache file.
func (m *CacheManager) Clear() error {
	err := os.Remove(m.cacheFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}

// Info returns information about the cache.
func (m *CacheManager) Info() (*CacheInfo, error) {
	cache, err := m.Load()
	if err != nil {
		return nil, err
	}

	if cache == nil {
		return nil, nil // No cache exists
	}

	return &CacheInfo{
		Path:         m.cacheFile,
		FetchedAt:    cache.FetchedAt,
		ExpiresAt:    cache.ExpiresAt,
		VersionCount: len(cache.Releases),
		IsExpired:    m.IsExpired(cache),
	}, nil
}

// CacheInfo contains information about the cache.
type CacheInfo struct {
	Path         string
	FetchedAt    time.Time
	ExpiresAt    time.Time
	VersionCount int
	IsExpired    bool
}

// ContainerCachePath returns the path to the container versions cache file.
func (m *CacheManager) ContainerCachePath() string {
	return filepath.Join(m.cacheDir, "container_versions.json")
}

// LoadContainerCache loads the container versions cache from disk.
func (m *CacheManager) LoadContainerCache() (*ContainerVersionCache, error) {
	data, err := os.ReadFile(m.ContainerCachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache file exists
		}
		return nil, fmt.Errorf("failed to read container cache file: %w", err)
	}

	var cache ContainerVersionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse container cache file: %w", err)
	}

	// Check schema version
	if cache.Version != CacheSchemaVersion {
		return nil, nil // Schema mismatch, treat as no cache
	}

	return &cache, nil
}

// SaveContainerCache saves the container versions cache to disk atomically.
func (m *CacheManager) SaveContainerCache(cache *ContainerVersionCache) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal container cache: %w", err)
	}

	// Write atomically
	return writeFileAtomic(m.ContainerCachePath(), data)
}

// IsContainerCacheExpired returns true if the container cache has expired.
func (m *CacheManager) IsContainerCacheExpired(cache *ContainerVersionCache) bool {
	if cache == nil {
		return true
	}
	return time.Now().After(cache.ExpiresAt)
}
