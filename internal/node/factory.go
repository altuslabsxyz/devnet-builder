package node

import (
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/output"
)

// ExecutionMode defines how nodes are executed.
type ExecutionMode string

const (
	// ModeDocker runs nodes in Docker containers.
	ModeDocker ExecutionMode = "docker"

	// ModeLocal runs nodes as local processes.
	ModeLocal ExecutionMode = "local"
)

// FactoryConfig contains configuration for creating a NodeManager.
type FactoryConfig struct {
	// Mode determines whether to use Docker or local execution.
	Mode ExecutionMode

	// BinaryPath is the path to the stabled binary (local mode only).
	// If empty, defaults to homeDir/bin/stabled.
	BinaryPath string

	// DockerImage is the Docker image to use (docker mode only).
	// If empty, defaults to DefaultDockerImage.
	DockerImage string

	// EVMChainID is the EVM chain ID (optional, used for --evm.evm-chain-id flag).
	EVMChainID string

	// Logger is the output logger. If nil, uses DefaultLogger.
	Logger *output.Logger
}

// Validate checks if the configuration is valid.
func (c *FactoryConfig) Validate() error {
	switch c.Mode {
	case ModeDocker, ModeLocal:
		return nil
	case "":
		return fmt.Errorf("execution mode is required")
	default:
		return fmt.Errorf("unknown execution mode: %s (valid: docker, local)", c.Mode)
	}
}

// NodeManagerFactory creates NodeManager instances based on execution mode.
// This provides a single point of truth for manager creation logic.
type NodeManagerFactory struct {
	config FactoryConfig
}

// NewNodeManagerFactory creates a new NodeManagerFactory with the given configuration.
func NewNodeManagerFactory(config FactoryConfig) *NodeManagerFactory {
	return &NodeManagerFactory{config: config}
}

// Create creates a NodeManager based on the configured execution mode.
// Returns DockerManager for ModeDocker, LocalManager for ModeLocal.
func (f *NodeManagerFactory) Create() (NodeManager, error) {
	if err := f.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid factory config: %w", err)
	}

	logger := f.config.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	switch f.config.Mode {
	case ModeDocker:
		return f.createDockerManager(logger), nil
	case ModeLocal:
		return f.createLocalManager(logger), nil
	default:
		// This should never happen due to Validate(), but handle it gracefully.
		return nil, fmt.Errorf("unsupported execution mode: %s", f.config.Mode)
	}
}

// createDockerManager creates a DockerManager with the factory's configuration.
func (f *NodeManagerFactory) createDockerManager(logger *output.Logger) NodeManager {
	if f.config.EVMChainID != "" {
		return NewDockerManagerWithEVMChainID(f.config.DockerImage, f.config.EVMChainID, logger)
	}
	return NewDockerManager(f.config.DockerImage, logger)
}

// createLocalManager creates a LocalManager with the factory's configuration.
func (f *NodeManagerFactory) createLocalManager(logger *output.Logger) NodeManager {
	if f.config.EVMChainID != "" {
		return NewLocalManagerWithEVMChainID(f.config.BinaryPath, f.config.EVMChainID, logger)
	}
	return NewLocalManager(f.config.BinaryPath, logger)
}

// Mode returns the configured execution mode.
func (f *NodeManagerFactory) Mode() ExecutionMode {
	return f.config.Mode
}

// IsDocker returns true if the factory is configured for Docker mode.
func (f *NodeManagerFactory) IsDocker() bool {
	return f.config.Mode == ModeDocker
}

// IsLocal returns true if the factory is configured for local mode.
func (f *NodeManagerFactory) IsLocal() bool {
	return f.config.Mode == ModeLocal
}
