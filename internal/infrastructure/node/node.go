package node

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/infrastructure/helpers"
	"github.com/b-harvest/devnet-builder/types"
)

// NodeStatus represents the current state of a node.
type NodeStatus string

const (
	NodeStatusStopped  NodeStatus = "stopped"
	NodeStatusStarting NodeStatus = "starting"
	NodeStatusRunning  NodeStatus = "running"
	NodeStatusSyncing  NodeStatus = "syncing"
	NodeStatusError    NodeStatus = "error"
)

// NodePorts is an alias to the canonical types.PortConfig.
//
// Deprecated: Use types.PortConfig directly.
type NodePorts = types.PortConfig

// DefaultPortsForNode returns the default ports for a node at the given index.
//
// Deprecated: Use types.PortConfigForNode() directly.
func DefaultPortsForNode(index int) NodePorts {
	return types.PortConfigForNode(index)
}

// Node represents a single validator node in the devnet.
type Node struct {
	// Identification
	Index int    `json:"index"` // 0-3
	Name  string `json:"name"`  // e.g., "node0", "node1"

	// Network Identity
	NetworkName string `json:"network_name,omitempty"` // e.g., "stable", "ault"
	BinaryName  string `json:"binary_name,omitempty"`  // e.g., "stabled", "aultd"

	// Paths
	HomeDir string `json:"home_dir"` // e.g., ~/.devnet-builder/devnet/node0

	// Network Ports
	Ports NodePorts `json:"ports"`

	// Process Management (local mode)
	PID     *int   `json:"pid,omitempty"`
	LogFile string `json:"log_file,omitempty"`

	// Container Management (docker mode)
	ContainerID   string `json:"container_id,omitempty"`
	ContainerName string `json:"container_name,omitempty"`

	// Validator Info
	ValidatorAddress string `json:"validator_address"`
	ValidatorPubKey  string `json:"validator_pubkey"`

	// Status
	Status NodeStatus `json:"status"`
}

// NewNode creates a new Node with default values for the given index.
func NewNode(index int, homeDir string) *Node {
	return &Node{
		Index:   index,
		Name:    fmt.Sprintf("node%d", index),
		HomeDir: homeDir,
		Ports:   DefaultPortsForNode(index),
		Status:  NodeStatusStopped,
	}
}

// ConfigPath returns the path to the node's config directory.
func (n *Node) ConfigPath() string {
	return filepath.Join(n.HomeDir, "config")
}

// DataPath returns the path to the node's data directory.
func (n *Node) DataPath() string {
	return filepath.Join(n.HomeDir, "data")
}

// KeyringPath returns the path to the node's keyring directory.
func (n *Node) KeyringPath() string {
	return filepath.Join(n.HomeDir, "keyring-test")
}

// NodeJSONPath returns the path to the node.json file.
func (n *Node) NodeJSONPath() string {
	return filepath.Join(n.HomeDir, "node.json")
}

// PIDFilePath returns the path to the PID file (local mode).
// Uses the node's BinaryName for dynamic file naming.
func (n *Node) PIDFilePath() string {
	binaryName := n.BinaryName
	if binaryName == "" {
		binaryName = "stabled" // Default fallback for backward compatibility
	}
	return filepath.Join(n.HomeDir, binaryName+".pid")
}

// LogFilePath returns the path to the log file (local mode).
// Uses the node's BinaryName for dynamic file naming.
func (n *Node) LogFilePath() string {
	binaryName := n.BinaryName
	if binaryName == "" {
		binaryName = "stabled" // Default fallback for backward compatibility
	}
	return filepath.Join(n.HomeDir, binaryName+".log")
}

// RPCURL returns the RPC endpoint URL.
func (n *Node) RPCURL() string {
	return fmt.Sprintf("http://localhost:%d", n.Ports.RPC)
}

// EVMRPCURL returns the EVM JSON-RPC endpoint URL.
func (n *Node) EVMRPCURL() string {
	return fmt.Sprintf("http://localhost:%d", n.Ports.EVMRPC)
}

// GRPCURL returns the gRPC endpoint URL.
func (n *Node) GRPCURL() string {
	return fmt.Sprintf("localhost:%d", n.Ports.GRPC)
}

// Save persists the node configuration to disk.
// Uses helpers.SaveJSON for consistent file I/O with automatic directory creation.
func (n *Node) Save() error {
	if err := helpers.SaveJSON(n.NodeJSONPath(), n, 0644); err != nil {
		return fmt.Errorf("failed to save node config: %w", err)
	}
	return nil
}

// LoadNode loads a node configuration from disk.
// Uses helpers.LoadJSON for consistent file I/O with structured error handling.
func LoadNode(homeDir string) (*Node, error) {
	nodePath := filepath.Join(homeDir, "node.json")

	node, err := helpers.LoadJSON[Node](nodePath)
	if err != nil {
		// Preserve backward-compatible error messages
		var jsonErr *helpers.JSONLoadError
		if errors.As(err, &jsonErr) {
			if jsonErr.Reason == "file not found" {
				return nil, fmt.Errorf("node not found at %s", homeDir)
			}
			if jsonErr.Reason == "failed to parse JSON in" {
				return nil, fmt.Errorf("failed to parse node config: %w", jsonErr.Wrapped)
			}
		}
		return nil, fmt.Errorf("failed to read node config: %w", err)
	}

	return node, nil
}

// SetRunning marks the node as running with the given process/container info.
func (n *Node) SetRunning(pid *int, containerID string) {
	n.Status = NodeStatusRunning
	if pid != nil {
		n.PID = pid
	}
	if containerID != "" {
		n.ContainerID = containerID
	}
}

// SetStopped marks the node as stopped.
func (n *Node) SetStopped() {
	n.Status = NodeStatusStopped
	n.PID = nil
	n.ContainerID = ""
}

// SetError marks the node as having an error.
func (n *Node) SetError() {
	n.Status = NodeStatusError
}

// IsRunning returns true if the node is in running state.
func (n *Node) IsRunning() bool {
	return n.Status == NodeStatusRunning || n.Status == NodeStatusSyncing
}

// ContainerNameForIndex returns the Docker container name for a node index.
//
// Deprecated: Use ContainerNameForNetwork instead for multi-network support.
func ContainerNameForIndex(index int) string {
	return fmt.Sprintf("stable-devnet-node%d", index)
}

// ContainerNameForNetwork returns the Docker container name for a node with network name.
func ContainerNameForNetwork(networkName string, index int) string {
	if networkName == "" {
		networkName = "stable"
	}
	return fmt.Sprintf("%s-devnet-node%d", networkName, index)
}

// DockerContainerName returns the Docker container name for this node.
func (n *Node) DockerContainerName() string {
	if n.ContainerName != "" {
		return n.ContainerName
	}
	if n.NetworkName != "" {
		return ContainerNameForNetwork(n.NetworkName, n.Index)
	}
	return ContainerNameForIndex(n.Index)
}
