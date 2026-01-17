package manage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBinaryPath_ValidExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")

	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	result, err := validateBinaryPath(binaryPath)
	if err != nil {
		t.Errorf("validateBinaryPath() failed with valid executable: %v", err)
	}
	if !filepath.IsAbs(result) {
		t.Errorf("validateBinaryPath() should return absolute path, got: %s", result)
	}

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
		expectedMsg := "binary not found at path"
		if len(err.Error()) >= len(expectedMsg) && err.Error()[:len(expectedMsg)] != expectedMsg {
			t.Errorf("Expected error message to start with '%s', got: %v", expectedMsg, err)
		}
	}
}

func TestValidateBinaryPath_NotExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")

	if err := os.WriteFile(binaryPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := validateBinaryPath(binaryPath)
	if err == nil {
		t.Error("validateBinaryPath() should fail with non-executable file")
	}
	if err != nil {
		expectedMsg := "binary is not executable"
		if len(err.Error()) >= len(expectedMsg) && err.Error()[:len(expectedMsg)] != expectedMsg {
			t.Errorf("Expected error message to start with '%s', got: %v", expectedMsg, err)
		}
	}
}

func TestValidateBinaryPath_IsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := validateBinaryPath(tmpDir)
	if err == nil {
		t.Error("validateBinaryPath() should fail when path is a directory")
	}
	if err != nil {
		expectedMsg := "path is a directory"
		if len(err.Error()) >= len(expectedMsg) && err.Error()[:len(expectedMsg)] != expectedMsg {
			t.Errorf("Expected error message to start with '%s', got: %v", expectedMsg, err)
		}
	}
}

func TestValidateBinaryPath_WithSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	realBinary := filepath.Join(tmpDir, "real-binary")
	symlinkPath := filepath.Join(tmpDir, "symlink-binary")

	if err := os.WriteFile(realBinary, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	if err := os.Symlink(realBinary, symlinkPath); err != nil {
		t.Skipf("Symlink creation not supported on this system: %v", err)
	}

	result, err := validateBinaryPath(symlinkPath)
	if err != nil {
		t.Errorf("validateBinaryPath() should work with symlinks: %v", err)
	}

	if !filepath.IsAbs(result) {
		t.Errorf("validateBinaryPath() should return absolute path for symlink, got: %s", result)
	}
}

func TestValidateBinaryPath_PathWithSpaces(t *testing.T) {
	tmpDir := t.TempDir()
	dirWithSpaces := filepath.Join(tmpDir, "dir with spaces")
	if err := os.Mkdir(dirWithSpaces, 0755); err != nil {
		t.Fatalf("Failed to create directory with spaces: %v", err)
	}

	binaryPath := filepath.Join(dirWithSpaces, "test binary")

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
