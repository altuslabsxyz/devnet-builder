// internal/daemon/types/transaction.go
package types

import "encoding/json"

// Transaction phase constants.
const (
	TxPhasePending   = "Pending"
	TxPhaseSubmitted = "Submitted"
	TxPhaseConfirmed = "Confirmed"
	TxPhaseFailed    = "Failed"
)

// Transaction represents a blockchain transaction operation.
type Transaction struct {
	Metadata ResourceMeta      `json:"metadata"`
	Spec     TransactionSpec   `json:"spec"`
	Status   TransactionStatus `json:"status"`
}

// TransactionSpec defines the desired transaction.
type TransactionSpec struct {
	// DevnetRef is the name of the target Devnet.
	DevnetRef string `json:"devnetRef"`

	// TxType is the transaction type (e.g., "gov/vote", "staking/delegate").
	TxType string `json:"txType"`

	// Signer identifies who signs the tx (e.g., "validator:0", "account:alice").
	Signer string `json:"signer"`

	// Payload is the transaction-specific data (JSON).
	Payload json.RawMessage `json:"payload"`

	// SDKVersion overrides auto-detected SDK version.
	SDKVersion string `json:"sdkVersion,omitempty"`
}

// TransactionStatus defines the observed state of a Transaction.
type TransactionStatus struct {
	// Phase is the current phase.
	Phase string `json:"phase"`

	// TxHash is the transaction hash.
	TxHash string `json:"txHash,omitempty"`

	// Height is the block height where tx was included.
	Height int64 `json:"height,omitempty"`

	// GasUsed is the gas consumed by the transaction.
	GasUsed int64 `json:"gasUsed,omitempty"`

	// Error contains error details if phase is Failed.
	Error string `json:"error,omitempty"`
}
