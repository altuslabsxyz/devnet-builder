// pkg/network/evm/txbuilder_test.go
package evm

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/stretchr/testify/require"
)

func TestNewTxBuilder(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		RPCEndpoint: "http://localhost:8545",
		ChainID:     "2200",
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	// The test may fail if there's no running EVM node
	// In that case, we skip the test
	if err != nil {
		t.Skipf("Skipping test: could not connect to EVM node: %v", err)
	}

	require.NotNil(t, builder)
	require.Implements(t, (*network.TxBuilder)(nil), builder)

	// Clean up
	builder.Close()
}

func TestNewTxBuilder_NilConfig(t *testing.T) {
	builder, err := NewTxBuilder(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, builder)
	require.Contains(t, err.Error(), "config is required")
}

func TestNewTxBuilder_MissingRPCEndpoint(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		ChainID: "2200",
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	require.Error(t, err)
	require.Nil(t, builder)
	require.Contains(t, err.Error(), "RPC endpoint is required")
}

func TestNewTxBuilder_MissingChainID(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		RPCEndpoint: "http://localhost:8545",
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	require.Error(t, err)
	require.Nil(t, builder)
	require.Contains(t, err.Error(), "chain ID is required")
}

func TestNewTxBuilder_InvalidChainID(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		RPCEndpoint: "http://localhost:8545",
		ChainID:     "not-a-number",
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	require.Error(t, err)
	require.Nil(t, builder)
	require.Contains(t, err.Error(), "invalid chain ID")
}

func TestSupportedTxTypes(t *testing.T) {
	builder := &TxBuilder{}
	types := builder.SupportedTxTypes()
	require.Contains(t, types, network.TxTypeBankSend)
}

func TestTxBuilder_BuildTx_NilRequest(t *testing.T) {
	builder := &TxBuilder{}
	_, err := builder.BuildTx(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "request is required")
}

func TestTxBuilder_SignTx_NoPrivateKey(t *testing.T) {
	builder := &TxBuilder{}
	key := &network.SigningKey{
		Address: "0x1234567890123456789012345678901234567890",
		PrivKey: nil, // No private key
	}
	unsignedTx := &network.UnsignedTx{
		SignDoc: []byte("test sign doc"), // Provide SignDoc so we get to private key check
	}
	_, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private key required")
}

func TestTxBuilder_BroadcastTx_NilBuilder(t *testing.T) {
	var builder *TxBuilder
	_, err := builder.BroadcastTx(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil TxBuilder")
}

func TestTxBuilder_InterfaceCompliance(t *testing.T) {
	// Compile-time check that TxBuilder implements network.TxBuilder
	var _ network.TxBuilder = (*TxBuilder)(nil)
}
