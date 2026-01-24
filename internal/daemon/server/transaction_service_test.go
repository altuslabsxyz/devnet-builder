// internal/daemon/server/transaction_service_test.go
package server

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestTransactionService_SubmitTransaction(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewTransactionService(ms, nil)

	resp, err := svc.SubmitTransaction(context.Background(), &v1.SubmitTransactionRequest{
		Devnet:  "mydevnet",
		TxType:  "gov/vote",
		Signer:  "validator:0",
		Payload: []byte(`{"proposal_id":1,"option":"yes"}`),
	})
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}

	if resp.Phase != types.TxPhasePending {
		t.Errorf("Phase = %q, want %q", resp.Phase, types.TxPhasePending)
	}
	if resp.Name == "" {
		t.Error("Name is empty")
	}
}

func TestTransactionService_GetTransaction(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewTransactionService(ms, nil)

	// Create a transaction
	tx := &types.Transaction{
		Metadata: types.ResourceMeta{Name: "tx-get-1"},
		Spec: types.TransactionSpec{
			DevnetRef: "mydevnet",
			TxType:    "bank/send",
		},
		Status: types.TransactionStatus{Phase: types.TxPhaseConfirmed},
	}
	ms.CreateTransaction(context.Background(), tx)

	// Get via service
	resp, err := svc.GetTransaction(context.Background(), &v1.GetTransactionRequest{
		Name: "tx-get-1",
	})
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if resp.TxType != "bank/send" {
		t.Errorf("TxType = %q, want %q", resp.TxType, "bank/send")
	}
}

func TestTransactionService_ListTransactions(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewTransactionService(ms, nil)

	// Create transactions
	for i := 0; i < 3; i++ {
		tx := &types.Transaction{
			Metadata: types.ResourceMeta{Name: fmt.Sprintf("tx-list-%d", i)},
			Spec:     types.TransactionSpec{DevnetRef: "mydevnet", TxType: "gov/vote"},
			Status:   types.TransactionStatus{Phase: types.TxPhasePending},
		}
		ms.CreateTransaction(context.Background(), tx)
	}

	resp, err := svc.ListTransactions(context.Background(), &v1.ListTransactionsRequest{
		Devnet: "mydevnet",
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(resp.Transactions) != 3 {
		t.Errorf("len(Transactions) = %d, want 3", len(resp.Transactions))
	}
}

func TestTransactionService_SubmitGovVote(t *testing.T) {
	ms := store.NewMemoryStore()
	svc := NewTransactionService(ms, nil)

	resp, err := svc.SubmitGovVote(context.Background(), &v1.SubmitGovVoteRequest{
		Devnet:     "mydevnet",
		ProposalId: 1,
		VoteOption: "yes",
		Voter:      "validator:0",
	})
	if err != nil {
		t.Fatalf("SubmitGovVote: %v", err)
	}

	if resp.TxType != "gov/vote" {
		t.Errorf("TxType = %q, want %q", resp.TxType, "gov/vote")
	}
}
