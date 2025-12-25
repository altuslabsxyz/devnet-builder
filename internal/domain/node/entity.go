// Package node provides domain entities for node management.
package node

import (
	"fmt"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/domain/common"
)

// Status represents the current state of a node.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusSyncing  Status = "syncing"
	StatusError    Status = "error"
)

// IsRunning returns true if the node is in a running state.
func (s Status) IsRunning() bool {
	return s == StatusRunning || s == StatusSyncing
}

// Ports defines all network ports for a node.
type Ports struct {
	RPC    int `json:"rpc"`     // Tendermint RPC (default: 26657)
	P2P    int `json:"p2p"`     // P2P networking (default: 26656)
	GRPC   int `json:"grpc"`    // gRPC server (default: 9090)
	EVMRPC int `json:"evm_rpc"` // EVM JSON-RPC (default: 8545)
	EVMWS  int `json:"evm_ws"`  // EVM WebSocket (default: 8546)
	PProf  int `json:"pprof"`   // pprof debugging (default: 6060)
}

// DefaultPorts returns the default ports for a node at the given index.
// Each node has a port offset of 10000 * index.
func DefaultPorts(index int) Ports {
	offset := index * 10000
	return Ports{
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
	// Identity
	Index int    `json:"index"`
	Name  string `json:"name"`

	// Network identity
	NetworkName string `json:"network_name,omitempty"`
	BinaryName  string `json:"binary_name,omitempty"`

	// Paths
	HomeDir string `json:"home_dir"`

	// Network configuration
	Ports Ports `json:"ports"`

	// Validator info
	Validator ValidatorInfo `json:"validator"`

	// Runtime state
	Runtime RuntimeState `json:"runtime"`

	// Status
	Status Status `json:"status"`
}

// ValidatorInfo holds validator-related information.
type ValidatorInfo struct {
	Address    string `json:"address,omitempty"`
	PubKey     string `json:"pubkey,omitempty"`
	ConsPubKey string `json:"cons_pubkey,omitempty"`
}

// RuntimeState holds runtime process/container state.
type RuntimeState struct {
	// Local mode
	PID     *int   `json:"pid,omitempty"`
	LogFile string `json:"log_file,omitempty"`

	// Docker mode
	ContainerID   string `json:"container_id,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
}

// New creates a new Node with default values.
func New(index int, homeDir string) *Node {
	return &Node{
		Index:   index,
		Name:    fmt.Sprintf("node%d", index),
		HomeDir: homeDir,
		Ports:   DefaultPorts(index),
		Status:  StatusStopped,
	}
}

// NewWithNetwork creates a new Node for a specific network.
func NewWithNetwork(index int, homeDir string, networkName, binaryName string) *Node {
	n := New(index, homeDir)
	n.NetworkName = networkName
	n.BinaryName = binaryName
	return n
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

// PIDFilePath returns the path to the PID file.
func (n *Node) PIDFilePath() string {
	binaryName := n.BinaryName
	if binaryName == "" {
		binaryName = "binary"
	}
	return filepath.Join(n.HomeDir, binaryName+".pid")
}

// LogFilePath returns the path to the log file.
func (n *Node) LogFilePath() string {
	binaryName := n.BinaryName
	if binaryName == "" {
		binaryName = "binary"
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

// ContainerName returns the Docker container name.
func (n *Node) ContainerName() string {
	if n.Runtime.ContainerName != "" {
		return n.Runtime.ContainerName
	}
	networkName := n.NetworkName
	if networkName == "" {
		networkName = "devnet"
	}
	return fmt.Sprintf("%s-node%d", networkName, n.Index)
}

// SetRunning marks the node as running.
func (n *Node) SetRunning(mode common.ExecutionMode, pid *int, containerID string) {
	n.Status = StatusRunning
	if mode == common.ModeLocal && pid != nil {
		n.Runtime.PID = pid
	}
	if mode == common.ModeDocker && containerID != "" {
		n.Runtime.ContainerID = containerID
	}
}

// SetStopped marks the node as stopped.
func (n *Node) SetStopped() {
	n.Status = StatusStopped
	n.Runtime.PID = nil
	n.Runtime.ContainerID = ""
}

// SetError marks the node as having an error.
func (n *Node) SetError() {
	n.Status = StatusError
}

// IsRunning returns true if the node is running.
func (n *Node) IsRunning() bool {
	return n.Status.IsRunning()
}
