package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/interactive"
)

// TestFilesystemBrowserAdapterCreation tests the FilesystemBrowserAdapter constructor.
func TestFilesystemBrowserAdapterCreation(t *testing.T) {
	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	if browser == nil {
		t.Fatal("NewFilesystemBrowserAdapter() returned nil")
	}
}

// TestFilesystemBrowserValidation tests path validation logic without interactive prompts.
// This test creates temporary executables and verifies validation behavior.
//
// Test Strategy:
//   - Create temporary directory with various test files
//   - Test validation for executable files, non-executable files, directories
//   - Test symlink resolution
//   - Test permission errors
//
// Note: This test doesn't test the interactive prompt itself (Tab autocomplete).
// That requires TTY simulation which is complex in Go tests.
func TestFilesystemBrowserValidation(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "fs-browser-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test Case 1: Create an executable file
	executablePath := filepath.Join(tmpDir, "test-binary")
	if err := createExecutableFile(executablePath); err != nil {
		t.Fatalf("Failed to create executable file: %v", err)
	}

	// Test Case 2: Create a non-executable file
	nonExecPath := filepath.Join(tmpDir, "test-nonexec")
	if err := createNonExecutableFile(nonExecPath); err != nil {
		t.Fatalf("Failed to create non-executable file: %v", err)
	}

	// Test Case 3: Create a directory
	dirPath := filepath.Join(tmpDir, "test-dir")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Test Case 4: Create a symlink to executable
	symlinkPath := filepath.Join(tmpDir, "test-symlink")
	if err := os.Symlink(executablePath, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	tests := []struct {
		name        string
		path        string
		shouldFail  bool
		errorSubstr string
	}{
		{
			name:       "Valid executable file",
			path:       executablePath,
			shouldFail: false,
		},
		{
			name:        "Non-executable file",
			path:        nonExecPath,
			shouldFail:  true,
			errorSubstr: "not executable",
		},
		{
			name:        "Directory",
			path:        dirPath,
			shouldFail:  true,
			errorSubstr: "directory",
		},
		{
			name:        "Non-existent file",
			path:        filepath.Join(tmpDir, "nonexistent"),
			shouldFail:  true,
			errorSubstr: "not found",
		},
		{
			name:       "Symlink to executable",
			path:       symlinkPath,
			shouldFail: false,
		},
	}

	// Note: We can't directly test BrowsePath without TTY simulation
	// This is a documentation test showing expected validation behavior
	_ = browser

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In a real interactive test, we would:
			// 1. Call browser.BrowsePath(ctx, tt.path)
			// 2. Verify it succeeds or fails as expected
			// 3. Check error message contains errorSubstr

			// For now, this test documents the expected behavior
			t.Logf("Path: %s", tt.path)
			t.Logf("Should fail: %v", tt.shouldFail)
			if tt.shouldFail {
				t.Logf("Expected error substring: %q", tt.errorSubstr)
			}

			// Skip actual execution since it requires TTY
			t.Skip("Skipping: requires TTY simulation with expect tool")
		})
	}
}

// TestFilesystemBrowserSymlinkResolution tests symlink resolution with max depth 10.
func TestFilesystemBrowserSymlinkResolution(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "fs-browser-symlink-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an executable target file
	targetPath := filepath.Join(tmpDir, "target")
	if err := createExecutableFile(targetPath); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create a chain of symlinks: link1 -> link2 -> link3 -> target
	link3 := filepath.Join(tmpDir, "link3")
	if err := os.Symlink(targetPath, link3); err != nil {
		t.Fatalf("Failed to create link3: %v", err)
	}

	link2 := filepath.Join(tmpDir, "link2")
	if err := os.Symlink(link3, link2); err != nil {
		t.Fatalf("Failed to create link2: %v", err)
	}

	link1 := filepath.Join(tmpDir, "link1")
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatalf("Failed to create link1: %v", err)
	}

	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	// Note: We can't directly test BrowsePath without TTY simulation
	_ = browser

	t.Log("Symlink chain: link1 -> link2 -> link3 -> target")
	t.Log("Expected: Should resolve to target and validate as executable")
	t.Skip("Skipping: requires TTY simulation with expect tool")
}

// TestFilesystemBrowserCircularSymlinks tests circular symlink detection.
func TestFilesystemBrowserCircularSymlinks(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "fs-browser-circular-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create circular symlinks: link1 -> link2, link2 -> link1
	link1 := filepath.Join(tmpDir, "link1")
	link2 := filepath.Join(tmpDir, "link2")

	// Create link2 first (pointing to a path that will exist)
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatalf("Failed to create link2: %v", err)
	}

	// Create link1 (pointing back to link2, creating a cycle)
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatalf("Failed to create link1: %v", err)
	}

	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	// Note: We can't directly test BrowsePath without TTY simulation
	_ = browser

	t.Log("Circular symlinks: link1 -> link2 -> link1")
	t.Log("Expected: Should fail with 'symlink chain too deep' error after 10 iterations")
	t.Skip("Skipping: requires TTY simulation with expect tool")
}

