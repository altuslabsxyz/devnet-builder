// pkg/network/evm/build_test.go
package evm

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/stretchr/testify/require"
)

func TestBuildTx_NativeTransfer(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:8545",
		chainID:     big.NewInt(2200),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60",
		"amount":     "1000000000000000000",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeBankSend,
		Sender:   "0x1234567890123456789012345678901234567890",
		ChainID:  "2200",
		Payload:  payload,
		GasLimit: 21000,
		GasPrice: "1000000000",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, unsignedTx)
	require.NotEmpty(t, unsignedTx.TxBytes)
	require.NotEmpty(t, unsignedTx.SignDoc)
	require.Equal(t, uint64(0), unsignedTx.Sequence) // nonce is 0 when no client
}

func TestBuildTx_NativeTransfer_WithData(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:8545",
		chainID:     big.NewInt(2200),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60",
		"amount":     "1000000000000000000",
		"data":       "0x68656c6c6f", // "hello" in hex
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeBankSend,
		Sender:   "0x1234567890123456789012345678901234567890",
		ChainID:  "2200",
		Payload:  payload,
		GasLimit: 30000,
		GasPrice: "1000000000",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, unsignedTx)
	require.NotEmpty(t, unsignedTx.TxBytes)
	require.NotEmpty(t, unsignedTx.SignDoc)
	require.Equal(t, uint64(0), unsignedTx.Sequence) // nonce is 0 when no client
}

func TestBuildTx_NilRequest(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:8545",
		chainID:     big.NewInt(2200),
	}

	unsignedTx, err := builder.BuildTx(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, unsignedTx)
	require.Contains(t, err.Error(), "request is required")
}

func TestBuildTx_UnsupportedTxType(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:8545",
		chainID:     big.NewInt(2200),
	}

	req := &network.TxBuildRequest{
		TxType:  network.TxTypeGovVote,
		Sender:  "0x1234567890123456789012345678901234567890",
		ChainID: "2200",
		Payload: json.RawMessage(`{}`),
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.Error(t, err)
	require.Nil(t, unsignedTx)
	require.Contains(t, err.Error(), "unsupported transaction type")
}

func TestBuildTx_InvalidToAddress(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:8545",
		chainID:     big.NewInt(2200),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"to_address": "invalid-address",
		"amount":     "1000000000000000000",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeBankSend,
		Sender:   "0x1234567890123456789012345678901234567890",
		ChainID:  "2200",
		Payload:  payload,
		GasLimit: 21000,
		GasPrice: "1000000000",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.Error(t, err)
	require.Nil(t, unsignedTx)
	require.Contains(t, err.Error(), "invalid to_address")
}

func TestBuildTx_InvalidAmount(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:8545",
		chainID:     big.NewInt(2200),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60",
		"amount":     "not-a-number",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeBankSend,
		Sender:   "0x1234567890123456789012345678901234567890",
		ChainID:  "2200",
		Payload:  payload,
		GasLimit: 21000,
		GasPrice: "1000000000",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.Error(t, err)
	require.Nil(t, unsignedTx)
	require.Contains(t, err.Error(), "invalid amount")
}

func TestBuildTx_DefaultGasLimit(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:8545",
		chainID:     big.NewInt(2200),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60",
		"amount":     "1000000000000000000",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeBankSend,
		Sender:   "0x1234567890123456789012345678901234567890",
		ChainID:  "2200",
		Payload:  payload,
		GasLimit: 0, // Should default to 21000
		GasPrice: "1000000000",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, unsignedTx)
	require.NotEmpty(t, unsignedTx.TxBytes)
}

// Test ParseNativeTransferPayload
func TestParseNativeTransferPayload(t *testing.T) {
	tests := []struct {
		name        string
		payload     json.RawMessage
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid payload",
			payload: json.RawMessage(`{
				"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60",
				"amount": "1000000000000000000"
			}`),
			expectError: false,
		},
		{
			name: "valid payload with data",
			payload: json.RawMessage(`{
				"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60",
				"amount": "1000000000000000000",
				"data": "0x68656c6c6f"
			}`),
			expectError: false,
		},
		{
			name: "invalid to_address",
			payload: json.RawMessage(`{
				"to_address": "not-an-address",
				"amount": "1000000000000000000"
			}`),
			expectError: true,
			errorMsg:    "invalid to_address",
		},
		{
			name: "empty to_address",
			payload: json.RawMessage(`{
				"to_address": "",
				"amount": "1000000000000000000"
			}`),
			expectError: true,
			errorMsg:    "to_address is required",
		},
		{
			name: "missing to_address",
			payload: json.RawMessage(`{
				"amount": "1000000000000000000"
			}`),
			expectError: true,
			errorMsg:    "to_address is required",
		},
		{
			name: "invalid data hex",
			payload: json.RawMessage(`{
				"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60",
				"amount": "1000000000000000000",
				"data": "0xGGGGGG"
			}`),
			expectError: true,
			errorMsg:    "invalid data",
		},
		{
			name: "missing amount",
			payload: json.RawMessage(`{
				"to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f7bD60"
			}`),
			expectError: true,
			errorMsg:    "amount is required",
		},
		{
			name:        "invalid JSON",
			payload:     json.RawMessage(`{invalid}`),
			expectError: true,
			errorMsg:    "failed to unmarshal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseNativeTransferPayload(tt.payload)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, p)
			}
		})
	}
}

// Test ParseAmount
func TestParseAmount(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *big.Int
		expectError bool
		errorMsg    string
	}{
		{
			name:     "valid amount",
			input:    "1000000000000000000",
			expected: big.NewInt(1000000000000000000),
		},
		{
			name:     "zero amount",
			input:    "0",
			expected: big.NewInt(0),
		},
		{
			name:     "small amount",
			input:    "1",
			expected: big.NewInt(1),
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			errorMsg:    "amount cannot be empty",
		},
		{
			name:        "negative amount",
			input:       "-100",
			expectError: true,
			errorMsg:    "invalid amount",
		},
		{
			name:        "non-numeric",
			input:       "abc",
			expectError: true,
			errorMsg:    "invalid amount",
		},
		{
			name:        "decimal amount",
			input:       "1.5",
			expectError: true,
			errorMsg:    "invalid amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAmount(tt.input)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, 0, tt.expected.Cmp(result))
			}
		})
	}
}
