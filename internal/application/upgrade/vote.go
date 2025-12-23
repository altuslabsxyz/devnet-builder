package upgrade

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	// VoteOptionYes is the vote option for YES.
	VoteOptionYes = 1
	// VoteOptionAbstain is the vote option for ABSTAIN.
	VoteOptionAbstain = 2
	// VoteOptionNo is the vote option for NO.
	VoteOptionNo = 3
	// VoteOptionNoWithVeto is the vote option for NO_WITH_VETO.
	VoteOptionNoWithVeto = 4
)

// VoteUseCase handles voting on upgrade proposals.
type VoteUseCase struct {
	devnetRepo         ports.DevnetRepository
	rpcClient          ports.RPCClient
	validatorKeyLoader ports.ValidatorKeyLoader
	logger             ports.Logger
}

// NewVoteUseCase creates a new VoteUseCase.
func NewVoteUseCase(
	devnetRepo ports.DevnetRepository,
	rpcClient ports.RPCClient,
	validatorKeyLoader ports.ValidatorKeyLoader,
	logger ports.Logger,
) *VoteUseCase {
	return &VoteUseCase{
		devnetRepo:         devnetRepo,
		rpcClient:          rpcClient,
		validatorKeyLoader: validatorKeyLoader,
		logger:             logger,
	}
}

// Execute votes on a proposal.
func (uc *VoteUseCase) Execute(ctx context.Context, input dto.VoteInput) (*dto.VoteOutput, error) {
	uc.logger.Info("Voting on proposal %d...", input.ProposalID)

	// Load devnet metadata
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Verify proposal exists and is in voting period
	proposal, err := uc.rpcClient.GetProposal(ctx, input.ProposalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get proposal: %w", err)
	}
	if proposal.Status != ports.ProposalStatusVoting {
		return nil, fmt.Errorf("proposal is not in voting period: %s", proposal.Status)
	}

	// Determine how many validators to vote with
	numVoters := 1
	if input.FromAll {
		numVoters = metadata.NumValidators
	}

	// Load validator keys
	validatorKeys, err := uc.validatorKeyLoader.LoadValidatorKeys(ctx, ports.ValidatorKeyOptions{
		HomeDir:       input.HomeDir,
		NumValidators: numVoters,
		ExecutionMode: metadata.ExecutionMode,
		Version:       metadata.CurrentVersion,
		BinaryName:    metadata.BinaryName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load validator keys: %w", err)
	}

	// Get EVM RPC URL (default EVM port for node0)
	evmRPCURL := "http://localhost:8545"

	// Parse vote option
	voteOption, err := ParseVoteOption(input.VoteOption)
	if err != nil {
		return nil, err
	}

	// Vote from each validator
	output := &dto.VoteOutput{
		TotalVoters: len(validatorKeys),
		TxHashes:    make([]string, 0, len(validatorKeys)),
		Errors:      make([]string, 0),
	}

	for _, key := range validatorKeys {
		txHash, err := uc.submitVote(ctx, input.ProposalID, voteOption, key, evmRPCURL)
		if err != nil {
			output.Errors = append(output.Errors, fmt.Sprintf("%s: %v", key.Name, err))
			uc.logger.Warn("Vote from %s failed: %v", key.Name, err)
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

func (uc *VoteUseCase) submitVote(ctx context.Context, proposalID uint64, voteOption int, voter ports.ValidatorKey, evmRPCURL string) (string, error) {
	// Connect to EVM RPC
	client, err := ethclient.DialContext(ctx, evmRPCURL)
	if err != nil {
		return "", fmt.Errorf("failed to connect to EVM RPC: %w", err)
	}
	defer client.Close()

	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(voter.PrivateKey, "0x"))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Get nonce
	fromAddr := common.HexToAddress(voter.HexAddress)
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Build call data for vote
	callData := buildVoteCallData(voter.HexAddress, proposalID, voteOption, "")

	// Create transaction
	govAddr := common.HexToAddress(GovPrecompileAddress)
	tx := types.NewTransaction(nonce, govAddr, big.NewInt(0), DefaultGasLimit, gasPrice, callData)

	// Sign transaction
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	uc.logger.Debug("Vote TX sent from %s: %s", voter.HexAddress, txHash)

	// Wait for receipt
	receipt, err := waitForReceipt(ctx, client, signedTx.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to wait for receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return "", fmt.Errorf("vote transaction reverted")
	}

	return txHash, nil
}

// buildVoteCallData builds the ABI-encoded call data for vote.
func buildVoteCallData(voter string, proposalID uint64, option int, metadata string) []byte {
	// Function: vote(address voter, uint64 proposalId, uint8 option, string metadata)
	// Method ID: first 4 bytes of keccak256("vote(address,uint64,uint8,string)")
	methodID := crypto.Keccak256([]byte("vote(address,uint64,uint8,string)"))[:4]

	voterAddr := common.HexToAddress(voter)

	data := make([]byte, 0, 4+32*5)
	data = append(data, methodID...)

	// Voter address (padded to 32 bytes)
	data = append(data, common.LeftPadBytes(voterAddr.Bytes(), 32)...)

	// Proposal ID (uint64, padded to 32 bytes)
	data = append(data, common.LeftPadBytes(big.NewInt(int64(proposalID)).Bytes(), 32)...)

	// Option (uint8, padded to 32 bytes)
	data = append(data, common.LeftPadBytes([]byte{byte(option)}, 32)...)

	// Metadata string offset (128 = 4 * 32)
	data = append(data, common.LeftPadBytes(big.NewInt(128).Bytes(), 32)...)

	// Metadata string length (0 for empty)
	data = append(data, common.LeftPadBytes(big.NewInt(int64(len(metadata))).Bytes(), 32)...)

	// Metadata string (empty, but pad to 32 bytes)
	if len(metadata) > 0 {
		paddedMetadata := make([]byte, ((len(metadata)+31)/32)*32)
		copy(paddedMetadata, []byte(metadata))
		data = append(data, paddedMetadata...)
	}

	return data
}

// ParseVoteOption converts string vote option to enum.
func ParseVoteOption(option string) (int, error) {
	switch strings.ToLower(option) {
	case "yes":
		return VoteOptionYes, nil
	case "no":
		return VoteOptionNo, nil
	case "abstain":
		return VoteOptionAbstain, nil
	case "no_with_veto":
		return VoteOptionNoWithVeto, nil
	default:
		return 0, fmt.Errorf("invalid vote option: %s", option)
	}
}
