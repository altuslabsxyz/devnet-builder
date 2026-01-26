// Package provisioner provides devnet provisioning implementations.
package provisioner

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// DevnetProvisioner implements the Provisioner interface for devnets.
// It creates Node resources when a devnet is provisioned.
type DevnetProvisioner struct {
	store   store.Store
	dataDir string
	logger  *slog.Logger
}

// Config configures the DevnetProvisioner.
type Config struct {
	// DataDir is the base directory for node data.
	DataDir string

	// Logger for provisioner operations.
	Logger *slog.Logger
}

// NewDevnetProvisioner creates a new DevnetProvisioner.
func NewDevnetProvisioner(s store.Store, cfg Config) *DevnetProvisioner {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &DevnetProvisioner{
		store:   s,
		dataDir: cfg.DataDir,
		logger:  logger,
	}
}

// Provision creates Node resources for all validators and fullnodes in the devnet.
func (p *DevnetProvisioner) Provision(ctx context.Context, devnet *types.Devnet) error {
	p.logger.Info("provisioning devnet",
		"name", devnet.Metadata.Name,
		"validators", devnet.Spec.Validators,
		"fullnodes", devnet.Spec.FullNodes)

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
