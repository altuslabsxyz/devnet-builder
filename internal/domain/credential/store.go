// Package credential provides secure credential management interfaces.
package credential

import "errors"

// =============================================================================
// Domain Interfaces (Dependency Inversion Principle)
// =============================================================================
// These interfaces define how credentials should be stored and retrieved.
// Implementations in the infrastructure layer handle the actual storage
// mechanism (keychain, environment, file, etc.)

// Common errors for credential operations.
var (
	ErrCredentialNotFound = errors.New("credential not found")
	ErrStorageUnavailable = errors.New("credential storage unavailable")
	ErrInvalidCredential  = errors.New("invalid credential format")
)

// CredentialType identifies the type of credential.
type CredentialType string

const (
	// TypeGitHubToken is a GitHub Personal Access Token.
	TypeGitHubToken CredentialType = "github-token"

	// TypeDockerRegistry is a Docker registry credential.
	TypeDockerRegistry CredentialType = "docker-registry"
)

// Credential represents a stored credential.
type Credential struct {
	Type   CredentialType
	Value  string
	Source Source
}

// Source indicates where a credential was retrieved from.
type Source string

const (
	SourceKeychain    Source = "keychain"     // System keychain (most secure)
	SourceEnvironment Source = "environment"  // Environment variable
	SourceConfigFile  Source = "config-file"  // Config file (least secure)
	SourceNone        Source = "none"         // Not found
)

// SecurityLevel returns the security level of the source (higher is better).
func (s Source) SecurityLevel() int {
	switch s {
	case SourceKeychain:
		return 3
	case SourceEnvironment:
		return 2
	case SourceConfigFile:
		return 1
	default:
		return 0
	}
}

// Store defines the interface for credential storage.
// Implementations must be thread-safe.
type Store interface {
	// Get retrieves a credential by type.
	// Returns ErrCredentialNotFound if not found.
	Get(credType CredentialType) (*Credential, error)

	// Set stores a credential.
	// Returns ErrStorageUnavailable if storage is not accessible.
	Set(credType CredentialType, value string) error

	// Delete removes a credential.
	Delete(credType CredentialType) error

	// Source returns the storage source type.
	Source() Source

	// IsAvailable checks if this store is available on the current system.
	IsAvailable() bool
}

// Resolver resolves credentials from multiple sources with priority.
type Resolver interface {
	// Resolve finds a credential, checking sources in priority order.
	// Returns the credential and its source.
	Resolve(credType CredentialType) (*Credential, error)

	// ResolveWithWarning resolves and warns if using insecure source.
	ResolveWithWarning(credType CredentialType) (*Credential, string, error)
}

// =============================================================================
// Validation Helpers
// =============================================================================

// ValidateGitHubToken checks if a token looks like a valid GitHub token.
func ValidateGitHubToken(token string) error {
	if token == "" {
		return ErrInvalidCredential
	}

	// GitHub tokens have specific prefixes
	validPrefixes := []string{
		"ghp_", // Personal access tokens
		"gho_", // OAuth access tokens
		"ghu_", // User-to-server tokens
		"ghs_", // Server-to-server tokens
		"ghr_", // Refresh tokens
	}

	for _, prefix := range validPrefixes {
		if len(token) > len(prefix) && token[:len(prefix)] == prefix {
			return nil
		}
	}

	// Also accept classic tokens (40 hex chars) for backward compatibility
	if len(token) == 40 && isHexString(token) {
		return nil
	}

	return ErrInvalidCredential
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
