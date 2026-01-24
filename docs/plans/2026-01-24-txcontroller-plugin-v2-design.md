# TxController and Plugin v2 Design

**Date:** 2026-01-24
**Status:** Approved
**Phase:** 3 Completion

## Overview

This design completes Phase 3 by adding transaction orchestration capabilities to the daemon. It enables governance workflows, multi-chain support, and a full transaction API through a clean abstraction layer.

## Use Cases

1. **Governance Workflows** - Submit proposals, vote, query status
2. **Multi-chain Support** - Abstract SDK differences (Cosmos v0.47/v0.50/v0.53, EVM, Tempo)
3. **Full Transaction API** - Build, sign, broadcast any supported transaction type

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI / gRPC                               │
│   dvb tx submit, dvb gov vote, TransactionService               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       TxController                               │
│   Reconciles Transaction resources through phases               │
│   Pending → Building → Signing → Submitted → Confirmed         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        TxRuntime                                 │
│   GetTxBuilder, GetSigningKey, WaitForConfirmation              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    NetworkModuleV2                               │
│   CreateTxBuilder(sdkVersion) → TxBuilder                       │
│   GetSDKVersion, GetSupportedTxTypes                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       TxBuilder                                  │
│   BuildTx, SignTx, BroadcastTx                                  │
│   Implementations: CosmosV47, CosmosV50, CosmosV53, EVM, Tempo  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Section 1: TxBuilder Interface

The TxBuilder interface provides a clean abstraction for transaction operations across different chain SDKs.

### Interface Definition

```go
// pkg/network/plugin/txbuilder.go
package plugin

import (
    "context"
    "encoding/json"
)

// TxBuilder is the core abstraction for building, signing, and broadcasting transactions.
type TxBuilder interface {
    // BuildTx constructs an unsigned transaction from a request.
    BuildTx(ctx context.Context, req *BuildTxRequest) (*UnsignedTx, error)

    // SignTx signs an unsigned transaction with the provided key.
    SignTx(ctx context.Context, tx *UnsignedTx, key *SigningKey) (*SignedTx, error)

    // BroadcastTx submits a signed transaction to the network.
    BroadcastTx(ctx context.Context, tx *SignedTx) (*BroadcastResult, error)

    // SupportedTxTypes returns the transaction types this builder supports.
    SupportedTxTypes() []TxType
}

// TxType identifies a category of transaction.
type TxType string

const (
    TxTypeGovProposal     TxType = "gov/proposal"
    TxTypeGovVote         TxType = "gov/vote"
    TxTypeStakingDelegate TxType = "staking/delegate"
    TxTypeStakingUnbond   TxType = "staking/unbond"
    TxTypeBankSend        TxType = "bank/send"
    TxTypeWasmExecute     TxType = "wasm/execute"
    TxTypeWasmInstantiate TxType = "wasm/instantiate"
    TxTypeIBCTransfer     TxType = "ibc/transfer"
)

// BuildTxRequest contains the parameters for building a transaction.
type BuildTxRequest struct {
    TxType   TxType          `json:"txType"`
    Sender   string          `json:"sender"`
    Payload  json.RawMessage `json:"payload"`
    GasLimit uint64          `json:"gasLimit,omitempty"`
    GasPrice string          `json:"gasPrice,omitempty"`
    Memo     string          `json:"memo,omitempty"`
}

// UnsignedTx represents a transaction ready for signing.
type UnsignedTx struct {
    TxBytes   []byte `json:"txBytes"`
    SignDoc   []byte `json:"signDoc"`
    AccountNo uint64 `json:"accountNumber"`
    Sequence  uint64 `json:"sequence"`
}

// SigningKey contains the key material for signing.
type SigningKey struct {
    Address    string `json:"address"`
    PrivKey    []byte `json:"privKey,omitempty"`
    KeyringRef string `json:"keyringRef,omitempty"`
}

// SignedTx represents a fully signed transaction.
type SignedTx struct {
    TxBytes   []byte `json:"txBytes"`
    Signature []byte `json:"signature"`
    PubKey    []byte `json:"pubKey"`
}

// BroadcastResult contains the result of broadcasting a transaction.
type BroadcastResult struct {
    TxHash string `json:"txHash"`
    Code   uint32 `json:"code"`
    Log    string `json:"log,omitempty"`
    Height int64  `json:"height,omitempty"`
}
```

### Design Rationale

