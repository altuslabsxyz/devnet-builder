package binary

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/cache"
)

// mockVersionDetector is a test double for BinaryVersionDetector
type mockVersionDetector struct {
	versionInfo *ports.BinaryVersionInfo
	err         error
	callCount   int
}

func (m *mockVersionDetector) DetectVersion(ctx context.Context, binaryPath string) (*ports.BinaryVersionInfo, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.versionInfo, nil
}

// TestBinaryValidator_ValidateBinary_Success tests successful validation
func TestBinaryValidator_ValidateBinary_Success(t *testing.T) {
	// Setup: Create valid executable binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Mock detector that returns success
	detector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.0.0",
			CommitHash: "80ad31b1234567890abcdef1234567890abcdef",
		},
	}

	validator := NewBinaryValidator(detector)

	// Execute validation
	err := validator.ValidateBinary(context.Background(), binaryPath)

	// Verify: No error
	if err != nil {
		t.Errorf("Expected successful validation, got error: %v", err)
	}

	// Verify detector was called
	if detector.callCount != 1 {
		t.Errorf("Expected detector to be called once, got: %d", detector.callCount)
	}
}

// TestBinaryValidator_ValidateBinary_BinaryNotFound tests error when binary doesn't exist
func TestBinaryValidator_ValidateBinary_BinaryNotFound(t *testing.T) {
	detector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.0.0",
			CommitHash: "80ad31b",
		},
	}

	validator := NewBinaryValidator(detector)

	// Execute with non-existent path
	err := validator.ValidateBinary(context.Background(), "/nonexistent/binary")

	// Verify: Error contains "binary not found"
	if err == nil {
		t.Fatal("Expected error for non-existent binary")
	}
	if !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("Expected 'binary not found' error, got: %v", err)
	}

	// Verify detector was NOT called (early exit)
	if detector.callCount != 0 {
		t.Errorf("Expected detector not to be called, got: %d calls", detector.callCount)
	}
}

// TestBinaryValidator_ValidateBinary_PathIsDirectory tests error when path is a directory
func TestBinaryValidator_ValidateBinary_PathIsDirectory(t *testing.T) {
	// Setup: Create directory
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "test-dir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	detector := &mockVersionDetector{}
	validator := NewBinaryValidator(detector)

	// Execute validation on directory
	err := validator.ValidateBinary(context.Background(), dirPath)

	// Verify: Error contains "directory"
	if err == nil {
		t.Fatal("Expected error for directory path")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Expected 'directory' error, got: %v", err)
	}

	// Verify detector was NOT called
	if detector.callCount != 0 {
		t.Errorf("Expected detector not to be called, got: %d calls", detector.callCount)
	}
}

