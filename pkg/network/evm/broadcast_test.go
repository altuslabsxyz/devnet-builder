// pkg/network/evm/broadcast_test.go
package evm

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/stretchr/testify/require"
)

func TestBroadcastTx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		require.Equal(t, "eth_sendRawTransaction", req["method"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0xabc123def456",
		})
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     big.NewInt(2200),
		httpClient:  &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte{0xf8, 0x65}, // Dummy RLP
		Signature: make([]byte, 65),
		PubKey:    make([]byte, 65),
	}

	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "0xabc123def456", result.TxHash)
}
