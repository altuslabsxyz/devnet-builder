// internal/daemon/controller/upgrade.go
package controller

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// UpgradeRuntime is the interface for chain upgrade operations.
// This will be implemented by the actual chain client.
type UpgradeRuntime interface {
	// SubmitUpgradeProposal submits a governance proposal for the upgrade.
	SubmitUpgradeProposal(ctx context.Context, devnetName string, upgradeName string, targetHeight int64) (proposalID uint64, err error)

	// GetProposalStatus returns the current voting status of a proposal.
	GetProposalStatus(ctx context.Context, devnetName string, proposalID uint64) (votesReceived, votesRequired int, passed bool, err error)

	// VoteOnProposal casts a vote on the proposal from a validator.
	VoteOnProposal(ctx context.Context, devnetName string, proposalID uint64, validatorIndex int, voteYes bool) error

	// GetCurrentHeight returns the chain's current block height.
	GetCurrentHeight(ctx context.Context, devnetName string) (int64, error)

	// SwitchNodeBinary replaces the binary on a node and restarts it.
	SwitchNodeBinary(ctx context.Context, devnetName string, nodeIndex int, newBinary types.BinarySource) error

	// VerifyNodeVersion checks that a node is running the expected version.
	VerifyNodeVersion(ctx context.Context, devnetName string, nodeIndex int, expectedVersion string) (bool, error)

	// ExportState exports the chain state to a file.
	ExportState(ctx context.Context, devnetName string, outputPath string) error

	// GetValidatorCount returns the number of validators in the devnet.
	GetValidatorCount(ctx context.Context, devnetName string) (int, error)
}

// UpgradeController reconciles Upgrade resources.
// It manages the lifecycle of chain upgrades, including governance proposals,
// voting, binary switching, and verification.
type UpgradeController struct {
	store   store.Store
	runtime UpgradeRuntime
	logger  *slog.Logger
}

// NewUpgradeController creates a new UpgradeController.
func NewUpgradeController(s store.Store, r UpgradeRuntime) *UpgradeController {
	return &UpgradeController{
		store:   s,
		runtime: r,
		logger:  slog.Default(),
	}
}

// SetLogger sets the logger for the controller.
func (c *UpgradeController) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

// Reconcile processes a single upgrade by key (format: "namespace/name" or just "name").
// It compares current phase with desired state and takes action to progress the upgrade.
func (c *UpgradeController) Reconcile(ctx context.Context, key string) error {
	c.logger.Debug("reconciling upgrade", "key", key)

	// Parse key - may be "namespace/name" or just "name" (uses default namespace)
	namespace, name := parseDevnetKey(key)

	// Get upgrade from store
	upgrade, err := c.store.GetUpgrade(ctx, namespace, name)
	if err != nil {
		if store.IsNotFound(err) {
			// Upgrade was deleted, nothing to do
			c.logger.Debug("upgrade not found (deleted?)", "key", key)
			return nil
		}
		return fmt.Errorf("failed to get upgrade %s: %w", key, err)
	}

	// Reconcile based on current phase
	switch upgrade.Status.Phase {
	case "", types.UpgradePhasePending:
		return c.reconcilePending(ctx, upgrade)
	case types.UpgradePhaseProposing:
		return c.reconcileProposing(ctx, upgrade)
	case types.UpgradePhaseVoting:
		return c.reconcileVoting(ctx, upgrade)
	case types.UpgradePhaseWaiting:
		return c.reconcileWaiting(ctx, upgrade)
	case types.UpgradePhaseSwitching:
		return c.reconcileSwitching(ctx, upgrade)
	case types.UpgradePhaseVerifying:
		return c.reconcileVerifying(ctx, upgrade)
	case types.UpgradePhaseCompleted, types.UpgradePhaseFailed:
		// Terminal states, nothing to do
		return nil
	default:
		c.logger.Warn("unknown upgrade phase", "key", key, "phase", upgrade.Status.Phase)
		return nil
	}
}

