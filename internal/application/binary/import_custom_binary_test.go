package binary

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/pkg/network"
)

// mockVersionDetector is a test double for BinaryVersionDetector
type mockVersionDetector struct {
	versionInfo *ports.BinaryVersionInfo
	err         error
}

func (m *mockVersionDetector) DetectVersion(ctx context.Context, binaryPath string) (*ports.BinaryVersionInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.versionInfo, nil
}

// mockBinaryCache is a test double for BinaryCache
type mockBinaryCache struct {
	storePath     string
	storeErr      error
	setActiveErr  error
	symlinkPath   string
	symlinksDir   string
	cachePath     string
	cacheKey      string
	storedKey     string
	storedSrcPath string
	cacheDir      string
}

func (m *mockBinaryCache) Store(ctx context.Context, cacheKey, srcPath string) (string, error) {
	if m.storeErr != nil {
		return "", m.storeErr
	}
	m.storedKey = cacheKey
	m.storedSrcPath = srcPath
	return m.storePath, nil
}

func (m *mockBinaryCache) SetActive(cacheKey string) error {
	if m.setActiveErr != nil {
		return m.setActiveErr
	}
	m.cacheKey = cacheKey
	return nil
}

func (m *mockBinaryCache) SymlinkPath() string {
	return m.symlinkPath
}

func (m *mockBinaryCache) SymlinksDir() string {
	return m.symlinksDir
}

func (m *mockBinaryCache) CachePath() string {
	return m.cachePath
}

func (m *mockBinaryCache) CacheDir() string {
	if m.cacheDir != "" {
		return m.cacheDir
	}
	return "/tmp/cache"
}

func (m *mockBinaryCache) Get(ref string) (string, bool) {
	return "", false
}

func (m *mockBinaryCache) Has(ref string) bool {
	return false
}

func (m *mockBinaryCache) List() []string {
	return nil
}

func (m *mockBinaryCache) ListDetailed() []ports.CachedBinaryInfo {
	return nil
}

func (m *mockBinaryCache) Stats() ports.CacheStats {
	return ports.CacheStats{}
}

func (m *mockBinaryCache) Remove(ref string) error {
	return nil
}

func (m *mockBinaryCache) Clean() error {
	return nil
}

func (m *mockBinaryCache) GetActive() (string, error) {
	return "", nil
}

func (m *mockBinaryCache) SymlinkInfo() (*ports.SymlinkInfo, error) {
	return nil, nil
}

// TestImportCustomBinaryUseCase_Success tests successful binary import flow
func TestImportCustomBinaryUseCase_Success(t *testing.T) {
	// Setup: Create a temporary binary file
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(testBinary, []byte("fake binary"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Setup: Mock dependencies
	versionDetector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.2.3",
			CommitHash: "80ad31b1234567890abcdef1234567890abcdef",
			GitCommit:  "80ad31b1234567890abcdef1234567890abcdef",
		},
	}

	cachedBinaryPath := filepath.Join(tmpDir, "cache", "mainnet", "80ad31b1234567890abcdef1234567890abcdef-abc123", "stabled")
	symlinkPath := filepath.Join(tmpDir, "bin", "stabled")

	// Create the cached binary path directory and file so stat succeeds
	if err := os.MkdirAll(filepath.Dir(cachedBinaryPath), 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}
	if err := os.WriteFile(cachedBinaryPath, []byte("cached binary"), 0755); err != nil {
		t.Fatalf("Failed to create cached binary: %v", err)
	}

	binaryCache := &mockBinaryCache{
		storePath:   cachedBinaryPath,
		symlinkPath: symlinkPath,
	}

	// Create use case
	useCase := NewImportCustomBinaryUseCase(
		versionDetector,
		binaryCache,
		tmpDir,
		"stabled",
	)

	// Execute
	input := &dto.CustomBinaryImportInput{
		BinaryPath:  testBinary,
		NetworkType: "mainnet",
		BuildConfig: &network.BuildConfig{},
		Ref:         "custom",
	}

	result, err := useCase.Execute(context.Background(), input)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: Result fields are correct
	if result.Version != "v1.2.3" {
		t.Errorf("Expected version v1.2.3, got: %s", result.Version)
	}

	if result.CommitHash != "80ad31b1234567890abcdef1234567890abcdef" {
		t.Errorf("Expected commit hash 80ad31b..., got: %s", result.CommitHash)
	}

	if result.CachedBinaryPath != cachedBinaryPath {
		t.Errorf("Expected cached path %s, got: %s", cachedBinaryPath, result.CachedBinaryPath)
	}

	if result.SymlinkPath != symlinkPath {
		t.Errorf("Expected symlink path %s, got: %s", symlinkPath, result.SymlinkPath)
	}

	if result.Size == 0 {
		t.Error("Expected non-zero file size")
	}

	// Verify: Cache was called correctly
	if binaryCache.storedSrcPath != testBinary {
		t.Errorf("Expected cache to store from %s, got: %s", testBinary, binaryCache.storedSrcPath)
	}

	expectedCacheKey := "mainnet/80ad31b1234567890abcdef1234567890abcdef-empty"
	if binaryCache.storedKey != expectedCacheKey {
		t.Errorf("Expected cache key %s, got: %s", expectedCacheKey, binaryCache.storedKey)
	}
}

