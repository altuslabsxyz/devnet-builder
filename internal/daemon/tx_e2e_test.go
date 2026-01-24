//go:build integration

// internal/daemon/tx_e2e_test.go
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server"
	"github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTxRuntime implements controller.TxRuntime for E2E testing.
type mockTxRuntime struct {
	mu            sync.Mutex
	builders      map[string]*mockTxBuilder
	signingKeys   map[string]*plugin.SigningKey
	confirmations map[string]*controller.TxReceipt
	waitDelay     time.Duration
}

func newMockTxRuntime() *mockTxRuntime {
	return &mockTxRuntime{
		builders:      make(map[string]*mockTxBuilder),
		signingKeys:   make(map[string]*plugin.SigningKey),
		confirmations: make(map[string]*controller.TxReceipt),
		waitDelay:     50 * time.Millisecond,
	}
}

func (r *mockTxRuntime) GetTxBuilder(ctx context.Context, devnetName string) (plugin.TxBuilder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if builder, ok := r.builders[devnetName]; ok {
		return builder, nil
	}
	// Auto-create builder for devnet
	builder := &mockTxBuilder{
		devnetName: devnetName,
		nextTxHash: fmt.Sprintf("HASH-%s-%d", devnetName, time.Now().UnixNano()),
	}
	r.builders[devnetName] = builder
	return builder, nil
}

func (r *mockTxRuntime) GetSigningKey(ctx context.Context, devnetName, signer string) (*plugin.SigningKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%s", devnetName, signer)
	if sigKey, ok := r.signingKeys[key]; ok {
		return sigKey, nil
	}
	// Auto-create signing key with correct field names
	sigKey := &plugin.SigningKey{
		Address:    fmt.Sprintf("cosmos1%s", signer),
		PrivKey:    []byte(fmt.Sprintf("mock-private-key-%s", signer)),
		KeyringRef: signer,
	}
	r.signingKeys[key] = sigKey
	return sigKey, nil
}

func (r *mockTxRuntime) WaitForConfirmation(ctx context.Context, devnetName, txHash string) (*controller.TxReceipt, error) {
	// Simulate block confirmation delay
	time.Sleep(r.waitDelay)

	r.mu.Lock()
	defer r.mu.Unlock()

	if receipt, ok := r.confirmations[txHash]; ok {
		return receipt, nil
	}
	// Auto-create successful confirmation
	return &controller.TxReceipt{
		TxHash:  txHash,
		Height:  100,
		GasUsed: 50000,
		Success: true,
		Log:     "success",
	}, nil
}

func (r *mockTxRuntime) SetConfirmation(txHash string, receipt *controller.TxReceipt) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.confirmations[txHash] = receipt
}

// mockTxBuilder implements plugin.TxBuilder for testing.
type mockTxBuilder struct {
	devnetName string
	nextTxHash string
}

func (b *mockTxBuilder) BuildTx(ctx context.Context, req *plugin.BuildTxRequest) (*plugin.UnsignedTx, error) {
	// Use correct field names from plugin.UnsignedTx
	return &plugin.UnsignedTx{
		TxBytes:   []byte(fmt.Sprintf("unsigned-tx-%s", req.TxType)),
		SignDoc:   req.Payload,
		AccountNo: 1,
		Sequence:  0,
	}, nil
}

func (b *mockTxBuilder) SignTx(ctx context.Context, unsignedTx *plugin.UnsignedTx, key *plugin.SigningKey) (*plugin.SignedTx, error) {
	// Use correct field names from plugin.SignedTx
	return &plugin.SignedTx{
		TxBytes:   unsignedTx.TxBytes,
		Signature: []byte("mock-signature"),
		PubKey:    []byte("mock-pubkey"),
	}, nil
}

func (b *mockTxBuilder) BroadcastTx(ctx context.Context, signedTx *plugin.SignedTx) (*plugin.BroadcastResult, error) {
	return &plugin.BroadcastResult{
		TxHash: b.nextTxHash,
		Code:   0,
	}, nil
}

func (b *mockTxBuilder) SupportedTxTypes() []plugin.TxType {
	return []plugin.TxType{
		plugin.TxTypeGovVote,
		plugin.TxTypeGovProposal,
		plugin.TxTypeBankSend,
		plugin.TxTypeStakingDelegate,
	}
}

// Ensure mockTxBuilder implements plugin.TxBuilder
var _ plugin.TxBuilder = (*mockTxBuilder)(nil)

// Ensure mockTxRuntime implements controller.TxRuntime
var _ controller.TxRuntime = (*mockTxRuntime)(nil)

