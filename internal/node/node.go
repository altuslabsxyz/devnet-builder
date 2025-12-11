package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// NodePorts defines all network ports for a node.
type NodePorts struct {
	RPC    int `json:"rpc"`     // Tendermint RPC (default: 26657)
	P2P    int `json:"p2p"`     // P2P networking (default: 26656)
	GRPC   int `json:"grpc"`    // gRPC server (default: 9090)
	EVMRPC int `json:"evm_rpc"` // EVM JSON-RPC (default: 8545)
	EVMWS  int `json:"evm_ws"`  // EVM WebSocket (default: 8546)
	PProf  int `json:"pprof"`   // pprof debugging (default: 6060)
}

// DefaultPortsForNode returns the default ports for a node at the given index.
// Each node has a port offset of 10000 * index.
func DefaultPortsForNode(index int) NodePorts {
	offset := index * 10000
	return NodePorts{
		RPC:    26657 + offset,
		P2P:    26656 + offset,
		GRPC:   9090 + offset,
		EVMRPC: 8545 + offset,
		EVMWS:  8546 + offset,
		PProf:  6060 + offset,
	}
}

// Node represents a single validator node in the devnet.
type Node struct {
	// Identification
	Index int    `json:"index"` // 0-3
	Name  string `json:"name"`  // e.g., "node0", "node1"

	// Paths
	HomeDir string `json:"home_dir"` // e.g., ~/.stable-devnet/devnet/node0

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
func (n *Node) PIDFilePath() string {
	return filepath.Join(n.HomeDir, "stabled.pid")
}

// LogFilePath returns the path to the log file (local mode).
func (n *Node) LogFilePath() string {
	return filepath.Join(n.HomeDir, "stabled.log")
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
func (n *Node) Save() error {
	// Ensure directory exists
	if err := os.MkdirAll(n.HomeDir, 0755); err != nil {
		return fmt.Errorf("failed to create node directory: %w", err)
	}

	data, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal node config: %w", err)
	}

	if err := os.WriteFile(n.NodeJSONPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write node config: %w", err)
	}

	return nil
}

// LoadNode loads a node configuration from disk.
func LoadNode(homeDir string) (*Node, error) {
	nodePath := filepath.Join(homeDir, "node.json")

	data, err := os.ReadFile(nodePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("node not found at %s", homeDir)
		}
		return nil, fmt.Errorf("failed to read node config: %w", err)
	}

	var node Node
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse node config: %w", err)
	}

	return &node, nil
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
func ContainerNameForIndex(index int) string {
	return fmt.Sprintf("stable-devnet-node%d", index)
}

// DockerContainerName returns the Docker container name for this node.
func (n *Node) DockerContainerName() string {
	if n.ContainerName != "" {
		return n.ContainerName
	}
	return ContainerNameForIndex(n.Index)
}
