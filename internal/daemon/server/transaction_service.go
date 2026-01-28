// internal/daemon/server/transaction_service.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TransactionService implements the gRPC TransactionServiceServer.
type TransactionService struct {
	v1.UnimplementedTransactionServiceServer
	store   store.Store
	manager *controller.Manager
	logger  *slog.Logger
}

// NewTransactionService creates a new TransactionService.
func NewTransactionService(s store.Store, m *controller.Manager) *TransactionService {
	return &TransactionService{
		store:   s,
		manager: m,
		logger:  slog.Default(),
	}
}

// SetLogger sets the logger.
func (s *TransactionService) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// SubmitTransaction creates and submits a new transaction.
func (s *TransactionService) SubmitTransaction(ctx context.Context, req *v1.SubmitTransactionRequest) (*v1.SubmitTransactionResponse, error) {
	if req.Devnet == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet is required")
	}
	if req.TxType == "" {
		return nil, status.Error(codes.InvalidArgument, "tx_type is required")
	}
	if req.Signer == "" {
		return nil, status.Error(codes.InvalidArgument, "signer is required")
	}

	s.logger.Info("submitting transaction",
		"devnet", req.Devnet,
		"txType", req.TxType,
		"signer", req.Signer)

	now := time.Now()
	tx := &types.Transaction{
		Metadata: types.ResourceMeta{
			Name:      fmt.Sprintf("tx-%s-%d", req.Devnet, now.UnixNano()),
			Namespace: types.DefaultNamespace, // Transactions use default namespace
			CreatedAt: now,
			UpdatedAt: now,
		},
		Spec: types.TransactionSpec{
			DevnetRef: req.Devnet,
			TxType:    req.TxType,
			Signer:    req.Signer,
			Payload:   req.Payload,
			GasLimit:  req.GasLimit,
			Memo:      req.Memo,
		},
		Status: types.TransactionStatus{
			Phase:   types.TxPhasePending,
			Message: "Transaction created",
		},
	}

	if err := s.store.CreateTransaction(ctx, tx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create transaction: %v", err)
	}

	// Enqueue for reconciliation with namespace/name key
	if s.manager != nil {
		s.manager.Enqueue("transactions", types.DefaultNamespace+"/"+tx.Metadata.Name)
	}

	return &v1.SubmitTransactionResponse{Transaction: transactionToProto(tx)}, nil
}

// GetTransaction retrieves a transaction by name.
func (s *TransactionService) GetTransaction(ctx context.Context, req *v1.GetTransactionRequest) (*v1.GetTransactionResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	tx, err := s.store.GetTransaction(ctx, req.Name)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "transaction %s not found", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to get transaction: %v", err)
	}

	return &v1.GetTransactionResponse{Transaction: transactionToProto(tx)}, nil
}

// ListTransactions lists transactions with optional filters.
func (s *TransactionService) ListTransactions(ctx context.Context, req *v1.ListTransactionsRequest) (*v1.ListTransactionsResponse, error) {
	if req.Devnet == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet is required")
	}

	opts := store.ListTxOptions{
		TxType: req.TxType,
		Phase:  req.Phase,
		Limit:  int(req.Limit),
	}

	// Use empty namespace to search across all namespaces (transactions are keyed by devnet)
	txs, err := s.store.ListTransactions(ctx, "", req.Devnet, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list transactions: %v", err)
	}

	resp := &v1.ListTransactionsResponse{
		Transactions: make([]*v1.Transaction, 0, len(txs)),
	}
	for _, tx := range txs {
		resp.Transactions = append(resp.Transactions, transactionToProto(tx))
	}

	return resp, nil
}

// CancelTransaction cancels a pending transaction.
func (s *TransactionService) CancelTransaction(ctx context.Context, req *v1.CancelTransactionRequest) (*v1.CancelTransactionResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	tx, err := s.store.GetTransaction(ctx, req.Name)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "transaction %s not found", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to get transaction: %v", err)
	}

	// Only pending transactions can be cancelled
	if tx.Status.Phase != types.TxPhasePending {
		return nil, status.Errorf(codes.FailedPrecondition, "can only cancel pending transactions, current phase: %s", tx.Status.Phase)
	}

	tx.Status.Phase = types.TxPhaseFailed
	tx.Status.Error = "Cancelled by user"
	tx.Status.Message = "Transaction cancelled"
	tx.Metadata.UpdatedAt = time.Now()

	if err := s.store.UpdateTransaction(ctx, tx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update transaction: %v", err)
	}

	return &v1.CancelTransactionResponse{Transaction: transactionToProto(tx)}, nil
}

// SubmitGovVote submits a governance vote transaction.
func (s *TransactionService) SubmitGovVote(ctx context.Context, req *v1.SubmitGovVoteRequest) (*v1.SubmitGovVoteResponse, error) {
	if req.Devnet == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet is required")
	}
	if req.ProposalId == 0 {
		return nil, status.Error(codes.InvalidArgument, "proposal_id is required")
	}
	if req.VoteOption == "" {
		return nil, status.Error(codes.InvalidArgument, "vote_option is required")
	}
	if req.Voter == "" {
		return nil, status.Error(codes.InvalidArgument, "voter is required")
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"proposal_id": req.ProposalId,
		"option":      req.VoteOption,
	})

	resp, err := s.SubmitTransaction(ctx, &v1.SubmitTransactionRequest{
		Devnet:  req.Devnet,
		TxType:  "gov/vote",
		Signer:  req.Voter,
		Payload: payload,
	})
	if err != nil {
		return nil, err
	}
	return &v1.SubmitGovVoteResponse{Transaction: resp.Transaction}, nil
}

// SubmitGovProposal submits a governance proposal transaction.
func (s *TransactionService) SubmitGovProposal(ctx context.Context, req *v1.SubmitGovProposalRequest) (*v1.SubmitGovProposalResponse, error) {
	if req.Devnet == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet is required")
	}
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	if req.Proposer == "" {
		return nil, status.Error(codes.InvalidArgument, "proposer is required")
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type":        req.ProposalType,
		"title":       req.Title,
		"description": req.Description,
		"content":     req.Content,
	})

	resp, err := s.SubmitTransaction(ctx, &v1.SubmitTransactionRequest{
		Devnet:  req.Devnet,
		TxType:  "gov/proposal",
		Signer:  req.Proposer,
		Payload: payload,
	})
	if err != nil {
		return nil, err
	}
	return &v1.SubmitGovProposalResponse{Transaction: resp.Transaction}, nil
}

// transactionToProto converts a Transaction to its proto representation.
func transactionToProto(tx *types.Transaction) *v1.Transaction {
	return &v1.Transaction{
		Name:      tx.Metadata.Name,
		DevnetRef: tx.Spec.DevnetRef,
		TxType:    tx.Spec.TxType,
		Signer:    tx.Spec.Signer,
		Payload:   tx.Spec.Payload,
		Phase:     tx.Status.Phase,
		TxHash:    tx.Status.TxHash,
		Height:    tx.Status.Height,
		GasUsed:   tx.Status.GasUsed,
		Error:     tx.Status.Error,
		Message:   tx.Status.Message,
		CreatedAt: timestamppb.New(tx.Metadata.CreatedAt),
		UpdatedAt: timestamppb.New(tx.Metadata.UpdatedAt),
	}
}
