package main

import (
	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/domain/credential"
	infracred "github.com/b-harvest/devnet-builder/internal/infrastructure/credential"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// =============================================================================
// Credential Helper (Presentation Layer)
// =============================================================================
// Provides unified credential resolution for CLI commands.
// Checks multiple sources in priority order:
// 1. System Keychain (most secure)
// 2. Environment Variables
// 3. Config File (least secure)

// resolveGitHubToken resolves the GitHub token from available sources.
// Returns the token and a boolean indicating if it was found.
func resolveGitHubToken(fileCfg *config.FileConfig) (string, bool) {
	// Priority 1 & 2: Try keychain and environment via resolver
	resolver := infracred.NewChainResolver()
	cred, warning, err := resolver.ResolveWithWarning(credential.TypeGitHubToken)
	if err == nil && cred != nil {
		// Show security warning if applicable
		if warning != "" && verbose {
			output.Warn(warning)
		}
		return cred.Value, true
	}

	// Priority 3: Fall back to config file
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		if verbose {
			output.Warn("Using GitHub token from config file (consider migrating to keychain)")
		}
		return *fileCfg.GitHubToken, true
	}

	return "", false
}

// getGitHubTokenSource returns the source of the GitHub token.
// Useful for displaying in config show.
func getGitHubTokenSource(fileCfg *config.FileConfig) (string, credential.Source) {
	// Check keychain first
	keychain := infracred.NewKeychainStore()
	if keychain.IsAvailable() {
		if cred, err := keychain.Get(credential.TypeGitHubToken); err == nil {
			return maskToken(cred.Value), credential.SourceKeychain
		}
	}

	// Check environment
	envStore := infracred.NewEnvironmentStore()
	if cred, err := envStore.Get(credential.TypeGitHubToken); err == nil {
		return maskToken(cred.Value), credential.SourceEnvironment
	}

	// Check config file
	if fileCfg != nil && fileCfg.GitHubToken != nil && *fileCfg.GitHubToken != "" {
		return maskToken(*fileCfg.GitHubToken), credential.SourceConfigFile
	}

	return "", credential.SourceNone
}

// hasSecureCredentialStorage returns true if system keychain is available.
func hasSecureCredentialStorage() bool {
	keychain := infracred.NewKeychainStore()
	return keychain.IsAvailable()
}
