package cache

import (
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/pkg/network"
)

// CachedBinary represents a single cached binary build.
// NEW FORMAT ONLY: NetworkType and BuildConfig are required.
type CachedBinary struct {
	CommitHash  string               `json:"commit_hash"`  // Full git commit hash (40 chars)
	Ref         string               `json:"ref"`          // Original ref used to build (branch, tag, commit)
	BuildTime   time.Time            `json:"build_time"`   // When the binary was built
	Size        int64                `json:"size"`         // Binary file size in bytes
	NetworkType string               `json:"network_type"` // Network type (mainnet, testnet, devnet) - REQUIRED
	BuildConfig *network.BuildConfig `json:"build_config"` // Full build configuration (tags, ldflags, env) - REQUIRED
	BinaryPath  string               `json:"-"`            // Absolute path to the cached binary (not persisted)
	DirKey      string               `json:"-"`            // Actual directory path on disk (networkType/hash-config)
}

// CacheKey returns the cache key for this cached binary.
// Format: {networkType}/{commitHash}-{configHash}
// Example: "mainnet/abc123...def-1a2b3c4d" or "testnet/abc123...def-2b3c4d5e"
// NEW FORMAT ONLY: NetworkType is required.
func (c *CachedBinary) CacheKey() string {
	// NetworkType is mandatory in new format
	if c.NetworkType == "" {
		panic("CachedBinary.NetworkType is required for cache key generation")
	}

	// BuildConfig is mandatory
	if c.BuildConfig == nil {
		panic("CachedBinary.BuildConfig is required for cache key generation")
	}

	return MakeCacheKey(c.NetworkType, c.CommitHash, c.BuildConfig)
}

// ActualDirKey returns the actual directory path on disk.
// Format: {networkType}/{commitHash}-{configHash}
// Falls back to CacheKey() if DirKey is not set.
func (c *CachedBinary) ActualDirKey() string {
	if c.DirKey != "" {
		return c.DirKey
	}
	return c.CacheKey()
}

// Metadata represents persistable metadata for a cached binary.
// NEW FORMAT ONLY: NetworkType and BuildConfig are required.
type Metadata struct {
	CommitHash  string               `json:"commit_hash"`
	Ref         string               `json:"ref"`
	BuildTime   time.Time            `json:"build_time"`
	Size        int64                `json:"size"`
	NetworkType string               `json:"network_type"` // Network type (mainnet, testnet, devnet) - REQUIRED
	BuildConfig *network.BuildConfig `json:"build_config"` // Full build configuration - REQUIRED
}

// ToMetadata converts CachedBinary to persistable Metadata.
func (c *CachedBinary) ToMetadata() *Metadata {
	return &Metadata{
		CommitHash:  c.CommitHash,
		Ref:         c.Ref,
		BuildTime:   c.BuildTime,
		Size:        c.Size,
		NetworkType: c.NetworkType,
		BuildConfig: c.BuildConfig,
	}
}

// MakeCacheKey creates a network-aware cache key from network type, commit hash, and build config.
// The key format is: {networkType}/{commitHash}-{configHash}
// where configHash is a hash of the complete BuildConfig (tags, ldflags, env vars).
//
// NEW FORMAT ONLY: All parameters are required.
// This ensures binaries built with different configurations are cached separately:
//   - Different network types (mainnet vs testnet) get separate directories
//   - Different ldflags (different EVMChainIDs) get different cache entries
//   - Same commit with different build configs creates different binaries
//
// Example cache keys:
//   - "mainnet/abc123def456-1a2b3c4d"
//   - "testnet/abc123def456-5e6f7g8h"
//   - "devnet/abc123def456-9i0j1k2l"
func MakeCacheKey(networkType, commitHash string, buildConfig *network.BuildConfig) string {
	// Validate required parameters
	if networkType == "" {
		panic("MakeCacheKey: networkType is required")
	}
	if commitHash == "" {
		panic("MakeCacheKey: commitHash is required")
	}
	if buildConfig == nil {
		panic("MakeCacheKey: buildConfig is required")
	}

	// Compute config hash
	configHash := HashBuildConfig(buildConfig)

	// Format: networkType/commitHash-configHash
	return fmt.Sprintf("%s/%s-%s", networkType, commitHash, configHash)
}

// HashBuildConfig returns a short hash of the build configuration.
// NEW FORMAT ONLY: BuildConfig is required.
// This delegates to BuildConfig.Hash() from the network package.
func HashBuildConfig(buildConfig *network.BuildConfig) string {
	if buildConfig == nil {
		panic("HashBuildConfig: buildConfig is required")
	}

	// Use the BuildConfig's own Hash() method (already deterministic)
	configHash := buildConfig.Hash()
	if configHash == "empty" {
		return "00000000"
	}

	// BuildConfig.Hash() returns 16 hex chars, we want first 8 for cache keys
	if len(configHash) >= 8 {
		return configHash[:8]
	}

	return configHash
}

// ActiveSymlink represents the current symlink state.
type ActiveSymlink struct {
	Path       string // Absolute path to symlink (~/.devnet-builder/bin/{binaryName})
	Target     string // Path to current active binary in cache
	CommitHash string // Commit hash of currently active binary
}

// CacheStats provides statistics about the cache.
type CacheStats struct {
	TotalEntries int   // Number of cached binaries
	TotalSize    int64 // Total size of all cached binaries in bytes
}
