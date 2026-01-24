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
	ChainID  string          `json:"chainId"`
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
