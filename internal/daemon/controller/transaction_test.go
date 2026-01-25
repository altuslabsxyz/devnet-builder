// internal/daemon/controller/transaction_test.go
package controller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

// mockTxRuntime implements TxRuntime for testing.
type mockTxRuntime struct {
	builder    network.TxBuilder
	signingKey *network.SigningKey
	receipt    *TxReceipt
	builderErr error
	keyErr     error
	confirmErr error
}

func (m *mockTxRuntime) GetTxBuilder(ctx context.Context, devnetName string) (network.TxBuilder, error) {
	if m.builderErr != nil {
		return nil, m.builderErr
	}
	return m.builder, nil
}

func (m *mockTxRuntime) GetSigningKey(ctx context.Context, devnetName string, signer string) (*network.SigningKey, error) {
	if m.keyErr != nil {
		return nil, m.keyErr
	}
	return m.signingKey, nil
}

func (m *mockTxRuntime) WaitForConfirmation(ctx context.Context, devnetName string, txHash string) (*TxReceipt, error) {
	if m.confirmErr != nil {
		return nil, m.confirmErr
	}
	return m.receipt, nil
}

func TestTxController_Reconcile_PendingToBuilding(t *testing.T) {
	ms := store.NewMemoryStore()
	runtime := &mockTxRuntime{
		builder:    plugin.NewMockTxBuilder(),
		signingKey: &network.SigningKey{Address: "cosmos1abc"},
		receipt:    &TxReceipt{TxHash: "abc123", Height: 100, Success: true},
	}
	tc := NewTxController(ms, runtime)

	// Create a pending transaction
	tx := &types.Transaction{
		Metadata: types.ResourceMeta{
			Name:      "tx-test-1",
			CreatedAt: time.Now(),
		},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
			Signer:    "validator:0",
			Payload:   json.RawMessage(`{"proposal_id":1}`),
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhasePending,
		},
	}
	if err := ms.CreateTransaction(context.Background(), tx); err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	// Reconcile - should transition to Building
	err := tc.Reconcile(context.Background(), "tx-test-1")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition
	got, _ := ms.GetTransaction(context.Background(), "tx-test-1")
	if got.Status.Phase != types.TxPhaseBuilding {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.TxPhaseBuilding)
	}
}

func TestTxController_Reconcile_FullCycle(t *testing.T) {
	ms := store.NewMemoryStore()
	runtime := &mockTxRuntime{
		builder:    plugin.NewMockTxBuilder(),
		signingKey: &network.SigningKey{Address: "cosmos1abc"},
		receipt:    &TxReceipt{TxHash: "abc123", Height: 100, Success: true},
	}
	tc := NewTxController(ms, runtime)

	// Create a pending transaction
	tx := &types.Transaction{
		Metadata: types.ResourceMeta{
			Name:      "tx-full-1",
			CreatedAt: time.Now(),
		},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
			Signer:    "validator:0",
			Payload:   json.RawMessage(`{"proposal_id":1}`),
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhasePending,
		},
	}
	ms.CreateTransaction(context.Background(), tx)

	// Reconcile through all phases
	phases := []string{
		types.TxPhaseBuilding,
		types.TxPhaseSigning,
		types.TxPhaseSubmitted,
		types.TxPhaseConfirmed,
	}

	for _, expectedPhase := range phases {
		err := tc.Reconcile(context.Background(), "tx-full-1")
		if err != nil {
			t.Fatalf("Reconcile to %s: %v", expectedPhase, err)
		}

		got, _ := ms.GetTransaction(context.Background(), "tx-full-1")
		if got.Status.Phase != expectedPhase {
			t.Errorf("Phase = %q, want %q", got.Status.Phase, expectedPhase)
		}
	}

	// Verify final state
	got, _ := ms.GetTransaction(context.Background(), "tx-full-1")
	if got.Status.TxHash == "" {
		t.Error("TxHash is empty")
	}
	if got.Status.Height == 0 {
		t.Error("Height is 0")
	}
}

func TestTxController_Reconcile_BuildError(t *testing.T) {
	ms := store.NewMemoryStore()
	builder := plugin.NewMockTxBuilder()
	builder.BuildErr = errors.New("build failed")
	runtime := &mockTxRuntime{builder: builder}
	tc := NewTxController(ms, runtime)

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-err-1"},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
			Signer:    "validator:0",
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhaseBuilding,
		},
	}
	ms.CreateTransaction(context.Background(), tx)

	// Reconcile - should fail
	tc.Reconcile(context.Background(), "tx-err-1")

	// Verify Failed state
	got, _ := ms.GetTransaction(context.Background(), "tx-err-1")
	if got.Status.Phase != types.TxPhaseFailed {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.TxPhaseFailed)
	}
	if got.Status.Error == "" {
		t.Error("Error message is empty")
	}
}