// reconcilePending handles upgrades in Pending phase.
// Transitions to Proposing to start the governance process.
func (c *UpgradeController) reconcilePending(ctx context.Context, upgrade *types.Upgrade) error {
	c.logger.Info("starting upgrade process",
		"name", upgrade.Metadata.Name,
		"devnet", upgrade.Spec.DevnetRef,
		"upgradeName", upgrade.Spec.UpgradeName)

	// Calculate target height if not specified
	if upgrade.Spec.TargetHeight == 0 && c.runtime != nil {
		currentHeight, err := c.runtime.GetCurrentHeight(ctx, upgrade.Spec.DevnetRef)
		if err != nil {
			return c.setFailed(ctx, upgrade, "failed to get current height: "+err.Error())
		}
		// Default offset: 100 blocks from current
		upgrade.Spec.TargetHeight = currentHeight + 100
		c.logger.Info("calculated target height",
			"name", upgrade.Metadata.Name,
			"currentHeight", currentHeight,
			"targetHeight", upgrade.Spec.TargetHeight)
	}

	// Get validator count for votes required
	if c.runtime != nil {
		validatorCount, err := c.runtime.GetValidatorCount(ctx, upgrade.Spec.DevnetRef)
		if err != nil {
			return c.setFailed(ctx, upgrade, "failed to get validator count: "+err.Error())
		}
		upgrade.Status.VotesRequired = validatorCount
	}

	// Pre-upgrade export if requested
	if upgrade.Spec.WithExport && c.runtime != nil {
		exportPath := fmt.Sprintf("/tmp/%s-pre-upgrade-export.json", upgrade.Metadata.Name)
		if err := c.runtime.ExportState(ctx, upgrade.Spec.DevnetRef, exportPath); err != nil {
			c.logger.Warn("pre-upgrade export failed",
				"name", upgrade.Metadata.Name,
				"error", err)
			// Non-fatal, continue with upgrade
		} else {
			upgrade.Status.PreExportPath = exportPath
		}
	}

	// Transition to Proposing
	upgrade.Status.Phase = types.UpgradePhaseProposing
	upgrade.Status.Message = "Creating governance proposal"

	return c.store.UpdateUpgrade(ctx, upgrade)
}

// reconcileProposing handles upgrades in Proposing phase.
// Submits the governance proposal and transitions to Voting.
func (c *UpgradeController) reconcileProposing(ctx context.Context, upgrade *types.Upgrade) error {
	c.logger.Debug("creating upgrade proposal",
		"name", upgrade.Metadata.Name,
		"devnet", upgrade.Spec.DevnetRef)

	// Submit proposal if we have a runtime
	if c.runtime != nil {
		proposalID, err := c.runtime.SubmitUpgradeProposal(
			ctx,
			upgrade.Spec.DevnetRef,
			upgrade.Spec.UpgradeName,
			upgrade.Spec.TargetHeight,
		)
		if err != nil {
			return c.setFailed(ctx, upgrade, "failed to submit proposal: "+err.Error())
		}
		upgrade.Status.ProposalID = proposalID
	} else {
		// No runtime - simulate proposal ID for testing
		upgrade.Status.ProposalID = 1
	}

	// Transition to Voting
	upgrade.Status.Phase = types.UpgradePhaseVoting
	upgrade.Status.Message = fmt.Sprintf("Waiting for votes (proposal #%d)", upgrade.Status.ProposalID)

	return c.store.UpdateUpgrade(ctx, upgrade)
}

