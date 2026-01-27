// internal/daemon/builder/cache.go
package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BinaryCache manages cached binary builds
type BinaryCache struct {
	cacheDir string // ~/.devnet-builder/binaries/
}

// NewBinaryCache creates a new binary cache
func NewBinaryCache(cacheDir string) *BinaryCache {
	return &BinaryCache{cacheDir: cacheDir}
}

// CacheKey generates a deterministic cache key for a build spec
func (c *BinaryCache) CacheKey(spec BuildSpec, resolvedCommit string) string {
	// Combine all inputs that affect the build
	h := sha256.New()
	h.Write([]byte(spec.GitRepo))
	h.Write([]byte(resolvedCommit))
	h.Write([]byte(spec.PluginName))
	h.Write([]byte(spec.GoVersion))

	// Sort build flags for deterministic hashing
	var flagKeys []string
	for k := range spec.BuildFlags {
		flagKeys = append(flagKeys, k)
	}
	sort.Strings(flagKeys)
	for _, k := range flagKeys {
		h.Write([]byte(k))
		h.Write([]byte(spec.BuildFlags[k]))
	}

	return hex.EncodeToString(h.Sum(nil))[:16] // Use first 16 chars
}

// CachePath returns the directory path for a cache key
func (c *BinaryCache) CachePath(cacheKey string) string {
	return filepath.Join(c.cacheDir, cacheKey)
}

// MetadataPath returns the metadata file path for a cache key
func (c *BinaryCache) MetadataPath(cacheKey string) string {
	return filepath.Join(c.CachePath(cacheKey), "metadata.json")
}

// Store saves a build result to cache
func (c *BinaryCache) Store(result *BuildResult) error {
	cacheDir := c.CachePath(result.CacheKey)

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Write metadata
	metadata, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metaPath := c.MetadataPath(result.CacheKey)
	if err := os.WriteFile(metaPath, metadata, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// Get retrieves a cached build result
func (c *BinaryCache) Get(cacheKey string) (*BuildResult, bool) {
	metaPath := c.MetadataPath(cacheKey)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, false
	}

	var result BuildResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}

	// Verify binary still exists
	if _, err := os.Stat(result.BinaryPath); os.IsNotExist(err) {
		return nil, false
	}

	return &result, true
}

// Clean removes cache entries older than maxAge
func (c *BinaryCache) Clean(maxAge time.Duration) error {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metaPath := c.MetadataPath(entry.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var result BuildResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}

		if result.BuiltAt.Before(cutoff) {
			cachePath := c.CachePath(entry.Name())
			os.RemoveAll(cachePath)
		}
	}

	return nil
}

// List returns all cached builds
func (c *BinaryCache) List() ([]*BuildResult, error) {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []*BuildResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if result, found := c.Get(entry.Name()); found {
			results = append(results, result)
		}
	}

	return results, nil
}

// FormatCacheKey returns a human-readable description of what a cache key represents
func (c *BinaryCache) FormatCacheKey(result *BuildResult) string {
	ref := result.GitRef
	if len(result.GitCommit) >= 7 {
		ref = fmt.Sprintf("%s (%s)", ref, result.GitCommit[:7])
	}
	return fmt.Sprintf("%s @ %s", strings.TrimPrefix(result.BinaryPath, c.cacheDir+"/"), ref)
}
