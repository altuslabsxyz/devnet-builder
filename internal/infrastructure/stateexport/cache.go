package stateexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultGenesisCacheExpiration matches snapshot cache expiration.
	// This ensures genesis export cache expires at the same time as the snapshot it came from.
	DefaultGenesisCacheExpiration = 30 * time.Minute
)

// GenesisCache represents a cached exported genesis.
// This cache stores the raw genesis exported from a snapshot,
// BEFORE any plugin modifications.
type GenesisCache struct {
	// Identification
	CacheKey string `json:"cache_key"` // "plugin-network" format (e.g., "stable-mainnet", "ault-testnet")

	// Genesis Data
	FilePath  string `json:"file_path"`  // Path to cached genesis.json
	SizeBytes int64  `json:"size_bytes"` // Size of genesis file
	ChainID   string `json:"chain_id"`   // Chain ID from genesis

	// Source
	SnapshotURL string `json:"snapshot_url"` // URL of the snapshot this genesis was exported from

	// Timestamps
	ExportedAt time.Time `json:"exported_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// NewGenesisCache creates a new GenesisCache entry with default expiration.
func NewGenesisCache(cacheKey, filePath, snapshotURL, chainID string, sizeBytes int64) *GenesisCache {
	return NewGenesisCacheWithExpiration(cacheKey, filePath, snapshotURL, chainID, sizeBytes, DefaultGenesisCacheExpiration)
}

// NewGenesisCacheWithExpiration creates a new GenesisCache entry with custom expiration.
func NewGenesisCacheWithExpiration(cacheKey, filePath, snapshotURL, chainID string, sizeBytes int64, expiration time.Duration) *GenesisCache {
	now := time.Now()
	return &GenesisCache{
		CacheKey:    cacheKey,
		FilePath:    filePath,
		SizeBytes:   sizeBytes,
		ChainID:     chainID,
		SnapshotURL: snapshotURL,
		ExportedAt:  now,
		ExpiresAt:   now.Add(expiration),
	}
}

// IsExpired returns true if the cache entry has expired.
func (g *GenesisCache) IsExpired() bool {
	return time.Now().After(g.ExpiresAt)
}

// IsValid checks if the cache entry is valid and the file exists.
func (g *GenesisCache) IsValid() bool {
	if g.IsExpired() {
		return false
	}
	// Check file exists
	if _, err := os.Stat(g.FilePath); os.IsNotExist(err) {
		return false
	}
	return true
}

// TimeUntilExpiry returns the duration until the cache expires.
func (g *GenesisCache) TimeUntilExpiry() time.Duration {
	return time.Until(g.ExpiresAt)
}

// GenesisCacheDir returns the cache directory for genesis (same as snapshot cache).
func GenesisCacheDir(homeDir, cacheKey string) string {
	return filepath.Join(homeDir, "snapshots", cacheKey)
}

// GenesisMetadataPath returns the path to the genesis cache metadata file.
func GenesisMetadataPath(homeDir, cacheKey string) string {
	return filepath.Join(GenesisCacheDir(homeDir, cacheKey), "genesis.meta.json")
}

// GenesisCachePath returns the path where the cached genesis should be stored.
func GenesisCachePath(homeDir, cacheKey string) string {
	return filepath.Join(GenesisCacheDir(homeDir, cacheKey), "genesis.cached.json")
}

// Save persists the genesis cache metadata to disk.
func (g *GenesisCache) Save(homeDir string) error {
	cacheDir := GenesisCacheDir(homeDir, g.CacheKey)

	// Ensure directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create genesis cache directory: %w", err)
	}

	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis cache metadata: %w", err)
	}

	metaPath := GenesisMetadataPath(homeDir, g.CacheKey)
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write genesis cache metadata: %w", err)
	}

	return nil
}

// LoadGenesisCache loads genesis cache metadata from disk.
func LoadGenesisCache(homeDir, cacheKey string) (*GenesisCache, error) {
	metaPath := GenesisMetadataPath(homeDir, cacheKey)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No cache exists
		}
		return nil, fmt.Errorf("failed to read genesis cache metadata: %w", err)
	}

	var cache GenesisCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse genesis cache metadata: %w", err)
	}

	return &cache, nil
}

// ClearGenesisCache removes the cached genesis for a cache key.
func ClearGenesisCache(homeDir, cacheKey string) error {
	// Remove cached genesis file
	genesisPath := GenesisCachePath(homeDir, cacheKey)
	if err := os.Remove(genesisPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cached genesis: %w", err)
	}

	// Remove metadata
	metaPath := GenesisMetadataPath(homeDir, cacheKey)
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove genesis cache metadata: %w", err)
	}

	return nil
}

// GetValidGenesisCache returns a valid genesis cache entry if one exists, nil otherwise.
func GetValidGenesisCache(homeDir, cacheKey string) (*GenesisCache, error) {
	cache, err := LoadGenesisCache(homeDir, cacheKey)
	if err != nil {
		return nil, err
	}
	if cache == nil || !cache.IsValid() {
		return nil, nil
	}
	return cache, nil
}

// GenesisCacheExists returns true if a valid genesis cache exists for the cache key.
func GenesisCacheExists(homeDir, cacheKey string) bool {
	cache, err := GetValidGenesisCache(homeDir, cacheKey)
	return err == nil && cache != nil
}

// SaveGenesisToCacheWithSnapshot saves exported genesis to cache.
// This should be called after successfully exporting genesis from a snapshot.
// The snapshotURL is used to associate this genesis with the snapshot it came from.
// cacheKey format: "plugin-network" (e.g., "stable-mainnet", "ault-testnet")
func SaveGenesisToCacheWithSnapshot(homeDir, cacheKey, snapshotURL string, genesis []byte) error {
	// Extract chain ID from genesis
	chainID, err := GetChainIDFromGenesis(genesis)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Write genesis to cache file
	genesisPath := GenesisCachePath(homeDir, cacheKey)
	if err := os.WriteFile(genesisPath, genesis, 0644); err != nil {
		return fmt.Errorf("failed to write cached genesis: %w", err)
	}

	// Create cache metadata
	cache := NewGenesisCache(
		cacheKey,
		genesisPath,
		snapshotURL,
		chainID,
		int64(len(genesis)),
	)

	// Save metadata
	if err := cache.Save(homeDir); err != nil {
		// Cleanup genesis file if metadata save fails
		os.Remove(genesisPath)
		return err
	}

	return nil
}
