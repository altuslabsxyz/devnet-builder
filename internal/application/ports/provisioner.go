// Package ports provides interfaces for the application layer.
// This file defines provisioner-related interfaces for dependency injection.
package ports

import (
	"context"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// =============================================================================
// GenesisForker Interface
// =============================================================================

// GenesisForker handles genesis forking from various sources (RPC, snapshot, local).
// This interface abstracts the genesis forking logic to enable clean dependency
// injection and testability in the DevnetProvisioner.
//
// The concrete implementation exists at internal/daemon/provisioner/genesis_forker.go
type GenesisForker interface {
	// Fork forks genesis from the specified source and applies patches.
	// The source can be RPC (fetch from network), snapshot (export from state),
	// or local (read from file).
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - opts: Options specifying the source and patch configuration
	//
	// Returns: ForkResult containing the forked genesis, or error
	Fork(ctx context.Context, opts ForkOptions) (*ForkResult, error)
}

// ForkOptions specifies options for forking genesis.
type ForkOptions struct {
	// Source specifies where to get genesis from (RPC, snapshot, or local file)
	Source types.GenesisSource

	// PatchOpts specifies modifications to apply to the forked genesis
	PatchOpts types.GenesisPatchOptions

	// BinaryPath is required for snapshot export mode
	BinaryPath string

	// NoCache skips caching when true
	NoCache bool
}

// ForkResult contains the result of a genesis fork operation.
type ForkResult struct {
	// Genesis is the forked and patched genesis JSON bytes
	Genesis []byte

	// SourceChainID is the original chain ID from the source genesis
	SourceChainID string

	// NewChainID is the chain ID after patching
	NewChainID string

	// SourceMode indicates how the genesis was obtained
	SourceMode types.GenesisMode

	// FetchedAt is when the genesis was fetched
	FetchedAt time.Time
}

// =============================================================================
// BinaryBuilder Interface Reference
// =============================================================================

// BinaryBuilder builds binaries from git sources.
// This interface is defined in internal/daemon/builder/types.go.
// Re-exported here for convenience when working with provisioner interfaces.
//
// DevnetProvisioner uses BinaryBuilder to:
// - Build chain binaries from source for specific versions
// - Retrieve cached builds for faster iteration
// - Clean up old builds to manage disk space
//
// See: internal/daemon/builder/types.go for full definition

// =============================================================================
// NodeInitializer Interface Reference
// =============================================================================

// NodeInitializer defines operations for initializing blockchain nodes.
// This interface is defined in this package (clients.go).
//
// DevnetProvisioner uses NodeInitializer to:
// - Initialize node home directories with chain init command
// - Create validator and account keys
// - Retrieve node IDs for peer configuration

// =============================================================================
// NodeRuntime Interface Reference
// =============================================================================

// NodeRuntime manages node processes/containers.
// This interface is defined in internal/daemon/runtime/interface.go.
//
// DevnetProvisioner uses NodeRuntime (also called ProcessRuntime) to:
// - Start node processes after initialization
// - Stop nodes gracefully during shutdown
// - Monitor node status during operation
// - Retrieve logs for debugging
//
// See: internal/daemon/runtime/interface.go for full definition

// =============================================================================
// Provisioner Interface
// =============================================================================

// Provisioner defines the interface for provisioning devnets.
// This is the high-level interface that DevnetProvisioner implements.
type Provisioner interface {
	// Provision creates all node resources for a devnet.
	// This includes creating Node resources in the store.
	Provision(ctx context.Context, devnet interface{}) error

	// Deprovision removes all node resources for a devnet.
	Deprovision(ctx context.Context, devnet interface{}) error

	// Start sets all nodes to desired=Running state.
	Start(ctx context.Context, devnet interface{}) error

	// Stop sets all nodes to desired=Stopped state.
	Stop(ctx context.Context, devnet interface{}) error

	// GetStatus aggregates status from all nodes in the devnet.
	GetStatus(ctx context.Context, devnet interface{}) (interface{}, error)
}

// =============================================================================
// FullProvisioner Interface
// =============================================================================

// FullProvisioner extends Provisioner with full lifecycle management.
// This interface composes all the component interfaces needed for
// complete devnet provisioning from scratch.
type FullProvisioner interface {
	Provisioner

	// Initialize performs full devnet initialization including:
	// - Building/retrieving the chain binary
	// - Forking genesis from the specified source
	// - Initializing all node home directories
	// - Configuring validators and peers
	Initialize(ctx context.Context, opts ProvisionOptions) (*ProvisionResult, error)
}

// ProvisionOptions contains all options for full devnet provisioning.
type ProvisionOptions struct {
	// DevnetName is the unique name for this devnet
	DevnetName string

	// ChainID for the devnet
	ChainID string

	// Network plugin name (e.g., "stable", "ault")
	Network string

	// NumValidators is the number of validator nodes
	NumValidators int

	// NumFullNodes is the number of non-validator full nodes
	NumFullNodes int

	// GenesisSource specifies where to get genesis from
	GenesisSource types.GenesisSource

	// GenesisPatchOpts specifies modifications to apply to genesis
	GenesisPatchOpts types.GenesisPatchOptions

	// BinaryVersion specifies the version of the binary to use
	BinaryVersion string

	// BinaryPath is an optional pre-built binary path (skips build)
	BinaryPath string

	// DataDir is the base directory for devnet data
	DataDir string
}

// ProvisionResult contains the result of a full provisioning operation.
type ProvisionResult struct {
	// DevnetName is the name of the provisioned devnet
	DevnetName string

	// ChainID of the provisioned devnet
	ChainID string

	// BinaryPath is the path to the chain binary
	BinaryPath string

	// GenesisPath is the path to the genesis file
	GenesisPath string

	// NodeCount is the total number of nodes provisioned
	NodeCount int

	// ValidatorCount is the number of validator nodes
	ValidatorCount int

	// FullNodeCount is the number of full nodes
	FullNodeCount int

	// DataDir is the base directory containing all node data
	DataDir string
}
