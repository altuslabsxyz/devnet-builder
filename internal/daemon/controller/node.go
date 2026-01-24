// internal/daemon/controller/node.go
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// NodeRuntime is the interface for node container/process operations.
// This will be implemented by the Docker or local runtime.
type NodeRuntime interface {
	// StartContainer starts a container for the node.
	StartContainer(ctx context.Context, node *types.Node) (containerID string, err error)

	// StopContainer stops a running container.
	StopContainer(ctx context.Context, containerID string, timeout time.Duration) error

	// GetContainerStatus checks if a container is running.
	GetContainerStatus(ctx context.Context, containerID string) (running bool, err error)

	// RemoveContainer removes a container.
	RemoveContainer(ctx context.Context, containerID string) error
}

// NodeController reconciles Node resources.
// It manages the lifecycle of individual nodes within devnets.
type NodeController struct {
	store   store.Store
	runtime NodeRuntime
	logger  *slog.Logger
}

// NewNodeController creates a new NodeController.
func NewNodeController(s store.Store, r NodeRuntime) *NodeController {
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

// ParseNodeKey parses a node key (format: "devnetName/index") into its components.
func ParseNodeKey(key string) (devnetName string, index int, err error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid node key format: %s", key)
	}
	devnetName = parts[0]
	index, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid node index in key %s: %w", key, err)
	}
	return devnetName, index, nil
}

// NodeKey creates a node key from devnet name and index.
func NodeKey(devnetName string, index int) string {
	return fmt.Sprintf("%s/%d", devnetName, index)
}

// Reconcile processes a single node by key (format: "devnetName/index").
// It compares desired state (spec.Desired) with actual state (status.Phase) and takes action.
func (c *NodeController) Reconcile(ctx context.Context, key string) error {
	c.logger.Debug("reconciling node", "key", key)

	// Parse key
	devnetName, index, err := ParseNodeKey(key)
	if err != nil {
		return err
	}

	// Get node from store
	node, err := c.store.GetNode(ctx, devnetName, index)
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
func (c *NodeController) reconcilePending(ctx context.Context, node *types.Node) error {
	// If desired is Running (or not explicitly Stopped), start the node
	if node.Spec.Desired == "" || node.Spec.Desired == types.NodePhaseRunning {
		c.logger.Info("starting node",
			"devnet", node.Spec.DevnetRef,
			"index", node.Spec.Index)

		node.Status.Phase = types.NodePhaseStarting
		node.Status.Message = "Starting node"

		return c.store.UpdateNode(ctx, node)
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

	// If we have a runtime and no container yet, start it
	if c.runtime != nil && node.Status.ContainerID == "" {
		containerID, err := c.runtime.StartContainer(ctx, node)
		if err != nil {
			c.logger.Error("failed to start container",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index,
				"error", err)

			node.Status.Phase = types.NodePhaseCrashed
			node.Status.Message = "Failed to start: " + err.Error()

			return c.store.UpdateNode(ctx, node)
		}
		node.Status.ContainerID = containerID
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

	// If we have a runtime, check container status
	if c.runtime != nil && node.Status.ContainerID != "" {
		running, err := c.runtime.GetContainerStatus(ctx, node.Status.ContainerID)
		if err != nil {
			c.logger.Warn("failed to get container status",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index,
				"error", err)
		} else if !running {
			// Container stopped unexpectedly
			c.logger.Warn("container stopped unexpectedly",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index)

			node.Status.Phase = types.NodePhaseCrashed
			node.Status.Message = "Container stopped unexpectedly"
			node.Status.ContainerID = ""

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

	// Stop the container if we have a runtime and container ID
	if c.runtime != nil && node.Status.ContainerID != "" {
		if err := c.runtime.StopContainer(ctx, node.Status.ContainerID, 30*time.Second); err != nil {
			c.logger.Warn("failed to stop container",
				"devnet", node.Spec.DevnetRef,
				"index", node.Spec.Index,
				"error", err)
			// Continue anyway - we'll clear the container ID
		}
		node.Status.ContainerID = ""
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
