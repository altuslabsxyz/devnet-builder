// pkg/network/cosmos/txbuilder.go
package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// TxBuilder implements the network.TxBuilder interface for Cosmos SDK chains.
// It handles transaction building, signing, and broadcasting for SDK v0.50+.
type TxBuilder struct {
	rpcEndpoint string
	chainID     string
	sdkVersion  *network.SDKVersion
	client      *http.Client
}

// NewTxBuilder creates a new TxBuilder for Cosmos SDK chains.
// If SDKVersion is not provided in the config, it will be auto-detected from the node.
func NewTxBuilder(ctx context.Context, cfg *network.TxBuilderConfig) (*TxBuilder, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	if cfg.RPCEndpoint == "" {
		return nil, fmt.Errorf("RPC endpoint is required")
	}

	if cfg.ChainID == "" {
		return nil, fmt.Errorf("chain ID is required")
	}

	// Use provided SDK version or auto-detect
	sdkVersion := cfg.SDKVersion
	if sdkVersion == nil {
		detected, err := DetectSDKVersion(ctx, cfg.RPCEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to detect SDK version: %w", err)
		}
		sdkVersion = detected
	}

	// Create HTTP client for RPC communication
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &TxBuilder{
		rpcEndpoint: cfg.RPCEndpoint,
		chainID:     cfg.ChainID,
		sdkVersion:  sdkVersion,
		client:      client,
	}, nil
}

// BuildTx constructs an unsigned transaction from a request.
// It creates the SDK message, queries account info, and builds the transaction.
func (b *TxBuilder) BuildTx(ctx context.Context, req *network.TxBuildRequest) (*network.UnsignedTx, error) {
	// 1. Build the SDK message from the request
	msg, err := BuildMessage(req.TxType, req.Sender, req.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to build message: %w", err)
	}

	// 2. Query account info to get account number and sequence
	accountInfo, err := b.QueryAccount(ctx, req.Sender)
	if err != nil {
		return nil, fmt.Errorf("failed to query account: %w", err)
	}

	// 3. Parse gas price if provided
	var gasPrice sdk.DecCoin
	if req.GasPrice != "" {
		gasPrice, err = ParseGasPrice(req.GasPrice)
		if err != nil {
			return nil, fmt.Errorf("failed to parse gas price: %w", err)
		}
	}

	// 4. Calculate fee from gas limit and gas price
	var fees sdk.Coins
	if req.GasLimit > 0 && !gasPrice.IsZero() {
		// Fee = gasLimit * gasPrice
		feeAmount := gasPrice.Amount.MulInt64(int64(req.GasLimit)).TruncateInt()
		fees = sdk.NewCoins(sdk.NewCoin(gasPrice.Denom, feeAmount))
	}

	// 5. Build the transaction body and auth info
	// TODO(Task 6): Replace with proper TxConfig implementation
	txBytes, signDoc, err := buildTxBytesAndSignDoc(
		b.chainID,
		accountInfo.AccountNumber,
		accountInfo.Sequence,
		req.GasLimit,
		fees,
		req.Memo,
		msg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx bytes: %w", err)
	}

	return &network.UnsignedTx{
		TxBytes:       txBytes,
		SignDoc:       signDoc,
		AccountNumber: accountInfo.AccountNumber,
		Sequence:      accountInfo.Sequence,
	}, nil
}

