// internal/daemon/types/node.go
package types

import "time"

// Node phase constants.
const (
	NodePhasePending  = "Pending"
	NodePhaseStarting = "Starting"
	NodePhaseRunning  = "Running"
	NodePhaseStopping = "Stopping"
	NodePhaseStopped  = "Stopped"
	NodePhaseCrashed  = "Crashed"
)

// Node represents a single blockchain node within a Devnet.
type Node struct {
	Metadata ResourceMeta `json:"metadata"`
	Spec     NodeSpec     `json:"spec"`
	Status   NodeStatus   `json:"status"`
}

// NodeSpec defines the desired state of a Node.
type NodeSpec struct {
	// DevnetRef is the name of the parent Devnet.
	DevnetRef string `json:"devnetRef"`

	// NamespaceRef is the namespace of the parent Devnet.
	// This allows looking up the parent devnet across namespaces.
	NamespaceRef string `json:"namespaceRef"`

	// Index is the node's index within the devnet (0-based).
	Index int `json:"index"`

	// Role is "validator" or "fullnode".
	Role string `json:"role"`

	// BinaryPath is the path to the node binary.
	BinaryPath string `json:"binaryPath"`

	// HomeDir is the node's data directory.
	HomeDir string `json:"homeDir"`

	// Address is the node's IP address (e.g., "127.0.42.1").
	// Used for loopback subnet aliasing where each node gets a unique IP.
	Address string `json:"address,omitempty"`

	// Desired is the desired state: "Running" or "Stopped".
	Desired string `json:"desired"`

	// ChainID is the chain ID for the node.
	// Copied from DevnetSpec at node creation time.
	ChainID string `json:"chainId,omitempty"`
}

// NodeStatus defines the observed state of a Node.
type NodeStatus struct {
	// Phase is the current phase.
	Phase string `json:"phase"`

	// PID is the process ID (local mode).
	PID int `json:"pid,omitempty"`

	// BlockHeight is the node's current block height.
	BlockHeight int64 `json:"blockHeight"`

	// PeerCount is the number of connected peers.
	PeerCount int `json:"peerCount"`

	// CatchingUp indicates if the node is syncing.
	CatchingUp bool `json:"catchingUp"`

	// RestartCount is how many times the node has been restarted.
	RestartCount int `json:"restartCount"`

	// ValidatorAddress is the validator's address (if validator).
	ValidatorAddress string `json:"validatorAddress,omitempty"`

	// ValidatorPubKey is the validator's public key (if validator).
	ValidatorPubKey string `json:"validatorPubKey,omitempty"`

	// Message provides additional status information.
	Message string `json:"message,omitempty"`

	// LastHealthCheck is when the node was last health-checked.
	LastHealthCheck time.Time `json:"lastHealthCheck,omitempty"`

	// LastBlockTime is when we last saw a new block on this node.
	LastBlockTime time.Time `json:"lastBlockTime,omitempty"`

	// ConsecutiveFailures counts consecutive health check failures.
	ConsecutiveFailures int `json:"consecutiveFailures"`

	// NextRestartTime is when the next restart attempt is allowed (backoff).
	NextRestartTime time.Time `json:"nextRestartTime,omitempty"`
}
