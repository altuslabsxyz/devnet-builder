// internal/plugin/types/initializer.go
package types

import "fmt"

// PluginInitializer abstracts network-specific node initialization operations.
// This interface provides customization points for initializing node home directories,
// creating validator keys, and configuring peer connections.
type PluginInitializer interface {
	// BinaryName returns the binary name for this network (e.g., "stabled", "gaiad").
	BinaryName() string

	// DefaultChainID returns a default chain ID suitable for devnets.
	DefaultChainID() string

	// DefaultMoniker returns a default moniker for validator at the given index.
	// Index is 0-based (e.g., index 0 might return "validator-0").
	DefaultMoniker(index int) string

	// InitCommandArgs returns the command arguments for initializing a node.
	// The binary path will be prepended by the caller.
	// Typical args: ["init", moniker, "--chain-id", chainID, "--home", homeDir]
	InitCommandArgs(homeDir, moniker, chainID string) []string

	// ConfigDir returns the path to the config directory within the home directory.
	// For Cosmos SDK chains, this is typically homeDir/config.
	ConfigDir(homeDir string) string

	// DataDir returns the path to the data directory within the home directory.
	// For Cosmos SDK chains, this is typically homeDir/data.
	DataDir(homeDir string) string

	// KeyringDir returns the path to the keyring directory.
	// For Cosmos SDK chains with test backend, this is typically the home directory itself.
	KeyringDir(homeDir string) string
}

// NodeInitConfig holds configuration for initializing a single node.
// This is a convenience struct for passing initialization parameters.
type NodeInitConfig struct {
	// HomeDir is the node's home directory (e.g., /path/to/devnet/node0)
	HomeDir string

	// Moniker is the node's human-readable name
	Moniker string

	// ChainID is the blockchain network identifier
	ChainID string

	// ValidatorIndex is the 0-based index of this validator in the network
	ValidatorIndex int
}

// Validate checks that required fields are set.
func (c *NodeInitConfig) Validate() error {
	if c.HomeDir == "" {
		return fmt.Errorf("HomeDir is required")
	}
	if c.Moniker == "" {
		return fmt.Errorf("Moniker is required")
	}
	if c.ChainID == "" {
		return fmt.Errorf("ChainID is required")
	}
	return nil
}
