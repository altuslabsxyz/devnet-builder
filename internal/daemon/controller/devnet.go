package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// parseDevnetKey parses a devnet key into namespace and name.
// Key can be "namespace/name" or just "name" (uses default namespace).
func parseDevnetKey(key string) (namespace, name string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return types.DefaultNamespace, key
}

// Provisioner is the interface for provisioning devnet infrastructure.
// This will be implemented by the DevnetProvisioner in Phase 3.
type Provisioner interface {
	// Provision creates the infrastructure for a devnet.
	Provision(ctx context.Context, devnet *types.Devnet) error

	// Deprovision removes the infrastructure for a devnet.
	Deprovision(ctx context.Context, devnet *types.Devnet) error

	// Start starts a stopped devnet.
	Start(ctx context.Context, devnet *types.Devnet) error

	// Stop stops a running devnet.
	Stop(ctx context.Context, devnet *types.Devnet) error

	// GetStatus gets the current status of a devnet.
	GetStatus(ctx context.Context, devnet *types.Devnet) (*types.DevnetStatus, error)
}

// ProgressProvisioner extends Provisioner with progress reporting capabilities.
// If a provisioner implements this interface, the controller will wire up
// progress callbacks to emit granular provisioning events.
type ProgressProvisioner interface {
	Provisioner

	// SetProgressCallback sets the callback for provisioning progress updates.
	// The callback receives the phase name and a message.
	SetProgressCallback(callback func(phase, message string))
}

// DevnetController reconciles Devnet resources.
type DevnetController struct {
	store       store.Store
	provisioner Provisioner
	manager     *Manager
	logger      *slog.Logger

	// logSubscribers holds log subscriber wrappers, keyed by devnet name.
	// Each subscriber has a channel for log entries and a done signal for safe cleanup.
	logSubscribers map[string][]*logSubscriber
	logMu          sync.RWMutex
}

// NewDevnetController creates a new DevnetController.
func NewDevnetController(s store.Store, p Provisioner) *DevnetController {
	return &DevnetController{
		store:       s,
		provisioner: p,
		logger:      slog.Default(),
	}
}

// SetLogger sets the logger for the controller.
func (c *DevnetController) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

// SetManager sets the controller manager for enqueueing nodes.
// This allows the DevnetController to trigger node reconciliation
// after provisioning creates nodes.
func (c *DevnetController) SetManager(mgr *Manager) {
	c.manager = mgr
}

// Reconcile processes a single devnet by key (format: "namespace/name" or just "name").
// It compares desired state (spec) with actual state (status) and takes action.
func (c *DevnetController) Reconcile(ctx context.Context, key string) error {
	c.logger.Debug("reconciling devnet", "key", key)

	// Parse key - may be "namespace/name" or just "name" (uses default namespace)
	namespace, name := parseDevnetKey(key)

	// Get the devnet from store
	devnet, err := c.store.GetDevnet(ctx, namespace, name)
	if err != nil {
		if store.IsNotFound(err) {
			// Devnet was deleted, nothing to do
			c.logger.Debug("devnet not found (deleted?)", "key", key)
			return nil
		}
		return err
	}

	// Reconcile based on current phase
	switch devnet.Status.Phase {
	case "", types.PhasePending:
		return c.reconcilePending(ctx, devnet)
	case types.PhaseProvisioning:
		return c.reconcileProvisioning(ctx, devnet)
	case types.PhaseRunning:
		return c.reconcileRunning(ctx, devnet)
	case types.PhaseDegraded:
		return c.reconcileDegraded(ctx, devnet)
	case types.PhaseStopped:
		return c.reconcileStopped(ctx, devnet)
	default:
		c.logger.Warn("unknown phase", "name", devnet.Metadata.Name, "phase", devnet.Status.Phase)
		return nil
	}
}