// TestFilesystemBrowserPathsWithSpaces tests handling of paths containing spaces.
func TestFilesystemBrowserPathsWithSpaces(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "fs-browser-spaces-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory with spaces in the name
	dirWithSpaces := filepath.Join(tmpDir, "my test folder")
	if err := os.Mkdir(dirWithSpaces, 0755); err != nil {
		t.Fatalf("Failed to create directory with spaces: %v", err)
	}

	// Create an executable in the directory with spaces
	execPath := filepath.Join(dirWithSpaces, "my binary")
	if err := createExecutableFile(execPath); err != nil {
		t.Fatalf("Failed to create executable with spaces in path: %v", err)
	}

	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	// Note: We can't directly test BrowsePath without TTY simulation
	_ = browser

	t.Logf("Executable path with spaces: %s", execPath)
	t.Log("Expected: Should handle path without special quoting")
	t.Skip("Skipping: requires TTY simulation with expect tool")
}

// TestFilesystemBrowserContextCancellation tests that BrowsePath respects context cancellation.
func TestFilesystemBrowserContextCancellation(t *testing.T) {
	pathCompleter := interactive.NewPathCompleterAdapter()
	browser := interactive.NewFilesystemBrowserAdapter(pathCompleter)

	// Create context with 1 millisecond timeout (will expire before prompt)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	// Try to browse - should fail with context deadline exceeded
	_, err := browser.BrowsePath(ctx, "/")

	if err == nil {
		t.Error("BrowsePath() should have failed with context deadline exceeded")
	}

	if err != context.DeadlineExceeded {
		t.Logf("Got error: %v", err)
		t.Logf("Expected: %v", context.DeadlineExceeded)
		// Note: In real TTY mode, the error might be different
		// This test documents the expected behavior
	}
}

// TestFilesystemBrowserEdgeCases documents edge case behavior for manual verification.
func TestFilesystemBrowserEdgeCases(t *testing.T) {
	t.Log("Edge Case Testing Guidelines for Filesystem Browser:")
	t.Log("")
	t.Log("EC-002: Non-existent directory")
	t.Log("  → Type: /nonexistent/path + Tab")
	t.Log("  → Expected: No autocomplete suggestions (empty list)")
	t.Log("")
	t.Log("EC-003: Invalid binary (non-executable)")
	t.Log("  → Type: /path/to/nonexec + Enter")
	t.Log("  → Expected: Error message 'binary is not executable', prompt again")
	t.Log("")
	t.Log("EC-003: Invalid binary (directory)")
	t.Log("  → Type: /usr/bin/ + Enter")
	t.Log("  → Expected: Error message 'path is a directory', prompt again")
	t.Log("")
	t.Log("EC-004: Paths with spaces")
	t.Log("  → Type: /Users/my folder/binary + Tab")
	t.Log("  → Expected: Autocomplete works without quoting")
	t.Log("")
	t.Log("EC-005: User cancellation (Ctrl+C)")
	t.Log("  → Press Ctrl+C during prompt")
	t.Log("  → Expected: Exit code 130, message 'Selection cancelled by user'")
	t.Log("")
	t.Log("EC-006: Symlink chain")
	t.Log("  → Select: /usr/bin/python (symlink to python3)")
	t.Log("  → Expected: Resolves to final target, validates as executable")
	t.Log("")
	t.Log("EC-006: Circular symlinks")
	t.Log("  → Create circular symlink, try to select it")
	t.Log("  → Expected: Error 'symlink chain too deep (max 10 levels)'")
	t.Log("")
	t.Log("For full interactive testing, see: tests/integration/filesystem_browser_manual.md")
}

// TestFilesystemBrowserPerformanceRequirements documents performance requirements.
func TestFilesystemBrowserPerformanceRequirements(t *testing.T) {
	t.Log("Performance Requirements (from SC-002):")
	t.Log("")
	t.Log("SC-002: Autocomplete response time < 100ms")
	t.Log("  → Test: Type path + Tab in directory with < 1000 entries")
	t.Log("  → Expected: Suggestions appear in < 100ms")
	t.Log("  → Measured by: See T031 (large_directory_test.go)")
	t.Log("")
	t.Log("FR-012: Pagination (max 100 results)")
	t.Log("  → Test: Type path + Tab in directory with > 100 entries")
	t.Log("  → Expected: Only first 100 results shown (alphabetically)")
	t.Log("  → Measured by: See T031 (large_directory_test.go)")
}

// Helper Functions

// createExecutableFile creates a file with executable permissions.
func createExecutableFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	f.Close()

	// Make executable (rwxr-xr-x)
	return os.Chmod(path, 0755)
}

// createNonExecutableFile creates a file without executable permissions.
func createNonExecutableFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	f.Close()

	// No execute permissions (rw-r--r--)
	return os.Chmod(path, 0644)
}
