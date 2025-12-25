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
	proposeUC     *ProposeUseCase
	voteUC        *VoteUseCase
	switchUC      *SwitchBinaryUseCase
	rpcClient     ports.RPCClient
	devnetRepo    ports.DevnetRepository
	healthChecker ports.HealthChecker
	logger        ports.Logger
}

// NewExecuteUpgradeUseCase creates a new ExecuteUpgradeUseCase.
func NewExecuteUpgradeUseCase(
	proposeUC *ProposeUseCase,
	voteUC *VoteUseCase,
	switchUC *SwitchBinaryUseCase,
	rpcClient ports.RPCClient,
	devnetRepo ports.DevnetRepository,
	healthChecker ports.HealthChecker,
	logger ports.Logger,
) *ExecuteUpgradeUseCase {
	return &ExecuteUpgradeUseCase{
		proposeUC:     proposeUC,
		voteUC:        voteUC,
		switchUC:      switchUC,
		rpcClient:     rpcClient,
		devnetRepo:    devnetRepo,
		healthChecker: healthChecker,
		logger:        logger,
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
		CommitHash:    input.CommitHash, // Deprecated, kept for compatibility
		CacheRef:      input.CacheRef,   // Use CacheRef for SetActive
		Mode:          input.Mode,
		UpgradeHeight: proposeResult.UpgradeHeight,
	})
	if err != nil {
		output.Error = err
		return output, err
	}
	output.NewBinary = switchResult.NewBinary

	// Verify chain resumed
	postHeight, err := uc.verifyChainResumed(ctx, input.HomeDir)
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
	var (
		lastHeight     int64
		lastUpdateTime time.Time
		blockRate      float64 // blocks per second (EMA)
		alpha          = 0.3    // smoothing factor for EMA
	)

	// Progress bar configuration
	const (
		barWidth      = 30
		updateMessage = "\r  Progress: [%s] %3d%% | Height: %d/%d | Remaining: %d blocks | ETA: %s"
	)

	// Clear any previous progress line on start
	fmt.Fprint(uc.logger.Writer(), "\n")

	for {
		// Check context before blocking RPC call
		select {
		case <-ctx.Done():
			fmt.Fprint(uc.logger.Writer(), "\n") // New line on cancellation
			return ctx.Err()
		default:
		}

		// Use timeout context for RPC call to prevent indefinite blocking
		rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		currentHeight, err := uc.rpcClient.GetBlockHeight(rpcCtx)
		cancel()

		if err != nil {
			// Check if parent context was cancelled
			if ctx.Err() != nil {
				fmt.Fprint(uc.logger.Writer(), "\n")
				return ctx.Err()
			}
			return fmt.Errorf("failed to get block height: %w", err)
		}

		if currentHeight >= targetHeight {
			// Print final progress at 100%
			bar := makeProgressBar(barWidth, 100)
			fmt.Fprintf(uc.logger.Writer(), updateMessage+"\n",
				bar, 100, currentHeight, targetHeight, 0, "0s")
			return nil
		}

		// Calculate progress metrics
		remaining := targetHeight - currentHeight
		progress := int(float64(currentHeight) / float64(targetHeight) * 100)

		// Calculate block rate and ETA
		var eta string
		if lastHeight > 0 && currentHeight > lastHeight {
			elapsed := time.Since(lastUpdateTime).Seconds()
			if elapsed > 0 {
				currentRate := float64(currentHeight-lastHeight) / elapsed
				// Use EMA for smooth rate calculation
				if blockRate == 0 {
					blockRate = currentRate
				} else {
					blockRate = alpha*currentRate + (1-alpha)*blockRate
				}

				if blockRate > 0 {
					etaSeconds := float64(remaining) / blockRate
					eta = formatDuration(time.Duration(etaSeconds * float64(time.Second)))
				} else {
					eta = "calculating..."
				}
			} else {
				eta = "calculating..."
			}
		} else {
			eta = "calculating..."
		}

		// Create and display progress bar
		bar := makeProgressBar(barWidth, progress)
		fmt.Fprintf(uc.logger.Writer(), updateMessage,
			bar, progress, currentHeight, targetHeight, remaining, eta)

		// Update tracking variables
		lastHeight = currentHeight
		lastUpdateTime = time.Now()

		uc.logger.Debug("Current height: %d, target: %d, rate: %.2f blocks/s",
			currentHeight, targetHeight, blockRate)

		select {
		case <-ctx.Done():
			fmt.Fprint(uc.logger.Writer(), "\n")
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// makeProgressBar creates an ASCII progress bar
func makeProgressBar(width, percent int) string {
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}

	filled := (width * percent) / 100
	bar := make([]byte, width)
	for i := 0; i < width; i++ {
		if i < filled {
			bar[i] = '='
		} else if i == filled {
			bar[i] = '>'
		} else {
			bar[i] = ' '
		}
	}
	return string(bar)
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func (uc *ExecuteUpgradeUseCase) waitForChainHalt(ctx context.Context, upgradeHeight int64) error {
	// Wait for the chain to stop producing blocks
	stableCount := 0
	var lastHeight int64

	// Set a maximum wait time to prevent infinite waiting
	// if chain doesn't halt (e.g., proposal failed)
	maxWaitTime := 10 * time.Minute
	deadline := time.Now().Add(maxWaitTime)

	for stableCount < 3 && time.Now().Before(deadline) {
		// Check context before blocking RPC call
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Use timeout context for RPC call
		rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		currentHeight, err := uc.rpcClient.GetBlockHeight(rpcCtx)
		cancel()

		if err != nil {
			// Check if parent context was cancelled
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// RPC error might indicate chain is halting
			stableCount++
			// Use select instead of continue to allow cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}

		// Check if chain has moved significantly past upgrade height
		// This could indicate the upgrade proposal failed
		if currentHeight > upgradeHeight+10 {
			return fmt.Errorf("chain continued past upgrade height %d (current: %d); upgrade proposal may have failed", upgradeHeight, currentHeight)
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

	if stableCount < 3 {
		return fmt.Errorf("timeout waiting for chain to halt at upgrade height %d", upgradeHeight)
	}

	return nil
}

func (uc *ExecuteUpgradeUseCase) verifyChainResumed(ctx context.Context, homeDir string) (int64, error) {
	uc.logger.Info("Waiting for chain to resume...")

	deadline := time.Now().Add(5 * time.Minute)
	var lastHeight int64

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		currentHeight, err := uc.rpcClient.GetBlockHeight(rpcCtx)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			uc.logger.Debug("RPC not responding yet...")
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue
		}

		if lastHeight > 0 && currentHeight > lastHeight {
			uc.logger.Debug("Chain resumed at height %d", currentHeight)
			uc.logger.Success("Chain is healthy and producing blocks!")
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
