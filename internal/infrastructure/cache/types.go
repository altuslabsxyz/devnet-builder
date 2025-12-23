package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// CachedBinary represents a single cached binary build.
type CachedBinary struct {
	CommitHash string    `json:"commit_hash"` // Full git commit hash (40 chars)
	Ref        string    `json:"ref"`         // Original ref used to build (branch, tag, commit)
	BuildTime  time.Time `json:"build_time"`  // When the binary was built
	Size       int64     `json:"size"`        // Binary file size in bytes
	Network    string    `json:"network"`     // Network type (mainnet, testnet)
	BuildTags  []string  `json:"build_tags"`  // Go build tags used (e.g., ["no_dynamic_precompiles"])
	BinaryPath string    `json:"-"`           // Absolute path to the cached binary (not persisted)
	DirKey     string    `json:"-"`           // Actual directory name on disk (may differ from CacheKey() for legacy entries)
}

// CacheKey returns the cache key combining commit hash and build tags.
// Format: {commitHash}-{tagsHash} where tagsHash is first 8 chars of SHA256 of sorted tags.
// Example: "abc123...def-1a2b3c4d" or "abc123...def-00000000" (no tags)
func (c *CachedBinary) CacheKey() string {
	return MakeCacheKey(c.CommitHash, c.BuildTags)
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

// Metadata returns the persistable metadata for this cached binary.
type Metadata struct {
	CommitHash string    `json:"commit_hash"`
	Ref        string    `json:"ref"`
	BuildTime  time.Time `json:"build_time"`
	Size       int64     `json:"size"`
	Network    string    `json:"network"`
	BuildTags  []string  `json:"build_tags,omitempty"`
}

// ToMetadata converts CachedBinary to persistable Metadata.
func (c *CachedBinary) ToMetadata() *Metadata {
	return &Metadata{
		CommitHash: c.CommitHash,
		Ref:        c.Ref,
		BuildTime:  c.BuildTime,
		Size:       c.Size,
		Network:    c.Network,
		BuildTags:  c.BuildTags,
	}
}

// MakeCacheKey creates a cache key from commit hash and build tags.
// The key format is: {commitHash}-{tagsHash}
// tagsHash is the first 8 characters of SHA256 hash of sorted, joined build tags.
// If no build tags, tagsHash is "00000000".
func MakeCacheKey(commitHash string, buildTags []string) string {
	tagsHash := HashBuildTags(buildTags)
	return commitHash + "-" + tagsHash
}

// HashBuildTags returns a short hash of the build tags.
// Returns "00000000" if no tags provided.
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
