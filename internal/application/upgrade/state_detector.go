// Package upgrade provides use cases for software upgrade operations.
package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
)

// StateDetector implements UpgradeStateDetector for detecting chain state.
// It queries the chain to reconcile saved upgrade state with actual chain reality.
type StateDetector struct {
	rpcClient ports.RPCClient
}

// NewStateDetector creates a new StateDetector.
func NewStateDetector(rpcClient ports.RPCClient) *StateDetector {
	return &StateDetector{
		rpcClient: rpcClient,
	}
}

// DetectProposalStatus queries governance for proposal status.
// Returns: "voting", "passed", "rejected", "failed", or "unknown".
func (d *StateDetector) DetectProposalStatus(ctx context.Context, proposalID uint64) (string, error) {
	if proposalID == 0 {
		return "unknown", fmt.Errorf("invalid proposal ID: 0")
	}

	proposal, err := d.rpcClient.GetProposal(ctx, proposalID)
	if err != nil {
		return "unknown", fmt.Errorf("failed to get proposal: %w", err)
	}

	switch proposal.Status {
	case ports.ProposalStatusVoting:
		return "voting", nil
	case ports.ProposalStatusPassed:
		return "passed", nil
	case ports.ProposalStatusRejected:
		return "rejected", nil
	case ports.ProposalStatusFailed:
		return "failed", nil
	case ports.ProposalStatusPending:
		return "pending", nil
	default:
		return "unknown", nil
	}
}

// DetectChainStatus checks if chain is running, halted, or unreachable.
// Returns: "running", "halted", or "unreachable".
//
// Detection logic:
// - If RPC is unreachable → "unreachable"
// - If RPC responds but blocks aren't advancing → "halted"
// - If blocks are advancing → "running"
func (d *StateDetector) DetectChainStatus(ctx context.Context) (string, error) {
	// First check if chain is reachable at all
	if !d.rpcClient.IsChainRunning(ctx) {
		return "unreachable", nil
	}

	// Get current height
	height1, err := d.rpcClient.GetBlockHeight(ctx)
	if err != nil {
		return "unreachable", nil
	}

	// Wait briefly and check again to see if blocks are advancing
	// Use a short timeout to detect if chain is halted
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Wait 2 seconds (typical block times are 1-6 seconds)
	select {
	case <-checkCtx.Done():
		// Context cancelled, assume halted
		return "halted", nil
	case <-time.After(2 * time.Second):
	}

	height2, err := d.rpcClient.GetBlockHeight(ctx)
	if err != nil {
		return "unreachable", nil
	}

	// If height hasn't changed, chain is likely halted
	if height2 <= height1 {
		return "halted", nil
	}

	return "running", nil
}

// DetectValidatorVotes queries which validators have voted on a proposal.
// Note: This is a placeholder implementation. Full implementation would require
// querying the governance module's vote endpoint for each validator.
func (d *StateDetector) DetectValidatorVotes(ctx context.Context, proposalID uint64) ([]ports.ValidatorVoteState, error) {
	if proposalID == 0 {
		return nil, fmt.Errorf("invalid proposal ID: 0")
	}

	// For now, return empty slice. The full implementation would query:
	// /cosmos/gov/v1/proposals/{proposal_id}/votes
	// and cross-reference with validator set
	//
	// This is acceptable because:
	// 1. The saved state tracks votes as they happen
	// 2. On resume, we trust the saved state for vote tracking
	// 3. This method is mainly for reconciliation edge cases
	return []ports.ValidatorVoteState{}, nil
}

// DetectCurrentStage queries chain state to determine actual stage.
// Used on resume to reconcile saved state with chain reality.
//
// Detection logic:
// 1. If no proposal ID → Initialized (or SwitchingBinary if skip-gov)
// 2. Query proposal status:
//   - If passed → check if upgrade height reached
//   - If rejected → ProposalRejected
//   - If voting → Voting
//   - If pending → ProposalSubmitted
//
// 3. If upgrade height reached, check chain status:
//   - If halted → ChainHalted
//   - If running with new binary → VerifyingResume or Completed
func (d *StateDetector) DetectCurrentStage(ctx context.Context, state *ports.UpgradeState) (ports.ResumableStage, error) {
	if state == nil {
		return "", fmt.Errorf("state is nil")
	}

	// Skip-governance path detection
	if state.SkipGovernance {
		return d.detectSkipGovStage(ctx, state)
	}

	// Governance path detection
	return d.detectGovPathStage(ctx, state)
}

