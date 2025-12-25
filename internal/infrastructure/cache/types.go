package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/pkg/network"
)

// CachedBinary represents a single cached binary build.
type CachedBinary struct {
	CommitHash  string               `json:"commit_hash"`  // Full git commit hash (40 chars)
	Ref         string               `json:"ref"`          // Original ref used to build (branch, tag, commit)
	BuildTime   time.Time            `json:"build_time"`   // When the binary was built
	Size        int64                `json:"size"`         // Binary file size in bytes
	NetworkType string               `json:"network_type"` // Network type (mainnet, testnet, devnet)
	BuildConfig *network.BuildConfig `json:"build_config"` // Full build configuration (tags, ldflags, env)
	BuildTags   []string             `json:"build_tags"`   // DEPRECATED: Use BuildConfig.Tags instead. Kept for backward compatibility.
	BinaryPath  string               `json:"-"`            // Absolute path to the cached binary (not persisted)
	DirKey      string               `json:"-"`            // Actual directory name on disk (may differ from CacheKey() for legacy entries)
}

// CacheKey returns the cache key for this cached binary.
// Format: {networkType}/{commitHash}-{configHash}
// Example: "mainnet/abc123...def-1a2b3c4d" or "testnet/abc123...def-2b3c4d5e"
//
// For backward compatibility with old cache entries, if NetworkType is empty,
// falls back to old format: {commitHash}-{tagsHash}
func (c *CachedBinary) CacheKey() string {
	if c.NetworkType == "" {
		// Legacy format for backward compatibility
		return MakeCacheKeyLegacy(c.CommitHash, c.BuildTags)
	}

	// New format with network type
	buildConfig := c.BuildConfig
	if buildConfig == nil && len(c.BuildTags) > 0 {
		// Handle transition: BuildConfig is nil but BuildTags exist
		buildConfig = &network.BuildConfig{Tags: c.BuildTags}
	}
	return MakeCacheKey(c.NetworkType, c.CommitHash, buildConfig)
}

// ActualDirKey returns the actual directory name on disk.
// This may differ from CacheKey() for legacy entries that don't have tag hashes.
// Falls back to CacheKey() if DirKey is not set.
func (c *CachedBinary) ActualDirKey() string {
	if c.DirKey != "" {
		return c.DirKey
	}
	return c.CacheKey()
}

// Metadata represents persistable metadata for a cached binary.
type Metadata struct {
	CommitHash  string               `json:"commit_hash"`
	Ref         string               `json:"ref"`
	BuildTime   time.Time            `json:"build_time"`
	Size        int64                `json:"size"`
	NetworkType string               `json:"network_type"`           // Network type (mainnet, testnet, devnet)
	BuildConfig *network.BuildConfig `json:"build_config,omitempty"` // Full build configuration
	BuildTags   []string             `json:"build_tags,omitempty"`   // DEPRECATED: For backward compatibility only
	Network     string               `json:"network,omitempty"`      // DEPRECATED: Use NetworkType instead
}

// ToMetadata converts CachedBinary to persistable Metadata.
func (c *CachedBinary) ToMetadata() *Metadata {
	metadata := &Metadata{
		CommitHash:  c.CommitHash,
		Ref:         c.Ref,
		BuildTime:   c.BuildTime,
		Size:        c.Size,
		NetworkType: c.NetworkType,
		BuildConfig: c.BuildConfig,
	}

	// For backward compatibility: populate legacy fields
	if metadata.BuildConfig == nil && len(c.BuildTags) > 0 {
		metadata.BuildTags = c.BuildTags
	}
	// Populate legacy Network field for old readers
	if metadata.NetworkType != "" {
		metadata.Network = metadata.NetworkType
	}

	return metadata
}

// MakeCacheKey creates a network-aware cache key from network type, commit hash, and build config.
// The key format is: {networkType}/{commitHash}-{configHash}
// where configHash is a hash of the complete BuildConfig (tags, ldflags, env vars).
//
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
	// Validate network type
	if networkType == "" {
		networkType = "default"
	}

	// Compute config hash
	configHash := HashBuildConfig(buildConfig)

	// Format: networkType/commitHash-configHash
	return fmt.Sprintf("%s/%s-%s", networkType, commitHash, configHash)
}

// MakeCacheKeyLegacy creates a cache key using the old format (for backward compatibility).
// Format: {commitHash}-{tagsHash}
// This is used for migrating old cache entries.
//
// DEPRECATED: New code should use MakeCacheKey with network type.
func MakeCacheKeyLegacy(commitHash string, buildTags []string) string {
	tagsHash := HashBuildTags(buildTags)
	return commitHash + "-" + tagsHash
}

// HashBuildConfig returns a short hash of the build configuration.
// Returns "00000000" if config is nil or empty.
// This delegates to BuildConfig.Hash() from the network package.
func HashBuildConfig(buildConfig *network.BuildConfig) string {
	if buildConfig == nil {
		return "00000000"
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

// HashBuildTags returns a short hash of the build tags.
// Returns "00000000" if no tags provided.
//
// DEPRECATED: Use HashBuildConfig instead. This is kept for backward compatibility.
func HashBuildTags(buildTags []string) string {
	if len(buildTags) == 0 {
		return "00000000"
	}

	// Sort tags for consistent hashing
	sorted := make([]string, len(buildTags))
	copy(sorted, buildTags)
	sort.Strings(sorted)

	// Join and hash
	joined := strings.Join(sorted, ",")
	hash := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(hash[:])[:8]
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
