package cache

import (
	"time"
)

// CachedBinary represents a single cached binary build.
type CachedBinary struct {
	CommitHash string    `json:"commit_hash"` // Full git commit hash (40 chars) - Primary key
	Ref        string    `json:"ref"`         // Original ref used to build (branch, tag, commit)
	BuildTime  time.Time `json:"build_time"`  // When the binary was built
	Size       int64     `json:"size"`        // Binary file size in bytes
	Network    string    `json:"network"`     // Network type (mainnet, testnet)
	BinaryPath string    `json:"-"`           // Absolute path to the cached binary (not persisted)
}

// Metadata returns the persistable metadata for this cached binary.
type Metadata struct {
	CommitHash string    `json:"commit_hash"`
	Ref        string    `json:"ref"`
	BuildTime  time.Time `json:"build_time"`
	Size       int64     `json:"size"`
	Network    string    `json:"network"`
}

// ToMetadata converts CachedBinary to persistable Metadata.
func (c *CachedBinary) ToMetadata() *Metadata {
	return &Metadata{
		CommitHash: c.CommitHash,
		Ref:        c.Ref,
		BuildTime:  c.BuildTime,
		Size:       c.Size,
		Network:    c.Network,
	}
}

// ActiveSymlink represents the current symlink state.
type ActiveSymlink struct {
	Path       string // Absolute path to symlink (~/.stable-devnet/bin/stabled)
	Target     string // Path to current active binary in cache
	CommitHash string // Commit hash of currently active binary
}

// CacheStats provides statistics about the cache.
type CacheStats struct {
	TotalEntries int   // Number of cached binaries
	TotalSize    int64 // Total size of all cached binaries in bytes
}
