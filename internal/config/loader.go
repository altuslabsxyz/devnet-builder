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

// LoadFileConfig loads and parses the config.toml file.
// Returns the FileConfig and the path to the loaded file.
// If no config file is found, returns an empty FileConfig with empty path.
func (l *ConfigLoader) LoadFileConfig() (*FileConfig, string, error) {
	configFile, err := l.findConfigFile()
	if err != nil {
		return nil, "", err
	}

	if configFile == "" {
		// No config file found - return empty config
		return &FileConfig{}, "", nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	var cfg FileConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		// Try to provide more helpful error message with line numbers
		return nil, "", fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	// Validate config values
	if err := ValidateFileConfig(&cfg); err != nil {
		return nil, "", fmt.Errorf("config validation failed in %s: %w", configFile, err)
	}

	// Warn about unknown keys
	l.warnUnknownKeys(data)

	if l.logger != nil {
		l.logger.Debug("Loaded config file: %s", configFile)
	}

	return &cfg, configFile, nil
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
	}

	for key := range raw {
		if !knownKeys[key] {
			l.logger.Warn("Unknown config key: %s", key)
		}
	}
}
