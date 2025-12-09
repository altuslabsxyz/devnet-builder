package devnet

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the runtime configuration for devnet operations.
type Config struct {
	// Base directory for devnet data
	HomeDir string

	// Network configuration
	Network       string        // "mainnet" or "testnet"
	NumValidators int           // 1-4
	NumAccounts   int           // Additional funded accounts
	ExecutionMode ExecutionMode // "docker" or "local"
	StableVersion string        // Stable repository version

	// Behavior flags
	NoCache bool // Skip snapshot cache
	Force   bool // Skip confirmation prompts
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		HomeDir:       DefaultHomeDir(),
		Network:       "mainnet",
		NumValidators: 4,
		NumAccounts:   0,
		ExecutionMode: ModeDocker,
		StableVersion: "latest",
		NoCache:       false,
		Force:         false,
	}
}

// DefaultHomeDir returns the default home directory.
func DefaultHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".stable-devnet"
	}
	return filepath.Join(home, ".stable-devnet")
}

// LoadConfigFromEnv loads configuration from environment variables.
// Environment variables override default values but not CLI flags.
func LoadConfigFromEnv() *Config {
	cfg := DefaultConfig()

	// STABLE_DEVNET_HOME - Base directory
	if val := os.Getenv("STABLE_DEVNET_HOME"); val != "" {
		cfg.HomeDir = val
	}

	// STABLE_VERSION - Default stable version
	if val := os.Getenv("STABLE_VERSION"); val != "" {
		cfg.StableVersion = val
	}

	// STABLE_DEVNET_NETWORK - Default network source
	if val := os.Getenv("STABLE_DEVNET_NETWORK"); val != "" {
		if val == "mainnet" || val == "testnet" {
			cfg.Network = val
		}
	}

	// STABLE_DEVNET_MODE - Default execution mode
	if val := os.Getenv("STABLE_DEVNET_MODE"); val != "" {
		if val == "docker" || val == "local" {
			cfg.ExecutionMode = ExecutionMode(val)
		}
	}

	// STABLE_DEVNET_VALIDATORS - Default number of validators
	if val := os.Getenv("STABLE_DEVNET_VALIDATORS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n >= 1 && n <= 4 {
			cfg.NumValidators = n
		}
	}

	return cfg
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Validate network
	if c.Network != "mainnet" && c.Network != "testnet" {
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", c.Network)
	}

	// Validate validators
	if c.NumValidators < 1 || c.NumValidators > 4 {
		return fmt.Errorf("invalid num_validators: %d (must be 1-4)", c.NumValidators)
	}

	// Validate execution mode
	if c.ExecutionMode != ModeDocker && c.ExecutionMode != ModeLocal {
		return fmt.Errorf("invalid execution_mode: %s (must be 'docker' or 'local')", c.ExecutionMode)
	}

	return nil
}

// DevnetDir returns the path to the devnet directory.
func (c *Config) DevnetDir() string {
	return filepath.Join(c.HomeDir, "devnet")
}

// SnapshotsDir returns the path to the snapshots directory.
func (c *Config) SnapshotsDir() string {
	return filepath.Join(c.HomeDir, "snapshots")
}

// NodeDir returns the path to a specific node's directory.
func (c *Config) NodeDir(index int) string {
	return filepath.Join(c.DevnetDir(), fmt.Sprintf("node%d", index))
}

// GenesisPath returns the path to the genesis file.
func (c *Config) GenesisPath() string {
	return filepath.Join(c.DevnetDir(), "genesis.json")
}

// MetadataPath returns the path to the metadata file.
func (c *Config) MetadataPath() string {
	return filepath.Join(c.DevnetDir(), "metadata.json")
}

// EnsureDirectories creates all necessary directories.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.HomeDir,
		c.DevnetDir(),
		c.SnapshotsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// LoadDevnet loads an existing devnet from the configured home directory.
func LoadDevnet(homeDir string) (*DevnetMetadata, error) {
	return LoadDevnetMetadata(homeDir)
}

// SaveDevnet saves devnet metadata to the configured home directory.
func SaveDevnet(metadata *DevnetMetadata) error {
	return metadata.Save()
}

// CreateDevnet creates a new devnet with the given configuration.
func CreateDevnet(cfg *Config) (*DevnetMetadata, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Check if devnet already exists
	if DevnetExists(cfg.HomeDir) {
		return nil, fmt.Errorf("devnet already exists at %s", cfg.HomeDir)
	}

	// Create metadata
	metadata := NewDevnetMetadata(cfg.HomeDir)
	metadata.NetworkSource = cfg.Network
	metadata.NumValidators = cfg.NumValidators
	metadata.NumAccounts = cfg.NumAccounts
	metadata.ExecutionMode = cfg.ExecutionMode
	metadata.StableVersion = cfg.StableVersion
	metadata.GenesisPath = cfg.GenesisPath()

	// Validate metadata
	if err := metadata.Validate(); err != nil {
		return nil, fmt.Errorf("invalid metadata: %w", err)
	}

	// Save metadata
	if err := metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return metadata, nil
}

// RemoveDevnet removes all devnet data.
func RemoveDevnet(homeDir string) error {
	devnetDir := filepath.Join(homeDir, "devnet")
	if err := os.RemoveAll(devnetDir); err != nil {
		return fmt.Errorf("failed to remove devnet: %w", err)
	}
	return nil
}

// GetSnapshotURL returns the snapshot download URL for the given network.
func GetSnapshotURL(network string) string {
	switch network {
	case "mainnet":
		return "https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst"
	case "testnet":
		return "https://stable-testnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst"
	default:
		return ""
	}
}

// GetRPCEndpoint returns the RPC endpoint for the given network.
func GetRPCEndpoint(network string) string {
	switch network {
	case "mainnet":
		return "https://rpc.stable.xyz"
	case "testnet":
		return "https://rpc.testnet.stable.xyz"
	default:
		return ""
	}
}

// GetBundledGenesisPath returns the path to the bundled genesis file for the given network.
// It looks for genesis files in the genesis/ directory relative to the executable.
func GetBundledGenesisPath(network string) string {
	// Try to find genesis relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// Check genesis/ directory relative to executable
		genesisPath := filepath.Join(execDir, "..", "genesis", network+"-genesis.json")
		if _, err := os.Stat(genesisPath); err == nil {
			return genesisPath
		}
		// Check in same directory as executable
		genesisPath = filepath.Join(execDir, "genesis", network+"-genesis.json")
		if _, err := os.Stat(genesisPath); err == nil {
			return genesisPath
		}
	}

	// Try current working directory
	cwd, err := os.Getwd()
	if err == nil {
		genesisPath := filepath.Join(cwd, "genesis", network+"-genesis.json")
		if _, err := os.Stat(genesisPath); err == nil {
			return genesisPath
		}
	}

	return ""
}
