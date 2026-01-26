// internal/plugin/cosmos/initializer.go
package cosmos

import (
	"fmt"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// CosmosInitializer handles node initialization for Cosmos SDK chains.
type CosmosInitializer struct {
	binaryName string
}

// NewCosmosInitializer creates a new Cosmos initializer.
func NewCosmosInitializer(binaryName string) *CosmosInitializer {
	return &CosmosInitializer{
		binaryName: binaryName,
	}
}

// BinaryName returns the binary name for this network (e.g., "stabled", "gaiad").
func (i *CosmosInitializer) BinaryName() string {
	return i.binaryName
}

// DefaultChainID returns a default chain ID suitable for devnets.
func (i *CosmosInitializer) DefaultChainID() string {
	return "devnet-1"
}

// DefaultMoniker returns a default moniker for validator at the given index.
// Index is 0-based (e.g., index 0 returns "validator-0").
func (i *CosmosInitializer) DefaultMoniker(index int) string {
	return fmt.Sprintf("validator-%d", index)
}

// InitCommandArgs returns the command arguments for initializing a node.
// The binary path will be prepended by the caller.
func (i *CosmosInitializer) InitCommandArgs(homeDir, moniker, chainID string) []string {
	return []string{
		"init", moniker,
		"--chain-id", chainID,
		"--home", homeDir,
		"--overwrite",
	}
}

// ConfigDir returns the path to the config directory within the home directory.
// For Cosmos SDK chains, this is homeDir/config.
func (i *CosmosInitializer) ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, "config")
}

// DataDir returns the path to the data directory within the home directory.
// For Cosmos SDK chains, this is homeDir/data.
func (i *CosmosInitializer) DataDir(homeDir string) string {
	return filepath.Join(homeDir, "data")
}

// KeyringDir returns the path to the keyring directory.
// For Cosmos SDK chains with test backend, this is the home directory itself.
func (i *CosmosInitializer) KeyringDir(homeDir string) string {
	return homeDir
}

// Ensure CosmosInitializer implements PluginInitializer
var _ types.PluginInitializer = (*CosmosInitializer)(nil)