// detectSkipGovStage handles stage detection for skip-governance upgrades.
func (d *StateDetector) detectSkipGovStage(ctx context.Context, state *ports.UpgradeState) (ports.ResumableStage, error) {
	// For skip-gov, the progression is:
	// Initialized → SwitchingBinary → VerifyingResume → Completed
	//
	// We check based on node switch status
	if len(state.NodeSwitches) == 0 {
		// No switches recorded, still at initialization
		return ports.ResumableStageInitialized, nil
	}

	// Check if all nodes have been switched
	allSwitched := true
	anySwitched := false
	for _, ns := range state.NodeSwitches {
		if ns.Switched {
			anySwitched = true
		} else {
			allSwitched = false
		}
	}

	if !anySwitched {
		// None switched yet
		return ports.ResumableStageSwitchingBinary, nil
	}

	if allSwitched {
		// All switched, verify chain is running
		chainStatus, err := d.DetectChainStatus(ctx)
		if err != nil {
			return state.Stage, nil // Keep saved stage on error
		}

		if chainStatus == "running" {
			return ports.ResumableStageVerifyingResume, nil
		}
		// Chain not running yet, still in switching
		return ports.ResumableStageSwitchingBinary, nil
	}

	// Some switched, some not
	return ports.ResumableStageSwitchingBinary, nil
}

// detectGovPathStage handles stage detection for governance upgrades.
func (d *StateDetector) detectGovPathStage(ctx context.Context, state *ports.UpgradeState) (ports.ResumableStage, error) {
	// No proposal ID means we haven't submitted yet
	if state.ProposalID == 0 {
		return ports.ResumableStageInitialized, nil
	}

	// Query proposal status from chain
	proposalStatus, err := d.DetectProposalStatus(ctx, state.ProposalID)
	if err != nil {
		// Can't query chain, trust saved state
		return state.Stage, nil
	}

	switch proposalStatus {
	case "pending":
		return ports.ResumableStageProposalSubmitted, nil

	case "voting":
		return ports.ResumableStageVoting, nil

	case "rejected":
		return ports.ResumableStageProposalRejected, nil

	case "failed":
		return ports.ResumableStageFailed, nil

	case "passed":
		// Proposal passed, check if we've reached upgrade height
		return d.detectPostPassedStage(ctx, state)
	}

	// Unknown status, trust saved state
	return state.Stage, nil
}

// detectPostPassedStage handles stage detection after proposal passes.
func (d *StateDetector) detectPostPassedStage(ctx context.Context, state *ports.UpgradeState) (ports.ResumableStage, error) {
	// No upgrade height means something is wrong
	if state.UpgradeHeight == 0 {
		return ports.ResumableStageWaitingForHeight, nil
	}

	// Check current chain status
	chainStatus, err := d.DetectChainStatus(ctx)
	if err != nil {
		return state.Stage, nil // Keep saved stage on error
	}

	// Get current block height if chain is reachable
	var currentHeight int64
	if chainStatus != "unreachable" {
		currentHeight, _ = d.rpcClient.GetBlockHeight(ctx)
	}

	switch chainStatus {
	case "halted":
		// Chain is halted, likely at upgrade height
		return ports.ResumableStageChainHalted, nil

	case "running":
		// Chain is running
		if currentHeight < state.UpgradeHeight {
			// Haven't reached upgrade height yet
			return ports.ResumableStageWaitingForHeight, nil
		}

		// Chain is running past upgrade height
		// Check if nodes have been switched
		if len(state.NodeSwitches) > 0 {
			allSwitched := true
			for _, ns := range state.NodeSwitches {
				if !ns.Switched {
					allSwitched = false
					break
				}
			}
			if allSwitched {
				return ports.ResumableStageVerifyingResume, nil
			}
			return ports.ResumableStageSwitchingBinary, nil
		}

		// No switches recorded but chain running past height
		// This could mean upgrade completed or binary switch in progress
		return ports.ResumableStageSwitchingBinary, nil

	case "unreachable":
		// Chain unreachable, could be halted or crashed
		// Check upgrade height vs last known state
		if state.Stage == ports.ResumableStageWaitingForHeight {
			// Was waiting, now unreachable - probably halted
			return ports.ResumableStageChainHalted, nil
		}
		// Keep saved stage
		return state.Stage, nil
	}

	return state.Stage, nil
}

// Ensure StateDetector implements UpgradeStateDetector.
var _ ports.UpgradeStateDetector = (*StateDetector)(nil)