// TestBinaryValidator_ValidateBinary_NotExecutable tests error when binary lacks execute permissions
func TestBinaryValidator_ValidateBinary_NotExecutable(t *testing.T) {
	// Setup: Create non-executable file (0644)
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "non-executable")
	if err := os.WriteFile(binaryPath, []byte("fake binary"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	detector := &mockVersionDetector{}
	validator := NewBinaryValidator(detector)

	// Execute validation
	err := validator.ValidateBinary(context.Background(), binaryPath)

	// Verify: Error contains "not executable" and "chmod"
	if err == nil {
		t.Fatal("Expected error for non-executable binary")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not executable") {
		t.Errorf("Expected 'not executable' in error, got: %v", err)
	}
	if !strings.Contains(errMsg, "chmod +x") {
		t.Errorf("Expected 'chmod +x' suggestion in error, got: %v", err)
	}

	// Verify detector was NOT called
	if detector.callCount != 0 {
		t.Errorf("Expected detector not to be called, got: %d calls", detector.callCount)
	}
}

// TestBinaryValidator_ValidateBinary_VersionDetectionFails tests error when version detection fails
func TestBinaryValidator_ValidateBinary_VersionDetectionFails(t *testing.T) {
	// Setup: Create valid executable
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Mock detector that returns error
	detector := &mockVersionDetector{
		err: errors.New("failed to parse version output"),
	}

	validator := NewBinaryValidator(detector)

	// Execute validation
	err := validator.ValidateBinary(context.Background(), binaryPath)

	// Verify: Error contains "version detection failed"
	if err == nil {
		t.Fatal("Expected error from version detection")
	}
	if !strings.Contains(err.Error(), "version detection failed") {
		t.Errorf("Expected 'version detection failed' error, got: %v", err)
	}

	// Verify detector was called
	if detector.callCount != 1 {
		t.Errorf("Expected detector to be called once, got: %d", detector.callCount)
	}
}

// TestBinaryValidator_ValidateAndEnrichMetadata_Success tests successful enrichment
func TestBinaryValidator_ValidateAndEnrichMetadata_Success(t *testing.T) {
	// Setup: Create valid executable
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Mock detector
	detector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.2.3",
			CommitHash: "80ad31b1234567890abcdef1234567890abcdef",
		},
	}

	validator := NewBinaryValidator(detector)

	// Create metadata
	metadata := &cache.CachedBinaryMetadata{
		Path:            binaryPath,
		Name:            "stabled",
		CommitHashShort: "80ad31b",
		IsValid:         false,
	}

	// Execute enrichment
	enriched, err := validator.ValidateAndEnrichMetadata(context.Background(), metadata)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected successful enrichment, got error: %v", err)
	}

	// Verify: Metadata enriched correctly
	if !enriched.IsValid {
		t.Error("Expected IsValid to be true")
	}
	if enriched.ValidationError != "" {
		t.Errorf("Expected empty ValidationError, got: %s", enriched.ValidationError)
	}
	if enriched.Version != "v1.2.3" {
		t.Errorf("Expected version v1.2.3, got: %s", enriched.Version)
	}
	if enriched.CommitHash != "80ad31b1234567890abcdef1234567890abcdef" {
		t.Errorf("Expected full commit hash, got: %s", enriched.CommitHash)
	}
	if enriched.CommitHashShort != "80ad31b1" {
		t.Errorf("Expected CommitHashShort to be updated to first 8 chars (80ad31b1), got: %s", enriched.CommitHashShort)
	}

	// Verify detector was called twice (once in ValidateBinary, once for enrichment)
	if detector.callCount != 2 {
		t.Errorf("Expected detector to be called twice, got: %d", detector.callCount)
	}
}

// TestBinaryValidator_ValidateAndEnrichMetadata_ValidationFails tests enrichment when validation fails
func TestBinaryValidator_ValidateAndEnrichMetadata_ValidationFails(t *testing.T) {
	// Setup: Use non-existent binary
	binaryPath := "/nonexistent/binary"

	detector := &mockVersionDetector{}
	validator := NewBinaryValidator(detector)

	// Create metadata
	metadata := &cache.CachedBinaryMetadata{
		Path:    binaryPath,
		Name:    "stabled",
		IsValid: false,
	}

	// Execute enrichment
	enriched, err := validator.ValidateAndEnrichMetadata(context.Background(), metadata)

	// Verify: Error returned
	if err == nil {
		t.Fatal("Expected error for non-existent binary")
	}

	// Verify: Metadata marked as invalid with error message
	if enriched.IsValid {
		t.Error("Expected IsValid to be false")
	}
	if enriched.ValidationError == "" {
		t.Error("Expected ValidationError to be set")
	}
	if !strings.Contains(enriched.ValidationError, "binary not found") {
		t.Errorf("Expected 'binary not found' in ValidationError, got: %s", enriched.ValidationError)
	}

	// Verify detector was NOT called (validation failed early)
	if detector.callCount != 0 {
		t.Errorf("Expected detector not to be called, got: %d calls", detector.callCount)
	}
}

