package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/b-harvest/devnet-builder/internal/infrastructure/filesystem"
)

// TestBinaryScanner_ScanCachedBinaries_EmptyCache tests scanning an empty cache directory
func TestBinaryScanner_ScanCachedBinaries_EmptyCache(t *testing.T) {
	// Setup: Create empty cache structure
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "binaries")
	networkDir := filepath.Join(cacheDir, "mainnet")
	if err := os.MkdirAll(networkDir, 0755); err != nil {
		t.Fatalf("Failed to create network directory: %v", err)
	}

	// Create scanner
	scanner := NewBinaryScanner(filesystem.NewOSFileSystem())

	// Execute scan
	binaries, err := scanner.ScanCachedBinaries(context.Background(), cacheDir, "mainnet", "stabled")

	// Verify: No error, empty result
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(binaries) != 0 {
		t.Errorf("Expected 0 binaries, got: %d", len(binaries))
	}
}

// TestBinaryScanner_ScanCachedBinaries_NonexistentCache tests scanning when cache doesn't exist
func TestBinaryScanner_ScanCachedBinaries_NonexistentCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "nonexistent", "cache")

	scanner := NewBinaryScanner(filesystem.NewOSFileSystem())
	binaries, err := scanner.ScanCachedBinaries(context.Background(), cacheDir, "mainnet", "stabled")

	// Verify: No error (EC-001: empty cache is not an error)
	if err != nil {
		t.Fatalf("Expected no error for nonexistent cache, got: %v", err)
	}
	if len(binaries) != 0 {
		t.Errorf("Expected 0 binaries, got: %d", len(binaries))
	}
}

// TestBinaryScanner_ScanCachedBinaries_SingleBinary tests scanning with one binary
func TestBinaryScanner_ScanCachedBinaries_SingleBinary(t *testing.T) {
	// Setup: Create cache with single binary
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "binaries")
	binaryPath := filepath.Join(cacheDir, "mainnet", "80ad31b-empty", "stabled")

	if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("fake binary"), 0755); err != nil {
		t.Fatalf("Failed to create binary: %v", err)
	}

	scanner := NewBinaryScanner(filesystem.NewOSFileSystem())
	binaries, err := scanner.ScanCachedBinaries(context.Background(), cacheDir, "mainnet", "stabled")

	// Verify: 1 binary found
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(binaries) != 1 {
		t.Fatalf("Expected 1 binary, got: %d", len(binaries))
	}

	// Verify metadata
	binary := binaries[0]
	if binary.Path != binaryPath {
		t.Errorf("Expected path %s, got: %s", binaryPath, binary.Path)
	}
	if binary.Name != "stabled" {
		t.Errorf("Expected name 'stabled', got: %s", binary.Name)
	}
	if binary.NetworkType != "mainnet" {
		t.Errorf("Expected network 'mainnet', got: %s", binary.NetworkType)
	}
	if binary.CommitHashShort != "80ad31b" {
		t.Errorf("Expected commit hash '80ad31b', got: %s", binary.CommitHashShort)
	}
	if binary.ConfigHash != "empty" {
		t.Errorf("Expected config hash 'empty', got: %s", binary.ConfigHash)
	}
	if binary.CacheKey != "mainnet/80ad31b-empty" {
		t.Errorf("Expected cache key 'mainnet/80ad31b-empty', got: %s", binary.CacheKey)
	}
	if binary.Size != 11 { // "fake binary" is 11 bytes
		t.Errorf("Expected size 11, got: %d", binary.Size)
	}
	if binary.SizeHuman != "11 B" {
		t.Errorf("Expected size '11 B', got: %s", binary.SizeHuman)
	}
	if binary.IsValid {
		t.Error("Expected IsValid to be false initially")
	}
}

// TestBinaryScanner_ScanCachedBinaries_MultipleBinaries tests scanning with multiple binaries
func TestBinaryScanner_ScanCachedBinaries_MultipleBinaries(t *testing.T) {
	// Setup: Create cache with 3 binaries at different times
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "binaries")

	// Binary 1: Oldest
	binary1 := filepath.Join(cacheDir, "mainnet", "80ad31b-empty", "stabled")
	if err := os.MkdirAll(filepath.Dir(binary1), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary1, []byte("binary1"), 0755); err != nil {
		t.Fatal(err)
	}
	time1 := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(binary1, time1, time1); err != nil {
		t.Fatal(err)
	}

	// Binary 2: Newest (most recent)
	binary2 := filepath.Join(cacheDir, "mainnet", "3b334fa-custom", "stabled")
	if err := os.MkdirAll(filepath.Dir(binary2), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary2, []byte("binary2"), 0755); err != nil {
		t.Fatal(err)
	}
	time2 := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(binary2, time2, time2); err != nil {
		t.Fatal(err)
	}

	// Binary 3: Middle
	binary3 := filepath.Join(cacheDir, "mainnet", "abcdef0-empty", "stabled")
	if err := os.MkdirAll(filepath.Dir(binary3), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary3, []byte("binary3"), 0755); err != nil {
		t.Fatal(err)
	}
	time3 := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(binary3, time3, time3); err != nil {
		t.Fatal(err)
	}

	scanner := NewBinaryScanner(filesystem.NewOSFileSystem())
	binaries, err := scanner.ScanCachedBinaries(context.Background(), cacheDir, "mainnet", "stabled")

	// Verify: 3 binaries found
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(binaries) != 3 {
		t.Fatalf("Expected 3 binaries, got: %d", len(binaries))
	}

	// Verify sorting: Most recent first (binary2, binary3, binary1)
	if binaries[0].CommitHashShort != "3b334fa" {
		t.Errorf("Expected first binary to be most recent (3b334fa), got: %s", binaries[0].CommitHashShort)
	}
	if binaries[1].CommitHashShort != "abcdef0" {
		t.Errorf("Expected second binary to be middle (abcdef0), got: %s", binaries[1].CommitHashShort)
	}
	if binaries[2].CommitHashShort != "80ad31b" {
		t.Errorf("Expected third binary to be oldest (80ad31b), got: %s", binaries[2].CommitHashShort)
	}
}