// reconcileVoting handles upgrades in Voting phase.
// Monitors voting progress and auto-votes if configured.
// Transitions to Waiting when vote passes.
func (c *UpgradeController) reconcileVoting(ctx context.Context, upgrade *types.Upgrade) error {
	c.logger.Debug("checking voting status",
		"name", upgrade.Metadata.Name,
		"proposalID", upgrade.Status.ProposalID)

	if c.runtime != nil {
		// Auto-vote if enabled
		if upgrade.Spec.AutoVote && upgrade.Status.VotesReceived < upgrade.Status.VotesRequired {
			for i := 0; i < upgrade.Status.VotesRequired; i++ {
				if err := c.runtime.VoteOnProposal(ctx, upgrade.Spec.DevnetRef, upgrade.Status.ProposalID, i, true); err != nil {
					c.logger.Warn("auto-vote failed",
						"name", upgrade.Metadata.Name,
						"validator", i,
						"error", err)
					// Continue with other validators
				}
			}
		}

		// Check proposal status
		votesReceived, votesRequired, passed, err := c.runtime.GetProposalStatus(
			ctx,
			upgrade.Spec.DevnetRef,
			upgrade.Status.ProposalID,
		)
		if err != nil {
			c.logger.Warn("failed to get proposal status",
				"name", upgrade.Metadata.Name,
				"error", err)
			// Will retry on next reconcile
			return nil
		}

		upgrade.Status.VotesReceived = votesReceived
		upgrade.Status.VotesRequired = votesRequired

		if !passed {
			// Still waiting for votes
			upgrade.Status.Message = fmt.Sprintf("Votes: %d/%d", votesReceived, votesRequired)
			return c.store.UpdateUpgrade(ctx, upgrade)
		}
	} else {
		// No runtime - simulate voting complete
		upgrade.Status.VotesReceived = upgrade.Status.VotesRequired
	}

	// Vote passed - transition to Waiting
	c.logger.Info("upgrade proposal passed",
		"name", upgrade.Metadata.Name,
		"votes", upgrade.Status.VotesReceived)

	upgrade.Status.Phase = types.UpgradePhaseWaiting
	upgrade.Status.Message = fmt.Sprintf("Waiting for block height %d", upgrade.Spec.TargetHeight)

	return c.store.UpdateUpgrade(ctx, upgrade)
}

// reconcileWaiting handles upgrades in Waiting phase.
// Monitors chain height and transitions to Switching at target height.
func (c *UpgradeController) reconcileWaiting(ctx context.Context, upgrade *types.Upgrade) error {
	c.logger.Debug("waiting for target height",
		"name", upgrade.Metadata.Name,
		"targetHeight", upgrade.Spec.TargetHeight)

	if c.runtime != nil {
		currentHeight, err := c.runtime.GetCurrentHeight(ctx, upgrade.Spec.DevnetRef)
		if err != nil {
			c.logger.Warn("failed to get current height",
				"name", upgrade.Metadata.Name,
				"error", err)
			// Will retry on next reconcile
			return nil
		}

		upgrade.Status.CurrentHeight = currentHeight

		if currentHeight < upgrade.Spec.TargetHeight {
			// Still waiting
			blocksRemaining := upgrade.Spec.TargetHeight - currentHeight
			upgrade.Status.Message = fmt.Sprintf("Height %d/%d (%d blocks remaining)",
				currentHeight, upgrade.Spec.TargetHeight, blocksRemaining)
			return c.store.UpdateUpgrade(ctx, upgrade)
		}
	}

	// Target height reached - transition to Switching
	c.logger.Info("target height reached, switching binaries",
		"name", upgrade.Metadata.Name,
		"targetHeight", upgrade.Spec.TargetHeight)

	upgrade.Status.Phase = types.UpgradePhaseSwitching
	upgrade.Status.Message = "Switching node binaries"

	return c.store.UpdateUpgrade(ctx, upgrade)
}

