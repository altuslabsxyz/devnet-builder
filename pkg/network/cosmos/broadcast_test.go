// pkg/network/cosmos/broadcast_test.go
package cosmos

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

func TestBroadcastTx(t *testing.T) {
	// Mock RPC server that expects broadcast_tx_sync
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST to root with broadcast_tx_sync method
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/", r.URL.Path)

		// Read and verify the request body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req BroadcastRequest
		err = json.Unmarshal(body, &req)
		require.NoError(t, err)

		require.Equal(t, "2.0", req.JSONRPC)
		require.Equal(t, "broadcast_tx_sync", req.Method)
		require.NotEmpty(t, req.Params["tx"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"code": 0,
				"hash": "ABC123DEF456",
				"log": "success"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte("signed-tx-bytes"),
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "ABC123DEF456", result.TxHash)
	require.Equal(t, uint32(0), result.Code)
	require.Equal(t, "success", result.Log)
}

func TestBroadcastTx_Error(t *testing.T) {
	// Mock RPC server that returns an error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"error": {
				"code": -32600,
				"message": "Invalid Request",
				"data": "some error data"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte("signed-tx-bytes"),
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "Invalid Request")
}

func TestBroadcastTx_NonZeroCode(t *testing.T) {
	// Mock RPC server that returns a transaction rejection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"code": 5,
				"hash": "REJECTED123",
				"log": "insufficient funds"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte("signed-tx-bytes"),
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	// Non-zero code is still returned (not an error in the RPC sense)
	// The caller can check the Code field to determine success
	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "REJECTED123", result.TxHash)
	require.Equal(t, uint32(5), result.Code)
	require.Equal(t, "insufficient funds", result.Log)
}

func TestBroadcastTx_NetworkError(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:99999", // Invalid port
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte("signed-tx-bytes"),
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.Error(t, err)
	require.Nil(t, result)
}

func TestBroadcastTx_InvalidJSON(t *testing.T) {
	// Mock RPC server that returns malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte("signed-tx-bytes"),
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to parse broadcast response")
}

func TestBroadcastTx_NilSignedTx(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
	}

	result, err := builder.BroadcastTx(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "signed transaction is required")
}

func TestBroadcastTx_EmptyTxBytes(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte{},
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "transaction bytes are required")
}

func TestBroadcastTx_HTTPError(t *testing.T) {
	// Mock RPC server that returns HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte("signed-tx-bytes"),
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	result, err := builder.BroadcastTx(context.Background(), signedTx)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "unexpected status code 500")
}

func TestBroadcastTx_ContextCancellation(t *testing.T) {
	// Create a server that waits forever
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	signedTx := &network.SignedTx{
		TxBytes:   []byte("signed-tx-bytes"),
		Signature: []byte("signature"),
		PubKey:    []byte("pubkey"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := builder.BroadcastTx(ctx, signedTx)
	require.Error(t, err)
	require.Nil(t, result)
}