// reconcilePending handles devnets in Pending phase.
// Transition: Pending -> Provisioning -> Running (or Degraded on failure)
// Note: This method continues directly to reconcileProvisioning to avoid
// requiring a store watcher or manual re-enqueue after phase transition.
func (c *DevnetController) reconcilePending(ctx context.Context, devnet *types.Devnet) error {
	c.logger.Info("starting provisioning", "name", devnet.Metadata.Name)

	// Set Progressing condition
	devnet.Status.Conditions = types.SetCondition(
		devnet.Status.Conditions,
		types.ConditionTypeProgressing,
		types.ConditionTrue,
		types.ReasonProvisioning,
		"Starting provisioning",
	)

	// Set Ready condition to false initially
	devnet.Status.Conditions = types.SetCondition(
		devnet.Status.Conditions,
		types.ConditionTypeReady,
		types.ConditionFalse,
		types.ReasonNodesNotReady,
		"Waiting for nodes to be created",
	)

	// Add event
	devnet.Status.Events = append(devnet.Status.Events, types.NewEvent(
		types.EventTypeNormal,
		types.ReasonProvisioning,
		fmt.Sprintf("Starting provisioning for %d validators", devnet.Spec.Validators),
		"devnet-controller",
	))

	devnet.Status.Phase = types.PhaseProvisioning
	devnet.Status.Message = "Starting provisioning"
	devnet.Metadata.UpdatedAt = time.Now()

	// Save the phase transition
	if err := c.store.UpdateDevnet(ctx, devnet); err != nil {
		return fmt.Errorf("failed to update devnet phase: %w", err)
	}

	// Continue directly to provisioning to avoid requiring re-enqueue.
	// Without a store watcher, returning here would leave the devnet stuck
	// in Provisioning phase with no subsequent reconcile triggered.
	return c.reconcileProvisioning(ctx, devnet)
}

// reconcileProvisioning handles devnets in Provisioning phase.
// Transition: Provisioning -> Running (or Degraded on failure)
func (c *DevnetController) reconcileProvisioning(ctx context.Context, devnet *types.Devnet) error {
	c.logger.Debug("checking provisioning progress", "name", devnet.Metadata.Name)

	// If we have a provisioner, use it
	if c.provisioner != nil {
		// Wire up progress callback if the provisioner supports it
		if progressProv, ok := c.provisioner.(ProgressProvisioner); ok {
			progressProv.SetProgressCallback(func(phase, message string) {
				c.handleProvisioningProgress(ctx, devnet, phase, message)
			})
		}

		err := c.provisioner.Provision(ctx, devnet)
		if err != nil {
			c.logger.Error("provisioning failed", "name", devnet.Metadata.Name, "error", err)

			// Classify the error and set appropriate conditions
			reason, message := c.classifyProvisioningError(err)

			// Set Progressing to false
			devnet.Status.Conditions = types.SetCondition(
				devnet.Status.Conditions,
				types.ConditionTypeProgressing,
				types.ConditionFalse,
				reason,
				message,
			)

			// Set Degraded to true
			devnet.Status.Conditions = types.SetCondition(
				devnet.Status.Conditions,
				types.ConditionTypeDegraded,
				types.ConditionTrue,
				reason,
				message,
			)

			// Add warning event with the classified reason
			devnet.Status.Events = append(devnet.Status.Events, types.NewEvent(
				types.EventTypeWarning,
				reason,
				message,
				"devnet-controller",
			))

			devnet.Status.Phase = types.PhaseDegraded
			devnet.Status.Message = "Provisioning failed: " + err.Error()
			devnet.Metadata.UpdatedAt = time.Now()
			return c.store.UpdateDevnet(ctx, devnet)
		}
	}

	// For now (Phase 2), we just mark as Running
	// Phase 3 will add actual Docker orchestration
	devnet.Status.Nodes = devnet.Spec.Validators + devnet.Spec.FullNodes
	devnet.Status.ReadyNodes = devnet.Status.Nodes // Assume all ready for now

	// Set Progressing to false (complete)
	devnet.Status.Conditions = types.SetCondition(
		devnet.Status.Conditions,
		types.ConditionTypeProgressing,
		types.ConditionFalse,
		"ProvisioningComplete",
		"Provisioning completed successfully",
	)

	// Set NodesCreated to true
	devnet.Status.Conditions = types.SetCondition(
		devnet.Status.Conditions,
		types.ConditionTypeNodesCreated,
		types.ConditionTrue,
		types.ReasonAllNodesReady,
		fmt.Sprintf("%d/%d nodes created", devnet.Status.Nodes, devnet.Status.Nodes),
	)

	// Set Ready to true
	devnet.Status.Conditions = types.SetCondition(
		devnet.Status.Conditions,
		types.ConditionTypeReady,
		types.ConditionTrue,
		types.ReasonAllNodesReady,
		fmt.Sprintf("%d/%d nodes ready", devnet.Status.ReadyNodes, devnet.Status.Nodes),
	)

	// Add event
	devnet.Status.Events = append(devnet.Status.Events, types.NewEvent(
		types.EventTypeNormal,
		"ProvisioningComplete",
		fmt.Sprintf("Successfully provisioned %d nodes", devnet.Status.Nodes),
		"devnet-controller",
	))

	devnet.Status.Phase = types.PhaseRunning
	devnet.Status.Message = "Devnet is running"
	devnet.Status.LastHealthCheck = time.Now()
	devnet.Metadata.UpdatedAt = time.Now()

	c.logger.Info("provisioning complete",
		"name", devnet.Metadata.Name,
		"nodes", devnet.Status.Nodes)

	// Enqueue all created nodes for reconciliation by the NodeController.
	// Without this, nodes remain in Pending phase with no process started.
	if c.manager != nil {
		namespace := devnet.Metadata.Namespace
		if namespace == "" {
			namespace = types.DefaultNamespace
		}
		nodes, err := c.store.ListNodes(ctx, namespace, devnet.Metadata.Name)
		if err != nil {
			c.logger.Warn("failed to list nodes for enqueueing",
				"name", devnet.Metadata.Name,
				"error", err)
		} else {
			for _, node := range nodes {
				key := NodeKeyWithNamespace(namespace, node.Spec.DevnetRef, node.Spec.Index)
				c.manager.Enqueue("nodes", key)
				c.logger.Debug("enqueued node for reconciliation",
					"devnet", devnet.Metadata.Name,
					"nodeIndex", node.Spec.Index,
					"key", key)
			}
		}
	}

	return c.store.UpdateDevnet(ctx, devnet)
}

