// internal/daemon/controller/health.go
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// HealthChecker abstracts health check operations for nodes.
// This will be implemented by RPC clients for different chain types.
type HealthChecker interface {
	// CheckHealth performs a health check on a node and returns the result.
	CheckHealth(ctx context.Context, node *types.Node) (*types.HealthCheckResult, error)
}

// HealthControllerConfig configures the HealthController.
type HealthControllerConfig struct {
	// CheckInterval is how often to run health checks.
	CheckInterval time.Duration

	// StuckThreshold is how long without new blocks before chain is considered stuck.
	StuckThreshold time.Duration

	// RestartPolicy defines auto-restart behavior.
	RestartPolicy types.RestartPolicy
}

// DefaultHealthControllerConfig returns sensible defaults.
func DefaultHealthControllerConfig() HealthControllerConfig {
	return HealthControllerConfig{
		CheckInterval:  30 * time.Second,
		StuckThreshold: 2 * time.Minute,
		RestartPolicy:  types.DefaultRestartPolicy(),
	}
}

// HealthController monitors node health and handles recovery.
// Unlike other controllers, it runs periodic sweeps rather than
// reconciling individual resources by key.
type HealthController struct {
	store   store.Store
	checker HealthChecker
	manager *Manager
	config  HealthControllerConfig
	logger  *slog.Logger

	// stopCh signals the health check loop to stop.
	stopCh chan struct{}
	// wg tracks running goroutines.
	wg sync.WaitGroup
}

// NewHealthController creates a new HealthController.
func NewHealthController(s store.Store, checker HealthChecker, mgr *Manager, config HealthControllerConfig) *HealthController {
	return &HealthController{
		store:   s,
		checker: checker,
		manager: mgr,
		config:  config,
		logger:  slog.Default(),
		stopCh:  make(chan struct{}),
	}
}

// SetLogger sets the logger for the controller.
func (c *HealthController) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

