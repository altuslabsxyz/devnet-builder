// pkg/network/evm/txbuilder.go
package evm

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// DefaultGasPriceGwei is the default gas price in wei (1 Gwei) used when no gas price is provided
// and no client is available to query the network.
const DefaultGasPriceGwei = 1000000000 // 1 Gwei in wei

// TxBuilder implements the network.TxBuilder interface for EVM chains.
// It handles transaction building, signing, and broadcasting for EVM-compatible networks.
type TxBuilder struct {
	rpcEndpoint string
	chainID     *big.Int
	client      *ethclient.Client
	httpClient  *http.Client
}

// Ensure TxBuilder fully implements network.TxBuilder.
var _ network.TxBuilder = (*TxBuilder)(nil)

// NewTxBuilder creates a new TxBuilder for EVM chains.
// It connects to the EVM RPC endpoint and validates the configuration.
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

	// Parse chain ID from string to big.Int
	chainID, ok := new(big.Int).SetString(cfg.ChainID, 10)
	if !ok {
		return nil, fmt.Errorf("invalid chain ID: %s", cfg.ChainID)
	}

	// Connect to EVM RPC using ethclient.DialContext
	client, err := ethclient.DialContext(ctx, cfg.RPCEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to EVM RPC: %w", err)
	}

	// Create HTTP client with 30s timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &TxBuilder{
		rpcEndpoint: cfg.RPCEndpoint,
		chainID:     chainID,
		client:      client,
		httpClient:  httpClient,
	}, nil
}

// SupportedTxTypes returns the transaction types this builder supports.
func (b *TxBuilder) SupportedTxTypes() []network.TxType {
	if b == nil {
		return nil
	}
	return []network.TxType{network.TxTypeBankSend}
}

// BuildTx constructs an unsigned transaction from a request.
// Supports TxTypeBankSend for native token transfers.
func (b *TxBuilder) BuildTx(ctx context.Context, req *network.TxBuildRequest) (*network.UnsignedTx, error) {
	if b == nil {
		return nil, fmt.Errorf("nil TxBuilder")
	}
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	switch req.TxType {
	case network.TxTypeBankSend:
		return b.buildNativeTransfer(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported transaction type: %s", req.TxType)
	}
}

// buildNativeTransfer builds an EVM native transfer transaction.
func (b *TxBuilder) buildNativeTransfer(ctx context.Context, req *network.TxBuildRequest) (*network.UnsignedTx, error) {
	// Parse the payload
	payload, err := ParseNativeTransferPayload(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	// Get the amount
	amount, err := payload.GetAmount()
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %w", err)
	}

	// Get transaction data (if any)
	data, err := payload.GetDataBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to parse data: %w", err)
	}

	// Get nonce
	nonce, err := b.getNonce(ctx, req.Sender)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Parse gas price
	gasPrice, err := b.getGasPrice(ctx, req.GasPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	// Use default gas limit for native transfers if not specified
	gasLimit := req.GasLimit
	if gasLimit == 0 {
		gasLimit = 21000 // Standard gas limit for native transfers
	}

	// Get recipient address
	toAddr := payload.GetToAddress()

	// Create the legacy transaction
	tx := types.NewTransaction(
		nonce,
		toAddr,
		amount,
		gasLimit,
		gasPrice,
		data,
	)

	// Encode transaction with RLP for TxBytes
	txBytes, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to encode transaction: %w", err)
	}

	// Create the sign document using the signer hash
	signer := types.LatestSignerForChainID(b.chainID)
	signDoc := signer.Hash(tx).Bytes()

	return &network.UnsignedTx{
		TxBytes:       txBytes,
		SignDoc:       signDoc,
		AccountNumber: 0, // Not applicable for EVM
		Sequence:      nonce,
	}, nil
}

// getNonce gets the next nonce for the given address.
// If a client is available, it queries the pending nonce from the network.
// Otherwise, it returns 0 (for testing purposes).
func (b *TxBuilder) getNonce(ctx context.Context, address string) (uint64, error) {
	if b.client == nil {
		// For testing without a client
		return 0, nil
	}

	addr := common.HexToAddress(address)
	nonce, err := b.client.PendingNonceAt(ctx, addr)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending nonce: %w", err)
	}
	return nonce, nil
}

