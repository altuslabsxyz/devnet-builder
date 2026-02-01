// Package provisioner provides devnet provisioning implementations.
package provisioner

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/subnet"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// Orchestrator defines the interface for provisioning orchestration.
// This allows DevnetProvisioner to optionally use an orchestrator for
// the full provisioning flow (build, fork, init, start).
type Orchestrator interface {
	// Execute runs the full provisioning flow.
	Execute(ctx context.Context, opts ports.ProvisionOptions) (*ports.ProvisionResult, error)

	// OnProgress sets the progress callback for phase updates.
	OnProgress(callback ProgressCallback)

	// CurrentPhase returns the current provisioning phase.
	CurrentPhase() ProvisioningPhase
}

// OrchestratorFactory creates orchestrators for different networks.
// This allows DevnetProvisioner to create the correct orchestrator
// based on the devnet's network/plugin type.
type OrchestratorFactory interface {
	// CreateOrchestrator creates an orchestrator for the given network.
	// Returns the Orchestrator interface for testability.
	CreateOrchestrator(network string) (Orchestrator, error)

	// GetNetworkDefaults returns default URLs for a network/plugin.
	// networkType is the target network (e.g., "mainnet", "testnet").
	// Returns nil if the network doesn't support defaults.
	GetNetworkDefaults(pluginName, networkType string) (*NetworkDefaults, error)
}

// NetworkDefaults contains default URLs from a network plugin.
type NetworkDefaults struct {
	// RPCURL is the default RPC endpoint for genesis forking.
	RPCURL string
	// SnapshotURL is the default snapshot URL for state downloads.
	SnapshotURL string
	// AvailableNetworks lists the network types this plugin supports.
	AvailableNetworks []string
}

// DevnetProvisioner implements the Provisioner interface for devnets.
// It creates Node resources when a devnet is provisioned.
// When an OrchestratorFactory is provided, it creates orchestrators for each
// network type and executes the full provisioning flow (build, fork, init)
// before creating Node resources.
type DevnetProvisioner struct {
	store               store.Store
	dataDir             string
	logger              *slog.Logger
	orchestratorFactory OrchestratorFactory
	subnetAllocator     *subnet.Allocator
	onProgress          ProgressCallback
}

// Config configures the DevnetProvisioner.
type Config struct {
	// DataDir is the base directory for node data.
	DataDir string

	// Logger for provisioner operations.
	Logger *slog.Logger

	// OrchestratorFactory is optional. When provided, Provision() creates
	// an orchestrator for the devnet's network type and executes the full
	// provisioning flow (build, fork, init). When nil, only Node resources
	// are created in the store (resource-only behavior).
	OrchestratorFactory OrchestratorFactory

	// SubnetAllocator manages subnet allocation for loopback network aliasing.
	// When provided, each devnet gets a unique subnet in the 127.0.X.0/24 range.
	SubnetAllocator *subnet.Allocator

	// OnProgress is an optional callback for provisioning progress updates.
	// This is used to update devnet status in the store during provisioning.
	OnProgress ProgressCallback
}

// NewDevnetProvisioner creates a new DevnetProvisioner.
func NewDevnetProvisioner(s store.Store, cfg Config) *DevnetProvisioner {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &DevnetProvisioner{
		store:               s,
		dataDir:             cfg.DataDir,
		logger:              logger,
		orchestratorFactory: cfg.OrchestratorFactory,
		subnetAllocator:     cfg.SubnetAllocator,
		onProgress:          cfg.OnProgress,
	}
}