// TestImportCustomBinaryUseCase_NilInput tests validation of nil input
func TestImportCustomBinaryUseCase_NilInput(t *testing.T) {
	useCase := NewImportCustomBinaryUseCase(
		&mockVersionDetector{},
		&mockBinaryCache{},
		"/tmp",
		"stabled",
	)

	_, err := useCase.Execute(context.Background(), nil)

	if err == nil {
		t.Fatal("Expected error for nil input")
	}

	if err.Error() != "invalid input: input is nil" {
		t.Errorf("Expected 'input is nil' error, got: %v", err)
	}
}

// TestImportCustomBinaryUseCase_EmptyBinaryPath tests validation of empty binary path
func TestImportCustomBinaryUseCase_EmptyBinaryPath(t *testing.T) {
	useCase := NewImportCustomBinaryUseCase(
		&mockVersionDetector{},
		&mockBinaryCache{},
		"/tmp",
		"stabled",
	)

	input := &dto.CustomBinaryImportInput{
		BinaryPath:  "", // Empty
		NetworkType: "mainnet",
	}

	_, err := useCase.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("Expected error for empty binary path")
	}

	if err.Error() != "invalid input: binary path is required" {
		t.Errorf("Expected 'binary path is required' error, got: %v", err)
	}
}

// TestImportCustomBinaryUseCase_EmptyNetworkType tests validation of empty network type
func TestImportCustomBinaryUseCase_EmptyNetworkType(t *testing.T) {
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(testBinary, []byte("fake"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	useCase := NewImportCustomBinaryUseCase(
		&mockVersionDetector{},
		&mockBinaryCache{},
		tmpDir,
		"stabled",
	)

	input := &dto.CustomBinaryImportInput{
		BinaryPath:  testBinary,
		NetworkType: "", // Empty
	}

	_, err := useCase.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("Expected error for empty network type")
	}

	if err.Error() != "invalid input: network type is required" {
		t.Errorf("Expected 'network type is required' error, got: %v", err)
	}
}

// TestImportCustomBinaryUseCase_BinaryNotFound tests error when binary doesn't exist
func TestImportCustomBinaryUseCase_BinaryNotFound(t *testing.T) {
	useCase := NewImportCustomBinaryUseCase(
		&mockVersionDetector{},
		&mockBinaryCache{},
		"/tmp",
		"stabled",
	)

	input := &dto.CustomBinaryImportInput{
		BinaryPath:  "/nonexistent/binary",
		NetworkType: "mainnet",
	}

	_, err := useCase.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("Expected error for non-existent binary")
	}

	// Should contain "binary not found" message
	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid input") || !strings.Contains(errMsg, "binary not found") {
		t.Errorf("Expected 'binary not found' error, got: %v", err)
	}
}

