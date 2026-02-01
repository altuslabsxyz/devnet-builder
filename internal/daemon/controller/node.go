// internal/daemon/controller/node.go
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// NodeController reconciles Node resources.
// It manages the lifecycle of individual nodes within devnets.
type NodeController struct {
	store   store.Store
	runtime runtime.NodeRuntime
	logger  *slog.Logger
}

// NewNodeController creates a new NodeController.
func NewNodeController(s store.Store, r runtime.NodeRuntime) *NodeController {
	return &NodeController{
		store:   s,
		runtime: r,
		logger:  slog.Default(),
	}
}

// SetLogger sets the logger for the controller.
func (c *NodeController) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

// ParseNodeKey parses a node key (format: "namespace/devnetName/index" or "devnetName/index") into its components.
// If no namespace is provided, returns default namespace.
func ParseNodeKey(key string) (namespace, devnetName string, index int, err error) {
	parts := strings.Split(key, "/")
	switch len(parts) {
	case 2:
		// "devnetName/index" - use default namespace
		devnetName = parts[0]
		index, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid node index in key %s: %w", key, err)
		}
		return types.DefaultNamespace, devnetName, index, nil
	case 3:
		// "namespace/devnetName/index"
		namespace = parts[0]
		devnetName = parts[1]
		index, err = strconv.Atoi(parts[2])
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid node index in key %s: %w", key, err)
		}
		return namespace, devnetName, index, nil
	default:
		return "", "", 0, fmt.Errorf("invalid node key format: %s", key)
	}
}

// NodeKey creates a node key from namespace, devnet name and index.
// If namespace is empty, only "devnetName/index" is returned.
func NodeKey(devnetName string, index int) string {
	return fmt.Sprintf("%s/%d", devnetName, index)
}

// NodeKeyWithNamespace creates a full node key with namespace.
func NodeKeyWithNamespace(namespace, devnetName string, index int) string {
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return fmt.Sprintf("%s/%s/%d", namespace, devnetName, index)
}

// Reconcile processes a single node by key (format: "namespace/devnetName/index" or "devnetName/index").
// It compares desired state (spec.Desired) with actual state (status.Phase) and takes action.
func (c *NodeController) Reconcile(ctx context.Context, key string) error {
	c.logger.Debug("reconciling node", "key", key)

	// Parse key
	namespace, devnetName, index, err := ParseNodeKey(key)
	if err != nil {
		return err
	}

	// Get node from store
	node, err := c.store.GetNode(ctx, namespace, devnetName, index)
	if err != nil {
		if store.IsNotFound(err) {
			// Node was deleted, nothing to do
			c.logger.Debug("node not found (deleted?)", "key", key)
			return nil
		}
		return fmt.Errorf("failed to get node %s: %w", key, err)
	}

	// Reconcile based on current phase
	switch node.Status.Phase {
	case "", types.NodePhasePending:
		return c.reconcilePending(ctx, node)
	case types.NodePhaseStarting:
		return c.reconcileStarting(ctx, node)
	case types.NodePhaseRunning:
		return c.reconcileRunning(ctx, node)
	case types.NodePhaseStopping:
		return c.reconcileStopping(ctx, node)
	case types.NodePhaseStopped:
		return c.reconcileStopped(ctx, node)
	case types.NodePhaseCrashed:
		return c.reconcileCrashed(ctx, node)
	default:
		c.logger.Warn("unknown node phase", "key", key, "phase", node.Status.Phase)
		return nil
	}
}

// reconcilePending handles nodes in Pending phase.
// Transitions to Starting if desired state is Running.
// Note: This method continues directly to reconcileStarting to avoid
// requiring a store watcher or manual re-enqueue after phase transition.
func (c *NodeController) reconcilePending(ctx context.Context, node *types.Node) error {
	// If desired is Running (or not explicitly Stopped), start the node
	if node.Spec.Desired == "" || node.Spec.Desired == types.NodePhaseRunning {
		c.logger.Info("starting node",
			"devnet", node.Spec.DevnetRef,
			"index", node.Spec.Index)

		node.Status.Phase = types.NodePhaseStarting
		node.Status.Message = "Starting node"

		// Save the phase transition
		if err := c.store.UpdateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to update node phase: %w", err)
		}

		// Continue directly to starting to avoid requiring re-enqueue.
		// Without a store watcher, returning here would leave the node stuck
		// in Starting phase with no subsequent reconcile triggered.
		return c.reconcileStarting(ctx, node)
	}

	// Desired is Stopped, transition directly to Stopped
	node.Status.Phase = types.NodePhaseStopped
	node.Status.Message = "Node created in stopped state"

	return c.store.UpdateNode(ctx, node)
}

