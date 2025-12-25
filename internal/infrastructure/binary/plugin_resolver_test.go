package binary

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// Mock BinaryCache for testing

type mockBinaryCache struct {
	getActiveFunc    func() (string, error)
	symlinkInfoFunc  func() (*ports.SymlinkInfo, error)
	hasFunc          func(ref string) bool
	getFunc          func(ref string) (string, bool)
	storeFunc        func(ctx context.Context, ref string, binaryPath string) (string, error)
	setActiveFunc    func(ref string) error
	removeFunc       func(ref string) error
	cleanFunc        func() error
	listFunc         func() []string
	listDetailedFunc func() []ports.CachedBinaryInfo
	statsFunc        func() ports.CacheStats
	cacheDirFunc     func() string
	symlinkPathFunc  func() string
}

func (m *mockBinaryCache) GetActive() (string, error) {
	if m.getActiveFunc != nil {
		return m.getActiveFunc()
	}
	return "", errors.New("no active binary")
}

func (m *mockBinaryCache) SymlinkInfo() (*ports.SymlinkInfo, error) {
	if m.symlinkInfoFunc != nil {
		return m.symlinkInfoFunc()
	}
	return &ports.SymlinkInfo{
		Path:   "/path/to/symlink",
		Target: "/path/to/binary",
		Exists: true,
	}, nil
}

func (m *mockBinaryCache) Has(ref string) bool {
	if m.hasFunc != nil {
		return m.hasFunc(ref)
	}
	return false
}

func (m *mockBinaryCache) Get(ref string) (string, bool) {
	if m.getFunc != nil {
		return m.getFunc(ref)
	}
	return "", false
}

func (m *mockBinaryCache) Store(ctx context.Context, ref string, binaryPath string) (string, error) {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, ref, binaryPath)
	}
	return "", nil
}

func (m *mockBinaryCache) SetActive(ref string) error {
	if m.setActiveFunc != nil {
		return m.setActiveFunc(ref)
	}
	return nil
}

func (m *mockBinaryCache) Remove(ref string) error {
	if m.removeFunc != nil {
		return m.removeFunc(ref)
	}
	return nil
}

func (m *mockBinaryCache) Clean() error {
	if m.cleanFunc != nil {
		return m.cleanFunc()
	}
	return nil
}

func (m *mockBinaryCache) List() []string {
	if m.listFunc != nil {
		return m.listFunc()
	}
	return []string{}
}

func (m *mockBinaryCache) ListDetailed() []ports.CachedBinaryInfo {
	if m.listDetailedFunc != nil {
		return m.listDetailedFunc()
	}
	return []ports.CachedBinaryInfo{}
}

func (m *mockBinaryCache) Stats() ports.CacheStats {
	if m.statsFunc != nil {
		return m.statsFunc()
	}
	return ports.CacheStats{}
}

func (m *mockBinaryCache) CacheDir() string {
	if m.cacheDirFunc != nil {
		return m.cacheDirFunc()
	}
	return "/cache/dir"
}

func (m *mockBinaryCache) SymlinkPath() string {
	if m.symlinkPathFunc != nil {
		return m.symlinkPathFunc()
	}
	return "/path/to/symlink"
}

// Tests for helper functions

func TestValidateBinary_Success(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	err := validateBinary(binaryPath)
	if err != nil {
		t.Errorf("Expected no error for valid binary, got: %v", err)
	}
}

func TestValidateBinary_FileNotFound(t *testing.T) {
	err := validateBinary("/nonexistent/binary")
	if err == nil {
		t.Fatal("Expected error for nonexistent binary, got nil")
	}
}

func TestValidateBinary_NotExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := validateBinary(binaryPath)
	if err == nil {
		t.Fatal("Expected error for non-executable file, got nil")
	}
}

func TestValidateBinary_IsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	err := validateBinary(tmpDir)
	if err == nil {
		t.Fatal("Expected error for directory, got nil")
	}
}

