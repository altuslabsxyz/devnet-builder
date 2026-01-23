// Package upgrade provides use cases for managing blockchain upgrades.
package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/types"
)

// ResumableExecuteUpgradeUseCase wraps ExecuteUpgradeUseCase with state persistence.
// It saves state after each stage transition, enabling resume after interruptions.
type ResumableExecuteUpgradeUseCase struct {
	executeUC     *ExecuteUpgradeUseCase
	proposeUC     *ProposeUseCase
	voteUC        *VoteUseCase
	switchUC      *SwitchBinaryUseCase
	stateManager  ports.UpgradeStateManager
	transitioner  ports.UpgradeStateTransitioner
	stateDetector ports.UpgradeStateDetector
	rpcClient     ports.RPCClient
	exportUC      ports.ExportUseCase
	devnetRepo    ports.DevnetRepository
	logger        ports.Logger
}

// NewResumableExecuteUpgradeUseCase creates a new ResumableExecuteUpgradeUseCase.
func NewResumableExecuteUpgradeUseCase(
	executeUC *ExecuteUpgradeUseCase,
	proposeUC *ProposeUseCase,
	voteUC *VoteUseCase,
	switchUC *SwitchBinaryUseCase,
	stateManager ports.UpgradeStateManager,
	transitioner ports.UpgradeStateTransitioner,
	stateDetector ports.UpgradeStateDetector,
	rpcClient ports.RPCClient,
	exportUC ports.ExportUseCase,
	devnetRepo ports.DevnetRepository,
	logger ports.Logger,
) *ResumableExecuteUpgradeUseCase {
	return &ResumableExecuteUpgradeUseCase{
		executeUC:     executeUC,
		proposeUC:     proposeUC,
		voteUC:        voteUC,
		switchUC:      switchUC,
		stateManager:  stateManager,
		transitioner:  transitioner,
		stateDetector: stateDetector,
		rpcClient:     rpcClient,
		exportUC:      exportUC,
		devnetRepo:    devnetRepo,
		logger:        logger,
	}
}

// Execute performs the upgrade workflow with state persistence.
// If state exists and resume is requested, continues from the saved stage.
func (uc *ResumableExecuteUpgradeUseCase) Execute(ctx context.Context, input dto.ExecuteUpgradeInput, state *ports.UpgradeState) (*dto.ExecuteUpgradeOutput, error) {
	startTime := time.Now()

	// If no state provided, create new one
	if state == nil {
		mode := "local"
		if input.Mode == types.ExecutionModeDocker {
			mode = "docker"
		}
		state = ports.NewUpgradeState(input.UpgradeName, mode, input.SkipGovernance)

		// Initialize with input parameters
		state.TargetBinary = input.TargetBinary
		state.TargetImage = input.TargetImage
		state.TargetVersion = input.TargetVersion
		state.UpgradeHeight = input.UpgradeHeight

		// Save initial state
		if err := uc.saveState(ctx, state); err != nil {
			return nil, fmt.Errorf("failed to save initial state: %w", err)
		}
	}

	// Branch based on skip-gov mode
	if input.SkipGovernance {
		return uc.executeSkipGovResumable(ctx, input, state, startTime)
	}

	return uc.executeWithGovResumable(ctx, input, state, startTime)
}

// executeSkipGovResumable performs binary replacement without governance (resumable).
func (uc *ResumableExecuteUpgradeUseCase) executeSkipGovResumable(
	ctx context.Context,
	input dto.ExecuteUpgradeInput,
	state *ports.UpgradeState,
	startTime time.Time,
) (*dto.ExecuteUpgradeOutput, error) {
	output := &dto.ExecuteUpgradeOutput{}

	// Resume from current stage
	switch state.Stage {
	case ports.ResumableStageInitialized:
		// Transition to SwitchingBinary
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageSwitchingBinary, "starting binary switch (skip-gov)"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageSwitchingBinary:
		uc.logger.Info("Switching binary...")
		switchResult, err := uc.executeSwitchBinary(ctx, input, state)
		if err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}
		output.NewBinary = switchResult.NewBinary

		// Transition to VerifyingResume
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageVerifyingResume, "binary switch complete"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageVerifyingResume:
		uc.logger.Info("Verifying chain health...")
		postHeight, err := uc.executeUC.verifyChainResumed(ctx, input.HomeDir)
		if err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}
		output.PostUpgradeHeight = postHeight

		// Transition to Completed
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageCompleted, "chain verified healthy"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageCompleted:
		// Update metadata version
		if input.TargetVersion != "" {
			if err := uc.executeUC.updateCurrentVersion(ctx, input.HomeDir, input.TargetVersion); err != nil {
				uc.logger.Warn("Failed to update version in metadata: %v", err)
			}
		}

		// Delete state file on success
		if err := uc.stateManager.DeleteState(ctx); err != nil {
			uc.logger.Warn("Failed to delete state file: %v", err)
		}

		output.Success = true
		output.Duration = time.Since(startTime)
		uc.logger.Success("Binary replacement complete! Duration: %v", output.Duration)
		return output, nil

	default:
		return nil, fmt.Errorf("cannot resume from stage: %s", state.Stage)
	}
}

