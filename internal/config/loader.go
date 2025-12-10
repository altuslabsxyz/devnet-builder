package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/stablelabs/stable-devnet/internal/output"
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
// 2. Current directory (./config.toml)
// 3. Home directory (~/.stable-devnet/config.toml)
// Returns empty string if no config file found (not an error).
func (l *ConfigLoader) findConfigFile() (string, error) {
	// 1. Explicit path
	if l.configPath != "" {
		if _, err := os.Stat(l.configPath); err != nil {
			return "", fmt.Errorf("config file not found: %s", l.configPath)
		}
		return l.configPath, nil
	}

	// 2. Current directory
	if _, err := os.Stat("./config.toml"); err == nil {
		return "./config.toml", nil
	}

	// 3. Home directory
	homePath := filepath.Join(l.homeDir, "config.toml")
	if _, err := os.Stat(homePath); err == nil {
		return homePath, nil
	}

	return "", nil // No config file found (not an error)
}

// LoadFileConfig loads and parses config files, merging them in priority order.
// Priority: explicit path > ./config.toml > ~/.stable-devnet/config.toml
// All config files are merged, with higher priority values overwriting lower ones.
// Returns the merged FileConfig and the primary (highest priority) config file path.
func (l *ConfigLoader) LoadFileConfig() (*FileConfig, string, error) {
	// Collect all config files in order of increasing priority
	var configFiles []string

	// 3. Home directory (lowest priority)
	homePath := filepath.Join(l.homeDir, "config.toml")
	if _, err := os.Stat(homePath); err == nil {
		configFiles = append(configFiles, homePath)
	}

	// 2. Current directory
	if _, err := os.Stat("./config.toml"); err == nil {
		// Don't add if it's the same as homePath
		if absPath, _ := filepath.Abs("./config.toml"); absPath != homePath {
			configFiles = append(configFiles, "./config.toml")
		}
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
			return nil, "", fmt.Errorf("failed to parse config file %s: %w", configFile, err)
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
	if src.Validators != nil {
		dst.Validators = src.Validators
	}
	if src.Mode != nil {
		dst.Mode = src.Mode
	}
	if src.StableVersion != nil {
		dst.StableVersion = src.StableVersion
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
		"home":           true,
		"no_color":       true,
		"verbose":        true,
		"json":           true,
		"network":        true,
		"validators":     true,
		"mode":           true,
		"stable_version": true,
		"no_cache":       true,
		"accounts":       true,
		"github_token":   true,
		"cache_ttl":      true,
	}

	for key := range raw {
		if !knownKeys[key] {
			l.logger.Warn("Unknown config key: %s", key)
		}
	}
}
