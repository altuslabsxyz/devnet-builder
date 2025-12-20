package upgrade

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ProposalOptions contains options for submitting a proposal.
type ProposalOptions struct {
	UpgradeName   string
	UpgradeHeight int64
	VotingPeriod  time.Duration // Expedited voting period
	ProposerKey   string        // EVM private key (hex, no 0x prefix)
	ProposerAddr  string        // EVM address (0x prefixed)
	EVMRPCURL     string        // EVM JSON-RPC URL
	DepositAmount string
	DepositDenom  string
	Logger        *output.Logger
}

// buildProposalJSON creates the MsgSoftwareUpgrade proposal JSON.
func buildProposalJSON(upgradeName string, upgradeHeight int64, info string) string {
	proposal := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"@type":     "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
				"authority": GovAuthority,
				"plan": map[string]interface{}{
					"name":   upgradeName,
					"height": fmt.Sprintf("%d", upgradeHeight),
					"info":   info,
				},
			},
		},
		"metadata":  "",
		"title":     fmt.Sprintf("Software Upgrade: %s", upgradeName),
		"summary":   fmt.Sprintf("Automated devnet upgrade to %s", upgradeName),
		"expedited": true,
	}

	jsonBytes, _ := json.Marshal(proposal)
	return string(jsonBytes)
}

// SubmitProposal submits an upgrade proposal via the EVM governance precompile.
func SubmitProposal(ctx context.Context, opts *ProposalOptions) (*UpgradeProposal, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Connect to EVM RPC
	client, err := ethclient.DialContext(ctx, opts.EVMRPCURL)
	if err != nil {
		return nil, WrapError(StageSubmitting, "connect to EVM RPC", err, "Check EVM RPC URL and network connectivity")
	}
	defer client.Close()

	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(opts.ProposerKey, "0x"))
	if err != nil {
		return nil, WrapError(StageSubmitting, "parse private key", err, "Verify validator key export")
	}

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, WrapError(StageSubmitting, "get chain ID", err, "Check EVM RPC connectivity")
	}

	// Get nonce
	fromAddr := common.HexToAddress(opts.ProposerAddr)
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return nil, WrapError(StageSubmitting, "get nonce", err, "Check account state")
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, WrapError(StageSubmitting, "get gas price", err, "Check EVM RPC")
	}

	// Build proposal JSON
	proposalJSON := buildProposalJSON(opts.UpgradeName, opts.UpgradeHeight, "Automated devnet upgrade")
	proposalHex := hex.EncodeToString([]byte(proposalJSON))

	logger.Debug("Proposal JSON: %s", proposalJSON)
	logger.Debug("Proposal hex length: %d", len(proposalHex))

	// Build call data for submitProposal
	// Function: submitProposal(address proposer, bytes proposalMsg, (string denom, uint256 amount)[] deposit)
	callData, err := buildSubmitProposalCallData(opts.ProposerAddr, proposalJSON, opts.DepositDenom, opts.DepositAmount)
	if err != nil {
		return nil, WrapError(StageSubmitting, "build call data", err, "Check proposal parameters")
	}

	// Estimate gas
	govAddr := common.HexToAddress(GovPrecompileAddress)
	msg := ethereum.CallMsg{
		From:     fromAddr,
		To:       &govAddr,
		GasPrice: gasPrice,
		Data:     callData,
	}

	gasLimit, err := client.EstimateGas(ctx, msg)
	if err != nil {
		logger.Debug("Gas estimation failed: %v, using default", err)
		gasLimit = DefaultGasLimit
	}
	// Add buffer for safety
	gasLimit = gasLimit * 150 / 100

	// Create transaction
	tx := types.NewTransaction(nonce, govAddr, big.NewInt(0), gasLimit, gasPrice, callData)

	// Sign transaction
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return nil, WrapError(StageSubmitting, "sign transaction", err, "Check private key")
	}

	// Send transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, WrapError(StageSubmitting, "send transaction", err, ErrorWithSuggestion(ErrProposalFailed))
	}

	txHash := signedTx.Hash().Hex()
	logger.Debug("Proposal TX sent: %s", txHash)

	// Wait for receipt
	receipt, err := waitForReceipt(ctx, client, signedTx.Hash())
	if err != nil {
		return nil, WrapError(StageSubmitting, "wait for receipt", err, "Transaction may have failed or timed out")
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, WrapError(StageSubmitting, "transaction reverted", ErrProposalFailed,
			"Check validator balance and proposal parameters")
	}

	// Parse proposal ID from logs
	proposalID, err := parseProposalIDFromLogs(receipt.Logs)
	if err != nil {
		logger.Debug("Could not parse proposal ID from logs: %v, querying REST API", err)
		// Fallback: query REST API for latest proposal ID
		// Derive REST URL from EVM RPC URL (assuming port 1317 for REST)
		restURL := "http://localhost:1317"
		proposalID, err = GetLatestProposalID(restURL)
		if err != nil {
			logger.Warn("Could not get proposal ID from REST API: %v", err)
			// Use a placeholder - the proposal was submitted
			proposalID = 1
		}
	}

	submittedAt := time.Now()
	votingPeriod := opts.VotingPeriod
	if votingPeriod == 0 {
		votingPeriod = DefaultVotingPeriod
	}
	return &UpgradeProposal{
		ID:            proposalID,
		TxHash:        txHash,
		UpgradeName:   opts.UpgradeName,
		UpgradeHeight: opts.UpgradeHeight,
		SubmittedAt:   submittedAt,
		VotingEndTime: submittedAt.Add(votingPeriod),
		Status:        ProposalPending,
	}, nil
}

