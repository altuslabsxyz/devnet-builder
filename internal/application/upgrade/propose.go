// Package upgrade contains UseCases for chain upgrade operations.
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

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	// GovPrecompileAddress is the EVM address of the governance precompile.
	GovPrecompileAddress = "0x0000000000000000000000000000000000000805"

	// GovAuthority is the governance module account address.
	GovAuthority = "stable10d07y265gmmuvt4z0w9aw880jnsr700jjjzdw5"

	// DefaultDepositAmount is the deposit amount in astable (50002 STABLE).
	DefaultDepositAmount = "50002000000000000000000"

	// DefaultDepositDenom is the denomination for deposit.
	DefaultDepositDenom = "astable"

	// DefaultGasLimit is the default gas limit for EVM transactions.
	DefaultGasLimit = uint64(500000)
)

// ProposeUseCase handles submitting upgrade proposals.
type ProposeUseCase struct {
	devnetRepo         ports.DevnetRepository
	rpcClient          ports.RPCClient
	validatorKeyLoader ports.ValidatorKeyLoader
	logger             ports.Logger
}

// NewProposeUseCase creates a new ProposeUseCase.
func NewProposeUseCase(
	devnetRepo ports.DevnetRepository,
	rpcClient ports.RPCClient,
	validatorKeyLoader ports.ValidatorKeyLoader,
	logger ports.Logger,
) *ProposeUseCase {
	return &ProposeUseCase{
		devnetRepo:         devnetRepo,
		rpcClient:          rpcClient,
		validatorKeyLoader: validatorKeyLoader,
		logger:             logger,
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

	// Load validator0 key for proposal submission
	validatorKeys, err := uc.validatorKeyLoader.LoadValidatorKeys(ctx, ports.ValidatorKeyOptions{
		HomeDir:       input.HomeDir,
		NumValidators: 1, // Only need first validator for proposal
		ExecutionMode: metadata.ExecutionMode,
		Version:       metadata.CurrentVersion,
		BinaryName:    metadata.BinaryName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load validator keys: %w", err)
	}
	proposerKey := validatorKeys[0]

	// Get EVM RPC URL (default EVM port for node0)
	evmRPCURL := "http://localhost:8545"

	// Build and submit proposal transaction
	txHash, proposalID, err := uc.submitProposal(ctx, input, upgradeHeight, proposerKey, evmRPCURL)
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
		buffer = 30
	}

	return currentHeight + votingBlocks + buffer, nil
}

func (uc *ProposeUseCase) submitProposal(ctx context.Context, input dto.ProposeInput, upgradeHeight int64, proposer ports.ValidatorKey, evmRPCURL string) (string, uint64, error) {
	// Connect to EVM RPC
	client, err := ethclient.DialContext(ctx, evmRPCURL)
	if err != nil {
		return "", 0, fmt.Errorf("failed to connect to EVM RPC: %w", err)
	}
	defer client.Close()

	// Parse private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(proposer.PrivateKey, "0x"))
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Get nonce
	fromAddr := common.HexToAddress(proposer.HexAddress)
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get gas price: %w", err)
	}

	// Build proposal JSON
	proposalJSON := buildProposalJSON(input.UpgradeName, upgradeHeight, "Automated devnet upgrade")

	uc.logger.Debug("Proposal JSON: %s", proposalJSON)
	uc.logger.Debug("Proposal hex length: %d", len(hex.EncodeToString([]byte(proposalJSON))))

	// Get deposit values
	depositAmount := input.DepositAmount
	if depositAmount == "" {
		depositAmount = DefaultDepositAmount
	}
	depositDenom := input.DepositDenom
	if depositDenom == "" {
		depositDenom = DefaultDepositDenom
	}

	// Build call data for submitProposal
	callData, err := buildSubmitProposalCallData(proposer.HexAddress, proposalJSON, depositDenom, depositAmount)
	if err != nil {
		return "", 0, fmt.Errorf("failed to build call data: %w", err)
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
		uc.logger.Debug("Gas estimation failed: %v, using default", err)
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
		return "", 0, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	uc.logger.Debug("Proposal TX sent: %s", txHash)

	// Wait for receipt
	receipt, err := waitForReceipt(ctx, client, signedTx.Hash())
	if err != nil {
		return "", 0, fmt.Errorf("failed to wait for receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return "", 0, fmt.Errorf("proposal transaction reverted")
	}

	// Parse proposal ID from logs
	proposalID, err := parseProposalIDFromLogs(receipt.Logs)
	if err != nil {
		uc.logger.Debug("Could not parse proposal ID from logs: %v, querying REST API", err)
		// Fallback: query REST API for latest proposal ID
		restURL := "http://localhost:1317"
		proposalID, err = getLatestProposalID(restURL)
		if err != nil {
			uc.logger.Warn("Could not get proposal ID from REST API: %v", err)
			// Use a placeholder - the proposal was submitted
			proposalID = 1
		}
	}

	return txHash, proposalID, nil
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

// buildSubmitProposalCallData builds the ABI-encoded call data for submitProposal.
func buildSubmitProposalCallData(proposer, proposalJSON, depositDenom, depositAmount string) ([]byte, error) {
	// Method ID: first 4 bytes of keccak256("submitProposal(address,bytes,(string,uint256)[])")
	methodID := crypto.Keccak256([]byte("submitProposal(address,bytes,(string,uint256)[])"))[:4]

	// Proposer address (32 bytes, padded)
	proposerAddr := common.HexToAddress(proposer)

	// Convert proposal JSON to bytes
	proposalBytes := []byte(proposalJSON)

	// Parse deposit amount
	depositAmt := new(big.Int)
	depositAmt.SetString(depositAmount, 10)

	// Build the call data using manual ABI encoding
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
	submitProposalEventSig := crypto.Keccak256Hash([]byte("SubmitProposal(address,uint64)"))

	for _, log := range logs {
		if len(log.Topics) > 0 && log.Topics[0] == submitProposalEventSig {
			if len(log.Data) >= 32 {
				proposalID := new(big.Int).SetBytes(log.Data[:32]).Uint64()
				return proposalID, nil
			}
		}
	}

	// Fallback: check if proposal ID is in any topic
	for _, log := range logs {
		if len(log.Topics) > 1 {
			proposalID := log.Topics[1].Big().Uint64()
			if proposalID > 0 && proposalID < 1000000 {
				return proposalID, nil
			}
		}
	}

	return 0, fmt.Errorf("no proposal ID in logs")
}

func getLatestProposalID(restURL string) (uint64, error) {
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
