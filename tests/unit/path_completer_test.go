package unit

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
)

// TestPathCompleterAdapterCreation tests the NewPathCompleterAdapter constructor.
func TestPathCompleterAdapterCreation(t *testing.T) {
	completer := interactive.NewPathCompleterAdapter()

	if completer == nil {
		t.Fatal("NewPathCompleterAdapter() returned nil")
	}
}

// TestPathCompleterEmptyInput tests autocomplete behavior with empty input.
// Expected: Should return root-level directories.
func TestPathCompleterEmptyInput(t *testing.T) {
	completer := interactive.NewPathCompleterAdapter()

	results := completer.Complete("")

	// Should return at least some root directories
	if len(results) == 0 {
		t.Error("Complete(\"\") returned no results, expected root directories")
	}

	// All results should be absolute paths starting with "/"
	for _, result := range results {
		if !filepath.IsAbs(result) {
			t.Errorf("Complete(\"\") returned non-absolute path: %s", result)
		}
	}

	// Directories should end with "/"
	// Files at root level (like /swapfile on Linux) are allowed but won't have trailing "/"
	hasDirectory := false
	for _, result := range results {
		if strings.HasSuffix(result, "/") {
			hasDirectory = true
			break
		}
	}

	if !hasDirectory {
		t.Error("Complete(\"\") returned no directories, expected at least one root directory")
	}
}

// TestPathCompleterRootInput tests autocomplete behavior with "/" input.
// Expected: Should return root-level directories (same as empty input).
func TestPathCompleterRootInput(t *testing.T) {
	completer := interactive.NewPathCompleterAdapter()

	results := completer.Complete("/")

	// Should return at least some root directories
	if len(results) == 0 {
		t.Error("Complete(\"/\") returned no results, expected root directories")
	}

	// All results should start with "/"
	for _, result := range results {
		if !strings.HasPrefix(result, "/") {
			t.Errorf("Complete(\"/\") returned path not starting with /: %s", result)
		}
	}
}