// executeWithGovResumable performs the full upgrade workflow with governance (resumable).
func (uc *ResumableExecuteUpgradeUseCase) executeWithGovResumable(
	ctx context.Context,
	input dto.ExecuteUpgradeInput,
	state *ports.UpgradeState,
	startTime time.Time,
) (*dto.ExecuteUpgradeOutput, error) {
	output := &dto.ExecuteUpgradeOutput{}

	// Pre-upgrade export (only if starting fresh and enabled)
	if state.Stage == ports.ResumableStageInitialized && input.WithExport {
		uc.logger.Info("Pre-upgrade: Exporting state before upgrade...")
		exportInput := dto.ExportInput{
			HomeDir:   input.HomeDir,
			OutputDir: input.GenesisDir,
			Force:     false,
		}

		preExportResultRaw, err := uc.exportUC.Execute(ctx, exportInput)
		if err != nil {
			uc.logger.Error("Pre-upgrade export failed: %v", err)
			output.Error = fmt.Errorf("pre-upgrade export failed: %w", err)
			return output, output.Error
		}
		if preExportResult, ok := preExportResultRaw.(*dto.ExportOutput); ok {
			output.PreGenesisPath = preExportResult.ExportPath
			uc.logger.Success("Pre-upgrade export complete: %s", preExportResult.ExportPath)
		}
	}

	// Resume from current stage
	switch state.Stage {
	case ports.ResumableStageInitialized:
		// Step 1: Submit proposal
		uc.logger.Info("Step 1/5: Submitting upgrade proposal...")
		proposeResult, err := uc.proposeUC.Execute(ctx, dto.ProposeInput{
			HomeDir:       input.HomeDir,
			UpgradeName:   input.UpgradeName,
			UpgradeHeight: input.UpgradeHeight,
			VotingPeriod:  input.VotingPeriod,
			HeightBuffer:  input.HeightBuffer,
		})
		if err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}

		// Update state with proposal info
		state.ProposalID = proposeResult.ProposalID
		state.UpgradeHeight = proposeResult.UpgradeHeight
		output.ProposalID = proposeResult.ProposalID
		output.UpgradeHeight = proposeResult.UpgradeHeight

		// Transition to ProposalSubmitted
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageProposalSubmitted, fmt.Sprintf("proposal %d submitted", proposeResult.ProposalID)); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageProposalSubmitted:
		// Transition to Voting (deposit period complete in devnet context)
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageVoting, "voting period started"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageVoting:
		// Step 2: Vote from all validators
		uc.logger.Info("Step 2/5: Voting from all validators...")
		output.ProposalID = state.ProposalID
		output.UpgradeHeight = state.UpgradeHeight

		voteResult, err := uc.executeVoting(ctx, input, state)
		if err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}

		if voteResult.VotesCast != voteResult.TotalVoters {
			err := fmt.Errorf("not all votes cast: %d/%d", voteResult.VotesCast, voteResult.TotalVoters)
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}

		// Transition to WaitingForHeight
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageWaitingForHeight, "voting complete, proposal passed"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageWaitingForHeight:
		// Step 3: Wait for upgrade height
		uc.logger.Info("Step 3/5: Waiting for upgrade height %d...", state.UpgradeHeight)
		output.ProposalID = state.ProposalID
		output.UpgradeHeight = state.UpgradeHeight

		if err := uc.executeUC.waitForUpgradeHeight(ctx, state.UpgradeHeight); err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}

		// Transition to ChainHalted
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageChainHalted, "upgrade height reached"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageChainHalted:
		// Step 4: Wait for chain halt
		uc.logger.Info("Step 4/5: Waiting for chain to halt...")
		output.ProposalID = state.ProposalID
		output.UpgradeHeight = state.UpgradeHeight

		if err := uc.executeUC.waitForChainHalt(ctx, state.UpgradeHeight); err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}

		// Transition to SwitchingBinary
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageSwitchingBinary, "chain halted at upgrade height"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageSwitchingBinary:
		// Step 5: Switch binary
		uc.logger.Info("Step 5/5: Switching binary...")
		output.ProposalID = state.ProposalID
		output.UpgradeHeight = state.UpgradeHeight

		switchResult, err := uc.executeSwitchBinary(ctx, input, state)
		if err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}
		output.NewBinary = switchResult.NewBinary

		// Transition to VerifyingResume
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageVerifyingResume, "binary switch complete"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageVerifyingResume:
		// Verify chain resumed
		output.ProposalID = state.ProposalID
		output.UpgradeHeight = state.UpgradeHeight

		postHeight, err := uc.executeUC.verifyChainResumed(ctx, input.HomeDir)
		if err != nil {
			if saveErr := uc.transitionAndSave(ctx, state, ports.ResumableStageFailed, err.Error()); saveErr != nil {
				uc.logger.Warn("Failed to save failed state: %v", saveErr)
			}
			output.Error = err
			return output, err
		}
		output.PostUpgradeHeight = postHeight

		// Post-upgrade export (if enabled)
		if input.WithExport {
			uc.logger.Info("Post-upgrade: Exporting state after upgrade...")
			exportInput := dto.ExportInput{
				HomeDir:   input.HomeDir,
				OutputDir: input.GenesisDir,
				Force:     false,
			}

			postExportResultRaw, err := uc.exportUC.Execute(ctx, exportInput)
			if err != nil {
				uc.logger.Warn("Post-upgrade export failed: %v", err)
			} else if postExportResult, ok := postExportResultRaw.(*dto.ExportOutput); ok {
				output.PostGenesisPath = postExportResult.ExportPath
				uc.logger.Success("Post-upgrade export complete: %s", postExportResult.ExportPath)
			}
		}

		// Transition to Completed
		if err := uc.transitionAndSave(ctx, state, ports.ResumableStageCompleted, "chain verified healthy"); err != nil {
			return nil, err
		}
		fallthrough

	case ports.ResumableStageCompleted:
		// Update metadata version
		if input.TargetVersion != "" {
			if err := uc.executeUC.updateCurrentVersion(ctx, input.HomeDir, input.TargetVersion); err != nil {
				uc.logger.Warn("Failed to update version in metadata: %v", err)
			}
		}

		// Delete state file on success
		if err := uc.stateManager.DeleteState(ctx); err != nil {
			uc.logger.Warn("Failed to delete state file: %v", err)
		}

		output.Success = true
		output.Duration = time.Since(startTime)
		uc.logger.Success("Upgrade complete! Duration: %v", output.Duration)
		return output, nil

	case ports.ResumableStageFailed:
		return nil, fmt.Errorf("upgrade previously failed: %s (use --force-restart to start fresh)", state.Error)

	case ports.ResumableStageProposalRejected:
		return nil, fmt.Errorf("proposal was rejected (use --force-restart to start fresh)")

	default:
		return nil, fmt.Errorf("cannot resume from stage: %s", state.Stage)
	}
}

