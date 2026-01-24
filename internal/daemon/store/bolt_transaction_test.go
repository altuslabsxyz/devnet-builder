// internal/daemon/store/bolt_transaction_test.go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestBoltStore_CreateTransaction(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-test-1"},
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

	err = s.CreateTransaction(context.Background(), tx)
	if err != nil {
		t.Fatalf("CreateTransaction: %v", err)
	}

	// Verify retrieval
	got, err := s.GetTransaction(context.Background(), "tx-test-1")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if got.Spec.TxType != "gov/vote" {
		t.Errorf("TxType = %q, want %q", got.Spec.TxType, "gov/vote")
	}
}

func TestBoltStore_CreateTransaction_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-dup-1"},
		Spec:     types.TransactionSpec{DevnetRef: "mydevnet"},
		Status:   types.TransactionStatus{Phase: types.TxPhasePending},
	}

	err = s.CreateTransaction(context.Background(), tx)
	if err != nil {
		t.Fatalf("First CreateTransaction: %v", err)
	}

	// Second create should fail
	err = s.CreateTransaction(context.Background(), tx)
	if !IsAlreadyExists(err) {
		t.Errorf("Expected AlreadyExists error, got: %v", err)
	}
}

func TestBoltStore_UpdateTransaction(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-update-1"},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "gov/vote",
		},
		Status: types.TransactionStatus{
			Phase: types.TxPhasePending,
		},
	}
	s.CreateTransaction(context.Background(), tx)

	// Update
	tx.Status.Phase = types.TxPhaseSubmitted
	tx.Status.TxHash = "abc123"
	err = s.UpdateTransaction(context.Background(), tx)
	if err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}

	// Verify
	got, _ := s.GetTransaction(context.Background(), "tx-update-1")
	if got.Status.Phase != types.TxPhaseSubmitted {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.TxPhaseSubmitted)
	}
	if got.Status.TxHash != "abc123" {
		t.Errorf("TxHash = %q, want %q", got.Status.TxHash, "abc123")
	}
}

func TestBoltStore_UpdateTransaction_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-nonexistent"},
		Status:   types.TransactionStatus{Phase: types.TxPhasePending},
	}

	err = s.UpdateTransaction(context.Background(), tx)
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error, got: %v", err)
	}
}

func TestBoltStore_ListTransactions(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create transactions for two devnets
	for i := 0; i < 3; i++ {
		tx := &types.Transaction{
			Metadata: types.ResourceMeta{Name: fmt.Sprintf("tx-devnet1-%d", i)},
			Spec:     types.TransactionSpec{DevnetRef: "devnet1", TxType: "gov/vote"},
			Status:   types.TransactionStatus{Phase: types.TxPhasePending},
		}
		s.CreateTransaction(context.Background(), tx)
	}
	for i := 0; i < 2; i++ {
		tx := &types.Transaction{
			Metadata: types.ResourceMeta{Name: fmt.Sprintf("tx-devnet2-%d", i)},
			Spec:     types.TransactionSpec{DevnetRef: "devnet2", TxType: "bank/send"},
			Status:   types.TransactionStatus{Phase: types.TxPhaseConfirmed},
		}
		s.CreateTransaction(context.Background(), tx)
	}

	// List for devnet1
	txs, err := s.ListTransactions(context.Background(), "devnet1", ListTxOptions{})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txs) != 3 {
		t.Errorf("len(txs) = %d, want 3", len(txs))
	}

	// List with type filter
	txs, err = s.ListTransactions(context.Background(), "devnet2", ListTxOptions{TxType: "bank/send"})
	if err != nil {
		t.Fatalf("ListTransactions with filter: %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("len(txs) = %d, want 2", len(txs))
	}

	// List with phase filter
	txs, err = s.ListTransactions(context.Background(), "devnet1", ListTxOptions{Phase: types.TxPhasePending})
	if err != nil {
		t.Fatalf("ListTransactions with phase filter: %v", err)
	}
	if len(txs) != 3 {
		t.Errorf("len(txs) = %d, want 3", len(txs))
	}

	// List with limit
	txs, err = s.ListTransactions(context.Background(), "devnet1", ListTxOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListTransactions with limit: %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("len(txs) = %d, want 2", len(txs))
	}
}

func TestBoltStore_DeleteTransaction(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-delete-1"},
		Spec:     types.TransactionSpec{DevnetRef: "mydevnet"},
		Status:   types.TransactionStatus{Phase: types.TxPhasePending},
	}
	s.CreateTransaction(context.Background(), tx)

	// Delete
	err = s.DeleteTransaction(context.Background(), "tx-delete-1")
	if err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}

	// Verify it's gone
	_, err = s.GetTransaction(context.Background(), "tx-delete-1")
	if !IsNotFound(err) {
		t.Errorf("Expected NotFound after delete, got: %v", err)
	}
}

func TestBoltStore_DeleteTransactionsByDevnet(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create transactions for two devnets
	for i := 0; i < 3; i++ {
		tx := &types.Transaction{
			Metadata: types.ResourceMeta{Name: fmt.Sprintf("tx-del-devnet1-%d", i)},
			Spec:     types.TransactionSpec{DevnetRef: "devnet1"},
			Status:   types.TransactionStatus{Phase: types.TxPhasePending},
		}
		s.CreateTransaction(context.Background(), tx)
	}
	for i := 0; i < 2; i++ {
		tx := &types.Transaction{
			Metadata: types.ResourceMeta{Name: fmt.Sprintf("tx-del-devnet2-%d", i)},
			Spec:     types.TransactionSpec{DevnetRef: "devnet2"},
			Status:   types.TransactionStatus{Phase: types.TxPhasePending},
		}
		s.CreateTransaction(context.Background(), tx)
	}

	// Delete devnet1's transactions
	err = s.DeleteTransactionsByDevnet(context.Background(), "devnet1")
	if err != nil {
		t.Fatalf("DeleteTransactionsByDevnet: %v", err)
	}

	// Verify devnet1 transactions are gone
	txs, _ := s.ListTransactions(context.Background(), "devnet1", ListTxOptions{})
	if len(txs) != 0 {
		t.Errorf("devnet1 txs = %d, want 0", len(txs))
	}

	// Verify devnet2 transactions still exist
	txs, _ = s.ListTransactions(context.Background(), "devnet2", ListTxOptions{})
	if len(txs) != 2 {
		t.Errorf("devnet2 txs = %d, want 2", len(txs))
	}
}
