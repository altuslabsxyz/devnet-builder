// Package upgrade contains UseCases for chain upgrade operations.
package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ProposeUseCase handles submitting upgrade proposals.
type ProposeUseCase struct {
	devnetRepo ports.DevnetRepository
	rpcClient  ports.RPCClient
	evmClient  ports.EVMClient
	keyManager ports.KeyManager
	logger     ports.Logger
}

// NewProposeUseCase creates a new ProposeUseCase.
func NewProposeUseCase(
	devnetRepo ports.DevnetRepository,
	rpcClient ports.RPCClient,
	evmClient ports.EVMClient,
	keyManager ports.KeyManager,
	logger ports.Logger,
) *ProposeUseCase {
	return &ProposeUseCase{
		devnetRepo: devnetRepo,
		rpcClient:  rpcClient,
		evmClient:  evmClient,
		keyManager: keyManager,
		logger:     logger,
	}
}

// Execute submits an upgrade proposal.
func (uc *ProposeUseCase) Execute(ctx context.Context, input dto.ProposeInput) (*dto.ProposeOutput, error) {
	uc.logger.Info("Submitting upgrade proposal...")

	// Verify devnet is running
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}
	if metadata.Status != ports.StateRunning {
		return nil, fmt.Errorf("devnet is not running")
	}

	// Calculate upgrade height if not specified
	upgradeHeight := input.UpgradeHeight
	if upgradeHeight == 0 {
		upgradeHeight, err = uc.calculateUpgradeHeight(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate upgrade height: %w", err)
		}
	}

	uc.logger.Debug("Upgrade height: %d", upgradeHeight)

	// Get proposer key
	proposerKey, err := uc.keyManager.GetKey("validator0")
	if err != nil {
		return nil, fmt.Errorf("failed to get proposer key: %w", err)
	}

	// Build and submit proposal transaction
	txHash, proposalID, err := uc.submitProposal(ctx, input, upgradeHeight, proposerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to submit proposal: %w", err)
	}

	// Calculate voting end time
	votingEndTime := time.Now().Add(input.VotingPeriod)

	uc.logger.Success("Proposal submitted: ID=%d, TX=%s", proposalID, txHash)
	return &dto.ProposeOutput{
		ProposalID:    proposalID,
		UpgradeHeight: upgradeHeight,
		TxHash:        txHash,
		VotingEndTime: votingEndTime,
	}, nil
}

func (uc *ProposeUseCase) calculateUpgradeHeight(ctx context.Context, input dto.ProposeInput) (int64, error) {
	// Get current height
	currentHeight, err := uc.rpcClient.GetBlockHeight(ctx)
	if err != nil {
		return 0, err
	}

	// Estimate block time
	blockTime, err := uc.rpcClient.GetBlockTime(ctx, 50)
	if err != nil {
		uc.logger.Debug("Could not estimate block time, using default 2s")
		blockTime = 2 * time.Second
	}

	// Calculate blocks during voting period
	votingBlocks := int64(input.VotingPeriod / blockTime)

	// Add buffer
	buffer := int64(input.HeightBuffer)
	if buffer == 0 {
		buffer = 10
	}

	return currentHeight + votingBlocks + buffer, nil
}

func (uc *ProposeUseCase) submitProposal(ctx context.Context, input dto.ProposeInput, upgradeHeight int64, proposerKey *ports.KeyInfo) (string, uint64, error) {
	// This would build and sign an EVM transaction to submit the upgrade proposal
	// For now, return placeholder values
	// In real implementation, this would:
	// 1. Build the proposal message
	// 2. Sign with proposer's private key
	// 3. Broadcast via EVM RPC
	// 4. Parse proposal ID from events

	return "0x...", 1, nil
}
