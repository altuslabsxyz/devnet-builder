// pkg/network/txbuilder.go
package network

import (
	"context"
	"encoding/json"
)

// TxBuilder is the core abstraction for building, signing, and broadcasting transactions.
// Plugin implementations provide chain-specific transaction logic through this interface.
//
// The TxBuilder uses a stateful pattern where an instance is created for a specific
// chain configuration, then used for multiple transaction operations before being destroyed.
type TxBuilder interface {
	// BuildTx constructs an unsigned transaction from a request.
	// Returns the raw transaction bytes and signing document.
	BuildTx(ctx context.Context, req *TxBuildRequest) (*UnsignedTx, error)

	// SignTx signs an unsigned transaction with the provided key.
	// Supports both raw private keys and keyring references.
	SignTx(ctx context.Context, tx *UnsignedTx, key *SigningKey) (*SignedTx, error)

	// BroadcastTx submits a signed transaction to the network.
	// Returns the result including transaction hash and any error codes.
	BroadcastTx(ctx context.Context, tx *SignedTx) (*TxBroadcastResult, error)

	// SupportedTxTypes returns the transaction types this builder supports.
	SupportedTxTypes() []TxType
}

// TxType identifies a category of transaction.
// Each plugin may support different transaction types based on its chain's capabilities.
type TxType string

// Common transaction types supported across Cosmos SDK chains.
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

// TxBuildRequest contains the parameters for building a transaction.
type TxBuildRequest struct {
	// TxType identifies the type of transaction to build.
	TxType TxType `json:"txType"`

	// Sender is the address of the transaction sender.
	Sender string `json:"sender"`

	// Payload contains transaction-specific data as JSON.
	// The structure depends on TxType.
	Payload json.RawMessage `json:"payload"`

	// ChainID is the chain identifier for the transaction.
	ChainID string `json:"chainId"`

	// GasLimit is the maximum gas for the transaction (0 = auto-estimate).
	GasLimit uint64 `json:"gasLimit,omitempty"`

	// GasPrice is the price per unit of gas.
	GasPrice string `json:"gasPrice,omitempty"`

	// Memo is an optional transaction memo.
	Memo string `json:"memo,omitempty"`
}

// UnsignedTx represents a transaction ready for signing.
type UnsignedTx struct {
	// TxBytes is the raw unsigned transaction bytes.
	TxBytes []byte `json:"txBytes"`

	// SignDoc is the document to be signed.
	SignDoc []byte `json:"signDoc"`

	// AccountNumber is the signer's account number.
	AccountNumber uint64 `json:"accountNumber"`

	// Sequence is the signer's sequence number.
	Sequence uint64 `json:"sequence"`
}

// SigningKey contains the key material for signing.
type SigningKey struct {
	// Address is the signer's address.
	Address string `json:"address"`

	// PrivKey is the raw private key bytes (optional if using keyring).
	PrivKey []byte `json:"privKey,omitempty"`

	// KeyringRef is a reference to a key in the keyring (optional if using PrivKey).
	KeyringRef string `json:"keyringRef,omitempty"`
}

// SignedTx represents a fully signed transaction.
type SignedTx struct {
	// TxBytes is the signed transaction bytes ready for broadcast.
	TxBytes []byte `json:"txBytes"`

	// Signature is the transaction signature.
	Signature []byte `json:"signature"`

	// PubKey is the signer's public key.
	PubKey []byte `json:"pubKey"`
}

// TxBroadcastResult contains the result of broadcasting a transaction.
type TxBroadcastResult struct {
	// TxHash is the transaction hash.
	TxHash string `json:"txHash"`

	// Code is the transaction result code (0 = success).
	Code uint32 `json:"code"`

	// Log contains any log messages from the transaction.
	Log string `json:"log,omitempty"`

	// Height is the block height where the transaction was included.
	Height int64 `json:"height,omitempty"`
}

// TxBuilderConfig configures TxBuilder creation.
type TxBuilderConfig struct {
	// RPCEndpoint is the node's RPC URL for querying chain state.
	RPCEndpoint string `json:"rpcEndpoint"`

	// ChainID is the chain identifier.
	ChainID string `json:"chainId"`

	// SDKVersion is an optional hint about the SDK version.
	// If nil, the plugin should auto-detect from the running node.
	SDKVersion *SDKVersion `json:"sdkVersion,omitempty"`
}

// SDKVersion identifies the chain's SDK framework and version.
// This is used to select the appropriate TxBuilder implementation
// for different SDK versions (e.g., Cosmos v0.47 vs v0.50).
type SDKVersion struct {
	// Framework identifies the SDK family: "cosmos-sdk", "evm", "tempo"
	Framework string `json:"framework"`

	// Version is the semantic version string, e.g., "v0.50.2"
	Version string `json:"version"`

	// Features lists optional capabilities, e.g., ["gov-v1", "authz", "group"]
	Features []string `json:"features"`
}

// Common SDK framework constants.
const (
	FrameworkCosmosSDK = "cosmos-sdk"
	FrameworkEVM       = "evm"
	FrameworkTempo     = "tempo"
)

// Common SDK feature constants.
const (
	FeatureGovV1    = "gov-v1"   // Governance v1 (new in SDK v0.46+)
	FeatureAuthz    = "authz"    // Authorization module
	FeatureGroup    = "group"    // Group module
	FeatureNFT      = "nft"      // NFT module
	FeatureFeegrant = "feegrant" // Fee grant module
)

// TxBuilderFactory is implemented by modules that can create TxBuilders.
type TxBuilderFactory interface {
	CreateTxBuilder(ctx context.Context, cfg *TxBuilderConfig) (TxBuilder, error)
}