// executeSwitchBinary handles binary switching with per-node tracking.
func (uc *ResumableExecuteUpgradeUseCase) executeSwitchBinary(
	ctx context.Context,
	input dto.ExecuteUpgradeInput,
	state *ports.UpgradeState,
) (*dto.SwitchBinaryOutput, error) {
	result, err := uc.switchUC.Execute(ctx, dto.SwitchBinaryInput{
		HomeDir:       input.HomeDir,
		TargetBinary:  input.TargetBinary,
		TargetImage:   input.TargetImage,
		TargetVersion: input.TargetVersion,
		CachePath:     input.CachePath,
		CommitHash:    input.CommitHash,
		CacheRef:      input.CacheRef,
		Mode:          input.Mode,
		UpgradeHeight: state.UpgradeHeight,
	})
	if err != nil {
		return nil, err
	}

	// Update node switch tracking in state
	now := time.Now()
	for i := 0; i < result.NodesRestarted; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		// Find or add node switch state
		found := false
		for j := range state.NodeSwitches {
			if state.NodeSwitches[j].NodeName == nodeName {
				state.NodeSwitches[j].Switched = true
				state.NodeSwitches[j].Stopped = true
				state.NodeSwitches[j].Started = true
				state.NodeSwitches[j].OldBinary = result.OldBinary
				state.NodeSwitches[j].NewBinary = result.NewBinary
				state.NodeSwitches[j].Timestamp = &now
				found = true
				break
			}
		}
		if !found {
			state.NodeSwitches = append(state.NodeSwitches, ports.NodeSwitchState{
				NodeName:  nodeName,
				Switched:  true,
				Stopped:   true,
				Started:   true,
				OldBinary: result.OldBinary,
				NewBinary: result.NewBinary,
				Timestamp: &now,
			})
		}
	}

	// Save updated state
	if err := uc.saveState(ctx, state); err != nil {
		uc.logger.Warn("Failed to save node switch state: %v", err)
	}

	return result, nil
}

