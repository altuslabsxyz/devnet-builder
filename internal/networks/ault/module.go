// Package ault provides the Ault network module for devnet-builder.
package ault

import (
	"github.com/stablelabs/stable-devnet/internal/network"
)

func init() {
	network.Register(&Module{})
}

// Module implements the NetworkModule interface for Ault network.
type Module struct{}

var _ network.NetworkModule = (*Module)(nil)

// Name returns the unique identifier for this network module.
func (m *Module) Name() string {
	return "ault"
}

// DisplayName returns a human-readable name for the network.
func (m *Module) DisplayName() string {
	return "Ault Network"
}

// Version returns the module version.
func (m *Module) Version() string {
	return "1.0.0"
}

// BinaryName returns the name of the network's CLI binary.
func (m *Module) BinaryName() string {
	return "aultd"
}

// DefaultBinaryVersion returns the default version to use if not specified.
func (m *Module) DefaultBinaryVersion() string {
	return "latest"
}

// DefaultChainID returns the default chain identifier for devnet.
func (m *Module) DefaultChainID() string {
	return "aultdevnet_900-1"
}

// Bech32Prefix returns the address prefix for this network.
func (m *Module) Bech32Prefix() string {
	return "ault"
}

// BaseDenom returns the base token denomination.
func (m *Module) BaseDenom() string {
	return "aault"
}

// DockerImage returns the Docker image name for this network.
func (m *Module) DockerImage() string {
	return "ghcr.io/bharvest/ault"
}

// BinarySource returns the configuration for binary acquisition.
func (m *Module) BinarySource() network.BinarySource {
	return BinarySourceConfig()
}

// GenesisConfig returns the default genesis configuration.
func (m *Module) GenesisConfig() network.GenesisConfig {
	return DefaultGenesisConfig()
}

// DockerImageTag returns the Docker tag for a given version.
func (m *Module) DockerImageTag(version string) string {
	if version == "" || version == "latest" {
		return "latest"
	}
	return version
}

// InitCommand returns the command arguments for initializing a node.
func (m *Module) InitCommand(homeDir string, chainID string, moniker string) []string {
	return []string{
		"init", moniker,
		"--chain-id", chainID,
		"--home", homeDir,
	}
}

// StartCommand returns the command arguments for starting a node.
func (m *Module) StartCommand(homeDir string) []string {
	return []string{
		"start",
		"--home", homeDir,
	}
}

// DefaultPorts returns the default port configuration for this network.
func (m *Module) DefaultPorts() network.PortConfig {
	return DefaultPortConfig()
}

// ModifyGenesis applies network-specific modifications to a genesis file.
func (m *Module) ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
	return ModifyGenesis(genesis, opts)
}

// Validate checks if the module configuration is valid.
func (m *Module) Validate() error {
	return nil
}
