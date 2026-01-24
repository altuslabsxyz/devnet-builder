// pkg/network/cosmos/txbuilder.go
package cosmos

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
// This is a stub implementation that will be completed in a later task.
func (b *TxBuilder) BuildTx(ctx context.Context, req *network.TxBuildRequest) (*network.UnsignedTx, error) {
	return nil, fmt.Errorf("BuildTx not yet implemented")
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
