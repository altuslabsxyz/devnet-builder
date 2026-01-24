package controller

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

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

// DevnetController reconciles Devnet resources.
type DevnetController struct {
	store       store.Store
	provisioner Provisioner
	logger      *slog.Logger
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

// Reconcile processes a single devnet by name.
// It compares desired state (spec) with actual state (status) and takes action.
func (c *DevnetController) Reconcile(ctx context.Context, key string) error {
	c.logger.Debug("reconciling devnet", "name", key)

	// Get the devnet from store
	devnet, err := c.store.GetDevnet(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Devnet was deleted, nothing to do
			c.logger.Debug("devnet not found (deleted?)", "name", key)
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
// Transition: Pending -> Provisioning
func (c *DevnetController) reconcilePending(ctx context.Context, devnet *types.Devnet) error {
	c.logger.Info("starting provisioning", "name", devnet.Metadata.Name)

	devnet.Status.Phase = types.PhaseProvisioning
	devnet.Status.Message = "Starting provisioning"
	devnet.Metadata.UpdatedAt = time.Now()

	return c.store.UpdateDevnet(ctx, devnet)
}

// reconcileProvisioning handles devnets in Provisioning phase.
// Transition: Provisioning -> Running (or Degraded on failure)
func (c *DevnetController) reconcileProvisioning(ctx context.Context, devnet *types.Devnet) error {
	c.logger.Debug("checking provisioning progress", "name", devnet.Metadata.Name)

	// If we have a provisioner, use it
	if c.provisioner != nil {
		err := c.provisioner.Provision(ctx, devnet)
		if err != nil {
			c.logger.Error("provisioning failed", "name", devnet.Metadata.Name, "error", err)
			devnet.Status.Phase = types.PhaseDegraded
			devnet.Status.Message = "Provisioning failed: " + err.Error()
			devnet.Metadata.UpdatedAt = time.Now()
			return c.store.UpdateDevnet(ctx, devnet)
		}
	}

	// For now (Phase 2), we just mark as Running
	// Phase 3 will add actual Docker orchestration
	devnet.Status.Phase = types.PhaseRunning
	devnet.Status.Nodes = devnet.Spec.Validators + devnet.Spec.FullNodes
	devnet.Status.ReadyNodes = devnet.Status.Nodes // Assume all ready for now
	devnet.Status.Message = "Devnet is running"
	devnet.Status.LastHealthCheck = time.Now()
	devnet.Metadata.UpdatedAt = time.Now()

	c.logger.Info("provisioning complete",
		"name", devnet.Metadata.Name,
		"nodes", devnet.Status.Nodes)

	return c.store.UpdateDevnet(ctx, devnet)
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
