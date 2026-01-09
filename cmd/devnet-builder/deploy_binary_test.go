package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBinaryPath_ValidExecutable(t *testing.T) {
	// Create a temporary executable file
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")

	// Create file and make it executable
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Test with absolute path
	result, err := validateBinaryPath(binaryPath)
	if err != nil {
		t.Errorf("validateBinaryPath() failed with valid executable: %v", err)
	}
	if !filepath.IsAbs(result) {
		t.Errorf("validateBinaryPath() should return absolute path, got: %s", result)
	}

	// Test with relative path (should convert to absolute)
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	relPath := "./test-binary"
	result, err = validateBinaryPath(relPath)
	if err != nil {
		t.Errorf("validateBinaryPath() failed with relative path: %v", err)
	}
	if !filepath.IsAbs(result) {
		t.Errorf("validateBinaryPath() should convert relative path to absolute, got: %s", result)
	}
}

func TestValidateBinaryPath_NotFound(t *testing.T) {
	nonExistentPath := "/nonexistent/path/to/binary"

	_, err := validateBinaryPath(nonExistentPath)
	if err == nil {
		t.Error("validateBinaryPath() should fail with non-existent path")
	}
	if err != nil && !os.IsNotExist(err) {
		// Error message should indicate file not found
		expectedMsg := "binary not found at path"
		if err.Error()[:len(expectedMsg)] != expectedMsg {
			t.Errorf("Expected error message to start with '%s', got: %v", expectedMsg, err)
		}
	}
}

func TestValidateBinaryPath_NotExecutable(t *testing.T) {
	// Create a temporary file without execute permissions
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")

	// Create file with read-only permissions (0644)
	if err := os.WriteFile(binaryPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := validateBinaryPath(binaryPath)
	if err == nil {
		t.Error("validateBinaryPath() should fail with non-executable file")
	}
	if err != nil {
		expectedMsg := "binary is not executable"
		if err.Error()[:len(expectedMsg)] != expectedMsg {
			t.Errorf("Expected error message to start with '%s', got: %v", expectedMsg, err)
		}
		// Should suggest chmod +x
		if err.Error()[len(err.Error())-9:] != "chmod +x)" {
			t.Errorf("Error message should suggest using chmod +x, got: %v", err)
		}
	}
}

func TestValidateBinaryPath_IsDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	_, err := validateBinaryPath(tmpDir)
	if err == nil {
		t.Error("validateBinaryPath() should fail when path is a directory")
	}
	if err != nil {
		expectedMsg := "path is a directory"
		if err.Error()[:len(expectedMsg)] != expectedMsg {
			t.Errorf("Expected error message to start with '%s', got: %v", expectedMsg, err)
		}
	}
}

func TestValidateBinaryPath_WithSymlink(t *testing.T) {
	// Create a temporary executable file
	tmpDir := t.TempDir()
	realBinary := filepath.Join(tmpDir, "real-binary")
	symlinkPath := filepath.Join(tmpDir, "symlink-binary")

	// Create real file and make it executable
	if err := os.WriteFile(realBinary, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Create symlink
	if err := os.Symlink(realBinary, symlinkPath); err != nil {
		t.Skipf("Symlink creation not supported on this system: %v", err)
	}

	// Test with symlink - should follow symlink and validate the real file
	result, err := validateBinaryPath(symlinkPath)
	if err != nil {
		t.Errorf("validateBinaryPath() should work with symlinks: %v", err)
	}

	// Result should be absolute path (symlink or target, both are valid)
	if !filepath.IsAbs(result) {
		t.Errorf("validateBinaryPath() should return absolute path for symlink, got: %s", result)
	}
}

func TestValidateBinaryPath_PathWithSpaces(t *testing.T) {
	// Create a temporary executable file with spaces in path
	tmpDir := t.TempDir()
	dirWithSpaces := filepath.Join(tmpDir, "dir with spaces")
	if err := os.Mkdir(dirWithSpaces, 0755); err != nil {
		t.Fatalf("Failed to create directory with spaces: %v", err)
	}

	binaryPath := filepath.Join(dirWithSpaces, "test binary")

	// Create file and make it executable
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	result, err := validateBinaryPath(binaryPath)
	if err != nil {
		t.Errorf("validateBinaryPath() should handle paths with spaces: %v", err)
	}
	if !filepath.IsAbs(result) {
		t.Errorf("validateBinaryPath() should return absolute path, got: %s", result)
	}
}
