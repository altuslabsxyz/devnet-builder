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

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// TxBuilder implements the network.TxBuilder interface for Cosmos SDK chains.
// It handles transaction building, signing, and broadcasting for SDK v0.50+.
type TxBuilder struct {
	rpcEndpoint string
	chainID     string
	sdkVersion  *network.SDKVersion
	client      *http.Client
	txConfig    client.TxConfig
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
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Initialize TxConfig for proper protobuf encoding
	txConfig := NewTxConfig()

	return &TxBuilder{
		rpcEndpoint: cfg.RPCEndpoint,
		chainID:     cfg.ChainID,
		sdkVersion:  sdkVersion,
		client:      httpClient,
		txConfig:    txConfig,
	}, nil
}

// BuildTx constructs an unsigned transaction from a request.
// It creates the SDK message, queries account info, and builds the transaction
// using proper protobuf encoding via the SDK's TxBuilder.
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

	// 5. Build the transaction using SDK's TxBuilder with proper protobuf encoding
	sdkTxBuilder := b.txConfig.NewTxBuilder()
	if err := sdkTxBuilder.SetMsgs(msg); err != nil {
		return nil, fmt.Errorf("failed to set messages: %w", err)
	}
	sdkTxBuilder.SetMemo(req.Memo)
	sdkTxBuilder.SetGasLimit(req.GasLimit)
	sdkTxBuilder.SetFeeAmount(fees)

	// 6. Encode the unsigned transaction to protobuf bytes
	// Note: This encodes the tx without signatures for reference
	txBytes, err := b.txConfig.TxEncoder()(sdkTxBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("failed to encode tx: %w", err)
	}

	// 7. Generate the sign bytes using SIGN_MODE_DIRECT
	signerData := authsigning.SignerData{
		ChainID:       b.chainID,
		AccountNumber: accountInfo.AccountNumber,
		Sequence:      accountInfo.Sequence,
	}

	signBytes, err := authsigning.GetSignBytesAdapter(
		ctx,
		b.txConfig.SignModeHandler(),
		signing.SignMode_SIGN_MODE_DIRECT,
		signerData,
		sdkTxBuilder.GetTx(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get sign bytes: %w", err)
	}

	return &network.UnsignedTx{
		TxBytes:       txBytes,
		SignDoc:       signBytes,
		AccountNumber: accountInfo.AccountNumber,
		Sequence:      accountInfo.Sequence,
	}, nil
}

// SignTx signs an unsigned transaction with the provided key.
// It decodes the unsigned tx, adds the signature, and re-encodes with proper protobuf format.
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

	// Decode the unsigned transaction
	tx, err := b.txConfig.TxDecoder()(unsignedTx.TxBytes)
	if err != nil {
		return nil, fmt.Errorf("decode unsigned tx: %w", err)
	}

	// Wrap the tx to allow modifications
	sdkTxBuilder, err := b.txConfig.WrapTxBuilder(tx)
	if err != nil {
		return nil, fmt.Errorf("wrap tx builder: %w", err)
	}

	// Set the signature with proper signing mode
	sigData := signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
		Signature: signature,
	}
	sig := signing.SignatureV2{
		PubKey:   pubKey,
		Data:     &sigData,
		Sequence: unsignedTx.Sequence,
	}

	if err := sdkTxBuilder.SetSignatures(sig); err != nil {
		return nil, fmt.Errorf("set signatures: %w", err)
	}

	// Encode the signed transaction
	signedTxBytes, err := b.txConfig.TxEncoder()(sdkTxBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("encode signed tx: %w", err)
	}

	return &network.SignedTx{
		TxBytes:   signedTxBytes,
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
