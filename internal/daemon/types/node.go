// internal/daemon/types/node.go
package types

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

	// Index is the node's index within the devnet (0-based).
	Index int `json:"index"`

	// Role is "validator" or "fullnode".
	Role string `json:"role"`

	// BinaryPath is the path to the node binary.
	BinaryPath string `json:"binaryPath"`

	// HomeDir is the node's data directory.
	HomeDir string `json:"homeDir"`

	// Desired is the desired state: "Running" or "Stopped".
	Desired string `json:"desired"`
}

// NodeStatus defines the observed state of a Node.
type NodeStatus struct {
	// Phase is the current phase.
	Phase string `json:"phase"`

	// ContainerID is the Docker container ID (docker mode).
	ContainerID string `json:"containerId,omitempty"`

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
}
