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

// DevnetProvisioner implements the Provisioner interface for devnets.
// It creates Node resources when a devnet is provisioned.
// When an Orchestrator is provided, it also executes the full provisioning
// flow (build, fork, init, start) before creating Node resources.
type DevnetProvisioner struct {
	store        store.Store
	dataDir      string
	logger       *slog.Logger
	orchestrator Orchestrator
	onProgress   ProgressCallback
}

// Config configures the DevnetProvisioner.
type Config struct {
	// DataDir is the base directory for node data.
	DataDir string

	// Logger for provisioner operations.
	Logger *slog.Logger

	// Orchestrator is optional. When provided, Provision() uses the orchestrator
	// to execute the full provisioning flow (build, fork, init, start).
	// When nil, only Node resources are created in the store (resource-only behavior).
	Orchestrator Orchestrator

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

	p := &DevnetProvisioner{
		store:        s,
		dataDir:      cfg.DataDir,
		logger:       logger,
		orchestrator: cfg.Orchestrator,
		onProgress:   cfg.OnProgress,
	}

	// If orchestrator is provided and progress callback is set, wire them together
	if p.orchestrator != nil && p.onProgress != nil {
		p.orchestrator.OnProgress(p.onProgress)
	}

	return p
}

// Provision creates Node resources for all validators and fullnodes in the devnet.
// When an orchestrator is configured, it first executes the full provisioning flow
// (build, fork, init, start) before creating Node resources.
func (p *DevnetProvisioner) Provision(ctx context.Context, devnet *types.Devnet) error {
	p.logger.Info("provisioning devnet",
		"name", devnet.Metadata.Name,
		"validators", devnet.Spec.Validators,
		"fullnodes", devnet.Spec.FullNodes,
		"hasOrchestrator", p.orchestrator != nil)

	// If orchestrator is present, execute the full provisioning flow
	if p.orchestrator != nil {
		if err := p.provisionWithOrchestrator(ctx, devnet); err != nil {
			return err
		}
	}

	// Create Node resources in the store (existing behavior)
	return p.createNodeResources(ctx, devnet)
}

// provisionWithOrchestrator executes the full provisioning flow using the orchestrator.
func (p *DevnetProvisioner) provisionWithOrchestrator(ctx context.Context, devnet *types.Devnet) error {
	p.logger.Info("executing orchestrator provisioning flow",
		"name", devnet.Metadata.Name)

	// Convert devnet spec to provisioning options
	opts := devnetToProvisionOptions(devnet, p.dataDir)

	// Execute the full provisioning flow
	result, err := p.orchestrator.Execute(ctx, opts)
	if err != nil {
		return fmt.Errorf("orchestrator execution failed: %w", err)
	}

	p.logger.Info("orchestrator provisioning completed",
		"name", result.DevnetName,
		"binaryPath", result.BinaryPath,
		"nodeCount", result.NodeCount)

	return nil
}

// createNodeResources creates Node resources in the store.
// This is the original resource-only provisioning behavior.
func (p *DevnetProvisioner) createNodeResources(ctx context.Context, devnet *types.Devnet) error {
	totalNodes := devnet.Spec.Validators + devnet.Spec.FullNodes
	devnetDataDir := filepath.Join(p.dataDir, devnet.Metadata.Name)

	// Create validator nodes (indices 0 to Validators-1)
	for i := 0; i < devnet.Spec.Validators; i++ {
		node := p.createNodeSpec(devnet, i, "validator", devnetDataDir)
		if err := p.createNodeIfNotExists(ctx, node); err != nil {
			return fmt.Errorf("failed to create validator node %d: %w", i, err)
		}
	}

	// Create fullnode nodes (indices Validators to totalNodes-1)
	for i := devnet.Spec.Validators; i < totalNodes; i++ {
		node := p.createNodeSpec(devnet, i, "fullnode", devnetDataDir)
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
func (p *DevnetProvisioner) createNodeSpec(devnet *types.Devnet, index int, role, devnetDataDir string) *types.Node {
	// Determine binary path from devnet spec
	binaryPath := ""
	if devnet.Spec.BinarySource.Type == "local" {
		binaryPath = devnet.Spec.BinarySource.Path
	}
	// For docker mode, binaryPath can be the docker image
	if devnet.Spec.Mode == "docker" && devnet.Spec.BinarySource.URL != "" {
		binaryPath = devnet.Spec.BinarySource.URL // Could be image reference
	}

	// Ensure namespace is set
	namespace := devnet.Metadata.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
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
func devnetToProvisionOptions(devnet *types.Devnet, dataDir string) ports.ProvisionOptions {
	opts := ports.ProvisionOptions{
		DevnetName:    devnet.Metadata.Name,
		ChainID:       devnet.Metadata.Name + "-1", // Generate chain ID from devnet name
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
	}

	// Map Genesis source
	opts.GenesisSource = mapGenesisSource(devnet)

	return opts
}

// mapGenesisSource determines the genesis source from devnet spec.
// Priority: GenesisPath (local) > SnapshotURL (snapshot) > default (RPC)
func mapGenesisSource(devnet *types.Devnet) plugintypes.GenesisSource {
	// If explicit genesis path is provided, use local mode
	if devnet.Spec.GenesisPath != "" {
		return plugintypes.GenesisSource{
			Mode:      plugintypes.GenesisModeLocal,
			LocalPath: devnet.Spec.GenesisPath,
		}
	}

	// If snapshot URL is provided, use snapshot mode
	if devnet.Spec.SnapshotURL != "" {
		return plugintypes.GenesisSource{
			Mode:        plugintypes.GenesisModeSnapshot,
			SnapshotURL: devnet.Spec.SnapshotURL,
		}
	}

	// Default to RPC mode
	return plugintypes.GenesisSource{
		Mode: plugintypes.GenesisModeRPC,
	}
}
