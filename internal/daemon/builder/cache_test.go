// internal/daemon/builder/cache_test.go
package builder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheKey(t *testing.T) {
	spec := BuildSpec{
		GitRepo:    "github.com/cosmos/gaia",
		GitRef:     "v15.0.0",
		PluginName: "cosmos",
		BuildFlags: map[string]string{"tags": "netgo"},
	}

	cache := NewBinaryCache(t.TempDir())

	key1 := cache.CacheKey(spec, "abc123def456")
	key2 := cache.CacheKey(spec, "abc123def456")
	key3 := cache.CacheKey(spec, "different123")

	if key1 != key2 {
		t.Error("Same inputs should produce same cache key")
	}

	if key1 == key3 {
		t.Error("Different commits should produce different cache keys")
	}

	t.Logf("Cache key: %s", key1)
}

func TestCacheStore(t *testing.T) {
	cacheDir := t.TempDir()
	cache := NewBinaryCache(cacheDir)

	// Create a fake binary file (Get verifies binary still exists)
	binaryPath := filepath.Join(cacheDir, "fake-binary")
	if err := os.WriteFile(binaryPath, []byte("fake binary content"), 0755); err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	result := &BuildResult{
		BinaryPath: binaryPath,
		GitCommit:  "abc123",
		GitRef:     "v1.0.0",
		BuiltAt:    time.Now(),
		CacheKey:   "test-cache-key",
	}

	// Store the result
	err := cache.Store(result)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Retrieve it
	retrieved, found := cache.Get("test-cache-key")
	if !found {
		t.Fatal("Cache entry not found")
	}

	if retrieved.GitCommit != result.GitCommit {
		t.Errorf("GitCommit mismatch: got %s, want %s", retrieved.GitCommit, result.GitCommit)
	}
}

func TestCacheClean(t *testing.T) {
	cacheDir := t.TempDir()
	cache := NewBinaryCache(cacheDir)

	// Create an old cache entry
	oldResult := &BuildResult{
		BinaryPath: filepath.Join(cacheDir, "old-binary"),
		GitCommit:  "old123",
		GitRef:     "v0.1.0",
		BuiltAt:    time.Now().Add(-48 * time.Hour), // 2 days ago
		CacheKey:   "old-cache-key",
	}

	// Create a fake binary file
	os.MkdirAll(filepath.Dir(oldResult.BinaryPath), 0755)
	os.WriteFile(oldResult.BinaryPath, []byte("fake"), 0755)

	cache.Store(oldResult)

	// Clean entries older than 24 hours
	err := cache.Clean(24 * time.Hour)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Old entry should be gone
	_, found := cache.Get("old-cache-key")
	if found {
		t.Error("Old cache entry should have been cleaned")
	}
}
