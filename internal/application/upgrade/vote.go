package upgrade

import (
	"context"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// VoteUseCase handles voting on upgrade proposals.
type VoteUseCase struct {
	devnetRepo ports.DevnetRepository
	rpcClient  ports.RPCClient
	evmClient  ports.EVMClient
	keyManager ports.KeyManager
	logger     ports.Logger
}

// NewVoteUseCase creates a new VoteUseCase.
func NewVoteUseCase(
	devnetRepo ports.DevnetRepository,
	rpcClient ports.RPCClient,
	evmClient ports.EVMClient,
	keyManager ports.KeyManager,
	logger ports.Logger,
) *VoteUseCase {
	return &VoteUseCase{
		devnetRepo: devnetRepo,
		rpcClient:  rpcClient,
		evmClient:  evmClient,
		keyManager: keyManager,
		logger:     logger,
	}
}

// Execute votes on a proposal.
func (uc *VoteUseCase) Execute(ctx context.Context, input dto.VoteInput) (*dto.VoteOutput, error) {
	uc.logger.Info("Voting on proposal %d...", input.ProposalID)

	// Verify proposal exists and is in voting period
	proposal, err := uc.rpcClient.GetProposal(ctx, input.ProposalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get proposal: %w", err)
	}
	if proposal.Status != ports.ProposalStatusVoting {
		return nil, fmt.Errorf("proposal is not in voting period: %s", proposal.Status)
	}

	// Get keys to vote with
	var keys []*ports.KeyInfo
	if input.FromAll {
		keys, err = uc.keyManager.ListKeys()
		if err != nil {
			return nil, fmt.Errorf("failed to list keys: %w", err)
		}
		// Filter to only validator keys
		validatorKeys := make([]*ports.KeyInfo, 0)
		for _, k := range keys {
			if len(k.Name) > 9 && k.Name[:9] == "validator" {
				validatorKeys = append(validatorKeys, k)
			}
		}
		keys = validatorKeys
	} else {
		key, err := uc.keyManager.GetKey("validator0")
		if err != nil {
			return nil, fmt.Errorf("failed to get validator key: %w", err)
		}
		keys = []*ports.KeyInfo{key}
	}

	// Vote from each key
	output := &dto.VoteOutput{
		TotalVoters: len(keys),
		TxHashes:    make([]string, 0, len(keys)),
		Errors:      make([]string, 0),
	}

	for _, key := range keys {
		txHash, err := uc.submitVote(ctx, input.ProposalID, input.VoteOption, key)
		if err != nil {
			output.Errors = append(output.Errors, fmt.Sprintf("%s: %v", key.Name, err))
			continue
		}
		output.TxHashes = append(output.TxHashes, txHash)
		output.VotesCast++
		uc.logger.Debug("Vote cast from %s: %s", key.Name, txHash)
	}

	if output.VotesCast == output.TotalVoters {
		uc.logger.Success("All votes cast successfully!")
	} else {
		uc.logger.Warn("Some votes failed: %d/%d", output.VotesCast, output.TotalVoters)
	}

	return output, nil
}

func (uc *VoteUseCase) submitVote(ctx context.Context, proposalID uint64, voteOption string, key *ports.KeyInfo) (string, error) {
	// This would build and sign an EVM transaction to vote
	// For now, return placeholder value
	return "0x...", nil
}

// VoteOption converts string vote option to enum.
func VoteOption(option string) (int, error) {
	switch option {
	case "yes":
		return 1, nil
	case "no":
		return 3, nil
	case "abstain":
		return 2, nil
	case "no_with_veto":
		return 4, nil
	default:
		return 0, fmt.Errorf("invalid vote option: %s", option)
	}
}