// TestTxController_ConcurrentReconciliation tests that concurrent reconciliation
// of the same transaction is safe (tests mutex protection).
func TestTxController_ConcurrentReconciliation(t *testing.T) {
	ms := store.NewMemoryStore()
	runtime := &mockTxRuntime{
		builder:    plugin.NewMockTxBuilder(),
		signingKey: &network.SigningKey{Address: "cosmos1abc", PrivKey: []byte("test")},
		receipt:    &TxReceipt{TxHash: "abc123", Height: 100, Success: true},
	}
	tc := NewTxController(ms, runtime)

	// Create transaction in Building phase (triggers cache write)
	tx := &types.Transaction{
		Metadata: types.ResourceMeta{
			Name:      "tx-concurrent-1",
			CreatedAt: time.Now(),
		},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
			Signer:    "validator:0",
			Payload:   json.RawMessage(`{"proposal_id":1,"vote_option":"yes"}`),
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhaseBuilding,
		},
	}
	if err := ms.CreateTransaction(context.Background(), tx); err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	// Run 10 concurrent reconciliations (tests race detector)
	const goroutines = 10
	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			errCh <- tc.Reconcile(context.Background(), "tx-concurrent-1")
		}()
	}

	// Collect results
	for i := 0; i < goroutines; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Reconcile goroutine %d failed: %v", i, err)
		}
	}

	// Verify final state is consistent
	got, err := ms.GetTransaction(context.Background(), "tx-concurrent-1")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}

	// Should have progressed past Building (cache operations succeeded)
	if got.Status.Phase == types.TxPhaseBuilding {
		t.Error("Transaction stuck in Building phase - possible concurrency issue")
	}
}

// TestTxController_ContextCancellation_BeforeReconcile tests that context
// cancellation is detected before reconciliation starts.
func TestTxController_ContextCancellation_BeforeReconcile(t *testing.T) {
	ms := store.NewMemoryStore()
	runtime := &mockTxRuntime{
		builder: plugin.NewMockTxBuilder(),
	}
	tc := NewTxController(ms, runtime)

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-cancel-1"},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhasePending,
		},
	}
	ms.CreateTransaction(context.Background(), tx)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Reconcile should fail with context error
	err := tc.Reconcile(ctx, "tx-cancel-1")
	if err == nil {
		t.Error("Expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// TestTxController_ContextCancellation_DuringBuilding tests that context
// cancellation is detected during the building phase.
func TestTxController_ContextCancellation_DuringBuilding(t *testing.T) {
	ms := store.NewMemoryStore()
	runtime := &mockTxRuntime{
		builder: plugin.NewMockTxBuilder(),
	}
	tc := NewTxController(ms, runtime)

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-cancel-2"},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
			Payload:   json.RawMessage(`{"proposal_id":1,"vote_option":"yes"}`),
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhaseBuilding,
		},
	}
	ms.CreateTransaction(context.Background(), tx)

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Reconcile should detect cancellation
	err := tc.Reconcile(ctx, "tx-cancel-2")
	if err == nil {
		t.Error("Expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// TestTxController_ContextDeadline tests timeout handling.
func TestTxController_ContextDeadline(t *testing.T) {
	ms := store.NewMemoryStore()
	runtime := &mockTxRuntime{
		builder: plugin.NewMockTxBuilder(),
	}
	tc := NewTxController(ms, runtime)

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-deadline-1"},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhasePending,
		},
	}
	ms.CreateTransaction(context.Background(), tx)

	// Create context with deadline in the past
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	// Reconcile should fail with deadline exceeded
	err := tc.Reconcile(ctx, "tx-deadline-1")
	if err == nil {
		t.Error("Expected error from deadline exceeded, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
	}
}

// memoCapturingBuilder captures BuildTx parameters for verification.
type memoCapturingBuilder struct {
	plugin.MockTxBuilder
	capturedMemo     string
	capturedGasLimit uint64
}

func (m *memoCapturingBuilder) BuildTx(ctx context.Context, req *network.TxBuildRequest) (*network.UnsignedTx, error) {
	// Capture the fields we're testing
	m.capturedMemo = req.Memo
	m.capturedGasLimit = req.GasLimit
	// Delegate to parent for actual mock behavior
	return m.MockTxBuilder.BuildTx(ctx, req)
}

// TestTxController_MemoPropagation verifies that Memo and GasLimit fields
// are properly propagated from TransactionSpec to TxBuildRequest.
func TestTxController_MemoPropagation(t *testing.T) {
	ms := store.NewMemoryStore()
	mockBuilder := &memoCapturingBuilder{
		MockTxBuilder: *plugin.NewMockTxBuilder(),
	}
	runtime := &mockTxRuntime{
		builder:    mockBuilder,
		signingKey: &network.SigningKey{Address: "cosmos1abc", PrivKey: []byte("test")},
		receipt:    &TxReceipt{TxHash: "abc123", Height: 100, Success: true},
	}
	tc := NewTxController(ms, runtime)

	// Create transaction with Memo and GasLimit
	tx := &types.Transaction{
		Metadata: types.ResourceMeta{
			Name:      "tx-memo-test",
			CreatedAt: time.Now(),
		},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
			Signer:    "validator:0",
			Payload:   json.RawMessage(`{"proposal_id":1,"vote_option":"yes"}`),
			Memo:      "reward-tag:pool-alpha",
			GasLimit:  300000,
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhaseBuilding,
		},
	}
	if err := ms.CreateTransaction(context.Background(), tx); err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	// Reconcile - triggers BuildTx
	err := tc.Reconcile(context.Background(), "tx-memo-test")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify Memo and GasLimit were passed to BuildTx
	if mockBuilder.capturedMemo != "reward-tag:pool-alpha" {
		t.Errorf("Memo = %q, want %q", mockBuilder.capturedMemo, "reward-tag:pool-alpha")
	}
	if mockBuilder.capturedGasLimit != 300000 {
		t.Errorf("GasLimit = %d, want %d", mockBuilder.capturedGasLimit, 300000)
	}
}
