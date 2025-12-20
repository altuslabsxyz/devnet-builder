// Package node provides node management implementations.
package node

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	legacynode "github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// LocalNodeManager implements NodeManager for local process execution.
type LocalNodeManager struct {
	node       *legacynode.Node
	manager    *legacynode.LocalManager
	logger     *output.Logger
	genesisPath string
}

// NewLocalNodeManager creates a new LocalNodeManager.
func NewLocalNodeManager(
	node *legacynode.Node,
	binaryPath string,
	evmChainID string,
	genesisPath string,
	logger *output.Logger,
) *LocalNodeManager {
	if logger == nil {
		logger = output.DefaultLogger
	}

	return &LocalNodeManager{
		node:        node,
		manager:     legacynode.NewLocalManagerWithEVMChainID(binaryPath, evmChainID, logger),
		logger:      logger,
		genesisPath: genesisPath,
	}
}

// Start starts the node.
func (m *LocalNodeManager) Start(ctx context.Context) error {
	if err := m.manager.Start(ctx, m.node, m.genesisPath); err != nil {
		return &NodeError{
			NodeIndex: m.node.Index,
			Operation: "start",
			Message:   err.Error(),
		}
	}
	return nil
}

// Stop stops the node.
func (m *LocalNodeManager) Stop(ctx context.Context, timeout time.Duration) error {
	if err := m.manager.Stop(ctx, m.node, timeout); err != nil {
		return &NodeError{
			NodeIndex: m.node.Index,
			Operation: "stop",
			Message:   err.Error(),
		}
	}
	return nil
}

// IsRunning checks if the node is running.
func (m *LocalNodeManager) IsRunning() bool {
	return m.manager.IsRunning(context.Background(), m.node)
}

// GetPID returns the process ID.
func (m *LocalNodeManager) GetPID() *int {
	return m.node.PID
}

// GetContainerID returns empty for local nodes.
func (m *LocalNodeManager) GetContainerID() string {
	return ""
}

// Logs retrieves the last N lines of logs.
func (m *LocalNodeManager) Logs(lines int) ([]string, error) {
	logContent, err := m.manager.GetLogs(context.Background(), m.node, lines)
	if err != nil {
		return nil, err
	}

	// Split into lines
	if logContent == "" {
		return nil, nil
	}

	result := make([]string, 0)
	start := 0
	for i, c := range logContent {
		if c == '\n' {
			if i > start {
				result = append(result, logContent[start:i])
			}
			start = i + 1
		}
	}
	if start < len(logContent) {
		result = append(result, logContent[start:])
	}

	return result, nil
}

// LogPath returns the path to the log file.
func (m *LocalNodeManager) LogPath() string {
	return m.node.LogFilePath()
}

// Ensure LocalNodeManager implements NodeManager.
var _ ports.NodeManager = (*LocalNodeManager)(nil)

// DockerNodeManager implements NodeManager for Docker container execution.
type DockerNodeManager struct {
	node        *legacynode.Node
	manager     *legacynode.DockerManager
	logger      *output.Logger
	genesisPath string
}

// NewDockerNodeManager creates a new DockerNodeManager.
func NewDockerNodeManager(
	node *legacynode.Node,
	image string,
	evmChainID string,
	genesisPath string,
	logger *output.Logger,
) *DockerNodeManager {
	if logger == nil {
		logger = output.DefaultLogger
	}

	return &DockerNodeManager{
		node:        node,
		manager:     legacynode.NewDockerManagerWithEVMChainID(image, evmChainID, logger),
		logger:      logger,
		genesisPath: genesisPath,
	}
}

// Start starts the node.
func (m *DockerNodeManager) Start(ctx context.Context) error {
	if err := m.manager.Start(ctx, m.node, m.genesisPath); err != nil {
		return &NodeError{
			NodeIndex: m.node.Index,
			Operation: "start",
			Message:   err.Error(),
		}
	}
	return nil
}

// Stop stops the node.
func (m *DockerNodeManager) Stop(ctx context.Context, timeout time.Duration) error {
	if err := m.manager.Stop(ctx, m.node, timeout); err != nil {
		return &NodeError{
			NodeIndex: m.node.Index,
			Operation: "stop",
			Message:   err.Error(),
		}
	}
	return nil
}

// IsRunning checks if the node is running.
func (m *DockerNodeManager) IsRunning() bool {
	return m.manager.IsRunning(context.Background(), m.node)
}

// GetPID returns nil for Docker nodes.
func (m *DockerNodeManager) GetPID() *int {
	return nil
}

// GetContainerID returns the container ID.
func (m *DockerNodeManager) GetContainerID() string {
	return m.node.ContainerID
}

// Logs retrieves the last N lines of logs.
func (m *DockerNodeManager) Logs(lines int) ([]string, error) {
	logContent, err := m.manager.GetLogs(context.Background(), m.node, lines, "")
	if err != nil {
		return nil, err
	}

	// Split into lines
	if logContent == "" {
		return nil, nil
	}

	result := make([]string, 0)
	start := 0
	for i, c := range logContent {
		if c == '\n' {
			if i > start {
				result = append(result, logContent[start:i])
			}
			start = i + 1
		}
	}
	if start < len(logContent) {
		result = append(result, logContent[start:])
	}

	return result, nil
}

// LogPath returns empty for Docker nodes (logs accessed via docker logs).
func (m *DockerNodeManager) LogPath() string {
	return fmt.Sprintf("docker logs %s", m.node.DockerContainerName())
}

// Ensure DockerNodeManager implements NodeManager.
var _ ports.NodeManager = (*DockerNodeManager)(nil)

// NodeManagerFactory creates NodeManager instances based on execution mode.
type NodeManagerFactory struct {
	logger *output.Logger
}

// NewNodeManagerFactory creates a new NodeManagerFactory.
func NewNodeManagerFactory(logger *output.Logger) *NodeManagerFactory {
	return &NodeManagerFactory{
		logger: logger,
	}
}

// CreateManager creates a NodeManager for the given node and execution mode.
func (f *NodeManagerFactory) CreateManager(
	node *legacynode.Node,
	mode ports.ExecutionMode,
	binaryOrImage string,
	evmChainID string,
	genesisPath string,
) ports.NodeManager {
	switch mode {
	case ports.ModeDocker:
		return NewDockerNodeManager(node, binaryOrImage, evmChainID, genesisPath, f.logger)
	default:
		return NewLocalNodeManager(node, binaryOrImage, evmChainID, genesisPath, f.logger)
	}
}
