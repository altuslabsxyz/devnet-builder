package binary

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDetectVersion_Success tests successful version detection from a mock binary.
func TestDetectVersion_Success(t *testing.T) {
	// Create a mock binary that outputs standard Cosmos SDK format
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Create a shell script that outputs version info
	scriptContent := `#!/bin/sh
echo "version: v1.2.3"
echo "commit: 80ad31b1234567890abcdef1234567890abcdef"
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	// Create detector and test
	detector := NewBinaryVersionDetector()
	ctx := context.Background()

	result, err := detector.DetectVersion(ctx, mockBinary)

	// Verify no error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify version
	if result.Version != "v1.2.3" {
		t.Errorf("Expected version 'v1.2.3', got: %s", result.Version)
	}

	// Verify commit hash
	expectedCommit := "80ad31b1234567890abcdef1234567890abcdef"
	if result.CommitHash != expectedCommit {
		t.Errorf("Expected commit '%s', got: %s", expectedCommit, result.CommitHash)
	}

	// Verify GitCommit alias
	if result.GitCommit != result.CommitHash {
		t.Errorf("GitCommit should equal CommitHash, got: %s != %s", result.GitCommit, result.CommitHash)
	}
}

// TestDetectVersion_VersionWithoutPrefix tests version without 'v' prefix (should be normalized).
func TestDetectVersion_VersionWithoutPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Version without 'v' prefix
	scriptContent := `#!/bin/sh
echo "version: 1.2.3"
echo "commit: 80ad31b1234567890abcdef1234567890abcdef"
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	detector := NewBinaryVersionDetector()
	result, err := detector.DetectVersion(context.Background(), mockBinary)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should normalize to include 'v' prefix
	if result.Version != "v1.2.3" {
		t.Errorf("Expected normalized version 'v1.2.3', got: %s", result.Version)
	}
}

// TestDetectVersion_CaseInsensitive tests case-insensitive parsing.
func TestDetectVersion_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Mixed case labels
	scriptContent := `#!/bin/sh
echo "Version: v1.2.3"
echo "Commit: 80ad31b1234567890abcdef1234567890abcdef"
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	detector := NewBinaryVersionDetector()
	result, err := detector.DetectVersion(context.Background(), mockBinary)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Version != "v1.2.3" {
		t.Errorf("Expected version 'v1.2.3', got: %s", result.Version)
	}

	if result.CommitHash != "80ad31b1234567890abcdef1234567890abcdef" {
		t.Errorf("Commit hash mismatch")
	}
}

// TestDetectVersion_ExtraWhitespace tests handling of extra whitespace.
func TestDetectVersion_ExtraWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Extra spaces around values
	scriptContent := `#!/bin/sh
echo "version:    v1.2.3   "
echo "commit:    80ad31b1234567890abcdef1234567890abcdef   "
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	detector := NewBinaryVersionDetector()
	result, err := detector.DetectVersion(context.Background(), mockBinary)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Version != "v1.2.3" {
		t.Errorf("Expected version 'v1.2.3', got: %s", result.Version)
	}
}

// TestDetectVersion_BinaryNotFound tests error when binary doesn't exist.
func TestDetectVersion_BinaryNotFound(t *testing.T) {
	detector := NewBinaryVersionDetector()
	_, err := detector.DetectVersion(context.Background(), "/nonexistent/binary")

	if err == nil {
		t.Fatal("Expected error for non-existent binary")
	}

	if !strings.Contains(err.Error(), "failed to execute binary version command") {
		t.Errorf("Expected execution error, got: %v", err)
	}
}

// TestDetectVersion_MissingVersion tests error when version is not in output.
func TestDetectVersion_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Output without version
	scriptContent := `#!/bin/sh
echo "commit: 80ad31b1234567890abcdef1234567890abcdef"
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	detector := NewBinaryVersionDetector()
	_, err := detector.DetectVersion(context.Background(), mockBinary)

	if err == nil {
		t.Fatal("Expected error for missing version")
	}

	if !strings.Contains(err.Error(), "failed to parse version from output") {
		t.Errorf("Expected version parse error, got: %v", err)
	}
}

// TestDetectVersion_MissingCommit tests error when commit hash is not in output.
func TestDetectVersion_MissingCommit(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Output without commit
	scriptContent := `#!/bin/sh