// TestBinaryValidator_ValidateAndEnrichMetadata_NonExecutable tests enrichment with non-executable binary
func TestBinaryValidator_ValidateAndEnrichMetadata_NonExecutable(t *testing.T) {
	// Setup: Create non-executable file
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "non-executable")
	if err := os.WriteFile(binaryPath, []byte("fake binary"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	detector := &mockVersionDetector{}
	validator := NewBinaryValidator(detector)

	// Create metadata
	metadata := &cache.CachedBinaryMetadata{
		Path:    binaryPath,
		Name:    "stabled",
		IsValid: false,
	}

	// Execute enrichment
	enriched, err := validator.ValidateAndEnrichMetadata(context.Background(), metadata)

	// Verify: Error returned
	if err == nil {
		t.Fatal("Expected error for non-executable binary")
	}

	// Verify: Metadata marked as invalid with helpful error message
	if enriched.IsValid {
		t.Error("Expected IsValid to be false")
	}
	if enriched.ValidationError == "" {
		t.Error("Expected ValidationError to be set")
	}
	if !strings.Contains(enriched.ValidationError, "not executable") {
		t.Errorf("Expected 'not executable' in ValidationError, got: %s", enriched.ValidationError)
	}
	if !strings.Contains(enriched.ValidationError, "chmod +x") {
		t.Errorf("Expected 'chmod +x' suggestion in ValidationError, got: %s", enriched.ValidationError)
	}
}

// TestBinaryValidator_ValidateAndEnrichMetadata_VersionDetectionFails tests when detection fails during enrichment
func TestBinaryValidator_ValidateAndEnrichMetadata_VersionDetectionFails(t *testing.T) {
	// Setup: Create valid executable
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Mock detector that returns error
	detector := &mockVersionDetector{
		err: errors.New("timeout executing binary"),
	}

	validator := NewBinaryValidator(detector)

	// Create metadata
	metadata := &cache.CachedBinaryMetadata{
		Path:    binaryPath,
		Name:    "stabled",
		IsValid: false,
	}

	// Execute enrichment
	enriched, err := validator.ValidateAndEnrichMetadata(context.Background(), metadata)

	// Verify: Error returned
	if err == nil {
		t.Fatal("Expected error from version detection")
	}

	// Verify: Metadata marked as invalid
	if enriched.IsValid {
		t.Error("Expected IsValid to be false")
	}
	if enriched.ValidationError == "" {
		t.Error("Expected ValidationError to be set")
	}
	if !strings.Contains(enriched.ValidationError, "version detection failed") {
		t.Errorf("Expected 'version detection failed' in ValidationError, got: %s", enriched.ValidationError)
	}
}

// TestBinaryValidator_ValidateAndEnrichMetadata_UpdatesCommitHashShort tests that short hash is updated
func TestBinaryValidator_ValidateAndEnrichMetadata_UpdatesCommitHashShort(t *testing.T) {
	// Setup: Create valid executable
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Mock detector with full hash
	detector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v2.0.0",
			CommitHash: "abcdef1234567890fedcba0987654321abcdef12",
		},
	}

	validator := NewBinaryValidator(detector)

	// Create metadata with different short hash (from cache directory name)
	metadata := &cache.CachedBinaryMetadata{
		Path:            binaryPath,
		Name:            "stabled",
		CommitHashShort: "80ad31b", // Old/incorrect short hash
		IsValid:         false,
	}

	// Execute enrichment
	enriched, err := validator.ValidateAndEnrichMetadata(context.Background(), metadata)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected successful enrichment, got error: %v", err)
	}

	// Verify: Short hash updated to match actual binary
	expectedShortHash := "abcdef12" // First 8 chars of full hash
	if enriched.CommitHashShort != expectedShortHash {
		t.Errorf("Expected CommitHashShort to be updated to %s, got: %s", expectedShortHash, enriched.CommitHashShort)
	}

	// Verify: Full hash is set
	if enriched.CommitHash != "abcdef1234567890fedcba0987654321abcdef12" {
		t.Errorf("Expected full commit hash, got: %s", enriched.CommitHash)
	}
}

// TestBinaryValidator_ValidateAndEnrichMetadata_ClearsErrorOnSuccess tests that ValidationError is cleared
func TestBinaryValidator_ValidateAndEnrichMetadata_ClearsErrorOnSuccess(t *testing.T) {
	// Setup: Create valid executable
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	detector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.0.0",
			CommitHash: "80ad31b1234567890abcdef1234567890abcdef",
		},
	}

	validator := NewBinaryValidator(detector)

	// Create metadata with previous error
	metadata := &cache.CachedBinaryMetadata{
		Path:            binaryPath,
		Name:            "stabled",
		IsValid:         false,
		ValidationError: "previous validation error",
	}

	// Execute enrichment
	enriched, err := validator.ValidateAndEnrichMetadata(context.Background(), metadata)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected successful enrichment, got error: %v", err)
	}

	// Verify: Error cleared
	if enriched.ValidationError != "" {
		t.Errorf("Expected ValidationError to be cleared, got: %s", enriched.ValidationError)
	}

	// Verify: IsValid set to true
	if !enriched.IsValid {
		t.Error("Expected IsValid to be true")
	}
}
