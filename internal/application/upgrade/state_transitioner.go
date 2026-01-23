// Package upgrade provides use cases for managing blockchain upgrades.
package upgrade

import (
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
)

// StateTransitioner implements UpgradeStateTransitioner with a valid state machine.
type StateTransitioner struct {
	// validTransitions maps from stage to valid target stages.
	validTransitions map[ports.ResumableStage][]ports.ResumableStage
}

// NewStateTransitioner creates a new StateTransitioner with predefined valid transitions.
func NewStateTransitioner() *StateTransitioner {
	return &StateTransitioner{
		validTransitions: buildTransitionMap(),
	}
}

// buildTransitionMap creates the state machine transition rules.
// Based on research.md Decision 4: Stage State Machine
func buildTransitionMap() map[ports.ResumableStage][]ports.ResumableStage {
	return map[ports.ResumableStage][]ports.ResumableStage{
		// Initial stage can go to proposal submission or directly to switching (skip-gov)
		ports.ResumableStageInitialized: {
			ports.ResumableStageProposalSubmitted, // Normal governance path
			ports.ResumableStageSwitchingBinary,   // Skip-governance path
			ports.ResumableStageFailed,            // Error during initialization
		},

		// Proposal submitted moves to voting or fails
		ports.ResumableStageProposalSubmitted: {
			ports.ResumableStageVoting,           // Deposit period complete
			ports.ResumableStageFailed,           // Submission error
			ports.ResumableStageProposalRejected, // Proposal can be rejected during deposit
		},

		// Voting moves to waiting, rejected, or fails
		ports.ResumableStageVoting: {
			ports.ResumableStageWaitingForHeight, // Proposal passed
			ports.ResumableStageProposalRejected, // Proposal rejected/failed
			ports.ResumableStageFailed,           // Voting error
		},

		// Waiting for height moves to halted or fails
		ports.ResumableStageWaitingForHeight: {
			ports.ResumableStageChainHalted, // Block height reached
			ports.ResumableStageFailed,      // Timeout or error
		},

		// Chain halted moves to switching or fails
		ports.ResumableStageChainHalted: {
			ports.ResumableStageSwitchingBinary, // Halt detected
			ports.ResumableStageFailed,          // Detection error
		},

		// Switching binary moves to verifying or fails
		ports.ResumableStageSwitchingBinary: {
			ports.ResumableStageVerifyingResume, // All nodes switched
			ports.ResumableStageFailed,          // Switch error
		},

		// Verifying resume moves to completed or fails
		ports.ResumableStageVerifyingResume: {
			ports.ResumableStageCompleted, // Chain producing blocks
			ports.ResumableStageFailed,    // Verification timeout
		},

		// Terminal states have no valid transitions
		ports.ResumableStageCompleted:        {},
		ports.ResumableStageFailed:           {},
		ports.ResumableStageProposalRejected: {},
	}
}

// TransitionTo attempts to transition the state to the target stage.
// Returns error if the transition is invalid.
func (t *StateTransitioner) TransitionTo(state *ports.UpgradeState, target ports.ResumableStage, reason string) error {
	if state == nil {
		return &ports.InvalidTransitionError{From: "", To: target}
	}

	if !t.CanTransition(state.Stage, target) {
		return &ports.InvalidTransitionError{From: state.Stage, To: target}
	}

	// Record transition in history
	now := time.Now()
	transition := ports.StageTransition{
		From:      state.Stage,
		To:        target,
		Timestamp: now,
		Reason:    reason,
	}
	state.StageHistory = append(state.StageHistory, transition)

	// Update stage
	state.Stage = target
	state.UpdatedAt = now

	// If transitioning to Failed, ensure error field is populated if reason exists
	if target == ports.ResumableStageFailed && state.Error == "" && reason != "" {
		state.Error = reason
	}

	return nil
}

// CanTransition checks if a transition from the given stage to target is valid.
func (t *StateTransitioner) CanTransition(from, to ports.ResumableStage) bool {
	validTargets, exists := t.validTransitions[from]
	if !exists {
		return false
	}

	for _, valid := range validTargets {
		if valid == to {
			return true
		}
	}
	return false
}

// GetValidTransitions returns all valid target stages from the current stage.
func (t *StateTransitioner) GetValidTransitions(from ports.ResumableStage) []ports.ResumableStage {
	targets, exists := t.validTransitions[from]
	if !exists {
		return nil
	}
	// Return a copy to prevent modification
	result := make([]ports.ResumableStage, len(targets))
	copy(result, targets)
	return result
}

// IsGovernanceRequired returns true if the state requires governance (not skip-gov mode).
func (t *StateTransitioner) IsGovernanceRequired(state *ports.UpgradeState) bool {
	return !state.SkipGovernance
}

// GetNextStageForGovPath returns the next stage in the governance path.
func (t *StateTransitioner) GetNextStageForGovPath(current ports.ResumableStage) ports.ResumableStage {
	switch current {
	case ports.ResumableStageInitialized:
		return ports.ResumableStageProposalSubmitted
	case ports.ResumableStageProposalSubmitted:
		return ports.ResumableStageVoting
	case ports.ResumableStageVoting:
		return ports.ResumableStageWaitingForHeight
	case ports.ResumableStageWaitingForHeight:
		return ports.ResumableStageChainHalted
	case ports.ResumableStageChainHalted:
		return ports.ResumableStageSwitchingBinary
	case ports.ResumableStageSwitchingBinary:
		return ports.ResumableStageVerifyingResume
	case ports.ResumableStageVerifyingResume:
		return ports.ResumableStageCompleted
	default:
		return ""
	}
}

// GetNextStageForSkipGovPath returns the next stage in the skip-governance path.
func (t *StateTransitioner) GetNextStageForSkipGovPath(current ports.ResumableStage) ports.ResumableStage {
	switch current {
	case ports.ResumableStageInitialized:
		return ports.ResumableStageSwitchingBinary
	case ports.ResumableStageSwitchingBinary:
		return ports.ResumableStageVerifyingResume
	case ports.ResumableStageVerifyingResume:
		return ports.ResumableStageCompleted
	default:
		return ""
	}
}

// Ensure StateTransitioner implements UpgradeStateTransitioner.
var _ ports.UpgradeStateTransitioner = (*StateTransitioner)(nil)