// reconcileSwitching handles upgrades in Switching phase.
// Replaces binaries on all nodes and transitions to Verifying.
func (c *UpgradeController) reconcileSwitching(ctx context.Context, upgrade *types.Upgrade) error {
	c.logger.Info("switching node binaries",
		"name", upgrade.Metadata.Name,
		"devnet", upgrade.Spec.DevnetRef)

	if c.runtime != nil {
		validatorCount, err := c.runtime.GetValidatorCount(ctx, upgrade.Spec.DevnetRef)
		if err != nil {
			return c.setFailed(ctx, upgrade, "failed to get validator count: "+err.Error())
		}

		// Switch binary on each node
		for i := 0; i < validatorCount; i++ {
			c.logger.Debug("switching binary on node",
				"name", upgrade.Metadata.Name,
				"nodeIndex", i)

			if err := c.runtime.SwitchNodeBinary(ctx, upgrade.Spec.DevnetRef, i, upgrade.Spec.NewBinary); err != nil {
				return c.setFailed(ctx, upgrade, fmt.Sprintf("failed to switch binary on node %d: %s", i, err.Error()))
			}
		}
	}

	// Transition to Verifying
	upgrade.Status.Phase = types.UpgradePhaseVerifying
	upgrade.Status.Message = "Verifying upgrade success"

	return c.store.UpdateUpgrade(ctx, upgrade)
}

// reconcileVerifying handles upgrades in Verifying phase.
// Checks that all nodes are running the new version.
// Transitions to Completed on success.
func (c *UpgradeController) reconcileVerifying(ctx context.Context, upgrade *types.Upgrade) error {
	c.logger.Debug("verifying upgrade",
		"name", upgrade.Metadata.Name,
		"devnet", upgrade.Spec.DevnetRef)

	if c.runtime != nil {
		validatorCount, err := c.runtime.GetValidatorCount(ctx, upgrade.Spec.DevnetRef)
		if err != nil {
			return c.setFailed(ctx, upgrade, "failed to get validator count: "+err.Error())
		}

		// Verify each node
		expectedVersion := upgrade.Spec.NewBinary.Version
		for i := 0; i < validatorCount; i++ {
			verified, err := c.runtime.VerifyNodeVersion(ctx, upgrade.Spec.DevnetRef, i, expectedVersion)
			if err != nil {
				c.logger.Warn("version verification failed",
					"name", upgrade.Metadata.Name,
					"nodeIndex", i,
					"error", err)
				// Will retry on next reconcile
				return nil
			}
			if !verified {
				c.logger.Warn("node not yet on expected version",
					"name", upgrade.Metadata.Name,
					"nodeIndex", i,
					"expectedVersion", expectedVersion)
				// Will retry on next reconcile
				return nil
			}
		}

		// Post-upgrade export if requested
		if upgrade.Spec.WithExport {
			exportPath := fmt.Sprintf("/tmp/%s-post-upgrade-export.json", upgrade.Metadata.Name)
			if err := c.runtime.ExportState(ctx, upgrade.Spec.DevnetRef, exportPath); err != nil {
				c.logger.Warn("post-upgrade export failed",
					"name", upgrade.Metadata.Name,
					"error", err)
				// Non-fatal, continue
			} else {
				upgrade.Status.PostExportPath = exportPath
			}
		}
	}

	// All nodes verified - upgrade complete
	c.logger.Info("upgrade completed successfully",
		"name", upgrade.Metadata.Name,
		"devnet", upgrade.Spec.DevnetRef,
		"upgradeName", upgrade.Spec.UpgradeName)

	upgrade.Status.Phase = types.UpgradePhaseCompleted
	upgrade.Status.Message = "Upgrade completed successfully"

	return c.store.UpdateUpgrade(ctx, upgrade)
}

// setFailed transitions the upgrade to Failed phase with an error message.
func (c *UpgradeController) setFailed(ctx context.Context, upgrade *types.Upgrade, errMsg string) error {
	c.logger.Error("upgrade failed",
		"name", upgrade.Metadata.Name,
		"error", errMsg)

	upgrade.Status.Phase = types.UpgradePhaseFailed
	upgrade.Status.Message = "Upgrade failed"
	upgrade.Status.Error = errMsg

	return c.store.UpdateUpgrade(ctx, upgrade)
}

// Ensure UpgradeController implements Controller interface
var _ Controller = (*UpgradeController)(nil)
