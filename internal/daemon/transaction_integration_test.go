//go:build integration

// internal/daemon/transaction_integration_test.go
package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionLifecycle(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "devnetd-tx-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "devnetd.sock")

	// Create and start server
	cfg := &server.Config{
		SocketPath: socketPath,
		DataDir:    tmpDir,
		Foreground: true,
		Workers:    1,
		LogLevel:   "error", // Quiet logs for tests
	}

	srv, err := server.New(cfg)
	require.NoError(t, err)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()

	// Wait for server to be ready
	require.Eventually(t, func() bool {
		return client.IsDaemonRunningAt(socketPath)
	}, 5*time.Second, 100*time.Millisecond, "server should be ready")

	// Connect client
	c, err := client.NewWithSocket(socketPath)
	require.NoError(t, err)
	defer c.Close()

	// First create a devnet (required for transaction operations)
	spec := &v1.DevnetSpec{
		Plugin:     "stable",
		Validators: 4,
		Mode:       "docker",
	}
	devnet, err := c.CreateDevnet(ctx, "tx-test-devnet", spec, nil)
	require.NoError(t, err)
	assert.Equal(t, "tx-test-devnet", devnet.Metadata.Name)

	var createdTxName string

	// Test: Submit transaction
	t.Run("SubmitTransaction", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"proposal_id": 1,
			"option":      "yes",
		})

		tx, err := c.SubmitTransaction(ctx, "tx-test-devnet", "gov/vote", "validator:0", payload)
		require.NoError(t, err)
		assert.NotEmpty(t, tx.Name)
		assert.Equal(t, "tx-test-devnet", tx.DevnetRef)
		assert.Equal(t, "gov/vote", tx.TxType)
		assert.Equal(t, "validator:0", tx.Signer)
		assert.Equal(t, "Pending", tx.Phase)

		createdTxName = tx.Name
	})

	// Test: Get transaction
	t.Run("GetTransaction", func(t *testing.T) {
		tx, err := c.GetTransaction(ctx, createdTxName)
		require.NoError(t, err)
		assert.Equal(t, createdTxName, tx.Name)
		assert.Equal(t, "gov/vote", tx.TxType)
	})

	// Test: Get non-existent transaction
	t.Run("GetTransactionNotFound", func(t *testing.T) {
		_, err := c.GetTransaction(ctx, "non-existent-tx")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	// Test: List transactions
	t.Run("ListTransactions", func(t *testing.T) {
		// Submit another transaction
		payload, _ := json.Marshal(map[string]interface{}{
			"to":     "cosmos1xyz",
			"amount": 1000,
		})
		_, err := c.SubmitTransaction(ctx, "tx-test-devnet", "bank/send", "validator:1", payload)
		require.NoError(t, err)

		// List all transactions
		txs, err := c.ListTransactions(ctx, "tx-test-devnet", "", "", 100)
		require.NoError(t, err)
		assert.Len(t, txs, 2)
	})

	// Test: List transactions with type filter
	t.Run("ListTransactionsWithTypeFilter", func(t *testing.T) {
		txs, err := c.ListTransactions(ctx, "tx-test-devnet", "gov/vote", "", 100)
		require.NoError(t, err)
		assert.Len(t, txs, 1)
		assert.Equal(t, "gov/vote", txs[0].TxType)
	})

	// Test: List transactions with phase filter
	t.Run("ListTransactionsWithPhaseFilter", func(t *testing.T) {
		txs, err := c.ListTransactions(ctx, "tx-test-devnet", "", "Pending", 100)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(txs), 1)
		for _, tx := range txs {
			assert.Equal(t, "Pending", tx.Phase)
		}
	})

	// Test: List transactions with limit
	t.Run("ListTransactionsWithLimit", func(t *testing.T) {
		txs, err := c.ListTransactions(ctx, "tx-test-devnet", "", "", 1)
		require.NoError(t, err)
		assert.Len(t, txs, 1)
	})

	// Test: Cancel transaction
	t.Run("CancelTransaction", func(t *testing.T) {
		// Submit a new transaction to cancel
		tx, err := c.SubmitTransaction(ctx, "tx-test-devnet", "staking/delegate", "validator:2", nil)
		require.NoError(t, err)
		assert.Equal(t, "Pending", tx.Phase)

		// Cancel it
		cancelled, err := c.CancelTransaction(ctx, tx.Name)
		require.NoError(t, err)
		assert.Equal(t, "Failed", cancelled.Phase)
		assert.Equal(t, "Cancelled by user", cancelled.Error)
	})

	// Test: Cancel non-existent transaction
	t.Run("CancelTransactionNotFound", func(t *testing.T) {
		_, err := c.CancelTransaction(ctx, "non-existent-tx")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	// Shutdown server
	cancel()

	// Wait for server to stop (or timeout)
	select {
	case <-errCh:
		// Server stopped
	case <-time.After(5 * time.Second):
		t.Log("Server shutdown timed out")
	}
}

func TestGovTransactionHelpers(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "devnetd-gov-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "devnetd.sock")

	// Create and start server
	cfg := &server.Config{
		SocketPath: socketPath,
		DataDir:    tmpDir,
		Foreground: true,
		Workers:    1,
		LogLevel:   "error",
	}

	srv, err := server.New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return client.IsDaemonRunningAt(socketPath)
	}, 5*time.Second, 100*time.Millisecond)

	c, err := client.NewWithSocket(socketPath)
	require.NoError(t, err)
	defer c.Close()

	// Create devnet
	spec := &v1.DevnetSpec{
		Plugin:     "stable",
		Validators: 4,
	}
	_, err = c.CreateDevnet(ctx, "gov-test-devnet", spec, nil)
	require.NoError(t, err)

	// Test: SubmitGovVote
	t.Run("SubmitGovVote", func(t *testing.T) {
		tx, err := c.SubmitGovVote(ctx, "gov-test-devnet", 1, "validator:0", "yes")
		require.NoError(t, err)
		assert.Equal(t, "gov/vote", tx.TxType)
		assert.Equal(t, "validator:0", tx.Signer)
		assert.Equal(t, "gov-test-devnet", tx.DevnetRef)

		// Verify payload contains vote info
		var payload map[string]interface{}
		err = json.Unmarshal(tx.Payload, &payload)
		require.NoError(t, err)
		assert.Equal(t, float64(1), payload["proposal_id"])
		assert.Equal(t, "yes", payload["option"])
	})

	// Test: SubmitGovProposal
	t.Run("SubmitGovProposal", func(t *testing.T) {
		content, _ := json.Marshal(map[string]interface{}{
			"changes": []map[string]string{
				{"subspace": "staking", "key": "MaxValidators", "value": "200"},
			},
		})

		tx, err := c.SubmitGovProposal(ctx, "gov-test-devnet", "validator:0", "params", "Increase MaxValidators", "Test description", content)
		require.NoError(t, err)
		assert.Equal(t, "gov/proposal", tx.TxType)
		assert.Equal(t, "validator:0", tx.Signer)

		// Verify payload contains proposal info
		var payload map[string]interface{}
		err = json.Unmarshal(tx.Payload, &payload)
		require.NoError(t, err)
		assert.Equal(t, "params", payload["type"])
		assert.Equal(t, "Increase MaxValidators", payload["title"])
	})

	// Shutdown
	cancel()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Log("Server shutdown timed out")
	}
}

