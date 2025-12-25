// Package dto contains Data Transfer Objects used for input/output
// of UseCases. These objects decouple the application layer from
// external concerns like CLI flags or JSON serialization.
package dto

import (
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ProvisionInput contains the input for provisioning a devnet.
type ProvisionInput struct {
	HomeDir           string
	Network           string // mainnet/testnet (snapshot source)
	BlockchainNetwork string // stable/ault (network module)
	NetworkVersion    string
	NumValidators     int
	NumAccounts       int
	Mode              string // docker/local
	SnapshotURL       string
	DockerImage       string
	StableVersion     string
	NoCache           bool
	CustomBinaryPath  string // For local mode with custom binary
	UseSnapshot       bool   // If true, export genesis from snapshot state instead of RPC genesis
	BinaryPath        string // Path to binary for state export (required when UseSnapshot=true)
	UseTestMnemonic   bool   // If true, use deterministic test mnemonics for validators
}

// ProvisionOutput contains the result of provisioning.
type ProvisionOutput struct {
	HomeDir       string
	ChainID       string
	GenesisPath   string
	NumValidators int
	NumAccounts   int
	Nodes         []NodeInfo
	Warnings      []string
}

// NodeInfo contains basic node information for output.
type NodeInfo struct {
	Index   int
	Name    string
	HomeDir string
	NodeID  string
	Ports   ports.PortConfig
	RPCURL  string
	EVMURL  string
}

// RunInput contains the input for running a devnet.
type RunInput struct {
	HomeDir       string
	ExecutionMode ports.ExecutionMode
	Background    bool
	WaitForSync   bool
	Timeout       time.Duration
}

// RunOutput contains the result of running.
type RunOutput struct {
	Nodes        []NodeStatus
	AllRunning   bool
	HealthySince time.Time
}

// NodeStatus contains the status of a running node.
type NodeStatus struct {
	Index       int
	Name        string
	IsRunning   bool
	PID         *int
	ContainerID string
	BlockHeight int64
	CatchingUp  bool
}

// StopInput contains the input for stopping a devnet.
type StopInput struct {
	HomeDir string
	Timeout time.Duration
	Force   bool
}

// StopOutput contains the result of stopping.
type StopOutput struct {
	StoppedNodes int
	Warnings     []string
}

// HealthInput contains the input for health checking.
type HealthInput struct {
	HomeDir string
	Verbose bool
}

// HealthOutput contains the health status of all nodes.
type HealthOutput struct {
	AllHealthy   bool
	Nodes        []NodeHealthStatus
	BlockHeights map[int]int64
}

// NodeHealthStatus contains health status for a single node.
type NodeHealthStatus struct {
	Index       int
	Name        string
	Status      ports.NodeStatus
	IsRunning   bool
	BlockHeight int64
	PeerCount   int
	CatchingUp  bool
	AppVersion  string // Application version from /abci_info
	Error       string
}

// ResetInput contains the input for resetting a devnet.
type ResetInput struct {
	HomeDir   string
	HardReset bool // If true, removes all data including genesis
}

// ResetOutput contains the result of resetting.
type ResetOutput struct {
	Type     string           // "soft" or "hard"
	Removed  []string         // Successfully removed paths
	Warnings []string         // Warnings during reset
	Failed   map[string]error // Failed deletions with error details
}

// StartInput contains the input for the combined provision+run operation.
type StartInput struct {
	ProvisionInput
	RunAfterProvision bool
}

// StartOutput contains the result of the start operation.
type StartOutput struct {
	Provisioned *ProvisionOutput
	Running     *RunOutput
}

// DestroyInput contains the input for destroying a devnet.
type DestroyInput struct {
	HomeDir    string
	Force      bool // Skip confirmation
	CleanCache bool // Also clean snapshot cache
}

// DestroyOutput contains the result of destroying.
type DestroyOutput struct {
	RemovedDir   string
	NodesStopped int
	CacheCleared bool
}

// DevnetInfo contains full devnet information for display.
type DevnetInfo struct {
	HomeDir           string
	ChainID           string
	NetworkSource     string // mainnet/testnet
	BlockchainNetwork string // stable/ault
	ExecutionMode     string // docker/local
	DockerImage       string
	NumValidators     int
	NumAccounts       int
	InitialVersion    string
	CurrentVersion    string
	Status            string
	CreatedAt         time.Time
	Nodes             []NodeInfo
}

// LoadInput contains the input for loading devnet info.
type LoadInput struct {
	HomeDir string
}

// LoadOutput contains the loaded devnet information.
type LoadOutput struct {
	Devnet *DevnetInfo
	Exists bool
}

// StatusInput contains the input for status checking.
type StatusInput struct {
	HomeDir string
}

// StatusOutput contains the full status of a devnet.
type StatusOutput struct {
	Devnet        *DevnetInfo
	OverallStatus string
	Nodes         []NodeHealthStatus
	AllHealthy    bool
}

// LogsInput contains the input for viewing logs.
type LogsInput struct {
	HomeDir   string
	NodeIndex int
	Follow    bool
	Lines     int
}

// ExportKeysInput contains the input for exporting keys.
type ExportKeysInput struct {
	HomeDir   string
	OutputDir string
	Format    string // json, env, shell
}

// ExportKeysOutput contains the exported keys.
type ExportKeysOutput struct {
	ValidatorKeys []ValidatorKeyInfo
	AccountKeys   []AccountKeyInfo
	OutputPath    string
}

// ValidatorKeyInfo contains validator key information.
type ValidatorKeyInfo struct {
	Index      int
	Name       string
	Address    string
	ValAddress string
	Mnemonic   string
}

// AccountKeyInfo contains account key information.
type AccountKeyInfo struct {
	Index    int
	Name     string
	Address  string
	Mnemonic string
}

// RestartInput contains the input for restarting devnet.
type RestartInput struct {
	HomeDir string
	Timeout time.Duration
}

// RestartOutput contains the result of restarting.
type RestartOutput struct {
	StoppedNodes int
	StartedNodes int
	AllRunning   bool
}

// NodeActionInput contains the input for a node action (start/stop).
type NodeActionInput struct {
	HomeDir   string
	NodeIndex int
	Timeout   time.Duration
}

// NodeActionOutput contains the result of a node action.
type NodeActionOutput struct {
	NodeIndex     int
	Action        string // "start" or "stop"
	Status        string // "success", "skipped", "error"
	PreviousState string
	CurrentState  string
	Error         string
}

// LogsOutput contains log content.
type LogsOutput struct {
	NodeIndex int
	Lines     []string
}

// ExecutionModeInfo contains information about how nodes are executed.
type ExecutionModeInfo struct {
	Mode          string // "docker" or "local"
	DockerImage   string
	ContainerName string
	LogPath       string
}

// DockerConfig contains Docker-specific configuration for devnet deployment
type DockerConfig struct {
	NetworkID      string              // Docker network ID
	NetworkName    string              // Docker network name
	Subnet         string              // Subnet CIDR
	PortRangeStart int                 // Start of allocated port range
	PortRangeEnd   int                 // End of allocated port range
	ResourceLimits *ResourceLimits     // Container resource limits
	Image          string              // Docker image reference
	CustomBuild    *CustomBuildConfig  // Optional custom image build config
}

// ResourceLimits defines container resource constraints
type ResourceLimits struct {
	Memory string // Memory limit (e.g., "2g", "512m")
	CPUs   string // CPU limit (e.g., "2.0", "0.5")
}

// CustomBuildConfig specifies parameters for building custom chain images
type CustomBuildConfig struct {
	PluginPath  string            // Path to plugin source code
	ChainBinary string            // Name of chain binary to build
	BuildArgs   map[string]string // Docker build args
}

// DeploymentInput contains the input for Docker-based deployment
type DeploymentInput struct {
	HomeDir        string              // Base directory for devnet data
	DevnetName     string              // Unique devnet identifier
	ValidatorCount int                 // Number of validators (1-100)
	Image          string              // Docker image reference
	ChainID        string              // Blockchain chain ID
	ResourceLimits *ResourceLimits     // Container resource limits
	CustomBuild    *CustomBuildConfig  // Optional custom image build
}

// Validate validates the deployment input
func (d *DeploymentInput) Validate() error {
	if d.DevnetName == "" {
		return ErrInvalidParameter{Field: "DevnetName", Reason: "cannot be empty"}
	}
	if d.ValidatorCount < 1 || d.ValidatorCount > 100 {
		return ErrInvalidParameter{Field: "ValidatorCount", Reason: "must be 1-100"}
	}
	if d.Image == "" && d.CustomBuild == nil {
		return ErrInvalidParameter{Field: "Image", Reason: "must specify image or custom build"}
	}
	if d.ChainID == "" {
		return ErrInvalidParameter{Field: "ChainID", Reason: "cannot be empty"}
	}
	return nil
}

// ErrInvalidParameter is returned when a parameter validation fails
type ErrInvalidParameter struct {
	Field  string
	Reason string
}

func (e ErrInvalidParameter) Error() string {
	return fmt.Sprintf("invalid parameter %s: %s", e.Field, e.Reason)
}

// DeploymentOutput contains the result of deployment
type DeploymentOutput struct {
	DevnetName     string              // Devnet identifier
	NetworkID      string              // Created Docker network ID
	Subnet         string              // Allocated subnet
	Containers     []*ContainerInfo    // Started container details
	PortRangeStart int                 // Start of allocated port range
	PortRangeEnd   int                 // End of allocated port range
	Success        bool                // Whether deployment succeeded
}

// ContainerInfo represents a deployed container
type ContainerInfo struct {
	ID         string // Docker container ID
	Name       string // Container name
	NodeIndex  int    // Validator index
	RPCPort    int    // RPC port
	P2PPort    int    // P2P port
	GRPCPort   int    // gRPC port
	EVMRPCPort int    // EVM RPC port
	EVMWSPort  int    // EVM WebSocket port
}