// buildTxBytesAndSignDoc creates the transaction bytes and sign document.
// TODO(Task 6): This will be replaced with proper TxConfig and TxBuilder from the SDK.
func buildTxBytesAndSignDoc(
	chainID string,
	accountNumber, sequence, gasLimit uint64,
	fees sdk.Coins,
	memo string,
	msgs ...sdk.Msg,
) (txBytes, signDoc []byte, err error) {
	// Create transaction body
	txBody := &TxBody{
		Messages: msgs,
		Memo:     memo,
	}

	// Create auth info
	authInfo := &AuthInfo{
		Fee: &Fee{
			Amount:   fees,
			GasLimit: gasLimit,
		},
	}

	// Create sign doc
	signDocStruct := &SignDoc{
		BodyBytes:     nil, // Will be set after marshaling
		AuthInfoBytes: nil, // Will be set after marshaling
		ChainID:       chainID,
		AccountNumber: accountNumber,
	}

	// For now, we create a simple JSON representation that can be signed
	// TODO(Task 6): Use proper protobuf encoding with TxConfig
	txBytesData, err := json.Marshal(map[string]interface{}{
		"body":      txBody,
		"auth_info": authInfo,
		"chain_id":  chainID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal tx: %w", err)
	}

	signDocData, err := json.Marshal(map[string]interface{}{
		"chain_id":       chainID,
		"account_number": accountNumber,
		"sequence":       sequence,
		"body":           txBody,
		"auth_info":      authInfo,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal sign doc: %w", err)
	}

	_ = signDocStruct // Suppress unused warning for now

	return txBytesData, signDocData, nil
}

// TxBody represents the body of a transaction.
// TODO(Task 6): Use proper protobuf types from the SDK.
type TxBody struct {
	Messages []sdk.Msg `json:"messages"`
	Memo     string    `json:"memo"`
}

// AuthInfo represents the authentication info of a transaction.
// TODO(Task 6): Use proper protobuf types from the SDK.
type AuthInfo struct {
	Fee *Fee `json:"fee"`
}

// Fee represents the fee for a transaction.
// TODO(Task 6): Use proper protobuf types from the SDK.
type Fee struct {
	Amount   sdk.Coins `json:"amount"`
	GasLimit uint64    `json:"gas_limit"`
}

// SignDoc represents the document to be signed.
// TODO(Task 6): Use proper protobuf types from the SDK.
type SignDoc struct {
	BodyBytes     []byte `json:"body_bytes"`
	AuthInfoBytes []byte `json:"auth_info_bytes"`
	ChainID       string `json:"chain_id"`
	AccountNumber uint64 `json:"account_number"`
}

// SignTx signs an unsigned transaction with the provided key.
// This is a stub implementation that will be completed in a later task.
func (b *TxBuilder) SignTx(ctx context.Context, tx *network.UnsignedTx, key *network.SigningKey) (*network.SignedTx, error) {
	return nil, fmt.Errorf("SignTx not yet implemented")
}

// BroadcastTx submits a signed transaction to the network.
// This is a stub implementation that will be completed in a later task.
func (b *TxBuilder) BroadcastTx(ctx context.Context, tx *network.SignedTx) (*network.TxBroadcastResult, error) {
	return nil, fmt.Errorf("BroadcastTx not yet implemented")
}

// SupportedTxTypes returns the transaction types this builder supports.
// The supported types depend on the detected SDK version and features.
func (b *TxBuilder) SupportedTxTypes() []network.TxType {
	types := []network.TxType{
		network.TxTypeBankSend,
		network.TxTypeStakingDelegate,
		network.TxTypeStakingUnbond,
	}

	// Add governance types based on features
	if b.hasFeature(network.FeatureGovV1) {
		types = append(types, network.TxTypeGovVote, network.TxTypeGovProposal)
	}

	// Add IBC transfer - available in most chains
	types = append(types, network.TxTypeIBCTransfer)

	return types
}

// hasFeature checks if the SDK version supports a given feature.
func (b *TxBuilder) hasFeature(feature string) bool {
	if b.sdkVersion == nil {
		return false
	}
	for _, f := range b.sdkVersion.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// RPCEndpoint returns the configured RPC endpoint.
func (b *TxBuilder) RPCEndpoint() string {
	return b.rpcEndpoint
}

// ChainID returns the configured chain ID.
func (b *TxBuilder) ChainID() string {
	return b.chainID
}

// SDKVersion returns the detected or configured SDK version.
func (b *TxBuilder) SDKVersion() *network.SDKVersion {
	return b.sdkVersion
}