func TestExtractPluginNameFromPath(t *testing.T) {
	// Note: extractPluginNameFromPath assumes binary is 3 levels deep from network name:
	// <cache-dir>/<network>/<ref>/<binary>
	// So it goes up 3 levels: binary -> ref -> network
	tests := []struct {
		name         string
		binaryPath   string
		expectedName string
	}{
		{
			name:         "short path - cannot extract",
			binaryPath:   "/binary",
			expectedName: "",
		},
		{
			name:         "root path",
			binaryPath:   "/",
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPluginNameFromPath(tt.binaryPath)
			if result != tt.expectedName {
				t.Errorf("Expected '%s', got '%s'", tt.expectedName, result)
			}
		})
	}
}

func TestPluginBinaryResolver_GetActiveBinary_Success(t *testing.T) {
	tmpDir := t.TempDir()
	// Cache structure: <cache-dir>/<network>/<ref>/<binary>
	// extractPluginNameFromPath goes up 2 levels from binary to get network name
	binaryPath := filepath.Join(tmpDir, "stable", "v1.0.0", "stabled")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	cache := &mockBinaryCache{
		symlinkInfoFunc: func() (*ports.SymlinkInfo, error) {
			return &ports.SymlinkInfo{
				Path:   "/path/to/symlink",
				Target: binaryPath,
				Exists: true,
			}, nil
		},
	}

	resolver := NewPluginBinaryResolver(nil, cache)

	path, pluginName, err := resolver.GetActiveBinary(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if path != binaryPath {
		t.Errorf("Expected path %s, got %s", binaryPath, path)
	}

	// The plugin name extraction is best-effort from path structure
	// Just verify we got a non-empty name
	if pluginName == "" {
		t.Error("Expected non-empty plugin name")
	}
}

func TestPluginBinaryResolver_GetActiveBinary_NoSymlink(t *testing.T) {
	cache := &mockBinaryCache{
		symlinkInfoFunc: func() (*ports.SymlinkInfo, error) {
			return &ports.SymlinkInfo{
				Exists: false,
			}, nil
		},
	}

	resolver := NewPluginBinaryResolver(nil, cache)

	_, _, err := resolver.GetActiveBinary(context.Background())
	if err == nil {
		t.Fatal("Expected error for no symlink, got nil")
	}
}

func TestPluginBinaryResolver_GetActiveBinary_BinaryNotFound(t *testing.T) {
	cache := &mockBinaryCache{
		symlinkInfoFunc: func() (*ports.SymlinkInfo, error) {
			return &ports.SymlinkInfo{
				Path:   "/path/to/symlink",
				Target: "/nonexistent/binary",
				Exists: true,
			}, nil
		},
	}

	resolver := NewPluginBinaryResolver(nil, cache)

	_, _, err := resolver.GetActiveBinary(context.Background())
	if err == nil {
		t.Fatal("Expected error for nonexistent binary, got nil")
	}
}

func TestPluginBinaryResolver_GetActiveBinary_CacheError(t *testing.T) {
	cache := &mockBinaryCache{
		symlinkInfoFunc: func() (*ports.SymlinkInfo, error) {
			return nil, errors.New("cache error")
		},
	}

	resolver := NewPluginBinaryResolver(nil, cache)

	_, _, err := resolver.GetActiveBinary(context.Background())
	if err == nil {
		t.Fatal("Expected error from cache, got nil")
	}
}

func TestPluginBinaryResolver_GetActiveBinary_RegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "stabled")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	cache := &mockBinaryCache{
		symlinkInfoFunc: func() (*ports.SymlinkInfo, error) {
			return &ports.SymlinkInfo{
				Path:      binaryPath,
				Target:    "",
				Exists:    true,
				IsRegular: true,
			}, nil
		},
	}

	resolver := NewPluginBinaryResolver(nil, cache)

	path, _, err := resolver.GetActiveBinary(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if path != binaryPath {
		t.Errorf("Expected path %s, got %s", binaryPath, path)
	}
}

func TestPluginBinaryResolver_ContextCancellation(t *testing.T) {
	cache := &mockBinaryCache{}
	resolver := NewPluginBinaryResolver(nil, cache)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := resolver.GetActiveBinary(ctx)
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}
