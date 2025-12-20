package ports

import (
	"context"
	"time"
)

// DefaultContext returns a background context with a reasonable timeout.
func DefaultContext() context.Context {
	return context.Background()
}

// ContextWithTimeout returns a context with the specified timeout.
func ContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// HealthChecker defines operations for checking node health.
type HealthChecker interface {
	// CheckNode checks the health of a single node.
	CheckNode(ctx context.Context, rpcEndpoint string) (*HealthStatus, error)

	// CheckAllNodes checks the health of all nodes.
	CheckAllNodes(ctx context.Context, nodes []*NodeMetadata) ([]*HealthStatus, error)
}

// HealthStatus represents the health status of a node.
type HealthStatus struct {
	NodeIndex   int
	NodeName    string
	IsRunning   bool
	Status      NodeStatus
	BlockHeight int64
	CatchingUp  bool
	LastChecked time.Time
	Error       error
}

// NodeStatus represents the status of a node.
type NodeStatus string

const (
	NodeStatusRunning  NodeStatus = "running"
	NodeStatusSyncing  NodeStatus = "syncing"
	NodeStatusStopped  NodeStatus = "stopped"
	NodeStatusError    NodeStatus = "error"
	NodeStatusUnknown  NodeStatus = "unknown"
)

// NetworkModule defines the interface for blockchain network modules.
// This is a simplified version focusing on what UseCases need.
type NetworkModule interface {
	// Identity
	Name() string
	DisplayName() string
	Version() string

	// Binary
	BinaryName() string
	DefaultBinaryVersion() string

	// Chain
	DefaultChainID() string
	Bech32Prefix() string
	BaseDenom() string

	// Commands
	InitCommand(homeDir, chainID, moniker string) []string
	StartCommand(homeDir string) []string
	ExportCommand(homeDir string) []string

	// Process
	DefaultNodeHome() string
	PIDFileName() string
	LogFileName() string

	// Docker
	DockerImage() string
	DockerImageTag(version string) string
	DockerHomeDir() string

	// Ports
	DefaultPorts() PortConfig

	// Snapshot - Network-specific snapshot and RPC configuration
	SnapshotURL(networkType string) string
	RPCEndpoint(networkType string) string
	AvailableNetworks() []string
}

// PluginLoader defines operations for loading network plugins.
type PluginLoader interface {
	// DiscoverPlugins finds all available plugins.
	DiscoverPlugins() ([]string, error)

	// LoadPlugin loads a plugin by name.
	LoadPlugin(name string) (NetworkModule, error)

	// UnloadPlugin unloads a plugin.
	UnloadPlugin(name string) error

	// GetLoadedPlugins returns all loaded plugins.
	GetLoadedPlugins() []string
}

// UpgradeOrchestrator defines operations for orchestrating upgrades.
type UpgradeOrchestrator interface {
	// Preflight performs pre-upgrade checks.
	Preflight(ctx context.Context, opts UpgradeOptions) error

	// Execute performs the full upgrade workflow.
	Execute(ctx context.Context, opts UpgradeOptions) (*UpgradeResult, error)

	// Monitor monitors upgrade progress.
	Monitor(ctx context.Context, proposalID uint64) (<-chan UpgradeProgress, error)
}

// UpgradeOptions holds options for an upgrade.
type UpgradeOptions struct {
	HomeDir       string
	UpgradeName   string
	TargetVersion string
	TargetBinary  string
	TargetImage   string
	VotingPeriod  time.Duration
	HeightBuffer  int
	UpgradeHeight int64
	ExportGenesis bool
}

// UpgradeResult holds the result of an upgrade.
type UpgradeResult struct {
	Success           bool
	ProposalID        uint64
	UpgradeHeight     int64
	PostUpgradeHeight int64
	NewBinary         string
	PreGenesisPath    string
	PostGenesisPath   string
	Duration          time.Duration
	Error             error
}

// UpgradeProgress represents the current progress of an upgrade.
type UpgradeProgress struct {
	Stage         UpgradeStage
	CurrentHeight int64
	TargetHeight  int64
	VotesCast     int
	TotalVoters   int
	Message       string
	Error         error
}

// UpgradeStage represents stages of the upgrade process.
type UpgradeStage string

const (
	StageVerifying       UpgradeStage = "verifying"
	StageSubmitting      UpgradeStage = "submitting"
	StageVoting          UpgradeStage = "voting"
	StageWaiting         UpgradeStage = "waiting"
	StageSwitching       UpgradeStage = "switching"
	StageVerifyingResume UpgradeStage = "verifying_resume"
	StageCompleted       UpgradeStage = "completed"
	StageFailed          UpgradeStage = "failed"
)