// Provision creates Node resources for all validators and fullnodes in the devnet.
// When an OrchestratorFactory is configured, it first executes the full provisioning
// flow (build, fork, init) before creating Node resources.
func (p *DevnetProvisioner) Provision(ctx context.Context, devnet *types.Devnet) error {
	p.logger.Info("provisioning devnet",
		"name", devnet.Metadata.Name,
		"validators", devnet.Spec.Validators,
		"fullnodes", devnet.Spec.FullNodes,
		"hasOrchestratorFactory", p.orchestratorFactory != nil,
		"hasSubnetAllocator", p.subnetAllocator != nil)

	// Allocate a subnet for loopback network aliasing
	var allocatedSubnet uint8
	if p.subnetAllocator != nil {
		namespace := devnet.Metadata.Namespace
		if namespace == "" {
			namespace = types.DefaultNamespace
		}

		allocated, err := p.subnetAllocator.Allocate(namespace, devnet.Metadata.Name)
		if err != nil {
			return fmt.Errorf("failed to allocate subnet: %w", err)
		}
		allocatedSubnet = allocated
		devnet.Status.Subnet = allocated

		p.logger.Info("allocated subnet for devnet",
			"devnet", devnet.Metadata.Name,
			"subnet", allocated,
			"subnetRange", fmt.Sprintf("127.0.%d.0/24", allocated))
	}

	// Track binary path from orchestration (may be built)
	var builtBinaryPath string

	// If orchestrator factory is present, execute the full provisioning flow
	if p.orchestratorFactory != nil {
		result, err := p.provisionWithOrchestrator(ctx, devnet)
		if err != nil {
			return err
		}
		builtBinaryPath = result.BinaryPath
	}

	// Create Node resources in the store (existing behavior)
	// Pass the built binary path and allocated subnet so nodes get correct addresses
	return p.createNodeResources(ctx, devnet, builtBinaryPath, allocatedSubnet)
}

// provisionWithOrchestrator executes the full provisioning flow using an orchestrator.
// Creates an orchestrator for the devnet's network type and returns the provision result.
func (p *DevnetProvisioner) provisionWithOrchestrator(ctx context.Context, devnet *types.Devnet) (*ports.ProvisionResult, error) {
	// Determine network from devnet spec (Plugin field)
	network := devnet.Spec.Plugin
	if network == "" {
		return nil, fmt.Errorf("devnet %s has no plugin/network specified", devnet.Metadata.Name)
	}

	p.logger.Info("creating orchestrator for network",
		"name", devnet.Metadata.Name,
		"network", network)

	// Fetch network defaults from plugin only when forking is explicitly requested.
	// If ForkNetwork is empty, we use fresh genesis (no forking).
	var networkDefaults *NetworkDefaults
	if devnet.Spec.ForkNetwork != "" {
		defaults, err := p.orchestratorFactory.GetNetworkDefaults(network, devnet.Spec.ForkNetwork)
		if err != nil {
			p.logger.Warn("failed to get network defaults",
				"plugin", network,
				"forkNetwork", devnet.Spec.ForkNetwork,
				"error", err)
		} else {
			networkDefaults = defaults
			p.logger.Debug("fetched network defaults",
				"plugin", network,
				"forkNetwork", devnet.Spec.ForkNetwork,
				"rpcURL", defaults.RPCURL,
				"snapshotURL", defaults.SnapshotURL)
		}
	}

	// Create orchestrator for this network
	orchestrator, err := p.orchestratorFactory.CreateOrchestrator(network)
	if err != nil {
		return nil, fmt.Errorf("failed to create orchestrator for network %q: %w", network, err)
	}

	// Wire progress callback if set
	if p.onProgress != nil {
		orchestrator.OnProgress(p.onProgress)
	}

	// Convert devnet spec to provisioning options, using plugin defaults when URLs not specified
	opts, err := devnetToProvisionOptions(devnet, p.dataDir, networkDefaults)
	if err != nil {
		return nil, err
	}

	// In daemon mode, skip start phase - NodeController will handle starting
	opts.SkipStart = true

	p.logger.Info("executing orchestrator provisioning flow",
		"name", devnet.Metadata.Name,
		"network", network)

	// Execute the full provisioning flow (build, fork, init)
	result, err := orchestrator.Execute(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("orchestrator execution failed: %w", err)
	}

	p.logger.Info("orchestrator provisioning completed",
		"name", result.DevnetName,
		"binaryPath", result.BinaryPath,
		"nodeCount", result.NodeCount)

	return result, nil
}

