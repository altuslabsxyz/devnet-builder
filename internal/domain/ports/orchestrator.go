package ports

import (
	"context"
	"time"
)

// DeploymentOrchestrator manages the deployment lifecycle with atomic rollback
type DeploymentOrchestrator interface {
	// Deploy orchestrates complete devnet deployment
	// Returns DeploymentResult with all deployment details
	// On any failure, automatically rolls back all changes
	Deploy(ctx context.Context, config *DeploymentConfig) (*DeploymentResult, error)

	// Rollback manually triggers rollback of a deployment
	// Used for cleanup after failures or explicit destroy operations
	Rollback(ctx context.Context, state *DeploymentState) error

	// GetState retrieves current deployment state for a devnet
	// Returns nil if no active deployment
	GetState(ctx context.Context, devnetName string) (*DeploymentState, error)
}

// DeploymentConfig specifies all parameters for a deployment
type DeploymentConfig struct {
	DevnetName     string             // Unique devnet identifier
	ValidatorCount int                // Number of validators (1-100)
	Image          string             // Docker image reference
	ChainID        string             // Blockchain chain ID
	HomeDir        string             // Base directory for devnet data
	ResourceLimits *ResourceLimits    // Container resource limits
	CustomBuild    *CustomBuildConfig // Optional custom image build
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

// DeploymentResult contains outcome of deployment
type DeploymentResult struct {
	DevnetName     string           // Devnet identifier
	NetworkID      string           // Created Docker network ID
	Subnet         string           // Allocated subnet
	Containers     []*ContainerInfo // Started container details
	PortAllocation *PortAllocation  // Allocated port range
	Duration       time.Duration    // Total deployment time
	Success        bool             // Whether deployment succeeded
}

// ContainerInfo represents a deployed container
type ContainerInfo struct {
	ID           string          // Docker container ID
	Name         string          // Container name
	NodeIndex    int             // Validator index
	Ports        *PortAssignment // Port mappings
	HealthStatus string          // Current health status
}

// PortAssignment represents port mappings for a container
type PortAssignment struct {
	RPC    int // Tendermint RPC port
	P2P    int // P2P networking port
	GRPC   int // gRPC server port
	EVMRPC int // EVM JSON-RPC port
	EVMWS  int // EVM WebSocket port
}

// DeploymentState tracks deployment progress for rollback
type DeploymentState struct {
	DevnetName        string
	Phase             DeploymentPhase
	NetworkID         *string           // nil if not created yet
	PortRange         *PortAllocation   // nil if not allocated yet
	StartedContainers []string          // Container IDs started so far
	HealthyContainers []string          // Container IDs that passed health checks
	Errors            []DeploymentError // Errors encountered
	StartedAt         time.Time
}

// DeploymentPhase represents current deployment stage
type DeploymentPhase string

const (
	PhaseValidating        DeploymentPhase = "VALIDATING"
	PhaseNetworkCreating   DeploymentPhase = "NETWORK_CREATING"
	PhasePortAllocating    DeploymentPhase = "PORT_ALLOCATING"
	PhaseImagePulling      DeploymentPhase = "IMAGE_PULLING"
	PhaseContainerStarting DeploymentPhase = "CONTAINER_STARTING"
	PhaseHealthChecking    DeploymentPhase = "HEALTH_CHECKING"
	PhaseRunning           DeploymentPhase = "RUNNING"
	PhaseRollingBack       DeploymentPhase = "ROLLING_BACK"
	PhaseFailed            DeploymentPhase = "FAILED"
)

// DeploymentError represents an error during deployment
type DeploymentError struct {
	Phase     DeploymentPhase
	Component string // e.g., "node0", "network"
	Error     error
	Timestamp time.Time
}
