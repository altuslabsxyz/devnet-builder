// Package auth provides API key authentication for the devnetd daemon.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// KeyPrefix is the prefix for all API keys.
	KeyPrefix = "devnet_"
	// KeyRandomBytes is the number of random bytes in the key (32 chars hex = 16 bytes).
	KeyRandomBytes = 16
	// DefaultKeysFileName is the default filename for the keys file.
	DefaultKeysFileName = "api-keys.yaml"
)

// APIKey represents an API key with associated metadata.
type APIKey struct {
	// Key is the full API key string (devnet_<32-char-random>).
	Key string `yaml:"key"`
	// Name is a human-readable identifier for the key owner.
	Name string `yaml:"name"`
	// Namespaces is the list of namespaces this key can access.
	// Use ["*"] to grant access to all namespaces.
	Namespaces []string `yaml:"namespaces"`
	// CreatedAt is when the key was created.
	CreatedAt time.Time `yaml:"created_at"`
}

// HasAllNamespaceAccess returns true if the key has access to all namespaces.
func (k *APIKey) HasAllNamespaceAccess() bool {
	for _, ns := range k.Namespaces {
		if ns == "*" {
			return true
		}
	}
	return false
}

// CanAccessNamespace returns true if the key can access the given namespace.
func (k *APIKey) CanAccessNamespace(namespace string) bool {
	if k.HasAllNamespaceAccess() {
		return true
	}
	for _, ns := range k.Namespaces {
		if ns == namespace {
			return true
		}
	}
	return false
}

// KeyStore defines the interface for API key storage.
type KeyStore interface {
	// Load loads keys from storage.
	Load() error
	// Save persists keys to storage.
	Save() error
	// Create creates a new API key with the given name and namespace access.
	Create(name string, namespaces []string) (*APIKey, error)
	// Get retrieves a key by its full key string.
	Get(key string) (*APIKey, bool)
	// List returns all stored keys.
	List() []*APIKey
	// Revoke removes a key by its full key string.
	Revoke(key string) error
}

// keysFile represents the YAML file structure.
type keysFile struct {
	Keys []*APIKey `yaml:"keys"`
}

// FileKeyStore implements KeyStore using a YAML file backend.
type FileKeyStore struct {
	path string
	mu   sync.RWMutex
	keys map[string]*APIKey // keyed by full key string
}

// NewFileKeyStore creates a new FileKeyStore with the given file path.
func NewFileKeyStore(path string) *FileKeyStore {
	return &FileKeyStore{
		path: path,
		keys: make(map[string]*APIKey),
	}
}

// DefaultKeysPath returns the default path for the keys file.
func DefaultKeysPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultKeysFileName
	}
	return filepath.Join(home, ".devnet-builder", DefaultKeysFileName)
}

// Load loads keys from the YAML file.
// If the file doesn't exist, it initializes an empty store.
func (s *FileKeyStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty store
			s.keys = make(map[string]*APIKey)
			return nil
		}
		return fmt.Errorf("failed to read keys file: %w", err)
	}

	var kf keysFile
	if err := yaml.Unmarshal(data, &kf); err != nil {
		return fmt.Errorf("failed to parse keys file: %w", err)
	}

	s.keys = make(map[string]*APIKey, len(kf.Keys))
	for _, key := range kf.Keys {
		s.keys[key.Key] = key
	}

	return nil
}

// Save persists keys to the YAML file.
func (s *FileKeyStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	// Convert map to slice for YAML
	kf := keysFile{
		Keys: make([]*APIKey, 0, len(s.keys)),
	}
	for _, key := range s.keys {
		kf.Keys = append(kf.Keys, key)
	}

	data, err := yaml.Marshal(&kf)
	if err != nil {
		return fmt.Errorf("failed to marshal keys: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write keys file: %w", err)
	}

	return nil
}

// Create creates a new API key with the given name and namespace access.
func (s *FileKeyStore) Create(name string, namespaces []string) (*APIKey, error) {
	key, err := GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	apiKey := &APIKey{
		Key:        key,
		Name:       name,
		Namespaces: namespaces,
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.keys[key] = apiKey
	s.mu.Unlock()

	return apiKey, nil
}

// Get retrieves a key by its full key string.
func (s *FileKeyStore) Get(key string) (*APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKey, ok := s.keys[key]
	return apiKey, ok
}

// List returns all stored keys.
func (s *FileKeyStore) List() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*APIKey, 0, len(s.keys))
	for _, key := range s.keys {
		keys = append(keys, key)
	}
	return keys
}

// Revoke removes a key by its full key string.
func (s *FileKeyStore) Revoke(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[key]; !ok {
		return fmt.Errorf("key not found: %s", maskKey(key))
	}

	delete(s.keys, key)
	return nil
}

// GenerateAPIKey generates a new API key in the format devnet_<32-char-hex>.
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, KeyRandomBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return KeyPrefix + hex.EncodeToString(bytes), nil
}

// IsValidKeyFormat checks if a key string has the correct format.
func IsValidKeyFormat(key string) bool {
	if len(key) != len(KeyPrefix)+KeyRandomBytes*2 {
		return false
	}
	if key[:len(KeyPrefix)] != KeyPrefix {
		return false
	}
	// Check that the rest is valid hex
	_, err := hex.DecodeString(key[len(KeyPrefix):])
	return err == nil
}

// maskKey returns a masked version of the key for logging (shows first/last few chars).
func maskKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:10] + "..." + key[len(key)-4:]
}
