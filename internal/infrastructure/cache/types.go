package cache

import (
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

type CachedBinary struct {
	CommitHash  string               `json:"commit_hash"`
	Ref         string               `json:"ref"`
	BuildTime   time.Time            `json:"build_time"`
	Size        int64                `json:"size"`
	NetworkType string               `json:"network_type"`
	BuildConfig *network.BuildConfig `json:"build_config"`
	BinaryPath  string               `json:"-"`
	DirKey      string               `json:"-"`
}

// CacheKey returns the cache key: {networkType}/{commitHash}-{configHash}
func (c *CachedBinary) CacheKey() string {
	if c.NetworkType == "" {
		panic("CachedBinary.NetworkType is required for cache key generation")
	}
	if c.BuildConfig == nil {
		panic("CachedBinary.BuildConfig is required for cache key generation")
	}
	return MakeCacheKey(c.NetworkType, c.CommitHash, c.BuildConfig)
}

func (c *CachedBinary) ActualDirKey() string {
	if c.DirKey != "" {
		return c.DirKey
	}
	return c.CacheKey()
}

type Metadata struct {
	CommitHash  string               `json:"commit_hash"`
	Ref         string               `json:"ref"`
	BuildTime   time.Time            `json:"build_time"`
	Size        int64                `json:"size"`
	NetworkType string               `json:"network_type"`
	BuildConfig *network.BuildConfig `json:"build_config"`
}

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

// MakeCacheKey creates a cache key: {networkType}/{commitHash}-{configHash}
// This ensures binaries with different configs are cached separately.
func MakeCacheKey(networkType, commitHash string, buildConfig *network.BuildConfig) string {
	if networkType == "" {
		panic("MakeCacheKey: networkType is required")
	}
	if commitHash == "" {
		panic("MakeCacheKey: commitHash is required")
	}
	if buildConfig == nil {
		panic("MakeCacheKey: buildConfig is required")
	}
	configHash := HashBuildConfig(buildConfig)
	return fmt.Sprintf("%s/%s-%s", networkType, commitHash, configHash)
}

func HashBuildConfig(buildConfig *network.BuildConfig) string {
	if buildConfig == nil {
		panic("HashBuildConfig: buildConfig is required")
	}
	configHash := buildConfig.Hash()
	if configHash == "empty" {
		return "00000000"
	}
	if len(configHash) >= 8 {
		return configHash[:8]
	}
	return configHash
}

type CacheStats struct {
	TotalEntries int
	TotalSize    int64
}