// classifyProvisioningError determines the reason code for an error.
func (c *DevnetController) classifyProvisioningError(err error) (reason, message string) {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "image") && strings.Contains(errStr, "not found"):
		return types.ReasonImageNotFound, fmt.Sprintf("Docker image not found: %v", err)
	case strings.Contains(errStr, "credentials"):
		return types.ReasonCredentialsNotFound, fmt.Sprintf("Credentials not found: %v", err)
	case strings.Contains(errStr, "mode") && strings.Contains(errStr, "not supported"):
		return types.ReasonModeNotSupported, fmt.Sprintf("Execution mode not supported: %v", err)
	case strings.Contains(errStr, "binary") && strings.Contains(errStr, "not found"):
		return types.ReasonBinaryNotFound, fmt.Sprintf("Binary not found: %v", err)
	case strings.Contains(errStr, "container"):
		return types.ReasonContainerFailed, fmt.Sprintf("Container operation failed: %v", err)
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection"):
		return types.ReasonNetworkError, fmt.Sprintf("Network error: %v", err)
	default:
		return types.ReasonProvisionFailed, fmt.Sprintf("Provisioning failed: %v", err)
	}
}

// handleProvisioningProgress processes provisioning phase updates and emits events.
// This is called by the provisioner's progress callback during provisioning.
// It maps orchestrator phases to granular events for the devnet status.
func (c *DevnetController) handleProvisioningProgress(ctx context.Context, devnet *types.Devnet, phase, message string) {
	var eventReason string
	var eventMessage string

	// Map orchestrator phases to event reasons
	switch phase {
	case "Building":
		eventReason = types.ReasonBinaryBuilding
		eventMessage = "Building binary from source"
	case "Forking":
		eventReason = types.ReasonGenesisFork
		eventMessage = "Forking genesis from source"
	case "Initializing":
		eventReason = types.ReasonNodesConfiguring
		eventMessage = "Configuring node directories"
	case "Starting":
		eventReason = types.ReasonNodesConfigured
		eventMessage = "Node configuration complete, starting nodes"
	case "HealthChecking":
		// Don't emit a separate event for health checking, it's part of starting
		return
	case "Running":
		// Running phase is handled in reconcileProvisioning completion
		return
	case "Failed":
		// Failed phase is handled in reconcileProvisioning error handling
		return
	default:
		// Unknown phase, log but don't emit event
		c.logger.Debug("unknown provisioning phase", "phase", phase, "message", message)
		return
	}

	// Use message from callback if provided, otherwise use default
	if message != "" {
		eventMessage = message
	}

	// Add the event to the devnet status
	devnet.Status.Events = append(devnet.Status.Events, types.NewEvent(
		types.EventTypeNormal,
		eventReason,
		eventMessage,
		"devnet-controller",
	))

	// Update status message
	devnet.Status.Message = eventMessage

	// Broadcast log entry for subscribers
	c.broadcastLog(devnet.Metadata.Namespace, devnet.Metadata.Name, &ProvisionLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   eventMessage,
		Phase:     phase,
	})

	c.logger.Debug("provisioning progress",
		"name", devnet.Metadata.Name,
		"phase", phase,
		"reason", eventReason,
		"message", eventMessage)

	// Save updated status to store (best effort, don't fail on error)
	if err := c.store.UpdateDevnet(ctx, devnet); err != nil {
		c.logger.Warn("failed to save provisioning progress",
			"name", devnet.Metadata.Name,
			"error", err)
	}
}

