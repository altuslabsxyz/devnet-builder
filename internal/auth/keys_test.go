package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	require.NoError(t, err)

	// Check format
	assert.True(t, IsValidKeyFormat(key), "key should have valid format")
	assert.True(t, len(key) == len(KeyPrefix)+KeyRandomBytes*2, "key should be correct length")
	assert.True(t, key[:len(KeyPrefix)] == KeyPrefix, "key should have correct prefix")

	// Generate another key to ensure they're unique
	key2, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2, "keys should be unique")
}

func TestIsValidKeyFormat(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid key", "devnet_0123456789abcdef0123456789abcdef", true},
		{"valid key uppercase", "devnet_0123456789ABCDEF0123456789ABCDEF", true},
		{"wrong prefix", "apikey_0123456789abcdef0123456789abcdef", false},
		{"too short", "devnet_0123456789abcdef", false},
		{"too long", "devnet_0123456789abcdef0123456789abcdef00", false},
		{"empty", "", false},
		{"only prefix", "devnet_", false},
		{"invalid hex chars", "devnet_ghijklmnopqrstuv0123456789abcdef", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidKeyFormat(tt.key))
		})
	}
}

func TestAPIKey_CanAccessNamespace(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		target     string
		canAccess  bool
	}{
		{"wildcard access", []string{"*"}, "any-namespace", true},
		{"exact match", []string{"team-a", "team-b"}, "team-a", true},
		{"no match", []string{"team-a", "team-b"}, "team-c", false},
		{"empty namespaces", []string{}, "any-namespace", false},
		{"wildcard in list", []string{"team-a", "*"}, "any-namespace", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{Namespaces: tt.namespaces}
			assert.Equal(t, tt.canAccess, key.CanAccessNamespace(tt.target))
		})
	}
}

func TestAPIKey_HasAllNamespaceAccess(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		hasAll     bool
	}{
		{"wildcard only", []string{"*"}, true},
		{"wildcard with others", []string{"team-a", "*", "team-b"}, true},
		{"no wildcard", []string{"team-a", "team-b"}, false},
		{"empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{Namespaces: tt.namespaces}
			assert.Equal(t, tt.hasAll, key.HasAllNamespaceAccess())
		})
	}
}

func TestFileKeyStore_CreateAndGet(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	keysPath := filepath.Join(tmpDir, "api-keys.yaml")

	store := NewFileKeyStore(keysPath)
	require.NoError(t, store.Load())

	// Create a key
	key, err := store.Create("alice", []string{"team-a", "team-b"})
	require.NoError(t, err)
	assert.Equal(t, "alice", key.Name)
	assert.Equal(t, []string{"team-a", "team-b"}, key.Namespaces)
	assert.True(t, IsValidKeyFormat(key.Key))
	assert.False(t, key.CreatedAt.IsZero())

	// Get the key
	retrieved, ok := store.Get(key.Key)
	require.True(t, ok)
	assert.Equal(t, key.Key, retrieved.Key)
	assert.Equal(t, key.Name, retrieved.Name)
}

func TestFileKeyStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	keysPath := filepath.Join(tmpDir, "api-keys.yaml")

	store := NewFileKeyStore(keysPath)
	require.NoError(t, store.Load())

	// Initially empty
	assert.Empty(t, store.List())

	// Create some keys
	_, err := store.Create("alice", []string{"*"})
	require.NoError(t, err)
	_, err = store.Create("bob", []string{"team-b"})
	require.NoError(t, err)

	// List should have 2 keys
	keys := store.List()
	assert.Len(t, keys, 2)

	names := make(map[string]bool)
	for _, k := range keys {
		names[k.Name] = true
	}
	assert.True(t, names["alice"])
	assert.True(t, names["bob"])
}

func TestFileKeyStore_Revoke(t *testing.T) {
	tmpDir := t.TempDir()
	keysPath := filepath.Join(tmpDir, "api-keys.yaml")

	store := NewFileKeyStore(keysPath)
	require.NoError(t, store.Load())

	// Create a key
	key, err := store.Create("alice", []string{"*"})
	require.NoError(t, err)

	// Key should exist
	_, ok := store.Get(key.Key)
	require.True(t, ok)

	// Revoke the key
	err = store.Revoke(key.Key)
	require.NoError(t, err)

	// Key should no longer exist
	_, ok = store.Get(key.Key)
	assert.False(t, ok)

	// Revoking again should fail
	err = store.Revoke(key.Key)
	assert.Error(t, err)
}

func TestFileKeyStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	keysPath := filepath.Join(tmpDir, "api-keys.yaml")

	// Create and populate store
	store1 := NewFileKeyStore(keysPath)
	require.NoError(t, store1.Load())

	key1, err := store1.Create("alice", []string{"*"})
	require.NoError(t, err)
	key2, err := store1.Create("bob", []string{"team-b", "shared"})
	require.NoError(t, err)

	// Save to file
	require.NoError(t, store1.Save())

	// Check file was created with restrictive permissions
	info, err := os.Stat(keysPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Load into new store
	store2 := NewFileKeyStore(keysPath)
	require.NoError(t, store2.Load())

	// Verify keys were loaded
	keys := store2.List()
	assert.Len(t, keys, 2)

	retrieved1, ok := store2.Get(key1.Key)
	require.True(t, ok)
	assert.Equal(t, "alice", retrieved1.Name)
	assert.Equal(t, []string{"*"}, retrieved1.Namespaces)

	retrieved2, ok := store2.Get(key2.Key)
	require.True(t, ok)
	assert.Equal(t, "bob", retrieved2.Name)
	assert.Equal(t, []string{"team-b", "shared"}, retrieved2.Namespaces)
}

func TestFileKeyStore_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	keysPath := filepath.Join(tmpDir, "nonexistent", "api-keys.yaml")

	store := NewFileKeyStore(keysPath)
	// Loading non-existent file should succeed with empty store
	err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, store.List())
}

func TestFileKeyStore_SaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	keysPath := filepath.Join(tmpDir, "subdir", "nested", "api-keys.yaml")

	store := NewFileKeyStore(keysPath)
	require.NoError(t, store.Load())

	_, err := store.Create("alice", []string{"*"})
	require.NoError(t, err)

	// Save should create directories
	err = store.Save()
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(keysPath)
	require.NoError(t, err)
}