// TestPathCompleterDirectoryListing tests autocomplete for a known directory.
// Uses a temporary directory to avoid platform-specific dependencies.
func TestPathCompleterDirectoryListing(t *testing.T) {
	// Create a temporary directory with test files and subdirectories
	tmpDir, err := os.MkdirTemp("", "path-completer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test structure:
	// tmpDir/
	//   ├── alice/
	//   ├── bob/
	//   ├── charlie/
	//   ├── file1.txt
	//   └── file2.txt
	testDirs := []string{"alice", "bob", "charlie"}
	for _, dir := range testDirs {
		if err := os.Mkdir(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	testFiles := []string{"file1.txt", "file2.txt"}
	for _, file := range testFiles {
		f, err := os.Create(filepath.Join(tmpDir, file))
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
		f.Close()
	}

	completer := interactive.NewPathCompleterAdapter()

	// Test listing the directory
	input := tmpDir + "/"
	results := completer.Complete(input)

	// Should return 5 entries (3 dirs + 2 files)
	if len(results) != 5 {
		t.Errorf("Complete(%q) returned %d results, expected 5", input, len(results))
	}

	// Verify directories have trailing "/"
	expectedDirs := []string{
		filepath.Join(tmpDir, "alice") + "/",
		filepath.Join(tmpDir, "bob") + "/",
		filepath.Join(tmpDir, "charlie") + "/",
	}
	for _, expectedDir := range expectedDirs {
		found := false
		for _, result := range results {
			if result == expectedDir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Complete(%q) missing expected directory: %s", input, expectedDir)
		}
	}

	// Verify files do NOT have trailing "/"
	expectedFiles := []string{
		filepath.Join(tmpDir, "file1.txt"),
		filepath.Join(tmpDir, "file2.txt"),
	}
	for _, expectedFile := range expectedFiles {
		found := false
		for _, result := range results {
			if result == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Complete(%q) missing expected file: %s", input, expectedFile)
		}
	}
}

// TestPathCompleterPrefixFiltering tests that autocomplete filters by prefix correctly.
func TestPathCompleterPrefixFiltering(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "path-completer-prefix-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test structure:
	// tmpDir/
	//   ├── alice/
	//   ├── bob/
	//   ├── anna/
	//   └── charlie/
	testDirs := []string{"alice", "bob", "anna", "charlie"}
	for _, dir := range testDirs {
		if err := os.Mkdir(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	completer := interactive.NewPathCompleterAdapter()

	// Test filtering by prefix "a"
	input := filepath.Join(tmpDir, "a")
	results := completer.Complete(input)

	// Should return only alice/ and anna/ (2 entries)
	if len(results) != 2 {
		t.Errorf("Complete(%q) returned %d results, expected 2", input, len(results))
	}

	// Verify only "alice" and "anna" are returned
	expectedMatches := []string{"alice", "anna"}
	for _, result := range results {
		base := filepath.Base(strings.TrimSuffix(result, "/"))
		found := false
		for _, expected := range expectedMatches {
			if base == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Complete(%q) returned unexpected result: %s", input, result)
		}
	}
}

// TestPathCompleterCaseInsensitiveFiltering tests case-insensitive prefix matching.
func TestPathCompleterCaseInsensitiveFiltering(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "path-completer-case-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test structure:
	// tmpDir/
	//   ├── Alice/
	//   ├── ANNA/
	//   └── bob/
	testDirs := []string{"Alice", "ANNA", "bob"}
	for _, dir := range testDirs {
		if err := os.Mkdir(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	completer := interactive.NewPathCompleterAdapter()

	// Test filtering with lowercase "a" should match "Alice" and "ANNA"
	input := filepath.Join(tmpDir, "a")
	results := completer.Complete(input)

	// Should return Alice/ and ANNA/ (2 entries)
	if len(results) != 2 {
		t.Errorf("Complete(%q) returned %d results, expected 2", input, len(results))
		for _, r := range results {
			t.Logf("  - %s", r)
		}
	}
}

// TestPathCompleterAlphabeticalSorting tests that results are sorted alphabetically.
func TestPathCompleterAlphabeticalSorting(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "path-completer-sort-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test structure in non-alphabetical order:
	// tmpDir/
	//   ├── zebra/
	//   ├── apple/
	//   ├── charlie/
	//   └── bob/
	testDirs := []string{"zebra", "apple", "charlie", "bob"}
	for _, dir := range testDirs {
		if err := os.Mkdir(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	completer := interactive.NewPathCompleterAdapter()

	input := tmpDir + "/"
	results := completer.Complete(input)

	// Should return 4 entries in alphabetical order
	if len(results) != 4 {
		t.Errorf("Complete(%q) returned %d results, expected 4", input, len(results))
	}

	// Create expected sorted order
	expected := []string{
		filepath.Join(tmpDir, "apple") + "/",
		filepath.Join(tmpDir, "bob") + "/",
		filepath.Join(tmpDir, "charlie") + "/",
		filepath.Join(tmpDir, "zebra") + "/",
	}

	// Verify results are in alphabetical order
	for i, result := range results {
		if result != expected[i] {
			t.Errorf("Complete(%q) result[%d] = %s, expected %s", input, i, result, expected[i])
		}
	}
}

// TestPathCompleterPaginationLimit tests that results are limited to 100 entries.
func TestPathCompleterPaginationLimit(t *testing.T) {
	// Create a temporary directory with 150 test files
	tmpDir, err := os.MkdirTemp("", "path-completer-limit-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 150 directories: dir-000, dir-001, ..., dir-149
	for i := 0; i < 150; i++ {
		dirName := filepath.Join(tmpDir, "dir-"+padInt(i, 3))
		if err := os.Mkdir(dirName, 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dirName, err)
		}
	}

	completer := interactive.NewPathCompleterAdapter()

	input := tmpDir + "/"
	results := completer.Complete(input)

	// Should return exactly 100 entries (FR-012: SC-002)
	if len(results) != 100 {
		t.Errorf("Complete(%q) returned %d results, expected 100 (pagination limit)", input, len(results))
	}

	// Verify results are the first 100 alphabetically
	// Since we named them dir-000 to dir-149, first 100 should be dir-000 to dir-099
	for i := 0; i < 100; i++ {
		expectedDir := filepath.Join(tmpDir, "dir-"+padInt(i, 3)) + "/"
		if results[i] != expectedDir {
			t.Errorf("Complete(%q) result[%d] = %s, expected %s", input, i, results[i], expectedDir)
		}
	}
}

// TestPathCompleterNonExistentDirectory tests behavior with non-existent directory.
// Expected: Should return empty slice (silent failure, EC-002).
func TestPathCompleterNonExistentDirectory(t *testing.T) {
	completer := interactive.NewPathCompleterAdapter()

	// Use a path that definitely doesn't exist
	input := "/nonexistent-directory-12345/"
	results := completer.Complete(input)

	// Should return empty slice (silent failure)
	if len(results) != 0 {
		t.Errorf("Complete(%q) returned %d results, expected 0 (non-existent directory)", input, len(results))
	}
}

// TestPathCompleterHiddenFiles tests that hidden files are excluded by default.
func TestPathCompleterHiddenFiles(t *testing.T) {
	// Create a temporary directory with hidden and visible files
	tmpDir, err := os.MkdirTemp("", "path-completer-hidden-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test structure:
	// tmpDir/
	//   ├── .hidden/
	//   ├── .hiddenfile
	//   ├── visible/
	//   └── visiblefile
	testItems := []struct {
		name  string
		isDir bool
	}{
		{".hidden", true},
		{".hiddenfile", false},
		{"visible", true},
		{"visiblefile", false},
	}

	for _, item := range testItems {
		path := filepath.Join(tmpDir, item.name)
		if item.isDir {
			if err := os.Mkdir(path, 0755); err != nil {
				t.Fatalf("Failed to create test directory %s: %v", item.name, err)
			}
		} else {
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("Failed to create test file %s: %v", item.name, err)
			}
			f.Close()
		}
	}

	completer := interactive.NewPathCompleterAdapter()

	// Test listing without "." prefix - should only show visible items
	input := tmpDir + "/"
	results := completer.Complete(input)

	// Should return only 2 visible items (visible/ and visiblefile)
	if len(results) != 2 {
		t.Errorf("Complete(%q) returned %d results, expected 2 (hidden files excluded)", input, len(results))
		for _, r := range results {
			t.Logf("  - %s", r)
		}
	}

	// Verify hidden items are NOT in results
	for _, result := range results {
		base := filepath.Base(result)
		if strings.HasPrefix(base, ".") {
			t.Errorf("Complete(%q) returned hidden item: %s", input, result)
		}
	}

	// Test with explicit "." prefix - should show hidden items
	inputWithDot := tmpDir + "/."
	resultsWithDot := completer.Complete(inputWithDot)

	// Should return hidden items when user explicitly types "."
	if len(resultsWithDot) < 2 {
		t.Errorf("Complete(%q) returned %d results, expected at least 2 (hidden files when prefix is .)", inputWithDot, len(resultsWithDot))
		for _, r := range resultsWithDot {
			t.Logf("  - %s", r)
		}
	}
}

// TestPathCompleterSortingConsistency tests that sorting is consistent across multiple calls.
func TestPathCompleterSortingConsistency(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "path-completer-consistency-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test directories
	testDirs := []string{"delta", "alpha", "charlie", "bravo"}
	for _, dir := range testDirs {
		if err := os.Mkdir(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	completer := interactive.NewPathCompleterAdapter()
	input := tmpDir + "/"

	// Call Complete multiple times
	results1 := completer.Complete(input)
	results2 := completer.Complete(input)
	results3 := completer.Complete(input)

	// All results should be identical
	if !slicesEqual(results1, results2) {
		t.Error("Complete() returned different results on second call (inconsistent sorting)")
	}
	if !slicesEqual(results1, results3) {
		t.Error("Complete() returned different results on third call (inconsistent sorting)")
	}

	// Verify results are sorted
	if !sort.StringsAreSorted(results1) {
		t.Error("Complete() returned unsorted results")
	}
}

// TestPathCompleterDirectoryTrailingSlash verifies that directories always have trailing "/".
func TestPathCompleterDirectoryTrailingSlash(t *testing.T) {
	// Create a temporary directory with test directories
	tmpDir, err := os.MkdirTemp("", "path-completer-slash-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create only directories
	testDirs := []string{"dir1", "dir2", "dir3"}
	for _, dir := range testDirs {
		if err := os.Mkdir(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	completer := interactive.NewPathCompleterAdapter()
	input := tmpDir + "/"
	results := completer.Complete(input)

	// All results should have trailing "/"
	for _, result := range results {
		if !strings.HasSuffix(result, "/") {
			t.Errorf("Complete(%q) returned directory without trailing slash: %s", input, result)
		}
	}
}

// TestPathCompleterFileNoTrailingSlash verifies that files do NOT have trailing "/".
func TestPathCompleterFileNoTrailingSlash(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "path-completer-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create only files
	testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, file := range testFiles {
		f, err := os.Create(filepath.Join(tmpDir, file))
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
		f.Close()
	}

	completer := interactive.NewPathCompleterAdapter()
	input := tmpDir + "/"
	results := completer.Complete(input)

	// No results should have trailing "/"
	for _, result := range results {
		if strings.HasSuffix(result, "/") {
			t.Errorf("Complete(%q) returned file with trailing slash: %s", input, result)
		}
	}
}

// Helper Functions

// padInt pads an integer with leading zeros to the specified width.
// Example: padInt(42, 3) → "042"
func padInt(n, width int) string {
	s := ""
	for i := 0; i < width; i++ {
		s = "0" + s
	}
	numStr := ""
	for n > 0 {
		numStr = string(rune('0'+n%10)) + numStr
		n /= 10
	}
	if len(numStr) == 0 {
		numStr = "0"
	}
	if len(numStr) >= width {
		return numStr
	}
	return s[:width-len(numStr)] + numStr
}

// slicesEqual checks if two string slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