// emitNodeReadyEvent emits a NodeReady event when a node becomes ready.
// This is a helper for emitting per-node ready events during provisioning.
func (c *DevnetController) emitNodeReadyEvent(devnet *types.Devnet, nodeIndex int) {
	devnet.Status.Events = append(devnet.Status.Events, types.NewEvent(
		types.EventTypeNormal,
		types.ReasonNodeReady,
		fmt.Sprintf("Node %d is ready", nodeIndex),
		"devnet-controller",
	))

	c.broadcastLog(devnet.Metadata.Namespace, devnet.Metadata.Name, &ProvisionLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   fmt.Sprintf("Node %d is ready", nodeIndex),
		Phase:     "NodeReady",
	})
}

// reconcileRunning handles devnets in Running phase.
// Ensures the devnet remains healthy.
func (c *DevnetController) reconcileRunning(ctx context.Context, devnet *types.Devnet) error {
	c.logger.Debug("checking running devnet", "name", devnet.Metadata.Name)

	// If we have a provisioner, check actual status
	if c.provisioner != nil {
		status, err := c.provisioner.GetStatus(ctx, devnet)
		if err != nil {
			c.logger.Warn("failed to get status", "name", devnet.Metadata.Name, "error", err)
			// Don't fail reconciliation, just log
			return nil
		}

		// Check for degraded state
		if status.ReadyNodes < status.Nodes {
			c.logger.Warn("devnet degraded",
				"name", devnet.Metadata.Name,
				"ready", status.ReadyNodes,
				"total", status.Nodes)

			devnet.Status.Phase = types.PhaseDegraded
			devnet.Status.ReadyNodes = status.ReadyNodes
			devnet.Status.Message = "Some nodes unhealthy"
			devnet.Metadata.UpdatedAt = time.Now()
			return c.store.UpdateDevnet(ctx, devnet)
		}

		// Update status
		devnet.Status.CurrentHeight = status.CurrentHeight
		devnet.Status.ReadyNodes = status.ReadyNodes
		devnet.Status.LastHealthCheck = time.Now()
	}

	// Nothing to do, devnet is healthy
	return nil
}

// reconcileDegraded handles devnets in Degraded phase.
// Attempts recovery.
func (c *DevnetController) reconcileDegraded(ctx context.Context, devnet *types.Devnet) error {
	c.logger.Debug("checking degraded devnet", "name", devnet.Metadata.Name)

	// If we have a provisioner, attempt recovery
	if c.provisioner != nil {
		status, err := c.provisioner.GetStatus(ctx, devnet)
		if err != nil {
			c.logger.Warn("failed to get status", "name", devnet.Metadata.Name, "error", err)
			return nil
		}

		// Check if recovered
		if status.ReadyNodes >= status.Nodes {
			c.logger.Info("devnet recovered", "name", devnet.Metadata.Name)
			devnet.Status.Phase = types.PhaseRunning
			devnet.Status.ReadyNodes = status.ReadyNodes
			devnet.Status.Message = "Devnet is running"
			devnet.Status.LastHealthCheck = time.Now()
			devnet.Metadata.UpdatedAt = time.Now()
			return c.store.UpdateDevnet(ctx, devnet)
		}

		// Still degraded, could attempt restart of unhealthy nodes
		// This will be implemented in Phase 3
	}

	// Without provisioner, we can't recover
	return nil
}

// reconcileStopped handles devnets in Stopped phase.
// Nothing to do unless explicit start is requested.
func (c *DevnetController) reconcileStopped(ctx context.Context, devnet *types.Devnet) error {
	c.logger.Debug("devnet is stopped", "name", devnet.Metadata.Name)
	// Nothing to do - wait for explicit StartDevnet call
	return nil
}