- **Separation of concerns**: Build, Sign, Broadcast are distinct operations
- **Payload flexibility**: `json.RawMessage` allows type-specific payloads without interface explosion
- **Key abstraction**: `SigningKey` supports both raw keys and keyring references
- **Explicit types**: `TxType` constants prevent stringly-typed errors

---

## Section 2: NetworkModule v2 Extension

Extends the existing HashiCorp go-plugin interface to support transaction building.

### Interface Definition

```go
// pkg/network/plugin/module_v2.go
package plugin

import "context"

// NetworkModuleV2 extends NetworkModule with transaction building capabilities.
type NetworkModuleV2 interface {
    // Embed the existing interface for backward compatibility
    NetworkModule

    // CreateTxBuilder returns a TxBuilder for the specified SDK version.
    CreateTxBuilder(ctx context.Context, req *CreateTxBuilderRequest) (TxBuilder, error)

    // GetSDKVersion detects the SDK version from a running node.
    GetSDKVersion(ctx context.Context, rpcEndpoint string) (*SDKVersion, error)

    // GetSupportedTxTypes returns all transaction types this module can handle.
    GetSupportedTxTypes() []TxType
}

// CreateTxBuilderRequest configures TxBuilder creation.
type CreateTxBuilderRequest struct {
    RPCEndpoint string      `json:"rpcEndpoint"`
    ChainID     string      `json:"chainId"`
    SDKVersion  *SDKVersion `json:"sdkVersion,omitempty"`
}

// SDKVersion identifies the chain's SDK framework and version.
type SDKVersion struct {
    Framework string   `json:"framework"` // "cosmos-sdk", "evm", "tempo"
    Version   string   `json:"version"`   // e.g., "v0.50.2"
    Features  []string `json:"features"`  // e.g., ["gov-v1", "authz"]
}
```

### Plugin Discovery and Loading

```go
// The daemon detects v2 capability at runtime
func (m *Manager) loadPlugin(path string) (NetworkModule, error) {
    // ... existing plugin loading ...

    // Check for v2 interface
    if v2, ok := module.(NetworkModuleV2); ok {
        return v2, nil
    }

    // Fall back to v1 for backward compatibility
    return module, nil
}
```

### SDK Version Auto-Detection

```go
// GetSDKVersion queries the node's /abci_info endpoint
func (p *CosmosPlugin) GetSDKVersion(ctx context.Context, rpcEndpoint string) (*SDKVersion, error) {
    info, err := p.client.ABCIInfo(ctx)
    if err != nil {
        return nil, err
    }

    // Parse version string to determine SDK version
    return &SDKVersion{
        Framework: "cosmos-sdk",
        Version:   info.Response.Version,
        Features:  detectFeatures(info.Response.Version),
    }, nil
}
```

---

## Section 3: Transaction Resource and Store

The Transaction resource follows the Spec/Status pattern used by other daemon resources.

### Type Definition

```go
// internal/daemon/types/transaction.go
package types

import (
    "encoding/json"
    "time"
)

// Transaction represents a blockchain transaction managed by the daemon.
type Transaction struct {
    Metadata ResourceMeta      `json:"metadata"`
    Spec     TransactionSpec   `json:"spec"`
    Status   TransactionStatus `json:"status"`
}

// TransactionSpec defines the desired transaction.
type TransactionSpec struct {
    DevnetRef string          `json:"devnetRef"`
    TxType    string          `json:"txType"`
    Signer    string          `json:"signer"`
    Payload   json.RawMessage `json:"payload"`
    GasLimit  uint64          `json:"gasLimit,omitempty"`
    Memo      string          `json:"memo,omitempty"`
}

// TransactionStatus tracks the transaction's progress.
type TransactionStatus struct {
    Phase       TransactionPhase `json:"phase"`
    TxHash      string           `json:"txHash,omitempty"`
    Height      int64            `json:"height,omitempty"`
    GasUsed     int64            `json:"gasUsed,omitempty"`
    Error       string           `json:"error,omitempty"`
    Message     string           `json:"message,omitempty"`
    SubmittedAt time.Time        `json:"submittedAt,omitempty"`
    ConfirmedAt time.Time        `json:"confirmedAt,omitempty"`
}

// TransactionPhase represents the current state of transaction processing.
type TransactionPhase string

const (
    TxPhasePending   TransactionPhase = "Pending"
    TxPhaseBuilding  TransactionPhase = "Building"
    TxPhaseSigning   TransactionPhase = "Signing"
    TxPhaseSubmitted TransactionPhase = "Submitted"
    TxPhaseConfirmed TransactionPhase = "Confirmed"
    TxPhaseFailed    TransactionPhase = "Failed"
)
```

