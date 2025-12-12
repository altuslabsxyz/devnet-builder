package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/config"
	"github.com/stablelabs/stable-devnet/internal/output"
	"golang.org/x/term"
)

// secretKeys defines which keys should use hidden input when value is not provided.
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
		Long: `Set a configuration value in the config file.

Available keys:
  github-token   GitHub Personal Access Token for private repos
  cache-ttl      Version cache TTL (e.g., "1h", "30m", "2h")
  network        Default network (mainnet, testnet)
  validators     Default number of validators (1-4)
  mode           Default execution mode (docker, local)
  stable-version Default stable version

Examples:
  # Set GitHub token (prompts for hidden input)
  devnet-builder config set github-token

  # Set GitHub token directly
  devnet-builder config set github-token ghp_xxxxxxxxxxxx

  # Set cache TTL to 2 hours
  devnet-builder config set cache-ttl 2h

  # Set default network
  devnet-builder config set network testnet`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runConfigSet,
	}

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

	// Find or create config file
	configFile := filepath.Join(homeDir, "config.toml")

	// Load existing config or create new
	var cfg config.FileConfig
	data, err := os.ReadFile(configFile)
	if err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Set the value
	switch key {
	case "github-token":
		// Validate token format
		if !isValidGitHubToken(value) {
			output.Warn("Token doesn't appear to be a valid GitHub token (expected ghp_* or github_pat_*)")
		}
		cfg.GitHubToken = &value

	case "cache-ttl":
		// Validate TTL format
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
		// Parse as int
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

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config
	newData, err := toml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config file with appropriate permissions
	// Use 0600 if token is present for security
	perm := os.FileMode(0644)
	if cfg.GitHubToken != nil && *cfg.GitHubToken != "" {
		perm = 0600
	}

	if err := os.WriteFile(configFile, newData, perm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Display success message
	displayValue := value
	if key == "github-token" {
		displayValue = maskToken(value)
	}
	output.Success("Set %s = %s", key, displayValue)
	output.Info("Config saved to: %s", configFile)

	return nil
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