// createNodeResources creates Node resources in the store.
// This is the original resource-only provisioning behavior.
// builtBinaryPath is the path to the binary built by orchestrator (empty if no orchestration).
// allocatedSubnet is the subnet for IP address assignment (0 means no subnet allocation).
func (p *DevnetProvisioner) createNodeResources(ctx context.Context, devnet *types.Devnet, builtBinaryPath string, allocatedSubnet uint8) error {
	totalNodes := devnet.Spec.Validators + devnet.Spec.FullNodes
	devnetDataDir := filepath.Join(p.dataDir, devnet.Metadata.Name)

	// Create validator nodes (indices 0 to Validators-1)
	for i := 0; i < devnet.Spec.Validators; i++ {
		node := p.createNodeSpec(devnet, i, "validator", devnetDataDir, builtBinaryPath, allocatedSubnet)
		if err := p.createNodeIfNotExists(ctx, node); err != nil {
			return fmt.Errorf("failed to create validator node %d: %w", i, err)
		}
	}

	// Create fullnode nodes (indices Validators to totalNodes-1)
	for i := devnet.Spec.Validators; i < totalNodes; i++ {
		node := p.createNodeSpec(devnet, i, "fullnode", devnetDataDir, builtBinaryPath, allocatedSubnet)
		if err := p.createNodeIfNotExists(ctx, node); err != nil {
			return fmt.Errorf("failed to create fullnode %d: %w", i, err)
		}
	}

	p.logger.Info("provisioned devnet nodes",
		"name", devnet.Metadata.Name,
		"total", totalNodes)

	return nil
}

