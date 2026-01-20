// Package credential provides credential storage implementations.
package credential

import (
	"github.com/altuslabsxyz/devnet-builder/internal/domain/credential"
	"github.com/zalando/go-keyring"
)

const (
	// ServiceName is the keychain service identifier.
	ServiceName = "devnet-builder"
)

// =============================================================================
// Keychain Store (Most Secure)
// =============================================================================
// Uses the system keychain:
// - macOS: Keychain Access
// - Linux: Secret Service API (GNOME Keyring, KWallet)
// - Windows: Windows Credential Manager

// KeychainStore stores credentials in the system keychain.
type KeychainStore struct {
	serviceName string
}

// NewKeychainStore creates a new keychain-based credential store.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{
		serviceName: ServiceName,
	}
}

// Get retrieves a credential from the system keychain.
func (s *KeychainStore) Get(credType credential.CredentialType) (*credential.Credential, error) {
	value, err := keyring.Get(s.serviceName, string(credType))
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, credential.ErrCredentialNotFound
		}
		return nil, credential.ErrStorageUnavailable
	}

	return &credential.Credential{
		Type:   credType,
		Value:  value,
		Source: credential.SourceKeychain,
	}, nil
}

// Set stores a credential in the system keychain.
func (s *KeychainStore) Set(credType credential.CredentialType, value string) error {
	err := keyring.Set(s.serviceName, string(credType), value)
	if err != nil {
		return credential.ErrStorageUnavailable
	}
	return nil
}

// Delete removes a credential from the system keychain.
func (s *KeychainStore) Delete(credType credential.CredentialType) error {
	err := keyring.Delete(s.serviceName, string(credType))
	if err != nil {
		if err == keyring.ErrNotFound {
			return credential.ErrCredentialNotFound
		}
		return credential.ErrStorageUnavailable
	}
	return nil
}

// Source returns the storage source type.
func (s *KeychainStore) Source() credential.Source {
	return credential.SourceKeychain
}

// IsAvailable checks if keychain is available on this system.
func (s *KeychainStore) IsAvailable() bool {
	// Try to access keychain with a non-existent key
	// If we get ErrNotFound, keychain is available
	// If we get a different error, it's not available
	_, err := keyring.Get(s.serviceName, "__test_availability__")
	return err == nil || err == keyring.ErrNotFound
}

// Ensure KeychainStore implements Store interface.
var _ credential.Store = (*KeychainStore)(nil)
