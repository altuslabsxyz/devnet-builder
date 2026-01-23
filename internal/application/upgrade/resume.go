// Package upgrade provides use cases for software upgrade operations.
package upgrade

import (
	"context"
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// ResumeUseCase handles resuming interrupted upgrades.
// It loads saved state, reconciles with chain reality, and routes to the appropriate handler.
type ResumeUseCase struct {
	stateManager    ports.UpgradeStateManager
	stateDetector   ports.UpgradeStateDetector
	transitioner    ports.UpgradeStateTransitioner
	resumableExecUC *ResumableExecuteUpgradeUseCase
	logger          *output.Logger
}

// NewResumeUseCase creates a new ResumeUseCase.
func NewResumeUseCase(
	stateManager ports.UpgradeStateManager,
	stateDetector ports.UpgradeStateDetector,
	transitioner ports.UpgradeStateTransitioner,
	resumableExecUC *ResumableExecuteUpgradeUseCase,
	logger *output.Logger,
) *ResumeUseCase {
	return &ResumeUseCase{
		stateManager:    stateManager,
		stateDetector:   stateDetector,
		transitioner:    transitioner,
		resumableExecUC: resumableExecUC,
		logger:          logger,
	}
}

// ResumeResult contains the outcome of a resume operation.
type ResumeResult struct {
	// Resumed indicates whether resume was successful or skipped.
	Resumed bool
	// State is the final upgrade state after resume.
	State *ports.UpgradeState
	// UpgradeOutput is the result from ExecuteUpgradeUseCase if resumed.
	UpgradeOutput *dto.ExecuteUpgradeOutput
	// Message describes what happened.
	Message string
}

// CheckState checks if there's an existing upgrade state.
// Returns the state if it exists, nil otherwise.
func (uc *ResumeUseCase) CheckState(ctx context.Context) (*ports.UpgradeState, error) {
	return uc.stateManager.LoadState(ctx)
}

// GetStatus returns the current upgrade state for display.
// Returns nil if no upgrade is in progress.
func (uc *ResumeUseCase) GetStatus(ctx context.Context) (*ports.UpgradeState, error) {
	state, err := uc.stateManager.LoadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	return state, nil
}

// ClearState removes the upgrade state file.
func (uc *ResumeUseCase) ClearState(ctx context.Context) error {
	return uc.stateManager.DeleteState(ctx)
}

// Resume attempts to resume an interrupted upgrade.
// It loads the saved state, detects the current stage from chain state,
// and continues execution from the appropriate point.
func (uc *ResumeUseCase) Resume(ctx context.Context, input dto.ExecuteUpgradeInput, options ports.ResumeOptions) (*ResumeResult, error) {
	// Handle clear state option
	if options.ClearState {
		if err := uc.stateManager.DeleteState(ctx); err != nil {
			return nil, fmt.Errorf("failed to clear state: %w", err)
		}
		return &ResumeResult{
			Resumed: false,
			Message: "State cleared successfully",
		}, nil
	}

	// Load existing state
	state, err := uc.stateManager.LoadState(ctx)
	if err != nil {
		// Check if it's a corruption error
		if corruptionErr, ok := err.(*ports.StateCorruptionError); ok {
			return nil, fmt.Errorf("state file is corrupted: %s (use --clear-state to remove)", corruptionErr.Reason)
		}
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Handle show status option
	if options.ShowStatus {
		return &ResumeResult{
			Resumed: false,
			State:   state,
			Message: "Current upgrade state",
		}, nil
	}

	// Handle force restart option
	if options.ForceRestart {
		if state != nil {
			if err := uc.stateManager.DeleteState(ctx); err != nil {
				uc.logger.Warn("Failed to clear old state: %v", err)
			}
		}
		// Start fresh - no state
		return &ResumeResult{
			Resumed: false,
			Message: "Starting fresh (--force-restart)",
		}, nil
	}

	// No existing state - nothing to resume
	if state == nil {
		return &ResumeResult{
			Resumed: false,
			Message: "No existing upgrade state found",
		}, nil
	}

	// Validate the loaded state
	if err := uc.stateManager.ValidateState(state); err != nil {
		return nil, fmt.Errorf("invalid state: %w (use --clear-state to remove)", err)
	}

	// Check if upgrade is in terminal state
	if state.Stage.IsTerminal() {
		return &ResumeResult{
			Resumed: false,
			State:   state,
			Message: fmt.Sprintf("Previous upgrade is in terminal state: %s", state.Stage),
		}, nil
	}

	// Handle resume-from override
	if options.ResumeFrom != "" {
		// Validate the override stage
		if !uc.transitioner.CanTransition(state.Stage, options.ResumeFrom) &&
			state.Stage != options.ResumeFrom {
			return nil, fmt.Errorf("cannot resume from %s (current stage: %s)", options.ResumeFrom, state.Stage)
		}

		// If different from current, transition
		if state.Stage != options.ResumeFrom {
			if err := uc.transitioner.TransitionTo(state, options.ResumeFrom, "manual override via --resume-from"); err != nil {
				return nil, fmt.Errorf("failed to transition to %s: %w", options.ResumeFrom, err)
			}
			if err := uc.stateManager.SaveState(ctx, state); err != nil {
				return nil, fmt.Errorf("failed to save state after override: %w", err)
			}
		}

		uc.logger.Info("Resuming from %s (manual override)", options.ResumeFrom)
	} else {
		// Detect current stage from chain state
		detectedStage, err := uc.stateDetector.DetectCurrentStage(ctx, state)
		if err != nil {
			uc.logger.Warn("Failed to detect current stage: %v, using saved state", err)
		} else if detectedStage != state.Stage {
			// Chain state has progressed beyond saved state
			uc.logger.Info("Detected stage progression: %s → %s", state.Stage, detectedStage)

			// Validate the transition
			if uc.transitioner.CanTransition(state.Stage, detectedStage) {
				if err := uc.transitioner.TransitionTo(state, detectedStage, "detected from chain state"); err != nil {
					uc.logger.Warn("Failed to transition to detected stage: %v", err)
				} else {
					if err := uc.stateManager.SaveState(ctx, state); err != nil {
						uc.logger.Warn("Failed to save updated state: %v", err)
					}
				}
			}
		}
	}

	// Check if detected stage is terminal after reconciliation
	if state.Stage.IsTerminal() {
		if state.Stage == ports.ResumableStageProposalRejected {
			return &ResumeResult{
				Resumed: false,
				State:   state,
				Message: "Governance proposal was rejected during interruption",
			}, nil
		}
		if state.Stage == ports.ResumableStageFailed {
			return &ResumeResult{
				Resumed: false,
				State:   state,
				Message: fmt.Sprintf("Upgrade failed: %s", state.Error),
			}, nil
		}
		if state.Stage == ports.ResumableStageCompleted {
			return &ResumeResult{
				Resumed: false,
				State:   state,
				Message: "Upgrade already completed",
			}, nil
		}
	}

	// Continue execution from current stage
	uc.logger.Info("Resuming upgrade '%s' from stage: %s", state.UpgradeName, state.Stage)

	output, err := uc.resumableExecUC.Execute(ctx, input, state)
	if err != nil {
		return nil, fmt.Errorf("resume execution failed: %w", err)
	}

	return &ResumeResult{
		Resumed:       true,
		State:         state,
		UpgradeOutput: output,
		Message:       "Upgrade resumed and completed",
	}, nil
}

// Reconcile compares saved state with chain state and updates if needed.
// This is useful for checking state without running the full upgrade.
func (uc *ResumeUseCase) Reconcile(ctx context.Context) (*ports.UpgradeState, error) {
	state, err := uc.stateManager.LoadState(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if state == nil {
		return nil, nil
	}

	// Detect current stage from chain state
	detectedStage, err := uc.stateDetector.DetectCurrentStage(ctx, state)
	if err != nil {
		return state, nil // Return saved state on detection failure
	}

	// If detected stage differs and transition is valid, update
	if detectedStage != state.Stage {
		if uc.transitioner.CanTransition(state.Stage, detectedStage) {
			if err := uc.transitioner.TransitionTo(state, detectedStage, "reconciliation"); err != nil {
				return state, nil
			}
			if err := uc.stateManager.SaveState(ctx, state); err != nil {
				uc.logger.Warn("Failed to save reconciled state: %v", err)
			}
		}
	}

	return state, nil
}
