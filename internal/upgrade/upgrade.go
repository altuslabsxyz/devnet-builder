package upgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// ExecuteUpgrade performs the complete upgrade process.
func ExecuteUpgrade(ctx context.Context, cfg *UpgradeConfig, opts *ExecuteOptions) (*UpgradeResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	startTime := time.Now()
	progress := UpgradeProgress{
		Stage:     StageVerifying,
		StartedAt: startTime,
	}

	notifyProgress := func() {
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(progress)
		}
	}

	result := &UpgradeResult{}

	// Stage 1: Verify devnet status
	progress.Stage = StageVerifying
	notifyProgress()

	if err := verifyDevnetRunning(ctx, opts.Metadata, logger); err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, err
	}

	// Load validator keys
	validators, err := LoadValidatorKeys(ctx, opts.Metadata, logger)
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, WrapError(StageVerifying, "load validator keys", err, "Check devnet accounts directory")
	}
	progress.TotalVoters = len(validators)

	// Get current height and calculate upgrade height
	rpc := NewRPCClient("localhost", DefaultRPCPort)
	currentHeight, err := rpc.GetCurrentHeight(ctx)
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, WrapError(StageVerifying, "get current height", err, "Check RPC connectivity")
	}
	progress.CurrentHeight = currentHeight

	// Calculate upgrade height
	upgradeHeight := cfg.UpgradeHeight
	if upgradeHeight == 0 {
		upgradeHeight, err = CalculateUpgradeHeight(ctx, cfg, rpc, logger)
		if err != nil {
			progress.Stage = StageFailed
			progress.Error = err
			notifyProgress()
			return nil, err
		}
	}
	progress.TargetHeight = upgradeHeight

	logger.Debug("Upgrade height: %d (current: %d)", upgradeHeight, currentHeight)

	// Stage 2: Submit proposal
	progress.Stage = StageSubmitting
	notifyProgress()

	evmRPCURL := fmt.Sprintf("http://localhost:%d", DefaultEVMPort)

	proposal, err := SubmitProposal(ctx, &ProposalOptions{
		UpgradeName:   cfg.Name,
		UpgradeHeight: upgradeHeight,
		ProposerKey:   validators[0].PrivateKey,
		ProposerAddr:  validators[0].HexAddress,
		EVMRPCURL:     evmRPCURL,
		DepositAmount: DefaultDepositAmount,
		DepositDenom:  DefaultDepositDenom,
		Logger:        logger,
	})
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, err
	}

	progress.Proposal = proposal
	result.ProposalID = proposal.ID
	result.UpgradeHeight = upgradeHeight
	notifyProgress()

	logger.Debug("Proposal submitted: ID=%d, TX=%s", proposal.ID, proposal.TxHash)

	// Stage 3: Vote from all validators
	progress.Stage = StageVoting
	notifyProgress()

	err = VoteFromAllValidators(ctx, validators, proposal.ID, evmRPCURL, logger,
		func(voted, total int) {
			progress.VotesCast = voted
			notifyProgress()
		})
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, err
	}

	logger.Debug("All votes cast: %d/%d", progress.VotesCast, progress.TotalVoters)

	// Stage 4: Wait for upgrade height
	progress.Stage = StageWaiting
	notifyProgress()

	err = WaitForUpgradeHeight(ctx, &MonitorOptions{
		TargetHeight: upgradeHeight,
		Logger:       logger,
		OnProgress: func(current, target int64) {
			progress.CurrentHeight = current
			notifyProgress()
		},
	})
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, err
	}

	// Wait for chain to halt
	logger.Debug("Waiting for chain to halt...")
	err = WaitForChainHalt(ctx, upgradeHeight, logger)
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, err
	}

	logger.Debug("Chain halted for upgrade")

	// Export pre-upgrade genesis if requested
	if cfg.ExportGenesis {
		logger.Debug("Exporting pre-upgrade genesis...")
		preSnapshot, err := ExportGenesis(ctx, &GenesisExportOptions{
			HomeDir:     opts.HomeDir,
			ExportDir:   cfg.GenesisDir,
			Metadata:    opts.Metadata,
			Logger:      logger,
			UpgradeName: cfg.Name,
			PreUpgrade:  true,
		})
		if err != nil {
			logger.Warn("Pre-upgrade genesis export failed: %v", err)
		} else {
			result.PreGenesisPath = preSnapshot.FilePath
			logger.Debug("Pre-upgrade genesis saved: %s", preSnapshot.FilePath)
		}
	}

	// Stage 5: Switch binary
	progress.Stage = StageSwitching
	notifyProgress()

	err = SwitchBinary(ctx, &SwitchOptions{
		Mode:         opts.Metadata.ExecutionMode,
		TargetImage:  cfg.TargetImage,
		TargetBinary: cfg.TargetBinary,
		CachePath:    cfg.CachePath,
		CommitHash:   cfg.CommitHash,
		HomeDir:      opts.HomeDir,
		Metadata:     opts.Metadata,
		Logger:       logger,
	})
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, err
	}

	logger.Debug("Binary switched, waiting for chain to resume...")

	// Stage 6: Verify chain resumed
	progress.Stage = StageVerifyingResume
	notifyProgress()

	postHeight, err := VerifyChainResumed(ctx, logger)
	if err != nil {
		progress.Stage = StageFailed
		progress.Error = err
		notifyProgress()
		return nil, err
	}

	// Export post-upgrade genesis if requested
	if cfg.ExportGenesis {
		logger.Debug("Exporting post-upgrade genesis...")
		postSnapshot, err := ExportGenesis(ctx, &GenesisExportOptions{
			HomeDir:     opts.HomeDir,
			ExportDir:   cfg.GenesisDir,
			Metadata:    opts.Metadata,
			Logger:      logger,
			UpgradeName: cfg.Name,
			PreUpgrade:  false,
		})
		if err != nil {
			logger.Warn("Post-upgrade genesis export failed: %v", err)
		} else {
			result.PostGenesisPath = postSnapshot.FilePath
			logger.Debug("Post-upgrade genesis saved: %s", postSnapshot.FilePath)
		}
	}

	// Success!
	progress.Stage = StageCompleted
	now := time.Now()
	progress.CompletedAt = &now
	progress.CurrentHeight = postHeight
	notifyProgress()

	result.Success = true
	result.PostUpgradeHeight = postHeight
	result.Duration = time.Since(startTime)
	if cfg.TargetImage != "" {
		result.NewBinary = cfg.TargetImage
	} else {
		result.NewBinary = cfg.TargetBinary
	}

	return result, nil
}

