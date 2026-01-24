// pkg/network/evm/txbuilder.go
package evm

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

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
// This is a placeholder implementation that returns "not implemented" error.
func (b *TxBuilder) BuildTx(ctx context.Context, req *network.TxBuildRequest) (*network.UnsignedTx, error) {
	if b == nil {
		return nil, fmt.Errorf("nil TxBuilder")
	}
	return nil, fmt.Errorf("BuildTx: not implemented")
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
