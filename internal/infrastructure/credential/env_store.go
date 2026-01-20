package credential

import (
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/domain/credential"
)

// =============================================================================
// Environment Variable Store (Read-Only)
// =============================================================================
// Reads credentials from environment variables.
// This is read-only - use shell configuration to set values.

// envVarMapping maps credential types to environment variable names.
var envVarMapping = map[credential.CredentialType]string{
	credential.TypeGitHubToken:    "GITHUB_TOKEN",
	credential.TypeDockerRegistry: "DOCKER_TOKEN",
}

// EnvironmentStore reads credentials from environment variables.
type EnvironmentStore struct{}

// NewEnvironmentStore creates a new environment variable credential store.
func NewEnvironmentStore() *EnvironmentStore {
	return &EnvironmentStore{}
}

// Get retrieves a credential from environment variables.
func (s *EnvironmentStore) Get(credType credential.CredentialType) (*credential.Credential, error) {
	envVar, ok := envVarMapping[credType]
	if !ok {
		return nil, credential.ErrCredentialNotFound
	}

	value := os.Getenv(envVar)
	if value == "" {
		return nil, credential.ErrCredentialNotFound
	}

	return &credential.Credential{
		Type:   credType,
		Value:  value,
		Source: credential.SourceEnvironment,
	}, nil
}

// Set is not supported for environment variables.
// Users should set environment variables in their shell configuration.
func (s *EnvironmentStore) Set(credType credential.CredentialType, value string) error {
	// Environment variables are read-only from the application's perspective
	return credential.ErrStorageUnavailable
}

// Delete is not supported for environment variables.
func (s *EnvironmentStore) Delete(credType credential.CredentialType) error {
	return credential.ErrStorageUnavailable
}

// Source returns the storage source type.
func (s *EnvironmentStore) Source() credential.Source {
	return credential.SourceEnvironment
}

// IsAvailable always returns true since environment is always available.
func (s *EnvironmentStore) IsAvailable() bool {
	return true
}

// GetEnvVarName returns the environment variable name for a credential type.
func GetEnvVarName(credType credential.CredentialType) string {
	return envVarMapping[credType]
}

// Ensure EnvironmentStore implements Store interface.
var _ credential.Store = (*EnvironmentStore)(nil)