// createNodeSpec creates a Node spec for the given devnet and index.
// builtBinaryPath is the path to the binary built by orchestrator (takes precedence if set).
// allocatedSubnet is the subnet for IP address assignment (0 means no subnet allocation).
func (p *DevnetProvisioner) createNodeSpec(devnet *types.Devnet, index int, role, devnetDataDir, builtBinaryPath string, allocatedSubnet uint8) *types.Node {
	// Determine binary path - built path takes precedence
	binaryPath := builtBinaryPath
	if binaryPath == "" {
		// Fall back to devnet spec
		if devnet.Spec.BinarySource.Type == "local" {
			binaryPath = devnet.Spec.BinarySource.Path
		}
		// For docker mode, binaryPath can be the docker image
		if devnet.Spec.Mode == "docker" && devnet.Spec.BinarySource.URL != "" {
			binaryPath = devnet.Spec.BinarySource.URL // Could be image reference
		}
	}

	// Ensure namespace is set
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	// Calculate node IP address from subnet allocation
	var nodeAddress string
	if allocatedSubnet > 0 {
		nodeAddress = subnet.NodeIP(allocatedSubnet, index)
	}

	return &types.Node{
		Metadata: types.ResourceMeta{
			Name:      fmt.Sprintf("%s-node-%d", devnet.Metadata.Name, index),
			Namespace: namespace,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Spec: types.NodeSpec{
			DevnetRef:  devnet.Metadata.Name,
			Index:      index,
			Role:       role,
			BinaryPath: binaryPath,
			HomeDir:    filepath.Join(devnetDataDir, fmt.Sprintf("node-%d", index)),
			Address:    nodeAddress,
			Desired:    types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:   types.NodePhasePending,
			Message: "Node created, awaiting start",
		},
	}
}

// createNodeIfNotExists creates a node if it doesn't already exist.
func (p *DevnetProvisioner) createNodeIfNotExists(ctx context.Context, node *types.Node) error {
	// Check if node already exists
	existing, err := p.store.GetNode(ctx, node.Metadata.Namespace, node.Spec.DevnetRef, node.Spec.Index)
	if err == nil && existing != nil {
		p.logger.Debug("node already exists",
			"devnet", node.Spec.DevnetRef,
			"index", node.Spec.Index)
		return nil
	}

	// Create the node
	if err := p.store.CreateNode(ctx, node); err != nil {
		return err
	}

	p.logger.Debug("created node",
		"devnet", node.Spec.DevnetRef,
		"index", node.Spec.Index,
		"role", node.Spec.Role)

	return nil
}

// Deprovision removes all nodes for the devnet.
// Note: This may already be handled by cascade delete in DevnetService.
func (p *DevnetProvisioner) Deprovision(ctx context.Context, devnet *types.Devnet) error {
	p.logger.Info("deprovisioning devnet", "name", devnet.Metadata.Name)

	// Get namespace from devnet
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	// Delete all nodes for this devnet
	if err := p.store.DeleteNodesByDevnet(ctx, namespace, devnet.Metadata.Name); err != nil {
		return fmt.Errorf("failed to delete nodes: %w", err)
	}

	p.logger.Info("deprovisioned devnet", "name", devnet.Metadata.Name)
	return nil
}

// Start sets all nodes to desired=Running state.
func (p *DevnetProvisioner) Start(ctx context.Context, devnet *types.Devnet) error {
	p.logger.Info("starting devnet", "name", devnet.Metadata.Name)

	// Get namespace from devnet
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	nodes, err := p.store.ListNodes(ctx, namespace, devnet.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes {
		if node.Spec.Desired != types.NodePhaseRunning {
			node.Spec.Desired = types.NodePhaseRunning
			node.Metadata.UpdatedAt = time.Now()
			if err := p.store.UpdateNode(ctx, node); err != nil {
				return fmt.Errorf("failed to update node %d: %w", node.Spec.Index, err)
			}
		}
	}

	return nil
}

// Stop sets all nodes to desired=Stopped state.
func (p *DevnetProvisioner) Stop(ctx context.Context, devnet *types.Devnet) error {
	p.logger.Info("stopping devnet", "name", devnet.Metadata.Name)

	// Get namespace from devnet
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	nodes, err := p.store.ListNodes(ctx, namespace, devnet.Metadata.Name)
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes {
		if node.Spec.Desired != types.NodePhaseStopped {
			node.Spec.Desired = types.NodePhaseStopped
			node.Metadata.UpdatedAt = time.Now()
			if err := p.store.UpdateNode(ctx, node); err != nil {
				return fmt.Errorf("failed to update node %d: %w", node.Spec.Index, err)
			}
		}
	}

	return nil
}

// GetStatus aggregates status from all nodes in the devnet.
func (p *DevnetProvisioner) GetStatus(ctx context.Context, devnet *types.Devnet) (*types.DevnetStatus, error) {
	// Get namespace from devnet
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	nodes, err := p.store.ListNodes(ctx, namespace, devnet.Metadata.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	status := &types.DevnetStatus{
		Nodes:           len(nodes),
		ReadyNodes:      0,
		CurrentHeight:   0,
		LastHealthCheck: time.Now(),
	}

	for _, node := range nodes {
		// Count running nodes as ready
		if node.Status.Phase == types.NodePhaseRunning {
			status.ReadyNodes++
		}
		// Track highest block height
		if node.Status.BlockHeight > status.CurrentHeight {
			status.CurrentHeight = node.Status.BlockHeight
		}
	}

	// Determine phase based on node status
	if status.ReadyNodes == 0 && status.Nodes > 0 {
		status.Phase = types.PhaseDegraded
		status.Message = "All nodes are down"
	} else if status.ReadyNodes < status.Nodes {
		status.Phase = types.PhaseDegraded
		status.Message = fmt.Sprintf("%d/%d nodes ready", status.ReadyNodes, status.Nodes)
	} else if status.ReadyNodes == status.Nodes && status.Nodes > 0 {
		status.Phase = types.PhaseRunning
		status.Message = "All nodes healthy"
	} else {
		status.Phase = types.PhasePending
		status.Message = "No nodes"
	}

	return status, nil
}

// =============================================================================
// Devnet to ProvisionOptions Conversion
// =============================================================================

// devnetToProvisionOptions converts a Devnet spec to ProvisionOptions for the orchestrator.
// This maps fields from the Devnet resource to the format expected by the provisioning flow.
// networkDefaults provides plugin defaults when URLs are not explicitly specified.
//
// Returns SnapshotVersionRequiredError if snapshot mode is detected but no binary version is set.
func devnetToProvisionOptions(devnet *types.Devnet, dataDir string, networkDefaults *NetworkDefaults) (ports.ProvisionOptions, error) {
	// Use spec ChainID if set, otherwise generate from devnet name
	chainID := devnet.Spec.ChainID
	if chainID == "" {
		chainID = devnet.Metadata.Name + "-1"
	}

	opts := ports.ProvisionOptions{
		DevnetName:    devnet.Metadata.Name,
		ChainID:       chainID,
		Network:       devnet.Spec.Plugin,
		NumValidators: devnet.Spec.Validators,
		NumFullNodes:  devnet.Spec.FullNodes,
		DataDir:       filepath.Join(dataDir, devnet.Metadata.Name),
	}

	// Map BinarySource to BinaryPath/BinaryVersion
	switch devnet.Spec.BinarySource.Type {
	case "local":
		opts.BinaryPath = devnet.Spec.BinarySource.Path
		opts.BinaryVersion = devnet.Spec.BinarySource.Version
	case "cache", "github", "url":
		// For non-local sources, version is used to build the binary
		opts.BinaryVersion = devnet.Spec.BinarySource.Version
	case "":
		// When Type is not specified but Version is set (e.g., from wizard via proto SdkVersion),
		// use the version for building. This handles the proto→domain conversion where SdkVersion
		// is mapped to BinarySource.Version without a corresponding Type field.
		if devnet.Spec.BinarySource.Version != "" {
			opts.BinaryVersion = devnet.Spec.BinarySource.Version
		}
	}

	// Map Genesis source, using plugin defaults when URLs not specified
	opts.GenesisSource = mapGenesisSource(devnet, networkDefaults)

	// Validate: snapshot mode requires explicit binary version to prevent schema mismatch panics
	if opts.GenesisSource.Mode == plugintypes.GenesisModeSnapshot && opts.BinaryVersion == "" {
		return ports.ProvisionOptions{}, &SnapshotVersionRequiredError{
			DevnetName: devnet.Metadata.Name,
		}
	}

	return opts, nil
}

// mapGenesisSource determines the genesis source from devnet spec.
// Priority: GenesisPath (local) > SnapshotURL (snapshot/spec or default) > RPCURL (spec or default) > fresh genesis
// networkDefaults provides plugin-defined URLs when not explicitly specified in the spec.
func mapGenesisSource(devnet *types.Devnet, networkDefaults *NetworkDefaults) plugintypes.GenesisSource {
	// If explicit genesis path is provided, use local mode
	if devnet.Spec.GenesisPath != "" {
		return plugintypes.GenesisSource{
			Mode:      plugintypes.GenesisModeLocal,
			LocalPath: devnet.Spec.GenesisPath,
		}
	}

	// Determine snapshot URL (from spec or plugin defaults)
	snapshotURL := devnet.Spec.SnapshotURL
	if snapshotURL == "" && networkDefaults != nil {
		snapshotURL = networkDefaults.SnapshotURL
	}

	// If snapshot URL is available, use snapshot mode
	if snapshotURL != "" {
		return plugintypes.GenesisSource{
			Mode:        plugintypes.GenesisModeSnapshot,
			SnapshotURL: snapshotURL,
			NetworkType: devnet.Spec.ForkNetwork,
		}
	}

	// Determine RPC URL (from spec or plugin defaults)
	rpcURL := devnet.Spec.RPCURL
	if rpcURL == "" && networkDefaults != nil {
		rpcURL = networkDefaults.RPCURL
	}

	// If RPC URL is available, use RPC mode for genesis forking
	if rpcURL != "" {
		return plugintypes.GenesisSource{
			Mode:        plugintypes.GenesisModeRPC,
			RPCURL:      rpcURL,
			NetworkType: devnet.Spec.ForkNetwork,
		}
	}

	// No fork configured - use fresh genesis (empty GenesisSource with no RPCURL)
	return plugintypes.GenesisSource{
		Mode: plugintypes.GenesisModeFresh,
	}
}