// TestBinaryScanner_ScanCachedBinaries_FilterByBinaryName tests filtering by binary name
func TestBinaryScanner_ScanCachedBinaries_FilterByBinaryName(t *testing.T) {
	// Setup: Create cache with different binary names
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "binaries")

	// Create stabled binary
	stabled := filepath.Join(cacheDir, "mainnet", "80ad31b-empty", "stabled")
	if err := os.MkdirAll(filepath.Dir(stabled), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stabled, []byte("stabled"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create aultd binary (should be filtered out)
	aultd := filepath.Join(cacheDir, "mainnet", "80ad31b-empty", "aultd")
	if err := os.WriteFile(aultd, []byte("aultd"), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewBinaryScanner(filesystem.NewOSFileSystem())
	binaries, err := scanner.ScanCachedBinaries(context.Background(), cacheDir, "mainnet", "stabled")

	// Verify: Only stabled found, aultd filtered out
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(binaries) != 1 {
		t.Fatalf("Expected 1 binary (stabled only), got: %d", len(binaries))
	}
	if binaries[0].Name != "stabled" {
		t.Errorf("Expected 'stabled', got: %s", binaries[0].Name)
	}
}

// TestBinaryScanner_ScanCachedBinaries_InvalidCacheKey tests handling of invalid cache key formats
func TestBinaryScanner_ScanCachedBinaries_InvalidCacheKey(t *testing.T) {
	// Setup: Create cache with invalid cache key formats
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "binaries")

	// Valid cache key
	validBinary := filepath.Join(cacheDir, "mainnet", "80ad31b-empty", "stabled")
	if err := os.MkdirAll(filepath.Dir(validBinary), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(validBinary, []byte("valid"), 0755); err != nil {
		t.Fatal(err)
	}

	// Invalid: No dash separator
	invalidBinary1 := filepath.Join(cacheDir, "mainnet", "invalid", "stabled")
	if err := os.MkdirAll(filepath.Dir(invalidBinary1), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidBinary1, []byte("invalid"), 0755); err != nil {
		t.Fatal(err)
	}

	// Invalid: Commit hash too short (less than 7 chars)
	invalidBinary2 := filepath.Join(cacheDir, "mainnet", "abc-empty", "stabled")
	if err := os.MkdirAll(filepath.Dir(invalidBinary2), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidBinary2, []byte("invalid"), 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewBinaryScanner(filesystem.NewOSFileSystem())
	binaries, err := scanner.ScanCachedBinaries(context.Background(), cacheDir, "mainnet", "stabled")

	// Verify: Only valid binary found, invalid ones skipped
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(binaries) != 1 {
		t.Fatalf("Expected 1 valid binary, got: %d", len(binaries))
	}
	if binaries[0].CommitHashShort != "80ad31b" {
		t.Errorf("Expected valid binary (80ad31b), got: %s", binaries[0].CommitHashShort)
	}
}

// TestBinaryScanner_ScanCachedBinaries_SkipsDirectories tests that directories are skipped
func TestBinaryScanner_ScanCachedBinaries_SkipsDirectories(t *testing.T) {
	// Setup: Create directory where binary should be
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "binaries")

	// Create directory instead of file (should be skipped)
	binaryDir := filepath.Join(cacheDir, "mainnet", "80ad31b-empty", "stabled")
	if err := os.MkdirAll(binaryDir, 0755); err != nil {
		t.Fatal(err)
	}

	scanner := NewBinaryScanner(filesystem.NewOSFileSystem())
	binaries, err := scanner.ScanCachedBinaries(context.Background(), cacheDir, "mainnet", "stabled")

	// Verify: Directory skipped, 0 binaries found
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(binaries) != 0 {
		t.Errorf("Expected 0 binaries (directory should be skipped), got: %d", len(binaries))
	}
}

// TestFormatBytes tests the formatBytes helper function
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"Zero bytes", 0, "0 B"},
		{"Small bytes", 512, "512 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1024 * 1024, "1.0 MB"},
		{"45.2 MB", 47394201, "45.2 MB"},
		{"1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"2.5 GB", 2684354560, "2.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %s; want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

// TestFormatRelativeTime tests the formatRelativeTime helper function
func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{"Just now", now.Add(-30 * time.Second), "just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1 min ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5 min ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1 hour ago"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"1 day ago", now.Add(-24 * time.Hour), "1 day ago"},
		{"5 days ago", now.Add(-5 * 24 * time.Hour), "5 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.time)
			if result != tt.expected {
				t.Errorf("formatRelativeTime() = %s; want %s", result, tt.expected)
			}
		})
	}
}
