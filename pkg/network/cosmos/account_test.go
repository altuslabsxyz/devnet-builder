// pkg/network/cosmos/account_test.go
package cosmos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryAccount(t *testing.T) {
	// Mock REST API response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/cosmos/auth/v1beta1/accounts/")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"account": {
				"@type": "/cosmos.auth.v1beta1.BaseAccount",
				"address": "cosmos1test",
				"account_number": "42",
				"sequence": "7"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	info, err := builder.QueryAccount(context.Background(), "cosmos1test")
	require.NoError(t, err)
	require.Equal(t, uint64(42), info.AccountNumber)
	require.Equal(t, uint64(7), info.Sequence)
}

func TestQueryAccount_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{
			"code": 5,
			"message": "account cosmos1notfound not found"
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	info, err := builder.QueryAccount(context.Background(), "cosmos1notfound")
	require.Error(t, err)
	require.Nil(t, info)
	require.Contains(t, err.Error(), "not found")
}

func TestQueryAccount_InvalidAddress(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:1317",
		chainID:     "test-1",
		client:      &http.Client{},
	}

	info, err := builder.QueryAccount(context.Background(), "")
	require.Error(t, err)
	require.Nil(t, info)
	require.Contains(t, err.Error(), "address is required")
}

func TestQueryAccount_ModuleAccount(t *testing.T) {
	// Test parsing a module account response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"account": {
				"@type": "/cosmos.auth.v1beta1.ModuleAccount",
				"base_account": {
					"address": "cosmos1module",
					"account_number": "100",
					"sequence": "0"
				},
				"name": "distribution",
				"permissions": ["burner"]
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	info, err := builder.QueryAccount(context.Background(), "cosmos1module")
	require.NoError(t, err)
	require.Equal(t, uint64(100), info.AccountNumber)
	require.Equal(t, uint64(0), info.Sequence)
}

func TestQueryAccount_ParsesAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"account": {
				"@type": "/cosmos.auth.v1beta1.BaseAccount",
				"address": "cosmos1abc123",
				"account_number": "1",
				"sequence": "5"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
	}

	info, err := builder.QueryAccount(context.Background(), "cosmos1abc123")
	require.NoError(t, err)
	require.Equal(t, "cosmos1abc123", info.Address)
}