### Store Interface Extension

```go
// internal/daemon/store/store.go (additions)

type Store interface {
    // ... existing methods ...

    // Transaction operations
    CreateTransaction(ctx context.Context, tx *types.Transaction) error
    GetTransaction(ctx context.Context, name string) (*types.Transaction, error)
    UpdateTransaction(ctx context.Context, tx *types.Transaction) error
    DeleteTransaction(ctx context.Context, name string) error
    ListTransactions(ctx context.Context, devnetRef string) ([]*types.Transaction, error)
}
```

### BoltDB Implementation

```go
// internal/daemon/store/bolt.go (additions)

const transactionsBucket = "transactions"

func (s *BoltStore) CreateTransaction(ctx context.Context, tx *types.Transaction) error {
    return s.db.Update(func(btx *bolt.Tx) error {
        b := btx.Bucket([]byte(transactionsBucket))
        data, err := json.Marshal(tx)
        if err != nil {
            return err
        }
        return b.Put([]byte(tx.Metadata.Name), data)
    })
}
```

### State Machine

```
┌─────────┐
│ Pending │ ─── User creates transaction
└────┬────┘
     │
     ▼
┌──────────┐
│ Building │ ─── TxBuilder.BuildTx() called
└────┬─────┘
     │
     ▼
┌─────────┐
│ Signing │ ─── TxBuilder.SignTx() called
└────┬────┘
     │
     ▼
┌───────────┐
│ Submitted │ ─── TxBuilder.BroadcastTx() called, txHash obtained
└─────┬─────┘
      │
      ▼
┌───────────┐
│ Confirmed │ ─── Transaction included in block
└───────────┘

At any point: → Failed (with error message)
```

---

## Section 4: TxController

The TxController follows the reconciler pattern, processing transactions through their phases.

### Runtime Interface

```go
// internal/daemon/controller/transaction.go
package controller

import (
    "context"

    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

// TxRuntime abstracts plugin access for the TxController.
type TxRuntime interface {
    // GetTxBuilder returns a TxBuilder for the specified devnet.
    GetTxBuilder(ctx context.Context, devnetName string) (plugin.TxBuilder, error)

    // GetSigningKey retrieves the signing key for a validator/account.
    GetSigningKey(ctx context.Context, devnetName string, signer string) (*plugin.SigningKey, error)

    // WaitForConfirmation blocks until the transaction is confirmed or fails.
    WaitForConfirmation(ctx context.Context, devnetName string, txHash string) (*TxReceipt, error)
}

// TxReceipt contains the result of a confirmed transaction.
type TxReceipt struct {
    TxHash  string
    Height  int64
    GasUsed int64
    Success bool
    Log     string
}
```

### Controller Implementation

```go
// TxController reconciles Transaction resources.
type TxController struct {
    store   store.Store
    runtime TxRuntime
    logger  *slog.Logger
}

// NewTxController creates a new TxController.
func NewTxController(s store.Store, r TxRuntime) *TxController {
    return &TxController{
        store:   s,
        runtime: r,
        logger:  slog.Default(),
    }
}

// Reconcile processes a single transaction by name.
func (c *TxController) Reconcile(ctx context.Context, key string) error {
    tx, err := c.store.GetTransaction(ctx, key)
    if err != nil {
        if store.IsNotFound(err) {
            return nil
        }
        return err
    }

    switch tx.Status.Phase {
    case "", types.TxPhasePending:
        return c.reconcilePending(ctx, tx)
    case types.TxPhaseBuilding:
        return c.reconcileBuilding(ctx, tx)
    case types.TxPhaseSigning:
        return c.reconcileSigning(ctx, tx)
    case types.TxPhaseSubmitted:
        return c.reconcileSubmitted(ctx, tx)
    case types.TxPhaseConfirmed, types.TxPhaseFailed:
        return nil // Terminal states
    }
    return nil
}
```

### Phase Handlers

