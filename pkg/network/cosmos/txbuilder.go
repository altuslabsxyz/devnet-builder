// pkg/network/cosmos/txbuilder.go
package cosmos

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

	// For now, we create a simple JSON representation that can be signed.
	// TODO(Task 6): Use proper protobuf encoding with TxConfig and SignDoc struct.
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
func (b *TxBuilder) SignTx(ctx context.Context, unsignedTx *network.UnsignedTx, key *network.SigningKey) (*network.SignedTx, error) {
	// Validate key
	if len(key.PrivKey) == 0 {
		return nil, fmt.Errorf("private key required for signing")
	}

	// Load the private key
	privKey, err := LoadPrivateKey(key.PrivKey)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}
	if privKey == nil {
		return nil, fmt.Errorf("loaded private key is nil")
	}

	// Sign the sign doc
	signature, err := SignBytes(privKey, unsignedTx.SignDoc)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	pubKey := privKey.PubKey()

	return &network.SignedTx{
		TxBytes:   unsignedTx.TxBytes, // Will be updated with signature in Task 6
		Signature: signature,
		PubKey:    pubKey.Bytes(),
	}, nil
}

// BroadcastTx submits a signed transaction to the network.
func (b *TxBuilder) BroadcastTx(ctx context.Context, tx *network.SignedTx) (*network.TxBroadcastResult, error) {
	// Validate input
	if tx == nil {
		return nil, fmt.Errorf("signed transaction is required")
	}
	if len(tx.TxBytes) == 0 {
		return nil, fmt.Errorf("transaction bytes are required")
	}

	// Encode tx as base64
	txBase64 := base64.StdEncoding.EncodeToString(tx.TxBytes)

	// Create JSON-RPC request
	reqBody := BroadcastRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "broadcast_tx_sync",
		Params:  map[string]string{"tx": txBase64},
	}

	// Marshal request
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal broadcast request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.rpcEndpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send broadcast request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read broadcast response: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var broadcastResp BroadcastResponse
	if err := json.Unmarshal(respBody, &broadcastResp); err != nil {
		return nil, fmt.Errorf("failed to parse broadcast response: %w", err)
	}

	// Check for RPC error
	if broadcastResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", broadcastResp.Error.Code, broadcastResp.Error.Message)
	}

	// Return result
	return &network.TxBroadcastResult{
		TxHash: broadcastResp.Result.Hash,
		Code:   broadcastResp.Result.Code,
		Log:    broadcastResp.Result.Log,
	}, nil
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

// Ensure TxBuilder fully implements network.TxBuilder.
var _ network.TxBuilder = (*TxBuilder)(nil)