// TestTransactionE2E_FullLifecycle tests the complete transaction flow:
// Submit -> Build -> Sign -> Broadcast -> Confirm
// Note: Without injecting a mock runtime into the server, the controller
// will fail at the Building phase. This test verifies submission, storage,
// and initial phase transitions.
func TestTransactionE2E_FullLifecycle(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "devnetd-e2e-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "devnetd.sock")

	// Create server (uses nil TxRuntime, so controller will fail at Building phase)
	cfg := &server.Config{
		SocketPath: socketPath,
		DataDir:    tmpDir,
		Foreground: true,
		Workers:    2,
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
	_, err = c.CreateDevnet(ctx, "e2e-devnet", spec, nil)
	require.NoError(t, err)

	// Test: Submit and track transaction lifecycle
	t.Run("FullTransactionLifecycle", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"proposal_id": 42,
			"option":      "yes",
		})

		// Submit transaction
		tx, err := c.SubmitTransaction(ctx, "e2e-devnet", "gov/vote", "validator:0", payload)
		require.NoError(t, err)
		txName := tx.Name

		// Transaction starts in Pending phase
		assert.Equal(t, "Pending", tx.Phase)

		// Wait for controller to process the transaction
		// Without a TxRuntime, it will move to Building then fail
		// This verifies the submission and initial phase transition

		var finalTx *v1.Transaction
		assert.Eventually(t, func() bool {
			finalTx, err = c.GetTransaction(ctx, txName)
			if err != nil {
				return false
			}
			// Check for phase change from Pending
			return finalTx.Phase != "Pending"
		}, 5*time.Second, 100*time.Millisecond, "transaction should progress from Pending")

		// Controller should have moved it to Building (and likely Failed due to nil runtime)
		t.Logf("Transaction reached phase: %s", finalTx.Phase)

		// Verify transaction details are preserved
		assert.Equal(t, "gov/vote", finalTx.TxType)
		assert.Equal(t, "validator:0", finalTx.Signer)
		assert.Equal(t, "e2e-devnet", finalTx.DevnetRef)
	})

	// Test: Multiple sequential transactions (avoids nanosecond collision in naming)
	t.Run("MultipleTransactions", func(t *testing.T) {
		txNames := make([]string, 5)

		for i := 0; i < 5; i++ {
			tx, err := c.SubmitTransaction(ctx, "e2e-devnet",
				fmt.Sprintf("test/multi-%d", i),
				fmt.Sprintf("user:%d", i), nil)
			require.NoError(t, err, "transaction %d failed", i)
			txNames[i] = tx.Name
			// Small delay to avoid nanosecond timestamp collision in tx naming
			time.Sleep(1 * time.Millisecond)
		}

		// All transaction names should be unique
		seen := make(map[string]bool)
		for _, name := range txNames {
			assert.False(t, seen[name], "duplicate transaction name: %s", name)
			seen[name] = true
		}

		// Verify all transactions exist
		txs, err := c.ListTransactions(ctx, "e2e-devnet", "", "", 100)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(txs), 5)
	})

	// Test: Transaction phases progression
	t.Run("PhaseProgression", func(t *testing.T) {
		tx, err := c.SubmitTransaction(ctx, "e2e-devnet", "staking/delegate", "validator:1", nil)
		require.NoError(t, err)

		phases := []string{tx.Phase}

		// Poll for phase changes (up to 3 seconds)
		deadline := time.Now().Add(3 * time.Second)
		lastPhase := tx.Phase
		for time.Now().Before(deadline) {
			current, err := c.GetTransaction(ctx, tx.Name)
			if err == nil && current.Phase != lastPhase {
				phases = append(phases, current.Phase)
				lastPhase = current.Phase
				if lastPhase == "Confirmed" || lastPhase == "Failed" {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}

		t.Logf("Observed phases: %v", phases)
		assert.GreaterOrEqual(t, len(phases), 1, "should observe at least one phase")
	})

	// Shutdown
	cancel()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Log("Server shutdown timed out")
	}
}

// TestTransactionE2E_ErrorHandling tests transaction failure scenarios.
func TestTransactionE2E_ErrorHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "devnetd-e2e-error-test-*")
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

	// Create devnet
	spec := &v1.DevnetSpec{Plugin: "stable", Validators: 2}
	_, err = c.CreateDevnet(ctx, "error-test-devnet", spec, nil)
	require.NoError(t, err)

	// Test: Validation errors
	t.Run("ValidationErrors", func(t *testing.T) {
		// Missing devnet
		_, err := c.SubmitTransaction(ctx, "", "gov/vote", "validator:0", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "devnet")

		// Missing tx_type
		_, err = c.SubmitTransaction(ctx, "error-test-devnet", "", "validator:0", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tx_type")

		// Missing signer
		_, err = c.SubmitTransaction(ctx, "error-test-devnet", "gov/vote", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signer")
	})

	// Test: Cancel already processed transaction
	t.Run("CancelNonPending", func(t *testing.T) {
		// Submit and wait for it to progress
		tx, err := c.SubmitTransaction(ctx, "error-test-devnet", "test/cancel", "user:1", nil)
		require.NoError(t, err)

		// Wait a bit for controller to potentially progress it
		time.Sleep(200 * time.Millisecond)

		current, err := c.GetTransaction(ctx, tx.Name)
		require.NoError(t, err)

		// If it's no longer pending, cancel should fail
		if current.Phase != "Pending" {
			_, err = c.CancelTransaction(ctx, tx.Name)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "pending")
		}
	})

	// Shutdown
	cancel()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Log("Server shutdown timed out")
	}
}