```go
func (c *TxController) reconcilePending(ctx context.Context, tx *types.Transaction) error {
    tx.Status.Phase = types.TxPhaseBuilding
    tx.Status.Message = "Building transaction"
    return c.store.UpdateTransaction(ctx, tx)
}

func (c *TxController) reconcileBuilding(ctx context.Context, tx *types.Transaction) error {
    builder, err := c.runtime.GetTxBuilder(ctx, tx.Spec.DevnetRef)
    if err != nil {
        return c.setFailed(ctx, tx, err.Error())
    }

    unsignedTx, err := builder.BuildTx(ctx, &plugin.BuildTxRequest{
        TxType:   plugin.TxType(tx.Spec.TxType),
        Sender:   tx.Spec.Signer,
        Payload:  tx.Spec.Payload,
        GasLimit: tx.Spec.GasLimit,
        Memo:     tx.Spec.Memo,
    })
    if err != nil {
        return c.setFailed(ctx, tx, err.Error())
    }

    // Store unsigned tx bytes for signing phase
    tx.Status.Phase = types.TxPhaseSigning
    tx.Status.Message = "Signing transaction"
    // Note: In practice, store unsignedTx in a separate field or cache
    return c.store.UpdateTransaction(ctx, tx)
}

func (c *TxController) reconcileSigning(ctx context.Context, tx *types.Transaction) error {
    // Get signing key and sign
    key, err := c.runtime.GetSigningKey(ctx, tx.Spec.DevnetRef, tx.Spec.Signer)
    if err != nil {
        return c.setFailed(ctx, tx, err.Error())
    }

    builder, _ := c.runtime.GetTxBuilder(ctx, tx.Spec.DevnetRef)
    signedTx, err := builder.SignTx(ctx, /* unsignedTx */, key)
    if err != nil {
        return c.setFailed(ctx, tx, err.Error())
    }

    // Broadcast immediately after signing
    result, err := builder.BroadcastTx(ctx, signedTx)
    if err != nil {
        return c.setFailed(ctx, tx, err.Error())
    }

    tx.Status.Phase = types.TxPhaseSubmitted
    tx.Status.TxHash = result.TxHash
    tx.Status.SubmittedAt = time.Now()
    tx.Status.Message = fmt.Sprintf("Submitted: %s", result.TxHash)
    return c.store.UpdateTransaction(ctx, tx)
}

func (c *TxController) reconcileSubmitted(ctx context.Context, tx *types.Transaction) error {
    receipt, err := c.runtime.WaitForConfirmation(ctx, tx.Spec.DevnetRef, tx.Status.TxHash)
    if err != nil {
        return c.setFailed(ctx, tx, err.Error())
    }

    tx.Status.Phase = types.TxPhaseConfirmed
    tx.Status.Height = receipt.Height
    tx.Status.GasUsed = receipt.GasUsed
    tx.Status.ConfirmedAt = time.Now()
    tx.Status.Message = fmt.Sprintf("Confirmed at height %d", receipt.Height)
    return c.store.UpdateTransaction(ctx, tx)
}
```

---

## Section 5: TransactionService gRPC and CLI

### Proto Definition

```protobuf
// api/proto/v1/transaction.proto
syntax = "proto3";
package devnetbuilder.v1;

import "google/protobuf/timestamp.proto";

service TransactionService {
  // Core transaction operations
  rpc SubmitTransaction(SubmitTransactionRequest) returns (Transaction);
  rpc GetTransaction(GetTransactionRequest) returns (Transaction);
  rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse);
  rpc CancelTransaction(CancelTransactionRequest) returns (Transaction);

  // Governance convenience methods
  rpc SubmitGovVote(SubmitGovVoteRequest) returns (Transaction);
  rpc SubmitGovProposal(SubmitGovProposalRequest) returns (Transaction);
}

message Transaction {
  string name = 1;
  string devnet_ref = 2;
  string tx_type = 3;
  string signer = 4;
  bytes payload = 5;
  string phase = 6;
  string tx_hash = 7;
  int64 height = 8;
  string error = 9;
  google.protobuf.Timestamp submitted_at = 10;
  google.protobuf.Timestamp confirmed_at = 11;
}

message SubmitTransactionRequest {
  string devnet = 1;
  string tx_type = 2;
  string signer = 3;
  bytes payload = 4;
  uint64 gas_limit = 5;
  string memo = 6;
}

message SubmitGovVoteRequest {
  string devnet = 1;
  uint64 proposal_id = 2;
  string vote_option = 3;  // yes, no, abstain, no_with_veto
  string voter = 4;        // validator index or address
}

message SubmitGovProposalRequest {
  string devnet = 1;
  string proposal_type = 2;  // upgrade, param_change, text
  string title = 3;
  string description = 4;
  bytes content = 5;         // type-specific content
  string proposer = 6;
}
```