// Reconcile implements the Controller interface.
// For HealthController, the key is a devnet name.
// It checks health of all nodes in that devnet.
func (c *HealthController) Reconcile(ctx context.Context, key string) error {
	c.logger.Debug("reconciling health", "devnet", key)

	devnet, err := c.store.GetDevnet(ctx, key)
	if err != nil {
		if store.IsNotFound(err) {
			// Devnet was deleted
			return nil
		}
		return fmt.Errorf("failed to get devnet: %w", err)
	}

	// Get all nodes for this devnet
	nodes, err := c.store.ListNodes(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Check health of each node
	var (
		healthyCount   int
		unhealthyCount int
		stuckCount     int
	)

	for _, node := range nodes {
		result := c.checkNodeHealth(ctx, node)
		if result.Healthy {
			healthyCount++
		} else {
			unhealthyCount++
			if result.Error != "" && result.Error == "chain stuck" {
				stuckCount++
			}
		}
	}

	// Update devnet status
	devnet.Status.ReadyNodes = healthyCount
	devnet.Status.LastHealthCheck = time.Now()

	// Update conditions
	c.updateDevnetConditions(devnet, healthyCount, unhealthyCount, stuckCount, len(nodes))

	// Update devnet phase based on health
	c.updateDevnetPhase(devnet, healthyCount, len(nodes))

	if err := c.store.UpdateDevnet(ctx, devnet); err != nil {
		if store.IsConflict(err) {
			// Concurrent update, will be requeued
			return err
		}
		c.logger.Error("failed to update devnet", "name", key, "error", err)
	}

	return nil
}

// checkNodeHealth checks a single node's health.
func (c *HealthController) checkNodeHealth(ctx context.Context, node *types.Node) *types.HealthCheckResult {
	result := &types.HealthCheckResult{
		NodeKey:   NodeKey(node.Spec.DevnetRef, node.Spec.Index),
		CheckedAt: time.Now(),
	}

	// Skip nodes that aren't supposed to be running
	if node.Spec.Desired == types.NodePhaseStopped {
		result.Healthy = true
		return result
	}

	// Check if node is in a running state
	if node.Status.Phase != types.NodePhaseRunning {
		result.Healthy = false
		result.Error = fmt.Sprintf("node not running (phase: %s)", node.Status.Phase)

		// Handle crash recovery
		if node.Status.Phase == types.NodePhaseCrashed {
			c.handleCrashedNode(ctx, node)
		}
		return result
	}

	// If we have a checker, use it
	if c.checker != nil {
		checkResult, err := c.checker.CheckHealth(ctx, node)
		if err != nil {
			c.logger.Warn("health check failed",
				"node", result.NodeKey,
				"error", err)
			result.Healthy = false
			result.Error = err.Error()

			// Update failure count
			node.Status.ConsecutiveFailures++
			node.Status.LastHealthCheck = time.Now()
			if updateErr := c.store.UpdateNode(ctx, node); updateErr != nil {
				c.logger.Error("failed to update node after health check error", "node", node.Metadata.Name, "error", updateErr)
			}

			return result
		}
		*result = *checkResult
	}

	// Check for stuck chain
	if c.isChainStuck(node) {
		result.Healthy = false
		result.Error = "chain stuck"
		c.logger.Warn("chain appears stuck",
			"node", result.NodeKey,
			"lastBlockHeight", node.Status.BlockHeight,
			"lastBlockTime", node.Status.LastBlockTime)
	} else {
		result.Healthy = true
	}

	// Update node health state
	node.Status.LastHealthCheck = time.Now()
	if result.Healthy {
		node.Status.ConsecutiveFailures = 0
		if result.BlockHeight > node.Status.BlockHeight {
			node.Status.BlockHeight = result.BlockHeight
			node.Status.LastBlockTime = time.Now()
		}
	} else {
		node.Status.ConsecutiveFailures++
	}
	node.Status.PeerCount = result.PeerCount
	node.Status.CatchingUp = result.CatchingUp

	if err := c.store.UpdateNode(ctx, node); err != nil {
		c.logger.Warn("failed to update node health state",
			"node", result.NodeKey,
			"error", err)
	}

	return result
}

// isChainStuck checks if a node's chain hasn't produced blocks recently.
func (c *HealthController) isChainStuck(node *types.Node) bool {
	// If we've never seen a block, can't determine if stuck
	if node.Status.LastBlockTime.IsZero() {
		return false
	}

	// If the node is catching up, it's not stuck
	if node.Status.CatchingUp {
		return false
	}

	// Check if block height hasn't advanced within threshold
	return time.Since(node.Status.LastBlockTime) > c.config.StuckThreshold
}

// handleCrashedNode handles a crashed node according to restart policy.
func (c *HealthController) handleCrashedNode(ctx context.Context, node *types.Node) {
	policy := c.config.RestartPolicy

	if !policy.Enabled {
		c.logger.Debug("auto-restart disabled, not restarting crashed node",
			"node", NodeKey(node.Spec.DevnetRef, node.Spec.Index))
		return
	}

	// Check restart limit
	if policy.MaxRestarts > 0 && node.Status.RestartCount >= policy.MaxRestarts {
		c.logger.Warn("node exceeded max restarts",
			"node", NodeKey(node.Spec.DevnetRef, node.Spec.Index),
			"restartCount", node.Status.RestartCount,
			"maxRestarts", policy.MaxRestarts)
		return
	}

	// Check backoff
	if !node.Status.NextRestartTime.IsZero() && time.Now().Before(node.Status.NextRestartTime) {
		c.logger.Debug("node in restart backoff",
			"node", NodeKey(node.Spec.DevnetRef, node.Spec.Index),
			"nextRestartTime", node.Status.NextRestartTime)
		return
	}

	// Calculate next backoff
	backoff := c.calculateBackoff(node.Status.RestartCount, policy)
	node.Status.NextRestartTime = time.Now().Add(backoff)

	c.logger.Info("restarting crashed node",
		"node", NodeKey(node.Spec.DevnetRef, node.Spec.Index),
		"restartCount", node.Status.RestartCount,
		"backoff", backoff)

	// Transition to Pending to trigger restart
	node.Status.Phase = types.NodePhasePending
	node.Status.Message = "Auto-restarting after crash"
	node.Status.RestartCount++

	if err := c.store.UpdateNode(ctx, node); err != nil {
		c.logger.Error("failed to update node for restart",
			"node", NodeKey(node.Spec.DevnetRef, node.Spec.Index),
			"error", err)
		return
	}

	// Enqueue for NodeController to handle
	if c.manager != nil {
		c.manager.Enqueue("nodes", NodeKey(node.Spec.DevnetRef, node.Spec.Index))
	}
}

// calculateBackoff calculates the backoff duration for a restart.
func (c *HealthController) calculateBackoff(restartCount int, policy types.RestartPolicy) time.Duration {
	backoff := policy.BackoffInitial
	for i := 0; i < restartCount; i++ {
		backoff = time.Duration(float64(backoff) * policy.BackoffMultiplier)
		if backoff > policy.BackoffMax {
			backoff = policy.BackoffMax
			break
		}
	}
	return backoff
}

// updateDevnetConditions updates the devnet's condition list based on health.
func (c *HealthController) updateDevnetConditions(devnet *types.Devnet, healthy, unhealthy, stuck, total int) {
	now := time.Now()

	// Find or create conditions
	conditions := make(map[string]*types.Condition)
	for i := range devnet.Status.Conditions {
		conditions[devnet.Status.Conditions[i].Type] = &devnet.Status.Conditions[i]
	}

	// Ready condition
	ready := conditions[types.ConditionTypeReady]
	if ready == nil {
		ready = &types.Condition{Type: types.ConditionTypeReady}
		devnet.Status.Conditions = append(devnet.Status.Conditions, *ready)
		conditions[types.ConditionTypeReady] = &devnet.Status.Conditions[len(devnet.Status.Conditions)-1]
		ready = conditions[types.ConditionTypeReady]
	}

	if healthy == total && total > 0 {
		c.setCondition(ready, types.ConditionTrue, "AllNodesReady", "All nodes are ready", now)
	} else if healthy > 0 {
		c.setCondition(ready, types.ConditionFalse, "SomeNodesNotReady",
			fmt.Sprintf("%d/%d nodes ready", healthy, total), now)
	} else if total > 0 {
		c.setCondition(ready, types.ConditionFalse, "NoNodesReady", "No nodes are ready", now)
	} else {
		c.setCondition(ready, types.ConditionUnknown, "NoNodes", "No nodes exist", now)
	}

	// Healthy condition
	healthyCond := conditions[types.ConditionTypeHealthy]
	if healthyCond == nil {
		healthyCond = &types.Condition{Type: types.ConditionTypeHealthy}
		devnet.Status.Conditions = append(devnet.Status.Conditions, *healthyCond)
		conditions[types.ConditionTypeHealthy] = &devnet.Status.Conditions[len(devnet.Status.Conditions)-1]
		healthyCond = conditions[types.ConditionTypeHealthy]
	}

	if unhealthy == 0 && total > 0 {
		c.setCondition(healthyCond, types.ConditionTrue, "AllNodesHealthy", "All nodes are healthy", now)
	} else if unhealthy > 0 {
		msg := fmt.Sprintf("%d/%d nodes unhealthy", unhealthy, total)
		if stuck > 0 {
			msg = fmt.Sprintf("%s (%d stuck)", msg, stuck)
		}
		c.setCondition(healthyCond, types.ConditionFalse, "SomeNodesUnhealthy", msg, now)
	} else {
		c.setCondition(healthyCond, types.ConditionUnknown, "NoNodes", "No nodes to check", now)
	}

	// Degraded condition
	degraded := conditions[types.ConditionTypeDegraded]
	if degraded == nil {
		degraded = &types.Condition{Type: types.ConditionTypeDegraded}
		devnet.Status.Conditions = append(devnet.Status.Conditions, *degraded)
		conditions[types.ConditionTypeDegraded] = &devnet.Status.Conditions[len(devnet.Status.Conditions)-1]
		degraded = conditions[types.ConditionTypeDegraded]
	}

	if stuck > 0 {
		c.setCondition(degraded, types.ConditionTrue, "ChainStuck",
			fmt.Sprintf("%d node(s) stuck", stuck), now)
	} else if unhealthy > 0 {
		c.setCondition(degraded, types.ConditionTrue, "NodesUnhealthy",
			fmt.Sprintf("%d node(s) unhealthy", unhealthy), now)
	} else {
		c.setCondition(degraded, types.ConditionFalse, "OperatingNormally", "All nodes operating normally", now)
	}
}

// setCondition updates a condition if the status changed.
func (c *HealthController) setCondition(cond *types.Condition, status, reason, message string, now time.Time) {
	if cond.Status != status {
		cond.LastTransitionTime = now
	}
	cond.Status = status
	cond.Reason = reason
	cond.Message = message
}

// updateDevnetPhase updates the devnet phase based on health.
func (c *HealthController) updateDevnetPhase(devnet *types.Devnet, healthy, total int) {
	// Only update phase if devnet is in a running-type state
	switch devnet.Status.Phase {
	case types.PhaseRunning, types.PhaseDegraded:
		if healthy == total && total > 0 {
			devnet.Status.Phase = types.PhaseRunning
			devnet.Status.Message = "All nodes healthy"
		} else if healthy > 0 {
			devnet.Status.Phase = types.PhaseDegraded
			devnet.Status.Message = fmt.Sprintf("%d/%d nodes healthy", healthy, total)
		} else if total > 0 {
			devnet.Status.Phase = types.PhaseDegraded
			devnet.Status.Message = "No nodes healthy"
		}
	}
}

// Start begins the periodic health check loop.
func (c *HealthController) Start(ctx context.Context) {
	c.wg.Add(1)
	go c.healthCheckLoop(ctx)
}

// Stop stops the health check loop.
func (c *HealthController) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// healthCheckLoop runs periodic health checks.
func (c *HealthController) healthCheckLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	// Run initial check
	c.runHealthSweep(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.runHealthSweep(ctx)
		}
	}
}

// runHealthSweep checks health of all devnets.
func (c *HealthController) runHealthSweep(ctx context.Context) {
	c.logger.Debug("running health sweep")

	devnets, err := c.store.ListDevnets(ctx)
	if err != nil {
		c.logger.Error("failed to list devnets for health sweep", "error", err)
		return
	}

	for _, devnet := range devnets {
		// Skip devnets that aren't running
		if devnet.Status.Phase != types.PhaseRunning && devnet.Status.Phase != types.PhaseDegraded {
			continue
		}

		// Enqueue for reconciliation
		if c.manager != nil {
			c.manager.Enqueue("health", devnet.Metadata.Name)
		} else {
			// Direct reconciliation if no manager
			if err := c.Reconcile(ctx, devnet.Metadata.Name); err != nil {
				c.logger.Error("health reconcile failed",
					"devnet", devnet.Metadata.Name,
					"error", err)
			}
		}
	}
}

// Ensure HealthController implements Controller interface.
var _ Controller = (*HealthController)(nil)
