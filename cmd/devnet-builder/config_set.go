package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/domain/credential"
	infracred "github.com/b-harvest/devnet-builder/internal/infrastructure/credential"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// secretKeys defines which keys should use hidden input and secure storage.
var secretKeys = map[string]bool{
	"github-token": true,
}

// keyAliases maps alternative key names to canonical names.
var keyAliases = map[string]string{
	"github_token":   "github-token",
	"GITHUB_TOKEN":   "github-token",
	"cache_ttl":      "cache-ttl",
	"stable_version": "stable-version",
}

// Command flags
var (
	configSetUseConfigFile bool // Force storage in config file instead of keychain
)

// normalizeKey converts a key to its canonical form.
func normalizeKey(key string) string {
	if canonical, ok := keyAliases[key]; ok {
		return canonical
	}
	return key
}

// NewConfigSetCmd creates the config set subcommand.
func NewConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set a configuration value",
		Long: `Set a configuration value securely.

For sensitive values (like github-token), the system keychain is used by default:
  - macOS: Keychain Access
  - Linux: Secret Service (GNOME Keyring, KWallet)
  - Windows: Windows Credential Manager

Available keys:
  github-token   GitHub Personal Access Token (stored in keychain)
  cache-ttl      Version cache TTL (e.g., "1h", "30m", "2h")
  network        Default network (mainnet, testnet)
  validators     Default number of validators (1-4)
  mode           Default execution mode (docker, local)
  stable-version Default stable version

Examples:
  # Set GitHub token securely (uses system keychain, prompts for hidden input)
  devnet-builder config set github-token

  # Set GitHub token directly
  devnet-builder config set github-token ghp_xxxxxxxxxxxx

  # Force storage in config file (less secure, not recommended)
  devnet-builder config set github-token --use-config-file

  # Set cache TTL to 2 hours
  devnet-builder config set cache-ttl 2h

  # Set default network
  devnet-builder config set network testnet`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runConfigSet,
	}

	cmd.Flags().BoolVar(&configSetUseConfigFile, "use-config-file", false,
		"Store in config file instead of system keychain (less secure)")

	return cmd
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := normalizeKey(args[0])
	var value string

	// If value is not provided, prompt for it
	if len(args) < 2 {
		var err error
		if secretKeys[key] {
			value, err = promptSecretValue(key)
		} else {
			value, err = promptValue(key)
		}
		if err != nil {
			return err
		}
	} else {
		value = args[1]
	}

	// Handle secret keys (like github-token) with secure storage
	if secretKeys[key] && !configSetUseConfigFile {
		return setSecretValue(key, value)
	}

	// Non-secret keys: store in config file
	return setConfigFileValue(key, value)
}

// setSecretValue stores a secret in the system keychain.
func setSecretValue(key, value string) error {
	// Validate the credential
	credType := keyToCredentialType(key)
	if credType == "" {
		return fmt.Errorf("unknown secret key: %s", key)
	}

	// Validate token format for GitHub tokens
	if credType == credential.TypeGitHubToken {
		if err := credential.ValidateGitHubToken(value); err != nil {
			output.Warn("Token doesn't appear to be a valid GitHub token (expected ghp_*, gho_*, etc.)")
		}
	}

	// Try to store in keychain
	keychain := infracred.NewKeychainStore()
	if keychain.IsAvailable() {
		if err := keychain.Set(credType, value); err != nil {
			output.Warn("Failed to store in system keychain: %v", err)
			output.Info("Falling back to config file storage...")
			return setConfigFileValueForSecret(key, value)
		}

		output.Success("Securely stored %s in system keychain", key)
		output.Info("Storage: %s", getKeychainDescription())

		// Remove from config file if present (migrate to secure storage)
		if err := removeSecretFromConfigFile(key); err != nil {
			output.Debug("Note: Could not remove old value from config file: %v", err)
		}

		return nil
	}

	// Keychain not available, warn and use config file
	output.Warn("System keychain not available on this system")
	output.Info("Storing in config file (less secure)")
	return setConfigFileValueForSecret(key, value)
}

