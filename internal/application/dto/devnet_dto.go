// Package dto contains Data Transfer Objects used for input/output
// of UseCases. These objects decouple the application layer from
// external concerns like CLI flags or JSON serialization.
package dto

import (
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ProvisionInput contains the input for provisioning a devnet.
type ProvisionInput struct {
	HomeDir           string
	NetworkName       string
	NetworkVersion    string
	NumValidators     int
	NumAccounts       int
	ExecutionMode     ports.ExecutionMode
	SnapshotURL       string
	DockerImage       string
	StableVersion     string
	NoCache           bool
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
	Nodes         []NodeStatus
	AllRunning    bool
	HealthySince  time.Time
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
	Error       string
}

// ResetInput contains the input for resetting a devnet.
type ResetInput struct {
	HomeDir  string
	HardReset bool  // If true, removes all data including genesis
}

// ResetOutput contains the result of resetting.
type ResetOutput struct {
	Type     string  // "soft" or "hard"
	Removed  []string
	Warnings []string
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
	RemovedDir    string
	NodesStopped  int
	CacheCleared  bool
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
	Devnet       *DevnetInfo
	OverallStatus string
	Nodes        []NodeHealthStatus
	AllHealthy   bool
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
	HomeDir    string
	OutputDir  string
	Format     string // json, env, shell
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
	StoppedNodes  int
	StartedNodes  int
	AllRunning    bool
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