// getGasPrice returns the gas price for the transaction.
// If a gas price string is provided in the request, it parses it.
// Otherwise, it queries the network for a suggested gas price.
// If no client is available, it returns a default gas price.
func (b *TxBuilder) getGasPrice(ctx context.Context, gasPriceStr string) (*big.Int, error) {
	// If gas price is provided, parse it
	if gasPriceStr != "" {
		gasPrice, err := ParseAmount(gasPriceStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse gas price: %w", err)
		}
		return gasPrice, nil
	}

	// If we have a client, query the suggested gas price
	if b.client != nil {
		gasPrice, err := b.client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get suggested gas price: %w", err)
		}
		return gasPrice, nil
	}

	// Default gas price for testing (1 Gwei)
	return big.NewInt(DefaultGasPriceGwei), nil
}

// SignTx signs an EVM transaction.
func (b *TxBuilder) SignTx(ctx context.Context, unsignedTx *network.UnsignedTx, key *network.SigningKey) (*network.SignedTx, error) {
	if b == nil {
		return nil, fmt.Errorf("nil TxBuilder")
	}
	if unsignedTx == nil {
		return nil, fmt.Errorf("unsigned transaction is required")
	}
	if len(unsignedTx.SignDoc) == 0 {
		return nil, fmt.Errorf("sign document is required")
	}
	if key == nil {
		return nil, fmt.Errorf("signing key is required")
	}
	if len(key.PrivKey) == 0 {
		return nil, fmt.Errorf("private key required")
	}

	// Decode the unsigned transaction from bytes
	var tx types.Transaction
	if err := rlp.DecodeBytes(unsignedTx.TxBytes, &tx); err != nil {
		return nil, fmt.Errorf("decode unsigned transaction: %w", err)
	}

	// Load ECDSA private key
	privKey, err := crypto.ToECDSA(key.PrivKey)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}

	// Create signer for the chain
	signer := types.LatestSignerForChainID(b.chainID)

	// Sign the transaction (this returns a new signed transaction with signature embedded)
	signedTx, err := types.SignTx(&tx, signer, privKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// Re-encode the signed transaction (now includes signature)
	signedTxBytes, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return nil, fmt.Errorf("encode signed transaction: %w", err)
	}

	// Extract signature components for storage (useful for debugging/verification)
	v, r, s := signedTx.RawSignatureValues()
	signature := make([]byte, 65)
	r.FillBytes(signature[0:32])
	s.FillBytes(signature[32:64])
	signature[64] = byte(v.Uint64())

	// Get public key bytes
	pubKeyBytes := crypto.FromECDSAPub(&privKey.PublicKey)

	return &network.SignedTx{
		TxBytes:   signedTxBytes, // Now properly includes the signature
		Signature: signature,
		PubKey:    pubKeyBytes,
	}, nil
}

// BroadcastTx submits a signed EVM transaction.
func (b *TxBuilder) BroadcastTx(ctx context.Context, signedTx *network.SignedTx) (*network.TxBroadcastResult, error) {
	if b == nil {
		return nil, fmt.Errorf("nil TxBuilder")
	}
	if signedTx == nil {
		return nil, fmt.Errorf("signed transaction is required")
	}
	if len(signedTx.TxBytes) == 0 {
		return nil, fmt.Errorf("transaction bytes are required")
	}

	// Use default HTTP client if not set
	httpClient := b.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	// Encode as hex with 0x prefix
	rawTxHex := "0x" + hex.EncodeToString(signedTx.TxBytes)

	// Create JSON-RPC request
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "eth_sendRawTransaction",
		"params":  []string{rawTxHex},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.rpcEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("broadcast: %w", err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return &network.TxBroadcastResult{
			Code: uint32(rpcResp.Error.Code),
			Log:  rpcResp.Error.Message,
		}, nil
	}

	return &network.TxBroadcastResult{
		TxHash: rpcResp.Result,
		Code:   0,
	}, nil
}

// Close closes the ethclient connection.
// It is safe to call on a nil receiver and is idempotent.
func (b *TxBuilder) Close() {
	if b == nil {
		return
	}
	if b.client != nil {
		b.client.Close()
	}
}

// RPCEndpoint returns the configured RPC endpoint.
func (b *TxBuilder) RPCEndpoint() string {
	if b == nil {
		return ""
	}
	return b.rpcEndpoint
}

// ChainID returns a copy of the configured chain ID.
// Returns nil if the builder is nil.
func (b *TxBuilder) ChainID() *big.Int {
	if b == nil || b.chainID == nil {
		return nil
	}
	return new(big.Int).Set(b.chainID)
}