// buildSubmitProposalCallData builds the ABI-encoded call data for submitProposal.
func buildSubmitProposalCallData(proposer, proposalJSON, depositDenom, depositAmount string) ([]byte, error) {
	// Function selector for submitProposal(address,bytes,(string,uint256)[])
	// We'll use a simplified approach - encode the call manually

	// For the precompile, we actually need to use the simpler interface
	// The governance precompile accepts the proposal as raw JSON bytes

	// Method ID: first 4 bytes of keccak256("submitProposal(address,bytes,(string,uint256)[])")
	methodID := crypto.Keccak256([]byte("submitProposal(address,bytes,(string,uint256)[])"))[:4]

	// For simplicity, we'll encode using a direct approach
	// The actual encoding would need proper ABI encoding

	// Proposer address (32 bytes, padded)
	proposerAddr := common.HexToAddress(proposer)

	// Convert proposal JSON to bytes
	proposalBytes := []byte(proposalJSON)

	// Parse deposit amount
	depositAmt := new(big.Int)
	depositAmt.SetString(depositAmount, 10)

	// Build the call data using manual ABI encoding
	// This is a simplified version - in production, use go-ethereum's abi package
	data := make([]byte, 0, 4+32*10+len(proposalBytes))
	data = append(data, methodID...)

	// Proposer address (padded to 32 bytes)
	data = append(data, common.LeftPadBytes(proposerAddr.Bytes(), 32)...)

	// Offset to bytes data (3 * 32 = 96)
	data = append(data, common.LeftPadBytes(big.NewInt(96).Bytes(), 32)...)

	// Offset to deposit array (will be after bytes)
	bytesLen := int64((len(proposalBytes) + 31) / 32 * 32) // padded length
	depositOffset := 96 + 32 + bytesLen
	data = append(data, common.LeftPadBytes(big.NewInt(depositOffset).Bytes(), 32)...)

	// Bytes length
	data = append(data, common.LeftPadBytes(big.NewInt(int64(len(proposalBytes))).Bytes(), 32)...)

	// Bytes data (padded to 32 bytes)
	paddedBytes := make([]byte, bytesLen)
	copy(paddedBytes, proposalBytes)
	data = append(data, paddedBytes...)

	// Deposit array length (1 element)
	data = append(data, common.LeftPadBytes(big.NewInt(1).Bytes(), 32)...)

	// Deposit tuple offset (32 bytes from array start)
	data = append(data, common.LeftPadBytes(big.NewInt(32).Bytes(), 32)...)

	// Deposit tuple: (string denom, uint256 amount)
	// String offset
	data = append(data, common.LeftPadBytes(big.NewInt(64).Bytes(), 32)...)

	// Amount
	data = append(data, common.LeftPadBytes(depositAmt.Bytes(), 32)...)

	// Denom string length
	data = append(data, common.LeftPadBytes(big.NewInt(int64(len(depositDenom))).Bytes(), 32)...)

	// Denom string (padded)
	denomPadded := make([]byte, 32)
	copy(denomPadded, []byte(depositDenom))
	data = append(data, denomPadded...)

	return data, nil
}

func waitForReceipt(ctx context.Context, client *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.NewTimer(2 * time.Minute)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("timeout waiting for receipt")
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil {
				return receipt, nil
			}
			// Continue waiting if receipt not found yet
		}
	}
}

func parseProposalIDFromLogs(logs []*types.Log) (uint64, error) {
	// The governance precompile emits a SubmitProposal event with proposal ID
	// Event signature: SubmitProposal(address indexed proposer, uint64 proposalId)
	submitProposalEventSig := crypto.Keccak256Hash([]byte("SubmitProposal(address,uint64)"))

	for _, log := range logs {
		if len(log.Topics) > 0 && log.Topics[0] == submitProposalEventSig {
			// The proposal ID is in the event data (not indexed)
			if len(log.Data) >= 32 {
				proposalID := new(big.Int).SetBytes(log.Data[:32]).Uint64()
				return proposalID, nil
			}
		}
	}

	// Fallback: check if proposal ID is in any topic
	for _, log := range logs {
		if len(log.Topics) > 1 {
			// Second topic might be proposal ID
			proposalID := log.Topics[1].Big().Uint64()
			// Sanity check - proposal IDs should be small numbers
			if proposalID > 0 && proposalID < 1000000 {
				return proposalID, nil
			}
		}
	}

	return 0, fmt.Errorf("no proposal ID in logs")
}

// GetLatestProposalID queries the REST API to get the latest proposal ID
func GetLatestProposalID(restURL string) (uint64, error) {
	// Query /cosmos/gov/v1/proposals endpoint
	resp, err := http.Get(restURL + "/cosmos/gov/v1/proposals?pagination.reverse=true&pagination.limit=1")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Proposals []struct {
			ID string `json:"id"`
		} `json:"proposals"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Proposals) == 0 {
		return 0, fmt.Errorf("no proposals found")
	}

	id := new(big.Int)
	id.SetString(result.Proposals[0].ID, 10)
	return id.Uint64(), nil
}

// CheckBalance checks if the account has sufficient balance for the deposit.
func CheckBalance(ctx context.Context, evmRPCURL, address, requiredAmount string) error {
	client, err := ethclient.DialContext(ctx, evmRPCURL)
	if err != nil {
		return err
	}
	defer client.Close()

	addr := common.HexToAddress(address)
	balance, err := client.BalanceAt(ctx, addr, nil)
	if err != nil {
		return err
	}

	required := new(big.Int)
	required.SetString(requiredAmount, 10)

	if balance.Cmp(required) < 0 {
		return fmt.Errorf("%w: have %s, need %s", ErrInsufficientBalance, balance.String(), required.String())
	}

	return nil
}