// setConfigFileValue stores a non-secret value in config.toml.
func setConfigFileValue(key, value string) error {
	configFile := filepath.Join(homeDir, "config.toml")

	// Load existing config or create new
	var cfg config.FileConfig
	data, err := os.ReadFile(configFile)
	if err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Set the value based on key
	switch key {
	case "cache-ttl":
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("invalid cache-ttl format: %w (expected duration like '1h', '30m')", err)
		}
		cfg.CacheTTL = &value

	case "network":
		if value != "mainnet" && value != "testnet" {
			return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", value)
		}
		cfg.Network = &value

	case "validators":
		var validators int
		if _, err := fmt.Sscanf(value, "%d", &validators); err != nil {
			return fmt.Errorf("invalid validators: %s (must be 1-4)", value)
		}
		if validators < 1 || validators > 4 {
			return fmt.Errorf("invalid validators: %d (must be 1-4)", validators)
		}
		cfg.Validators = &validators

	case "mode":
		if value != "docker" && value != "local" {
			return fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", value)
		}
		cfg.Mode = &value

	case "stable-version":
		cfg.StableVersion = &value

	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return writeConfigFile(configFile, &cfg, value, key)
}

// setConfigFileValueForSecret stores a secret in config file (fallback).
func setConfigFileValueForSecret(key, value string) error {
	configFile := filepath.Join(homeDir, "config.toml")

	var cfg config.FileConfig
	data, err := os.ReadFile(configFile)
	if err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	switch key {
	case "github-token":
		cfg.GitHubToken = &value
	default:
		return fmt.Errorf("unknown secret key: %s", key)
	}

	output.Warn("Storing sensitive data in config file is less secure than system keychain")

	return writeConfigFile(configFile, &cfg, maskToken(value), key)
}

// writeConfigFile writes the config and displays success message.
func writeConfigFile(configFile string, cfg *config.FileConfig, displayValue, key string) error {
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	newData, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Use restrictive permissions if secrets are present
	perm := os.FileMode(0644)
	if cfg.GitHubToken != nil && *cfg.GitHubToken != "" {
		perm = 0600
	}

	if err := os.WriteFile(configFile, newData, perm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	output.Success("Set %s = %s", key, displayValue)
	output.Info("Config saved to: %s", configFile)

	return nil
}

// removeSecretFromConfigFile removes a secret from config file after migrating to keychain.
func removeSecretFromConfigFile(key string) error {
	configFile := filepath.Join(homeDir, "config.toml")

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil // No config file, nothing to remove
	}

	var cfg config.FileConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	// Clear the secret
	if key == "github-token" {
		if cfg.GitHubToken != nil {
			cfg.GitHubToken = nil
			newData, err := toml.Marshal(&cfg)
			if err != nil {
				return err
			}
			return os.WriteFile(configFile, newData, 0644)
		}
	}

	return nil
}

// keyToCredentialType maps config keys to credential types.
func keyToCredentialType(key string) credential.CredentialType {
	switch key {
	case "github-token":
		return credential.TypeGitHubToken
	default:
		return ""
	}
}

// getKeychainDescription returns a user-friendly description of the keychain.
func getKeychainDescription() string {
	switch {
	case isMacOS():
		return "macOS Keychain Access"
	case isLinux():
		return "Secret Service (GNOME Keyring / KWallet)"
	case isWindows():
		return "Windows Credential Manager"
	default:
		return "System Keychain"
	}
}

func isMacOS() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OSTYPE")), "darwin") ||
		fileExists("/System/Library/CoreServices/SystemVersion.plist")
}

func isLinux() bool {
	return fileExists("/etc/os-release")
}

func isWindows() bool {
	return os.Getenv("OS") == "Windows_NT"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isValidGitHubToken checks if a token looks like a valid GitHub token.
func isValidGitHubToken(token string) bool {
	// GitHub classic tokens start with ghp_
	// GitHub fine-grained tokens start with github_pat_
	return strings.HasPrefix(token, "ghp_") || strings.HasPrefix(token, "github_pat_")
}

// maskToken masks most of the token for display.
func maskToken(token string) string {
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// promptSecretValue prompts for a secret value with hidden input.
func promptSecretValue(key string) (string, error) {
	fmt.Printf("Enter %s: ", key)
	byteValue, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Print newline after hidden input
	if err != nil {
		return "", fmt.Errorf("failed to read secret input: %w", err)
	}
	value := strings.TrimSpace(string(byteValue))
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}
	return value, nil
}

// promptValue prompts for a regular value with visible input.
func promptValue(key string) (string, error) {
	fmt.Printf("Enter %s: ", key)
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}
	return value, nil
}
