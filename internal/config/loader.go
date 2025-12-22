package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/pelletier/go-toml/v2"
)

// ConfigLoader is responsible for loading and merging configuration.
type ConfigLoader struct {
	homeDir    string
	configPath string // Explicit --config path
	logger     *output.Logger
}

// NewConfigLoader creates a new ConfigLoader.
func NewConfigLoader(homeDir, configPath string, logger *output.Logger) *ConfigLoader {
	return &ConfigLoader{
		homeDir:    homeDir,
		configPath: configPath,
		logger:     logger,
	}
}

// findConfigFile searches for config.toml in the following order:
// 1. Explicit path (--config flag)
// 2. Home directory (~/.devnet-builder/config.toml)
//
// Note: Current directory (./config.toml) is intentionally NOT checked.
// Use --config flag to specify a custom config file location.
// Returns empty string if no config file found (not an error).
func (l *ConfigLoader) findConfigFile() (string, error) {
	// 1. Explicit path
	if l.configPath != "" {
		if _, err := os.Stat(l.configPath); err != nil {
			return "", fmt.Errorf("config file not found: %s", l.configPath)
		}
		return l.configPath, nil
	}

	// 2. Home directory only
	homePath := filepath.Join(l.homeDir, "config.toml")
	if _, err := os.Stat(homePath); err == nil {
		return homePath, nil
	}

	return "", nil // No config file found (not an error)
}

// LoadFileConfig loads and parses config files from homeDir or explicit path.
// Priority: explicit path (--config) > ~/.devnet-builder/config.toml
//
// Note: Current directory (./config.toml) is intentionally NOT checked.
// Use --config flag to specify a custom config file location.
// Returns the merged FileConfig and the config file path.
func (l *ConfigLoader) LoadFileConfig() (*FileConfig, string, error) {
	// Collect config files in order of increasing priority
	var configFiles []string

	// 2. Home directory (lower priority)
	homePath := filepath.Join(l.homeDir, "config.toml")
	if _, err := os.Stat(homePath); err == nil {
		configFiles = append(configFiles, homePath)
	}

	// 1. Explicit path (highest priority)
	if l.configPath != "" {
		if _, err := os.Stat(l.configPath); err != nil {
			return nil, "", fmt.Errorf("config file not found: %s", l.configPath)
		}
		// Don't add duplicates
		absPath, _ := filepath.Abs(l.configPath)
		isDuplicate := false
		for _, cf := range configFiles {
			if abs, _ := filepath.Abs(cf); abs == absPath {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			configFiles = append(configFiles, l.configPath)
		}
	}

	if len(configFiles) == 0 {
		// No config file found - return empty config
		return &FileConfig{}, "", nil
	}

	// Load and merge all configs (later files override earlier ones)
	var merged FileConfig
	var primaryFile string
	for _, configFile := range configFiles {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read config file %s: %w", configFile, err)
		}

		var cfg FileConfig
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, "", fmt.Errorf("invalid TOML syntax in %s: %w\nPlease check the file for syntax errors", configFile, err)
		}

		// Merge: only set values that are not nil in cfg
		mergeFileConfig(&merged, &cfg)
		primaryFile = configFile

		// Warn about unknown keys
		l.warnUnknownKeys(data)

		if l.logger != nil {
			l.logger.Debug("Loaded config file: %s", configFile)
		}
	}

	// Validate merged config
	if err := ValidateFileConfig(&merged); err != nil {
		return nil, "", fmt.Errorf("config validation failed: %w", err)
	}

	return &merged, primaryFile, nil
}

// mergeFileConfig merges src into dst. Non-nil values in src overwrite dst.
func mergeFileConfig(dst, src *FileConfig) {
	if src.Home != nil {
		dst.Home = src.Home
	}
	if src.NoColor != nil {
		dst.NoColor = src.NoColor
	}
	if src.Verbose != nil {
		dst.Verbose = src.Verbose
	}
	if src.JSON != nil {
		dst.JSON = src.JSON
	}
	if src.Network != nil {
		dst.Network = src.Network
	}
	if src.BlockchainNetwork != nil {
		dst.BlockchainNetwork = src.BlockchainNetwork
	}
	if src.Validators != nil {
		dst.Validators = src.Validators
	}
	if src.Mode != nil {
		dst.Mode = src.Mode
	}
	if src.StableVersion != nil {
		dst.StableVersion = src.StableVersion
	}
	if src.NetworkVersion != nil {
		dst.NetworkVersion = src.NetworkVersion
	}
	if src.NoCache != nil {
		dst.NoCache = src.NoCache
	}
	if src.Accounts != nil {
		dst.Accounts = src.Accounts
	}
	if src.GitHubToken != nil {
		dst.GitHubToken = src.GitHubToken
	}
	if src.CacheTTL != nil {
		dst.CacheTTL = src.CacheTTL
	}
}

// warnUnknownKeys checks for unknown keys in the config file and logs warnings.
func (l *ConfigLoader) warnUnknownKeys(data []byte) {
	if l.logger == nil {
		return
	}

	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return // Ignore errors here - main parsing will catch them
	}

	knownKeys := map[string]bool{
		"home":               true,
		"no_color":           true,
		"verbose":            true,
		"json":               true,
		"network":            true,
		"blockchain_network": true,
		"validators":         true,
		"mode":               true,
		"stable_version":     true,
		"network_version":    true,
		"no_cache":           true,
		"accounts":           true,
		"github_token":       true,
		"cache_ttl":          true,
	}

	for key := range raw {
		if !knownKeys[key] {
			l.logger.Warn("Unknown config key: %s", key)
		}
	}
}
