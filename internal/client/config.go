// Package client provides the dvb CLI client for connecting to devnetd.
package client

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ClientConfig represents the dvb CLI configuration stored in ~/.dvb/config.yaml.
type ClientConfig struct {
	// Server is the remote devnetd server address (e.g., "devnetd.example.com:9000").
	// If empty, the local Unix socket is used.
	Server string `yaml:"server,omitempty"`

	// APIKey is the API key for authenticating with the remote server.
	// Expected format: "devnet_<32-char-random>".
	APIKey string `yaml:"api_key,omitempty"`

	// Namespace is the default namespace for commands.
	Namespace string `yaml:"namespace,omitempty"`
}

// configFilePath returns the path to the config file (~/.dvb/config.yaml).
func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".dvb", "config.yaml"), nil
}

// LoadConfig reads the client configuration from ~/.dvb/config.yaml.
// Returns an empty config (not error) if the file doesn't exist.
func LoadConfig() (*ClientConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Return empty config if file doesn't exist
		return &ClientConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ClientConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the configuration to ~/.dvb/config.yaml.
func (c *ClientConfig) Save() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Get retrieves a configuration value by key.
// Supported keys: "server", "api-key", "namespace".
func (c *ClientConfig) Get(key string) string {
	switch key {
	case "server":
		return c.Server
	case "api-key":
		return c.APIKey
	case "namespace":
		return c.Namespace
	default:
		return ""
	}
}

// Set sets a configuration value by key.
// Supported keys: "server", "api-key", "namespace".
// Returns an error for unknown keys.
func (c *ClientConfig) Set(key, value string) error {
	switch key {
	case "server":
		c.Server = value
	case "api-key":
		c.APIKey = value
	case "namespace":
		c.Namespace = value
	default:
		return fmt.Errorf("unknown config key: %s (supported: server, api-key, namespace)", key)
	}
	return nil
}

// IsRemote returns true if the config specifies a remote server.
func (c *ClientConfig) IsRemote() bool {
	return c.Server != ""
}