echo "version: v1.2.3"
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	detector := NewBinaryVersionDetector()
	_, err := detector.DetectVersion(context.Background(), mockBinary)

	if err == nil {
		t.Fatal("Expected error for missing commit")
	}

	if !strings.Contains(err.Error(), "failed to parse commit hash from output") {
		t.Errorf("Expected commit parse error, got: %v", err)
	}
}

// TestDetectVersion_InvalidCommitHash tests error when commit hash is invalid.
func TestDetectVersion_InvalidCommitHash(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Commit hash too short
	scriptContent := `#!/bin/sh
echo "version: v1.2.3"
echo "commit: 80ad31b123"
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	detector := NewBinaryVersionDetector()
	_, err := detector.DetectVersion(context.Background(), mockBinary)

	if err == nil {
		t.Fatal("Expected error for invalid commit hash")
	}

	if !strings.Contains(err.Error(), "commit hash not found") {
		t.Errorf("Expected commit validation error, got: %v", err)
	}
}

// TestDetectVersion_Timeout tests that the detector respects context timeout.
func TestDetectVersion_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "testbin")

	// Binary that sleeps forever
	scriptContent := `#!/bin/sh
sleep 100
`
	if err := os.WriteFile(mockBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create mock binary: %v", err)
	}

	// Create detector with very short timeout
	detector := NewBinaryVersionDetectorWithTimeout(100 * time.Millisecond)

	_, err := detector.DetectVersion(context.Background(), mockBinary)

	if err == nil {
		t.Fatal("Expected timeout error")
	}

	// Should contain context deadline or signal killed message
	errStr := err.Error()
	if !strings.Contains(errStr, "context deadline exceeded") && !strings.Contains(errStr, "signal: killed") {
		t.Errorf("Expected timeout-related error, got: %v", err)
	}
}

// TestParseVersion tests the version parsing logic directly.
func TestParseVersion(t *testing.T) {
	detector := &versionDetector{}

	tests := []struct {
		name     string
		output   string
		expected string
		wantErr  bool
	}{
		{
			name:     "Standard format with v prefix",
			output:   "version: v1.2.3\ncommit: abc123",
			expected: "v1.2.3",
			wantErr:  false,
		},
		{
			name:     "Without v prefix",
			output:   "version: 1.2.3\ncommit: abc123",
			expected: "v1.2.3",
			wantErr:  false,
		},
		{
			name:     "With pre-release tag",
			output:   "version: v1.2.3-rc1\ncommit: abc123",
			expected: "v1.2.3-rc1",
			wantErr:  false,
		},
		{
			name:     "Case insensitive",
			output:   "Version: v1.2.3\nCommit: abc123",
			expected: "v1.2.3",
			wantErr:  false,
		},
		{
			name:     "Missing version",
			output:   "commit: abc123",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.parseVersion(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

// TestParseCommitHash tests the commit hash parsing logic directly.
func TestParseCommitHash(t *testing.T) {
	detector := &versionDetector{}

	tests := []struct {
		name     string
		output   string
		expected string
		wantErr  bool
	}{
		{
			name:     "Valid 40-char hash",
			output:   "version: v1.0.0\ncommit: 80ad31b1234567890abcdef1234567890abcdef",
			expected: "80ad31b1234567890abcdef1234567890abcdef",
			wantErr:  false,
		},
		{
			name:     "Case insensitive label",
			output:   "version: v1.0.0\nCommit: 80ad31b1234567890abcdef1234567890abcdef",
			expected: "80ad31b1234567890abcdef1234567890abcdef",
			wantErr:  false,
		},
		{
			name:     "Missing commit",
			output:   "version: v1.0.0",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Hash too short",
			output:   "version: v1.0.0\ncommit: 80ad31b123",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.parseCommitHash(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}
