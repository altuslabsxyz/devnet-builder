package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// getBinaryPath returns the path to the devnet-builder binary for testing
func getBinaryPath(t *testing.T) string {
	// Binary should be built at the module root
	binaryPath := filepath.Join("..", "..", "devnet-builder")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("devnet-builder binary not found at %s. Run 'go build ./cmd/devnet-builder' first", binaryPath)
	}
	absPath, _ := filepath.Abs(binaryPath)
	return absPath
}

// TestDeployBinary_ValidLocalMode tests deploying with a valid custom binary in local mode
func TestDeployBinary_ValidLocalMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a mock executable binary
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\necho 'mock binary'\n"), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	// Run deploy command with --binary flag
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	binary := getBinaryPath(t)
	cmd := exec.CommandContext(ctx, binary, "deploy",
		"--mode", "local",
		"--binary", mockBinary,
		"--no-interactive",
		"--export-version", "v1.0.0",
		"--start-version", "v1.0.0")

	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	// Check that binary validation succeeded (before any deployment failures)
	// The validation happens early, so we should see the log message
	if !strings.Contains(outputStr, "Using custom binary") && !strings.Contains(outputStr, mockBinary) {
		t.Errorf("Expected binary path in output, got: %s", outputStr)
	}

	// Should not have binary-related errors
	if strings.Contains(outputStr, "binary not found") {
		t.Errorf("Should not have 'binary not found' error: %s", outputStr)
	}
	if strings.Contains(outputStr, "binary is not executable") {
		t.Errorf("Should not have 'binary is not executable' error: %s", outputStr)
	}
}

// TestDeployBinary_DockerModeError tests that --binary flag errors with docker mode
func TestDeployBinary_DockerModeError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a mock executable binary
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	// Run deploy command with --binary and --mode docker (should error)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	binary := getBinaryPath(t)
	cmd := exec.CommandContext(ctx, binary, "deploy",
		"--mode", "docker",
		"--binary", mockBinary,
		"--no-interactive",
		"--image", "v1.0.0")

	output, err := cmd.CombinedOutput()

	// Should fail with mode compatibility error
	if err == nil {
		t.Error("Expected command to fail with docker mode + binary flag")
	}

	outputStr := string(output)
	expectedMsg := "--binary flag is only supported in local mode"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, outputStr)
	}
}

// TestDeployBinary_NonExistentPath tests error handling for non-existent binary path
func TestDeployBinary_NonExistentPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	nonExistentPath := "/nonexistent/path/to/stabled"

	// Run deploy command with non-existent binary path
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	binary := getBinaryPath(t)
	cmd := exec.CommandContext(ctx, binary, "deploy",
		"--mode", "local",
		"--binary", nonExistentPath,
		"--no-interactive",
		"--export-version", "v1.0.0",
		"--start-version", "v1.0.0")

	output, err := cmd.CombinedOutput()

	// Should fail with file not found error
	if err == nil {
		t.Error("Expected command to fail with non-existent binary path")
	}

	outputStr := string(output)
	expectedMsg := "binary not found at path"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, outputStr)
	}
}

// TestDeployBinary_NotExecutable tests error handling for non-executable file
func TestDeployBinary_NotExecutable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a non-executable file
	tmpDir := t.TempDir()
	nonExecFile := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(nonExecFile, []byte("not executable"), 0644); err != nil {
		t.Fatalf("Failed to create non-executable file: %v", err)
	}

	// Run deploy command with non-executable file
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	binary := getBinaryPath(t)
	cmd := exec.CommandContext(ctx, binary, "deploy",
		"--mode", "local",
		"--binary", nonExecFile,
		"--no-interactive",
		"--export-version", "v1.0.0",
		"--start-version", "v1.0.0")

	output, err := cmd.CombinedOutput()

	// Should fail with not executable error
	if err == nil {
		t.Error("Expected command to fail with non-executable file")
	}

	outputStr := string(output)
	expectedMsg := "binary is not executable"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, outputStr)
	}

	// Should suggest chmod +x
	if !strings.Contains(outputStr, "chmod +x") {
		t.Errorf("Expected error to suggest 'chmod +x', got: %s", outputStr)
	}
}

// TestDeployBinary_IsDirectory tests error handling when path is a directory
func TestDeployBinary_IsDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Use temp directory as the binary path
	tmpDir := t.TempDir()

	// Run deploy command with directory path
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	binary := getBinaryPath(t)
	cmd := exec.CommandContext(ctx, binary, "deploy",
		"--mode", "local",
		"--binary", tmpDir,
		"--no-interactive",
		"--export-version", "v1.0.0",
		"--start-version", "v1.0.0")

	output, err := cmd.CombinedOutput()

	// Should fail with directory error
	if err == nil {
		t.Error("Expected command to fail when binary path is a directory")
	}

	outputStr := string(output)
	expectedMsg := "path is a directory"
	if !strings.Contains(outputStr, expectedMsg) {
		t.Errorf("Expected error message '%s', got: %s", expectedMsg, outputStr)
	}
}

// TestDeployBinary_RelativePath tests that relative paths are converted to absolute
func TestDeployBinary_RelativePath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a mock executable binary in temp dir
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\necho 'mock'\n"), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	// Get binary path before changing directory
	binary := getBinaryPath(t)

	// Change to temp directory and use relative path
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)
	os.Chdir(tmpDir)

	// Run deploy command with relative path
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "deploy",
		"--mode", "local",
		"--binary", "./stabled",
		"--no-interactive",
		"--export-version", "v1.0.0",
		"--start-version", "v1.0.0")

	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	// Should successfully validate and convert to absolute path
	if !strings.Contains(outputStr, "Using custom binary") {
		t.Errorf("Expected 'Using custom binary' in output, got: %s", outputStr)
	}

	// The logged path should be absolute
	if strings.Contains(outputStr, "./stabled") && !strings.Contains(outputStr, tmpDir) {
		t.Errorf("Expected absolute path in output, got relative path: %s", outputStr)
	}
}
