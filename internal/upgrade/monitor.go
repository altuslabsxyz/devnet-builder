package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/stablelabs/stable-devnet/internal/output"
)

// MonitorOptions contains options for monitoring the upgrade.
type MonitorOptions struct {
	TargetHeight int64
	RPCURL       string
	Logger       *output.Logger
	OnProgress   func(current, target int64)
}

// WaitForUpgradeHeight waits until the chain reaches the upgrade height.
func WaitForUpgradeHeight(ctx context.Context, opts *MonitorOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	rpc := NewRPCClient("localhost", DefaultRPCPort)

	ticker := time.NewTicker(BlockPollInterval)
	defer ticker.Stop()

	timeout := time.NewTimer(UpgradeTimeout)
	defer timeout.Stop()

	lastHeight := int64(0)
	unchangedCount := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return WrapError(StageWaiting, "timeout waiting for upgrade height", ErrUpgradeTimeout,
				fmt.Sprintf("Current height did not reach %d in time", opts.TargetHeight))
		case <-ticker.C:
			height, err := rpc.GetCurrentHeight(ctx)
			if err != nil {
				// Chain might be halted for upgrade
				logger.Debug("RPC error (chain may be halting): %v", err)
				unchangedCount++

				// If we were close to target and now can't reach RPC, chain may have halted
				if lastHeight >= opts.TargetHeight-1 && unchangedCount >= ChainHaltThreshold {
					logger.Debug("Chain appears to have halted at target height")
					return nil
				}
				continue
			}

			// Check for chain halt (height not changing)
			if height == lastHeight {
				unchangedCount++
				if unchangedCount >= ChainHaltThreshold && height >= opts.TargetHeight {
					logger.Debug("Chain halted at height %d (target: %d)", height, opts.TargetHeight)
					return nil
				}
			} else {
				unchangedCount = 0
				lastHeight = height
			}

			if opts.OnProgress != nil {
				opts.OnProgress(height, opts.TargetHeight)
			}

			if height >= opts.TargetHeight {
				// Give a moment for the chain to process and halt
				time.Sleep(ChainHaltDetectionInterval)
				return nil
			}
		}
	}
}

// DetectChainHalt verifies that the chain has stopped at the upgrade height.
func DetectChainHalt(ctx context.Context, logger *output.Logger) (bool, int64, error) {
	if logger == nil {
		logger = output.DefaultLogger
	}

	rpc := NewRPCClient("localhost", DefaultRPCPort)

	// Get initial height
	initialHeight, err := rpc.GetCurrentHeight(ctx)
	if err != nil {
		// RPC not responding could mean chain halted
		return true, 0, nil
	}

	// Wait and check if height changes
	time.Sleep(ChainHaltDetectionInterval * 2)

	newHeight, err := rpc.GetCurrentHeight(ctx)
	if err != nil {
		// RPC not responding - chain halted
		return true, initialHeight, nil
	}

	if newHeight == initialHeight {
		// Height unchanged - chain halted
		return true, initialHeight, nil
	}

	// Chain is still running
	return false, newHeight, nil
}

// VerifyChainResumed confirms that the chain is producing blocks after restart.
func VerifyChainResumed(ctx context.Context, logger *output.Logger) (int64, error) {
	if logger == nil {
		logger = output.DefaultLogger
	}

	rpc := NewRPCClient("localhost", DefaultRPCPort)

	timeout := time.NewTimer(PostUpgradeTimeout)
	defer timeout.Stop()

	ticker := time.NewTicker(BlockPollInterval)
	defer ticker.Stop()

	var lastHeight int64 = 0
	var firstValidHeight int64 = 0
	consecutiveBlocks := 0

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timeout.C:
			return 0, WrapError(StageVerifyingResume, "chain did not resume", ErrChainNotResumed,
				"The upgrade handler may not exist in the new binary. Check upgrade name.")
		case <-ticker.C:
			height, err := rpc.GetCurrentHeight(ctx)
			if err != nil {
				logger.Debug("Waiting for chain to resume: %v", err)
				consecutiveBlocks = 0
				continue
			}

			if firstValidHeight == 0 {
				firstValidHeight = height
				logger.Debug("Chain responded at height %d", height)
			}

			if height > lastHeight {
				consecutiveBlocks++
				lastHeight = height
				logger.Debug("Block %d produced (%d consecutive)", height, consecutiveBlocks)

				// Require a few consecutive blocks to confirm chain is healthy
				if consecutiveBlocks >= 3 {
					return height, nil
				}
			}
		}
	}
}

// WaitForChainHalt waits for the chain to halt at upgrade height.
func WaitForChainHalt(ctx context.Context, targetHeight int64, logger *output.Logger) error {
	if logger == nil {
		logger = output.DefaultLogger
	}

	timeout := time.NewTimer(2 * time.Minute)
	defer timeout.Stop()

	ticker := time.NewTicker(ChainHaltDetectionInterval)
	defer ticker.Stop()

	rpc := NewRPCClient("localhost", DefaultRPCPort)
	unchangedCount := 0
	lastHeight := int64(0)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return WrapError(StageWaiting, "timeout waiting for chain halt", ErrUpgradeTimeout,
				"Chain may not have the upgrade handler registered")
		case <-ticker.C:
			height, err := rpc.GetCurrentHeight(ctx)
			if err != nil {
				// RPC not responding - likely halted
				logger.Debug("RPC error, chain may have halted")
				unchangedCount++
				if unchangedCount >= ChainHaltThreshold {
					return nil
				}
				continue
			}

			if height == lastHeight {
				unchangedCount++
				if unchangedCount >= ChainHaltThreshold {
					logger.Debug("Chain halted at height %d", height)
					return nil
				}
			} else {
				unchangedCount = 0
				lastHeight = height
			}

			if height >= targetHeight {
				// At or past target, give it time to halt
				logger.Debug("Reached target height %d, waiting for halt...", height)
			}
		}
	}
}