func TestTransactionDevnetScope(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "devnetd-txscope-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "devnetd.sock")

	cfg := &server.Config{
		SocketPath: socketPath,
		DataDir:    tmpDir,
		Foreground: true,
		Workers:    1,
		LogLevel:   "error",
	}

	srv, err := server.New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return client.IsDaemonRunningAt(socketPath)
	}, 5*time.Second, 100*time.Millisecond)

	c, err := client.NewWithSocket(socketPath)
	require.NoError(t, err)
	defer c.Close()

	// Create two devnets
	spec := &v1.DevnetSpec{Plugin: "stable", Validators: 2}
	_, err = c.CreateDevnet(ctx, "devnet-a", spec, nil)
	require.NoError(t, err)
	_, err = c.CreateDevnet(ctx, "devnet-b", spec, nil)
	require.NoError(t, err)

	// Submit transactions to each devnet
	_, err = c.SubmitTransaction(ctx, "devnet-a", "bank/send", "user:a1", nil)
	require.NoError(t, err)
	_, err = c.SubmitTransaction(ctx, "devnet-a", "bank/send", "user:a2", nil)
	require.NoError(t, err)
	_, err = c.SubmitTransaction(ctx, "devnet-b", "staking/delegate", "user:b1", nil)
	require.NoError(t, err)

	// Test: List transactions shows only devnet-a transactions
	t.Run("ListTransactionsDevnetA", func(t *testing.T) {
		txs, err := c.ListTransactions(ctx, "devnet-a", "", "", 100)
		require.NoError(t, err)
		assert.Len(t, txs, 2)
		for _, tx := range txs {
			assert.Equal(t, "devnet-a", tx.DevnetRef)
		}
	})

	// Test: List transactions shows only devnet-b transactions
	t.Run("ListTransactionsDevnetB", func(t *testing.T) {
		txs, err := c.ListTransactions(ctx, "devnet-b", "", "", 100)
		require.NoError(t, err)
		assert.Len(t, txs, 1)
		assert.Equal(t, "devnet-b", txs[0].DevnetRef)
		assert.Equal(t, "staking/delegate", txs[0].TxType)
	})

	// Shutdown
	cancel()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Log("Server shutdown timed out")
	}
}
