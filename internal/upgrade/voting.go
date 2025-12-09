package upgrade

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// VoteOptions contains options for casting a vote.
type VoteOptions struct {
	ProposalID uint64
	VoterKey   string // EVM private key (hex, no 0x prefix)
	VoterAddr  string // EVM address (0x prefixed)
	EVMRPCURL  string // EVM JSON-RPC URL
	Logger     *output.Logger
}

// CastVote casts a YES vote from a validator via the EVM governance precompile.
func CastVote(ctx context.Context, opts *VoteOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Connect to EVM RPC
	client, err := ethclient.DialContext(ctx, opts.EVMRPCURL)
	if err != nil {
		return WrapError(StageVoting, "connect to EVM RPC", err, "Check EVM RPC URL")
	}
	defer client.Close()

	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(opts.VoterKey, "0x"))
	if err != nil {
		return WrapError(StageVoting, "parse private key", err, "Verify validator key")
	}

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return WrapError(StageVoting, "get chain ID", err, "Check EVM RPC")
	}

	// Get nonce
	fromAddr := common.HexToAddress(opts.VoterAddr)
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return WrapError(StageVoting, "get nonce", err, "Check account state")
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return WrapError(StageVoting, "get gas price", err, "Check EVM RPC")
	}

	// Build call data for vote
	callData := buildVoteCallData(opts.VoterAddr, opts.ProposalID, VoteOptionYes, "")

	// Create transaction
	govAddr := common.HexToAddress(GovPrecompileAddress)
	tx := types.NewTransaction(nonce, govAddr, big.NewInt(0), DefaultGasLimit, gasPrice, callData)

	// Sign transaction
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return WrapError(StageVoting, "sign transaction", err, "Check private key")
	}

	// Send transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return WrapError(StageVoting, "send transaction", err, "Vote transaction failed")
	}

	logger.Debug("Vote TX sent from %s: %s", opts.VoterAddr, signedTx.Hash().Hex())

	// Wait for receipt
	receipt, err := waitForReceipt(ctx, client, signedTx.Hash())
	if err != nil {
		return WrapError(StageVoting, "wait for receipt", err, "Transaction may have failed")
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return WrapError(StageVoting, "vote transaction reverted", ErrVotingFailed,
			"The vote may have already been cast or the proposal may have ended")
	}

	return nil
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

// VoteFromAllValidators orchestrates voting from all validators.
func VoteFromAllValidators(ctx context.Context, validators []ValidatorAccount, proposalID uint64, evmRPCURL string, logger *output.Logger, progressCallback func(voted, total int)) error {
	if logger == nil {
		logger = output.DefaultLogger
	}

	totalVoters := len(validators)
	votedCount := 0
	var lastError error

	for _, validator := range validators {
		err := CastVote(ctx, &VoteOptions{
			ProposalID: proposalID,
			VoterKey:   validator.PrivateKey,
			VoterAddr:  validator.HexAddress,
			EVMRPCURL:  evmRPCURL,
			Logger:     logger,
		})

		if err != nil {
			logger.Warn("Vote from %s failed: %v", validator.Name, err)
			lastError = err
			// Continue with other validators
		} else {
			votedCount++
			logger.Debug("Vote from %s successful", validator.Name)
		}

		if progressCallback != nil {
			progressCallback(votedCount, totalVoters)
		}
	}

	// Check if we have quorum (majority)
	if votedCount == 0 {
		return WrapError(StageVoting, "all votes failed", ErrVotingFailed,
			"Check validator keys and balances")
	}

	if votedCount < (totalVoters+1)/2 {
		return WrapError(StageVoting, fmt.Sprintf("only %d/%d votes succeeded", votedCount, totalVoters), lastError,
			"Quorum may not be reached")
	}

	return nil
}