// CalculateUpgradeHeight calculates the upgrade height based on voting period and block time.
func CalculateUpgradeHeight(ctx context.Context, cfg *UpgradeConfig, rpc *RPCClient, logger *output.Logger) (int64, error) {
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Get current height
	currentHeight, err := rpc.GetCurrentHeight(ctx)
	if err != nil {
		return 0, WrapError(StageVerifying, "get current height", err, "Check RPC connectivity")
	}

	// Estimate block time
	blockTime, err := rpc.GetBlockTime(ctx, 50)
	if err != nil {
		logger.Debug("Could not estimate block time, using default: %v", err)
		blockTime = 2 * time.Second
	}

	logger.Debug("Estimated block time: %v", blockTime)

	// Calculate blocks during voting period
	votingPeriod := cfg.VotingPeriod
	if votingPeriod == 0 {
		votingPeriod = DefaultVotingPeriod
	}

	blocksForVoting := int64(votingPeriod/blockTime) + 1

	// Calculate upgrade height
	buffer := int64(cfg.HeightBuffer)
	if buffer == 0 {
		buffer = DefaultHeightBuffer
	}

	upgradeHeight := currentHeight + blocksForVoting + buffer

	logger.Debug("Calculated upgrade height: %d (current=%d, voting=%d blocks, buffer=%d)",
		upgradeHeight, currentHeight, blocksForVoting, buffer)

	return upgradeHeight, nil
}

// verifyDevnetRunning checks if the devnet is running and healthy.
func verifyDevnetRunning(ctx context.Context, metadata *devnet.DevnetMetadata, logger *output.Logger) error {
	if metadata == nil {
		return WrapError(StageVerifying, "load metadata", ErrDevnetNotRunning,
			"Start devnet with 'devnet-builder start' first")
	}

	if metadata.Status != devnet.StatusRunning {
		return WrapError(StageVerifying, "check status", ErrDevnetNotRunning,
			"Start devnet with 'devnet-builder start' first")
	}

	// Verify RPC is responding
	rpc := NewRPCClient("localhost", DefaultRPCPort)
	if !rpc.IsChainRunning(ctx) {
		return WrapError(StageVerifying, "check RPC", ErrDevnetNotRunning,
			"Chain is not responding. Try 'devnet-builder status' to check.")
	}

	return nil
}

// PreflightCheck performs pre-flight validation before upgrade.
func PreflightCheck(ctx context.Context, cfg *UpgradeConfig, opts *ExecuteOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return err
	}

	// Check devnet is running
	if err := verifyDevnetRunning(ctx, opts.Metadata, logger); err != nil {
		return err
	}

	// Check Docker availability if needed
	if cfg.IsDockerMode() {
		if err := checkDockerAvailable(ctx); err != nil {
			return WrapError(StageVerifying, "check Docker", ErrDockerNotAvailable,
				"Install and start Docker, or use --binary for local mode")
		}
	} else {
		// Check binary exists
		if cfg.TargetBinary != "" {
			if err := checkBinaryExists(cfg.TargetBinary); err != nil {
				return err
			}
		}
	}

	// Check validator balance (optional, just warn)
	validators, err := LoadValidatorKeys(ctx, opts.Metadata, logger)
	if err != nil {
		return WrapError(StageVerifying, "load validator keys", err, "Check devnet accounts")
	}

	if len(validators) == 0 {
		return WrapError(StageVerifying, "check validators", ErrNoValidators, "No validators found")
	}

	evmRPCURL := fmt.Sprintf("http://localhost:%d", DefaultEVMPort)
	if err := CheckBalance(ctx, evmRPCURL, validators[0].HexAddress, DefaultDepositAmount); err != nil {
		logger.Warn("Balance check failed: %v", err)
		// Don't fail here, let the actual submission fail with a clearer error
	}

	return nil
}

func checkDockerAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run()
}

func checkBinaryExists(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return WrapError(StageVerifying, "check binary", ErrBinaryNotFound,
			fmt.Sprintf("Binary not found at %s", path))
	}
	if err != nil {
		return err
	}
	if info.Mode()&0111 == 0 {
		return WrapError(StageVerifying, "check binary", ErrBinaryNotFound,
			fmt.Sprintf("Binary at %s is not executable", path))
	}
	return nil
}