### Service Implementation

```go
// internal/daemon/server/transaction_service.go
package server

type TransactionService struct {
    v1.UnimplementedTransactionServiceServer
    store   store.Store
    manager *controller.Manager
    logger  *slog.Logger
}

func (s *TransactionService) SubmitTransaction(ctx context.Context, req *v1.SubmitTransactionRequest) (*v1.Transaction, error) {
    tx := &types.Transaction{
        Metadata: types.ResourceMeta{
            Name:      fmt.Sprintf("tx-%s-%d", req.Devnet, time.Now().UnixNano()),
            CreatedAt: time.Now(),
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
            Phase: types.TxPhasePending,
        },
    }

    if err := s.store.CreateTransaction(ctx, tx); err != nil {
        return nil, err
    }

    s.manager.Enqueue("transactions", tx.Metadata.Name)
    return toProtoTransaction(tx), nil
}

func (s *TransactionService) SubmitGovVote(ctx context.Context, req *v1.SubmitGovVoteRequest) (*v1.Transaction, error) {
    payload, _ := json.Marshal(map[string]interface{}{
        "proposal_id": req.ProposalId,
        "option":      req.VoteOption,
    })

    return s.SubmitTransaction(ctx, &v1.SubmitTransactionRequest{
        Devnet:  req.Devnet,
        TxType:  "gov/vote",
        Signer:  req.Voter,
        Payload: payload,
    })
}
```

### CLI Commands

```go
// cmd/dvb/cmd/tx.go
package cmd

var txCmd = &cobra.Command{
    Use:   "tx",
    Short: "Transaction operations",
}

var txSubmitCmd = &cobra.Command{
    Use:   "submit <devnet>",
    Short: "Submit a transaction",
    RunE: func(cmd *cobra.Command, args []string) error {
        txType, _ := cmd.Flags().GetString("type")
        payload, _ := cmd.Flags().GetString("payload")
        signer, _ := cmd.Flags().GetString("signer")

        resp, err := client.SubmitTransaction(ctx, &v1.SubmitTransactionRequest{
            Devnet:  args[0],
            TxType:  txType,
            Signer:  signer,
            Payload: []byte(payload),
        })
        // ...
    },
}

// cmd/dvb/cmd/gov.go
var govCmd = &cobra.Command{
    Use:   "gov",
    Short: "Governance operations",
}

var govVoteCmd = &cobra.Command{
    Use:   "vote <devnet> <proposal-id> <option>",
    Short: "Vote on a governance proposal",
    Args:  cobra.ExactArgs(3),
    RunE: func(cmd *cobra.Command, args []string) error {
        proposalID, _ := strconv.ParseUint(args[1], 10, 64)
        voter, _ := cmd.Flags().GetString("voter")

        resp, err := client.SubmitGovVote(ctx, &v1.SubmitGovVoteRequest{
            Devnet:     args[0],
            ProposalId: proposalID,
            VoteOption: args[2],
            Voter:      voter,
        })
        // ...
    },
}
```

### CLI Usage Examples

```bash
# Generic transaction submission
dvb tx submit mydevnet --type gov/vote --payload '{"proposal_id":1,"option":"yes"}' --signer validator0
dvb tx get tx-mydevnet-1234567890
dvb tx list mydevnet

# Governance shortcuts
dvb gov vote mydevnet 1 yes --voter validator0
dvb gov propose mydevnet --type upgrade --name v2 --height 1000
dvb gov list mydevnet
```

---

## Implementation Order

1. **TxBuilder interface** in `pkg/network/plugin/`
2. **Extend NetworkModule** with `CreateTxBuilder`
3. **Implement TxBuilder** for Cosmos SDK (v0.50 first)
4. **Add Transaction resource** to types + BoltDB store
5. **Implement TxController** with reconciliation logic
6. **Add TransactionService** gRPC + CLI commands
7. **Add governance shortcuts** in CLI

## Testing Strategy

- Unit tests for TxBuilder implementations
- Mock TxRuntime for TxController tests
- Integration tests with actual devnet
- CLI integration tests

## Future Extensions

- EVM TxBuilder implementation
- Tempo/Commonware TxBuilder
- Transaction batching
- Retry policies with exponential backoff
- Transaction history/audit log
