package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ExecuteUpgradeUseCase orchestrates the full upgrade workflow.
type ExecuteUpgradeUseCase struct {
	proposeUC *ProposeUseCase
	voteUC    *VoteUseCase
	switchUC  *SwitchBinaryUseCase
	rpcClient ports.RPCClient
	logger    ports.Logger
}

// NewExecuteUpgradeUseCase creates a new ExecuteUpgradeUseCase.
func NewExecuteUpgradeUseCase(
	proposeUC *ProposeUseCase,
	voteUC *VoteUseCase,
	switchUC *SwitchBinaryUseCase,
	rpcClient ports.RPCClient,
	logger ports.Logger,
) *ExecuteUpgradeUseCase {
	return &ExecuteUpgradeUseCase{
		proposeUC: proposeUC,
		voteUC:    voteUC,
		switchUC:  switchUC,
		rpcClient: rpcClient,
		logger:    logger,
	}
}

// Execute performs the full upgrade workflow.
func (uc *ExecuteUpgradeUseCase) Execute(ctx context.Context, input dto.ExecuteUpgradeInput) (*dto.ExecuteUpgradeOutput, error) {
	startTime := time.Now()
	uc.logger.Info("Starting upgrade workflow...")

	output := &dto.ExecuteUpgradeOutput{}

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
		output.Error = err
		return output, err
	}
	output.ProposalID = proposeResult.ProposalID
	output.UpgradeHeight = proposeResult.UpgradeHeight

	// Step 2: Vote from all validators
	uc.logger.Info("Step 2/5: Voting from all validators...")
	voteResult, err := uc.voteUC.Execute(ctx, dto.VoteInput{
		HomeDir:    input.HomeDir,
		ProposalID: proposeResult.ProposalID,
		VoteOption: "yes",
		FromAll:    true,
	})
	if err != nil {
		output.Error = err
		return output, err
	}
	if voteResult.VotesCast != voteResult.TotalVoters {
		err := fmt.Errorf("not all votes cast: %d/%d", voteResult.VotesCast, voteResult.TotalVoters)
		output.Error = err
		return output, err
	}

	// Step 3: Wait for upgrade height
	uc.logger.Info("Step 3/5: Waiting for upgrade height %d...", proposeResult.UpgradeHeight)
	if err := uc.waitForUpgradeHeight(ctx, proposeResult.UpgradeHeight); err != nil {
		output.Error = err
		return output, err
	}

	// Step 4: Wait for chain halt
	uc.logger.Info("Step 4/5: Waiting for chain to halt...")
	if err := uc.waitForChainHalt(ctx, proposeResult.UpgradeHeight); err != nil {
		output.Error = err
		return output, err
	}

	// Step 5: Switch binary
	uc.logger.Info("Step 5/5: Switching binary...")
	switchResult, err := uc.switchUC.Execute(ctx, dto.SwitchBinaryInput{
		HomeDir:       input.HomeDir,
		TargetBinary:  input.TargetBinary,
		TargetImage:   input.TargetImage,
		TargetVersion: input.TargetVersion,
		CachePath:     input.CachePath,
		CommitHash:    input.CommitHash,
		Mode:          input.Mode,
	})
	if err != nil {
		output.Error = err
		return output, err
	}
	output.NewBinary = switchResult.NewBinary

	// Verify chain resumed
	postHeight, err := uc.verifyChainResumed(ctx)
	if err != nil {
		output.Error = err
		return output, err
	}
	output.PostUpgradeHeight = postHeight

	// Success
	output.Success = true
	output.Duration = time.Since(startTime)
	uc.logger.Success("Upgrade complete! Duration: %v", output.Duration)

	return output, nil
}

func (uc *ExecuteUpgradeUseCase) waitForUpgradeHeight(ctx context.Context, targetHeight int64) error {
	for {
		currentHeight, err := uc.rpcClient.GetBlockHeight(ctx)
		if err != nil {
			return fmt.Errorf("failed to get block height: %w", err)
		}

		if currentHeight >= targetHeight {
			return nil
		}

		uc.logger.Debug("Current height: %d, target: %d", currentHeight, targetHeight)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (uc *ExecuteUpgradeUseCase) waitForChainHalt(ctx context.Context, upgradeHeight int64) error {
	// Wait for the chain to stop producing blocks
	stableCount := 0
	var lastHeight int64

	for stableCount < 3 {
		currentHeight, err := uc.rpcClient.GetBlockHeight(ctx)
		if err != nil {
			// RPC error might indicate chain is halting
			stableCount++
			continue
		}

		if currentHeight == lastHeight && currentHeight >= upgradeHeight {
			stableCount++
		} else {
			stableCount = 0
		}
		lastHeight = currentHeight

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return nil
}

func (uc *ExecuteUpgradeUseCase) verifyChainResumed(ctx context.Context) (int64, error) {
	// Wait for chain to start producing blocks again
	deadline := time.Now().Add(5 * time.Minute)
	var lastHeight int64

	for time.Now().Before(deadline) {
		currentHeight, err := uc.rpcClient.GetBlockHeight(ctx)
		if err != nil {
			uc.logger.Debug("RPC not responding yet...")
			time.Sleep(5 * time.Second)
			continue
		}

		if lastHeight > 0 && currentHeight > lastHeight {
			uc.logger.Debug("Chain resumed at height %d", currentHeight)
			return currentHeight, nil
		}
		lastHeight = currentHeight

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return 0, fmt.Errorf("timeout waiting for chain to resume")
}

// MonitorUseCase handles monitoring upgrade progress.
type MonitorUseCase struct {
	rpcClient ports.RPCClient
	logger    ports.Logger
}

// NewMonitorUseCase creates a new MonitorUseCase.
func NewMonitorUseCase(rpcClient ports.RPCClient, logger ports.Logger) *MonitorUseCase {
	return &MonitorUseCase{
		rpcClient: rpcClient,
		logger:    logger,
	}
}

// Execute monitors the upgrade progress and sends updates to a channel.
func (uc *MonitorUseCase) Execute(ctx context.Context, input dto.MonitorInput) (<-chan dto.MonitorProgress, error) {
	ch := make(chan dto.MonitorProgress, 10)

	go func() {
		defer close(ch)

		for {
			currentHeight, err := uc.rpcClient.GetBlockHeight(ctx)
			if err != nil {
				ch <- dto.MonitorProgress{
					Stage:   ports.StageFailed,
					Error:   err,
					Message: "Failed to get block height",
				}
				return
			}

			progress := dto.MonitorProgress{
				CurrentHeight: currentHeight,
				TargetHeight:  input.TargetHeight,
			}

			if currentHeight >= input.TargetHeight {
				progress.Stage = ports.StageCompleted
				progress.IsComplete = true
				progress.Message = "Upgrade height reached"
				ch <- progress
				return
			}

			progress.Stage = ports.StageWaiting
			progress.Message = fmt.Sprintf("Height %d / %d", currentHeight, input.TargetHeight)
			ch <- progress

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()

	return ch, nil
}
