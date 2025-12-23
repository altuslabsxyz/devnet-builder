package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultCacheExpiration is the default duration after which a cached snapshot expires.
	// Snapshots are large files (1-10GB), so caching for 30 minutes provides a good balance
	// between avoiding redundant downloads during development and ensuring fresh state.
	DefaultCacheExpiration = 30 * time.Minute
)

// SnapshotCache represents a downloaded and cached state snapshot.
type SnapshotCache struct {
	// Identification
	CacheKey string `json:"cache_key"` // "plugin-network" format (e.g., "stable-mainnet", "ault-testnet")

	// File Info
	FilePath     string `json:"file_path"`
	SizeBytes    int64  `json:"size_bytes"`
	Decompressor string `json:"decompressor"`       // "zstd" or "lz4"
	Checksum     string `json:"checksum,omitempty"` // SHA256

	// Source
	SourceURL string `json:"source_url"`

	// Timestamps
	DownloadedAt time.Time `json:"downloaded_at"`

	// Expiration (24 hours)
	ExpiresAt time.Time `json:"expires_at"`
}

// NewSnapshotCache creates a new SnapshotCache entry with default expiration.
func NewSnapshotCache(cacheKey, filePath, sourceURL, decompressor string, sizeBytes int64) *SnapshotCache {
	return NewSnapshotCacheWithExpiration(cacheKey, filePath, sourceURL, decompressor, sizeBytes, DefaultCacheExpiration)
}

// NewSnapshotCacheWithExpiration creates a new SnapshotCache entry with custom expiration.
func NewSnapshotCacheWithExpiration(cacheKey, filePath, sourceURL, decompressor string, sizeBytes int64, expiration time.Duration) *SnapshotCache {
	now := time.Now()
	return &SnapshotCache{
		CacheKey:     cacheKey,
		FilePath:     filePath,
		SizeBytes:    sizeBytes,
		Decompressor: decompressor,
		SourceURL:    sourceURL,
		DownloadedAt: now,
		ExpiresAt:    now.Add(expiration),
	}
}

// IsExpired returns true if the cache entry has expired.
func (s *SnapshotCache) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid checks if the cache entry is valid and the file exists.
func (s *SnapshotCache) IsValid() bool {
	if s.IsExpired() {
		return false
	}
	// Check file exists
	if _, err := os.Stat(s.FilePath); os.IsNotExist(err) {
		return false
	}
	return true
}

// TimeUntilExpiry returns the duration until the cache expires.
func (s *SnapshotCache) TimeUntilExpiry() time.Duration {
	return time.Until(s.ExpiresAt)
}

// CacheDir returns the cache directory for a given home directory and cache key.
func CacheDir(homeDir, cacheKey string) string {
	return filepath.Join(homeDir, "snapshots", cacheKey)
}

// MetadataPath returns the path to the cache metadata file.
func MetadataPath(homeDir, cacheKey string) string {
	return filepath.Join(CacheDir(homeDir, cacheKey), "snapshot.meta.json")
}

// SnapshotPath returns the path where the snapshot file should be stored.
func SnapshotPath(homeDir, cacheKey, extension string) string {
	return filepath.Join(CacheDir(homeDir, cacheKey), "snapshot"+extension)
}

// Save persists the cache metadata to disk.
func (s *SnapshotCache) Save(homeDir string) error {
	cacheDir := CacheDir(homeDir, s.CacheKey)

	// Ensure directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache metadata: %w", err)
	}

	metaPath := MetadataPath(homeDir, s.CacheKey)
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache metadata: %w", err)
	}

	return nil
}

// LoadSnapshotCache loads cache metadata from disk.
func LoadSnapshotCache(homeDir, cacheKey string) (*SnapshotCache, error) {
	metaPath := MetadataPath(homeDir, cacheKey)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache exists
		}
		return nil, fmt.Errorf("failed to read cache metadata: %w", err)
	}

	var cache SnapshotCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache metadata: %w", err)
	}

	return &cache, nil
}

// ClearCache removes the cached snapshot for a cache key.
func ClearCache(homeDir, cacheKey string) error {
	cacheDir := CacheDir(homeDir, cacheKey)
	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}

// ClearAllCaches removes all cached snapshots.
func ClearAllCaches(homeDir string) error {
	snapshotsDir := filepath.Join(homeDir, "snapshots")
	if err := os.RemoveAll(snapshotsDir); err != nil {
		return fmt.Errorf("failed to clear all caches: %w", err)
	}
	return nil
}

// GetValidCache returns a valid cache entry if one exists, nil otherwise.
func GetValidCache(homeDir, cacheKey string) (*SnapshotCache, error) {
	cache, err := LoadSnapshotCache(homeDir, cacheKey)
	if err != nil {
		return nil, err
	}
	if cache == nil || !cache.IsValid() {
		return nil, nil
	}
	return cache, nil
}

// CacheExists returns true if a valid cache exists for the cache key.
func CacheExists(homeDir, cacheKey string) bool {
	cache, err := GetValidCache(homeDir, cacheKey)
	return err == nil && cache != nil
}
