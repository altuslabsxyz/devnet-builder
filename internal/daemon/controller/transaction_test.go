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
