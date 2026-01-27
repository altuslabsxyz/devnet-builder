// internal/daemon/builder/builder_test.go
package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuilderCacheHit(t *testing.T) {
	tempDir := t.TempDir()
	cache := NewBinaryCache(tempDir)

	result := &BuildResult{
		BinaryPath: "/usr/bin/echo",
		GitCommit:  "abc123def456789",
		GitRef:     "v1.0.0",
		BuiltAt:    time.Now(),
		CacheKey:   "test-key-123",
	}
	if err := cache.Store(result); err != nil {
		t.Fatalf("Failed to store result: %v", err)
	}

	builder := &DefaultBuilder{
		cache:   cache,
		dataDir: tempDir,
	}

	spec := BuildSpec{
		GitRepo:    "github.com/test/repo",
		GitRef:     "v1.0.0",
		PluginName: "test",
	}

	cached, found := builder.GetCached(context.Background(), spec)
	if found && cached.CacheKey != "test-key-123" {
		t.Error("Unexpected cache hit")
	}
}

func TestBuilderClean(t *testing.T) {
	tempDir := t.TempDir()
	cache := NewBinaryCache(tempDir)

	builder := &DefaultBuilder{
		cache:   cache,
		dataDir: tempDir,
	}

	err := builder.Clean(context.Background(), 24*time.Hour)
	if err != nil {
		t.Errorf("Clean failed on empty cache: %v", err)
	}
}

func TestBuilderCleanWithExpiredEntries(t *testing.T) {
	tempDir := t.TempDir()
	cache := NewBinaryCache(tempDir)

	// Create fake binary files in the cache directory
	// (Get verifies binary still exists)
	oldBinaryPath := filepath.Join(tempDir, "old-key", "old-binary")
	newBinaryPath := filepath.Join(tempDir, "new-key", "new-binary")

	// These directories will be created by Store, but we need binaries to exist
	// Store creates the directory, so we create binaries after storing

	// Store an old result
	oldResult := &BuildResult{
		BinaryPath: oldBinaryPath,
		GitCommit:  "old123",
		GitRef:     "v0.9.0",
		BuiltAt:    time.Now().Add(-48 * time.Hour), // 2 days old
		CacheKey:   "old-key",
	}
	if err := cache.Store(oldResult); err != nil {
		t.Fatalf("Failed to store old result: %v", err)
	}
	// Create the fake binary after the directory exists
	if err := os.WriteFile(oldBinaryPath, []byte("old"), 0755); err != nil {
		t.Fatalf("Failed to create old binary: %v", err)
	}

	// Store a recent result
	newResult := &BuildResult{
		BinaryPath: newBinaryPath,
		GitCommit:  "new456",
		GitRef:     "v1.0.0",
		BuiltAt:    time.Now(),
		CacheKey:   "new-key",
	}
	if err := cache.Store(newResult); err != nil {
		t.Fatalf("Failed to store new result: %v", err)
	}
	// Create the fake binary after the directory exists
	if err := os.WriteFile(newBinaryPath, []byte("new"), 0755); err != nil {
		t.Fatalf("Failed to create new binary: %v", err)
	}

	builder := &DefaultBuilder{
		cache:   cache,
		dataDir: tempDir,
	}

	// Clean entries older than 24 hours
	err := builder.Clean(context.Background(), 24*time.Hour)
	if err != nil {
		t.Errorf("Clean failed: %v", err)
	}

	// Old entry should be gone
	if _, found := cache.Get("old-key"); found {
		t.Error("Expected old entry to be cleaned")
	}

	// New entry should still exist
	if _, found := cache.Get("new-key"); !found {
		t.Error("Expected new entry to still exist")
	}
}

func TestGetCachedReturnsNilWithoutResolvedCommit(t *testing.T) {
	tempDir := t.TempDir()
	cache := NewBinaryCache(tempDir)

	builder := &DefaultBuilder{
		cache:   cache,
		dataDir: tempDir,
	}

	spec := BuildSpec{
		GitRepo:    "github.com/test/repo",
		GitRef:     "main", // Branch ref, not a resolved commit
		PluginName: "test",
	}

	// GetCached should return nil, false because we need to resolve the commit
	// before we can check cache (the cache key depends on the resolved commit)
	cached, found := builder.GetCached(context.Background(), spec)
	if found {
		t.Errorf("Expected no cache hit without resolved commit, got: %v", cached)
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://github.com/cosmos/gaia", true},
		{"http://github.com/cosmos/gaia", true},
		{"git@github.com:cosmos/gaia.git", true},
		{"github.com/cosmos/gaia", false},
		{"cosmos/gaia", false},
		{"", false},
	}

	for _, tc := range tests {
		result := isURL(tc.input)
		if result != tc.expected {
			t.Errorf("isURL(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestMergeBuildFlags(t *testing.T) {
	defaults := map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "linux",
		"GOARCH":      "amd64",
	}

	overrides := map[string]string{
		"GOARCH": "arm64", // Override
		"DEBUG":  "true",  // New key
	}

	merged := mergeBuildFlags(defaults, overrides)

	if merged["CGO_ENABLED"] != "0" {
		t.Errorf("Expected CGO_ENABLED=0, got %s", merged["CGO_ENABLED"])
	}
	if merged["GOOS"] != "linux" {
		t.Errorf("Expected GOOS=linux, got %s", merged["GOOS"])
	}
	if merged["GOARCH"] != "arm64" {
		t.Errorf("Expected GOARCH=arm64 (override), got %s", merged["GOARCH"])
	}
	if merged["DEBUG"] != "true" {
		t.Errorf("Expected DEBUG=true (new), got %s", merged["DEBUG"])
	}
}

func TestMergeBuildFlagsWithNilInputs(t *testing.T) {
	// Both nil
	merged := mergeBuildFlags(nil, nil)
	if merged == nil {
		t.Error("Expected non-nil map, got nil")
	}
	if len(merged) != 0 {
		t.Errorf("Expected empty map, got %v", merged)
	}

	// Defaults only
	defaults := map[string]string{"KEY": "value"}
	merged = mergeBuildFlags(defaults, nil)
	if merged["KEY"] != "value" {
		t.Errorf("Expected KEY=value, got %s", merged["KEY"])
	}

	// Overrides only
	overrides := map[string]string{"KEY2": "value2"}
	merged = mergeBuildFlags(nil, overrides)
	if merged["KEY2"] != "value2" {
		t.Errorf("Expected KEY2=value2, got %s", merged["KEY2"])
	}
}
