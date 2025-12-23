package credential

import (
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/domain/credential"
)

// =============================================================================
// Credential Resolver (Chain of Responsibility Pattern)
// =============================================================================
// Resolves credentials from multiple sources in priority order:
// 1. System Keychain (most secure)
// 2. Environment Variables
// 3. Config File (least secure - deprecated)

// ChainResolver resolves credentials from multiple stores in priority order.
type ChainResolver struct {
	stores []credential.Store
}

// NewChainResolver creates a resolver with default priority order.
// Priority: Keychain > Environment > (Config file handled separately)
func NewChainResolver() *ChainResolver {
	stores := []credential.Store{}

	// Add keychain if available
	keychain := NewKeychainStore()
	if keychain.IsAvailable() {
		stores = append(stores, keychain)
	}

	// Add environment store
	stores = append(stores, NewEnvironmentStore())

	return &ChainResolver{stores: stores}
}

// NewChainResolverWithStores creates a resolver with custom stores.
func NewChainResolverWithStores(stores ...credential.Store) *ChainResolver {
	return &ChainResolver{stores: stores}
}

// Resolve finds a credential by checking each store in priority order.
func (r *ChainResolver) Resolve(credType credential.CredentialType) (*credential.Credential, error) {
	for _, store := range r.stores {
		cred, err := store.Get(credType)
		if err == nil {
			return cred, nil
		}
		// Continue to next store if not found
		if err == credential.ErrCredentialNotFound {
			continue
		}
		// Log other errors but continue
	}

	return nil, credential.ErrCredentialNotFound
}

// ResolveWithWarning resolves and returns a security warning if applicable.
func (r *ChainResolver) ResolveWithWarning(credType credential.CredentialType) (*credential.Credential, string, error) {
	cred, err := r.Resolve(credType)
	if err != nil {
		return nil, "", err
	}

	var warning string
	switch cred.Source {
	case credential.SourceConfigFile:
		warning = fmt.Sprintf(`Security Warning: %s is stored in config file (plaintext).
For better security, migrate to system keychain:
  devnet-builder config set %s --migrate-to-keychain`, credType, credType)
	case credential.SourceEnvironment:
		// Environment variables are acceptable but keychain is better
		if r.hasKeychain() {
			warning = fmt.Sprintf(`Tip: For enhanced security, store %s in system keychain:
  devnet-builder config set %s`, credType, credType)
		}
	}

	return cred, warning, nil
}

// PreferredStore returns the most secure available store for setting credentials.
func (r *ChainResolver) PreferredStore() credential.Store {
	for _, store := range r.stores {
		if store.IsAvailable() && store.Source() == credential.SourceKeychain {
			return store
		}
	}
	// Fallback to first available store
	for _, store := range r.stores {
		if store.IsAvailable() {
			return store
		}
	}
	return nil
}

// hasKeychain checks if keychain is in the store chain.
func (r *ChainResolver) hasKeychain() bool {
	for _, store := range r.stores {
		if store.Source() == credential.SourceKeychain {
			return true
		}
	}
	return false
}

// AvailableSources returns all available storage sources.
func (r *ChainResolver) AvailableSources() []credential.Source {
	var sources []credential.Source
	for _, store := range r.stores {
		if store.IsAvailable() {
			sources = append(sources, store.Source())
		}
	}
	return sources
}

// Ensure ChainResolver implements Resolver interface.
var _ credential.Resolver = (*ChainResolver)(nil)
