package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/internal/domain/credential"
	infracred "github.com/altuslabsxyz/devnet-builder/internal/infrastructure/credential"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
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
	"github_token":    "github-token",
	"GITHUB_TOKEN":    "github-token",
	"cache_ttl":       "cache-ttl",
	"network_version": "network-version",
	"stable_version":  "network-version",
}

var setUseConfigFile bool

func normalizeKey(key string) string {
	if canonical, ok := keyAliases[key]; ok {
		return canonical
	}
	return key
}

// NewSetCmd creates the config set subcommand.
func NewSetCmd() *cobra.Command {
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
  network-version Default network version

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
		RunE: runSet,
	}

	cmd.Flags().BoolVar(&setUseConfigFile, "use-config-file", false,
		"Store in config file instead of system keychain (less secure)")

	return cmd
}

func runSet(cmd *cobra.Command, args []string) error {
	cfg := ctxconfig.FromContext(cmd.Context())
	key := normalizeKey(args[0])
	var value string

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

	if secretKeys[key] && !setUseConfigFile {
		return setSecretValue(cfg.HomeDir(), key, value)
	}

	return setConfigFileValue(cfg.HomeDir(), key, value)
}

func setSecretValue(homeDir, key, value string) error {
	credType := keyToCredentialType(key)
	if credType == "" {
		return fmt.Errorf("unknown secret key: %s", key)
	}

	if credType == credential.TypeGitHubToken {
		if err := credential.ValidateGitHubToken(value); err != nil {
			output.Warn("Token doesn't appear to be a valid GitHub token (expected ghp_*, gho_*, etc.)")
		}
	}

	keychain := infracred.NewKeychainStore()
	if keychain.IsAvailable() {
		if err := keychain.Set(credType, value); err != nil {
			output.Warn("Failed to store in system keychain: %v", err)
			output.Info("Falling back to config file storage...")
			return setConfigFileValueForSecret(homeDir, key, value)
		}

		output.Success("Securely stored %s in system keychain", key)
		output.Info("Storage: %s", getKeychainDescription())

		if err := removeSecretFromConfigFile(homeDir, key); err != nil {
			output.Debug("Note: Could not remove old value from config file: %v", err)
		}

		return nil
	}

	output.Warn("System keychain not available on this system")
	output.Info("Storing in config file (less secure)")
	return setConfigFileValueForSecret(homeDir, key, value)
}

func setConfigFileValue(homeDir, key, value string) error {
	configFile := filepath.Join(homeDir, "config.toml")

	var cfg config.FileConfig
	data, err := os.ReadFile(configFile)
	if err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	switch key {
	case "cache-ttl":
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("invalid cache-ttl format: %w (expected duration like '1h', '30m')", err)
		}
		cfg.CacheTTL = &value

	case "network":
		if !types.NetworkSource(value).IsValid() {
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
		if !types.ExecutionMode(value).IsValid() {
			return fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", value)
		}
		mode := types.ExecutionMode(value)
		cfg.ExecutionMode = &mode

	case "network-version":
		cfg.NetworkVersion = &value

	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	return writeConfigFile(configFile, &cfg, value, key)
}

func setConfigFileValueForSecret(homeDir, key, value string) error {
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

func writeConfigFile(configFile string, cfg *config.FileConfig, displayValue, key string) error {
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	newData, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

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

func removeSecretFromConfigFile(homeDir, key string) error {
	configFile := filepath.Join(homeDir, "config.toml")

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil
	}

	var cfg config.FileConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return err
	}

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

func keyToCredentialType(key string) credential.CredentialType {
	switch key {
	case "github-token":
		return credential.TypeGitHubToken
	default:
		return ""
	}
}

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

func promptSecretValue(key string) (string, error) {
	fmt.Printf("Enter %s: ", key)
	byteValue, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("failed to read secret input: %w", err)
	}
	value := strings.TrimSpace(string(byteValue))
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}
	return value, nil
}

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