// reconcileStarting handles nodes in Starting phase.
// Starts the container/process and transitions to Running.
func (c *NodeController) reconcileStarting(ctx context.Context, node *types.Node) error {
	c.logger.Debug("checking starting node",
		"devnet", node.Spec.DevnetRef,
		"index", node.Spec.Index)

	// Start the node if we have a runtime
	if c.runtime != nil {
		opts := runtime.StartOptions{
			RestartPolicy: runtime.DefaultRestartPolicy(),
		}
		if err := c.runtime.StartNode(ctx, node, opts); err != nil {
			c.logger.Error("failed to start node",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index,
				"error", err)

			node.Status.Phase = types.NodePhaseCrashed
			node.Status.Message = "Failed to start: " + err.Error()

			return c.store.UpdateNode(ctx, node)
		}
	}

	// Transition to Running
	node.Status.Phase = types.NodePhaseRunning
	node.Status.Message = "Node is running"

	return c.store.UpdateNode(ctx, node)
}

// reconcileRunning handles nodes in Running phase.
// Checks if desired state changed to Stopped, performs health checks.
func (c *NodeController) reconcileRunning(ctx context.Context, node *types.Node) error {
	// Check if user wants to stop the node
	if node.Spec.Desired == types.NodePhaseStopped {
		c.logger.Info("stopping node (user requested)",
			"devnet", node.Spec.DevnetRef,
			"index", node.Spec.Index)

		node.Status.Phase = types.NodePhaseStopping
		node.Status.Message = "Stopping node"

		return c.store.UpdateNode(ctx, node)
	}

	// If we have a runtime, check node status
	if c.runtime != nil {
		nodeID := node.Metadata.Name
		status, err := c.runtime.GetNodeStatus(ctx, nodeID)
		if err != nil {
			c.logger.Warn("failed to get node status",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index,
				"error", err)
		} else if !status.Running {
			// Node stopped unexpectedly
			c.logger.Warn("node stopped unexpectedly",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index)

			node.Status.Phase = types.NodePhaseCrashed
			node.Status.Message = "Node stopped unexpectedly"

			return c.store.UpdateNode(ctx, node)
		}
	}

	// Node is healthy, nothing to do
	return nil
}

// reconcileStopping handles nodes in Stopping phase.
// Stops the container/process and transitions to Stopped.
func (c *NodeController) reconcileStopping(ctx context.Context, node *types.Node) error {
	c.logger.Info("stopping node",
		"devnet", node.Spec.DevnetRef,
		"index", node.Spec.Index)

	// Stop the node if we have a runtime
	if c.runtime != nil {
		nodeID := node.Metadata.Name
		if err := c.runtime.StopNode(ctx, nodeID, true); err != nil {
			c.logger.Warn("failed to stop node",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index,
				"error", err)
			// Continue anyway - the node may already be stopped
		}
	}

	// Clear PID if set
	node.Status.PID = 0

	// Transition to Stopped
	node.Status.Phase = types.NodePhaseStopped
	node.Status.Message = "Node stopped"

	return c.store.UpdateNode(ctx, node)
}

// reconcileStopped handles nodes in Stopped phase.
// Checks if desired state changed to Running, restarts if needed.
func (c *NodeController) reconcileStopped(ctx context.Context, node *types.Node) error {
	// Check if user wants to start the node
	if node.Spec.Desired == "" || node.Spec.Desired == types.NodePhaseRunning {
		c.logger.Info("restarting node",
			"devnet", node.Spec.DevnetRef,
			"index", node.Spec.Index)

		node.Status.Phase = types.NodePhasePending
		node.Status.Message = "Restarting node"
		node.Status.RestartCount++

		return c.store.UpdateNode(ctx, node)
	}

	// Node should remain stopped
	return nil
}

// reconcileCrashed handles nodes in Crashed phase.
// May attempt restart based on restart policy.
func (c *NodeController) reconcileCrashed(ctx context.Context, node *types.Node) error {
	// For now, just transition to Stopped and let user decide
	// Future: implement restart policies
	c.logger.Info("node crashed, transitioning to stopped",
		"devnet", node.Spec.DevnetRef,
		"index", node.Spec.Index)

	node.Status.Phase = types.NodePhaseStopped
	node.Status.Message = "Node crashed and stopped"

	return c.store.UpdateNode(ctx, node)
}

// Ensure NodeController implements Controller interface
var _ Controller = (*NodeController)(nil)