// TestImportCustomBinaryUseCase_VersionDetectionFails tests error handling when version detection fails
func TestImportCustomBinaryUseCase_VersionDetectionFails(t *testing.T) {
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(testBinary, []byte("fake"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Mock detector that returns error
	versionDetector := &mockVersionDetector{
		err: errors.New("failed to parse version"),
	}

	useCase := NewImportCustomBinaryUseCase(
		versionDetector,
		&mockBinaryCache{},
		tmpDir,
		"stabled",
	)

	input := &dto.CustomBinaryImportInput{
		BinaryPath:  testBinary,
		NetworkType: "mainnet",
	}

	_, err := useCase.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("Expected error from version detection")
	}

	if !strings.Contains(err.Error(), "failed to detect binary version") {
		t.Errorf("Expected version detection error, got: %v", err)
	}
}

// TestImportCustomBinaryUseCase_CacheStoreFails tests error handling when cache storage fails
func TestImportCustomBinaryUseCase_CacheStoreFails(t *testing.T) {
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(testBinary, []byte("fake"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	versionDetector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.0.0",
			CommitHash: "80ad31b1234567890abcdef1234567890abcdef",
		},
	}

	// Mock cache that fails on Store
	binaryCache := &mockBinaryCache{
		storeErr: errors.New("disk full"),
	}

	useCase := NewImportCustomBinaryUseCase(
		versionDetector,
		binaryCache,
		tmpDir,
		"stabled",
	)

	input := &dto.CustomBinaryImportInput{
		BinaryPath:  testBinary,
		NetworkType: "mainnet",
	}

	_, err := useCase.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("Expected error from cache storage")
	}

	if !strings.Contains(err.Error(), "failed to store binary in cache") {
		t.Errorf("Expected cache storage error, got: %v", err)
	}
}

// TestImportCustomBinaryUseCase_SetActiveFails tests error handling when symlink creation fails
func TestImportCustomBinaryUseCase_SetActiveFails(t *testing.T) {
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(testBinary, []byte("fake"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	versionDetector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.0.0",
			CommitHash: "80ad31b1234567890abcdef1234567890abcdef",
		},
	}

	cachedPath := filepath.Join(tmpDir, "cached-binary")
	if err := os.WriteFile(cachedPath, []byte("fake"), 0755); err != nil {
		t.Fatalf("Failed to create cached binary: %v", err)
	}

	// Mock cache that fails on SetActive
	binaryCache := &mockBinaryCache{
		storePath:    cachedPath,
		setActiveErr: errors.New("symlink failed"),
	}

	useCase := NewImportCustomBinaryUseCase(
		versionDetector,
		binaryCache,
		tmpDir,
		"stabled",
	)

	input := &dto.CustomBinaryImportInput{
		BinaryPath:  testBinary,
		NetworkType: "mainnet",
	}

	_, err := useCase.Execute(context.Background(), input)

	if err == nil {
		t.Fatal("Expected error from SetActive")
	}

	if !strings.Contains(err.Error(), "failed to set binary as active") {
		t.Errorf("Expected SetActive error, got: %v", err)
	}
}

// TestImportCustomBinaryUseCase_WithNilBuildConfig tests handling of nil build config
func TestImportCustomBinaryUseCase_WithNilBuildConfig(t *testing.T) {
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(testBinary, []byte("fake"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	versionDetector := &mockVersionDetector{
		versionInfo: &ports.BinaryVersionInfo{
			Version:    "v1.0.0",
			CommitHash: "80ad31b1234567890abcdef1234567890abcdef",
		},
	}

	cachedPath := filepath.Join(tmpDir, "cached")
	if err := os.WriteFile(cachedPath, []byte("fake"), 0755); err != nil {
		t.Fatalf("Failed to create cached binary: %v", err)
	}

	binaryCache := &mockBinaryCache{
		storePath:   cachedPath,
		symlinkPath: filepath.Join(tmpDir, "symlink"),
	}

	useCase := NewImportCustomBinaryUseCase(
		versionDetector,
		binaryCache,
		tmpDir,
		"stabled",
	)

	input := &dto.CustomBinaryImportInput{
		BinaryPath:  testBinary,
		NetworkType: "mainnet",
		BuildConfig: nil, // Nil build config should use empty hash
	}

	result, err := useCase.Execute(context.Background(), input)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should use empty build config hash ("empty")
	expectedCacheKey := "mainnet/80ad31b1234567890abcdef1234567890abcdef-empty"
	if result.CacheKey != expectedCacheKey {
		t.Errorf("Expected cache key %s, got: %s", expectedCacheKey, result.CacheKey)
	}
}
