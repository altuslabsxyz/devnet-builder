// pkg/network/evm/txbuilder.go
package evm

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

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
	return big.NewInt(1000000000), nil
}

// SignTx signs an unsigned transaction with the provided key.
// This is a placeholder implementation that returns "not implemented" error.
func (b *TxBuilder) SignTx(ctx context.Context, unsignedTx *network.UnsignedTx, key *network.SigningKey) (*network.SignedTx, error) {
	if b == nil {
		return nil, fmt.Errorf("nil TxBuilder")
	}
	return nil, fmt.Errorf("SignTx: not implemented")
}

// BroadcastTx submits a signed transaction to the network.
// This is a placeholder implementation that returns "not implemented" error.
func (b *TxBuilder) BroadcastTx(ctx context.Context, tx *network.SignedTx) (*network.TxBroadcastResult, error) {
	if b == nil {
		return nil, fmt.Errorf("nil TxBuilder")
	}
	return nil, fmt.Errorf("BroadcastTx: not implemented")
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