// executeVoting handles voting with per-validator tracking.
func (uc *ResumableExecuteUpgradeUseCase) executeVoting(
	ctx context.Context,
	input dto.ExecuteUpgradeInput,
	state *ports.UpgradeState,
) (*dto.VoteOutput, error) {
	result, err := uc.voteUC.Execute(ctx, dto.VoteInput{
		HomeDir:    input.HomeDir,
		ProposalID: state.ProposalID,
		VoteOption: "yes",
		FromAll:    true,
	})
	if err != nil {
		return nil, err
	}

	// Update validator vote tracking in state
	now := time.Now()
	for i, txHash := range result.TxHashes {
		// Find or add validator vote state
		if i >= len(state.ValidatorVotes) {
			state.ValidatorVotes = append(state.ValidatorVotes, ports.ValidatorVoteState{
				Address:   fmt.Sprintf("validator-%d", i),
				Voted:     true,
				TxHash:    txHash,
				Timestamp: &now,
			})
		} else {
			state.ValidatorVotes[i].Voted = true
			state.ValidatorVotes[i].TxHash = txHash
			state.ValidatorVotes[i].Timestamp = &now
		}
	}

	// Save updated state
	if err := uc.saveState(ctx, state); err != nil {
		uc.logger.Warn("Failed to save vote state: %v", err)
	}

	return result, nil
}

// transitionAndSave transitions the state and saves it to disk.
func (uc *ResumableExecuteUpgradeUseCase) transitionAndSave(
	ctx context.Context,
	state *ports.UpgradeState,
	target ports.ResumableStage,
	reason string,
) error {
	if err := uc.transitioner.TransitionTo(state, target, reason); err != nil {
		return fmt.Errorf("invalid state transition to %s: %w", target, err)
	}

	if err := uc.saveState(ctx, state); err != nil {
		return fmt.Errorf("failed to save state after transition to %s: %w", target, err)
	}

	uc.logger.Debug("State transitioned: %s -> %s (%s)", state.StageHistory[len(state.StageHistory)-1].From, target, reason)
	return nil
}

// saveState saves the current state to disk.
func (uc *ResumableExecuteUpgradeUseCase) saveState(ctx context.Context, state *ports.UpgradeState) error {
	state.UpdatedAt = time.Now()
	return uc.stateManager.SaveState(ctx, state)
}

// GetCurrentState returns the current upgrade state.
func (uc *ResumableExecuteUpgradeUseCase) GetCurrentState(ctx context.Context) (*ports.UpgradeState, error) {
	return uc.stateManager.LoadState(ctx)
}

// ClearState clears the upgrade state file.
func (uc *ResumableExecuteUpgradeUseCase) ClearState(ctx context.Context) error {
	return uc.stateManager.DeleteState(ctx)
}
