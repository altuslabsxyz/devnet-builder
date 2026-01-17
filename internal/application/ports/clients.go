package ports

import (
	"context"
	"time"

	"github.com/b-harvest/devnet-builder/types"
)

// RPCClient defines operations for interacting with Cosmos RPC.
type RPCClient interface {
	// GetBlockHeight returns the current block height.
	GetBlockHeight(ctx context.Context) (int64, error)

	// GetBlockTime estimates the average block time.
	GetBlockTime(ctx context.Context, sampleSize int) (time.Duration, error)

	// IsChainRunning checks if the chain is responding.
	IsChainRunning(ctx context.Context) bool

	// WaitForBlock waits until the chain reaches the specified height.
	WaitForBlock(ctx context.Context, height int64) error

	// GetProposal retrieves a governance proposal by ID.
	GetProposal(ctx context.Context, id uint64) (*Proposal, error)

	// GetUpgradePlan retrieves the current upgrade plan.
	GetUpgradePlan(ctx context.Context) (*UpgradePlan, error)

	// GetAppVersion returns the application version from /abci_info.
	// Returns empty string if the version is not set or cannot be retrieved.
	GetAppVersion(ctx context.Context) (string, error)

	// GetGovParams retrieves governance parameters from the chain.
	GetGovParams(ctx context.Context) (*GovParams, error)
}

// Proposal represents a governance proposal.
type Proposal struct {
	ID                uint64
	Title             string
	Description       string
	Status            ProposalStatus
	VotingEndTime     time.Time
	SubmitTime        time.Time
	DepositEndTime    time.Time
	TotalDeposit      string
	FinalTallyYes     string
	FinalTallyNo      string
	FinalTallyAbstain string
}

// ProposalStatus represents the status of a proposal.
type ProposalStatus string

const (
	ProposalStatusPending  ProposalStatus = "PROPOSAL_STATUS_DEPOSIT_PERIOD"
	ProposalStatusVoting   ProposalStatus = "PROPOSAL_STATUS_VOTING_PERIOD"
	ProposalStatusPassed   ProposalStatus = "PROPOSAL_STATUS_PASSED"
	ProposalStatusRejected ProposalStatus = "PROPOSAL_STATUS_REJECTED"
	ProposalStatusFailed   ProposalStatus = "PROPOSAL_STATUS_FAILED"
)

// UpgradePlan represents a scheduled upgrade.
type UpgradePlan struct {
	Name   string
	Height int64
	Info   string
	Time   time.Time
}

// GovParams represents governance parameters.
type GovParams struct {
	VotingPeriod          time.Duration
	ExpeditedVotingPeriod time.Duration
	MinDeposit            string
	ExpeditedMinDeposit   string
}

// EVMClient defines operations for interacting with EVM RPC.
type EVMClient interface {
	// SendTransaction sends a signed transaction.
	SendTransaction(ctx context.Context, signedTx []byte) (string, error)

	// SendRawTransaction sends a pre-built and signed transaction.
	SendRawTransaction(ctx context.Context, tx *EVMTransaction) (string, error)

	// GetBalance retrieves the balance of an address.
	GetBalance(ctx context.Context, address string) (string, error)

	// GetNonce retrieves the nonce for an address.
	GetNonce(ctx context.Context, address string) (uint64, error)

	// GetChainID returns the chain ID.
	GetChainID(ctx context.Context) (int64, error)

	// SuggestGasPrice returns a suggested gas price.
	SuggestGasPrice(ctx context.Context) (string, error)

	// EstimateGas estimates gas for a transaction.
	EstimateGas(ctx context.Context, msg *EVMCallMsg) (uint64, error)

	// WaitForTransaction waits for a transaction to be mined.
	WaitForTransaction(ctx context.Context, txHash string, timeout time.Duration) (*TxReceipt, error)

	// Close closes the client connection.
	Close() error
}

// EVMTransaction represents an EVM transaction to be sent.
type EVMTransaction struct {
	To       string
	Value    string
	GasLimit uint64
	GasPrice string
	Data     []byte
	Nonce    uint64
}

// EVMCallMsg represents a call message for gas estimation.
type EVMCallMsg struct {
	From     string
	To       string
	GasPrice string
	Data     []byte
}

// TxReceipt represents a transaction receipt.
type TxReceipt struct {
	TxHash      string
	BlockNumber int64
	Status      bool
	GasUsed     uint64
	Logs        []TxLog
}

// TxLog represents a log entry from a transaction.
type TxLog struct {
	Address string
	Topics  []string
	Data    []byte
}

// SnapshotFetcher defines operations for fetching chain snapshots.
type SnapshotFetcher interface {
	// Download downloads a snapshot from the given URL.
	Download(ctx context.Context, url string, destPath string) error

	// DownloadWithCache downloads a snapshot with caching support.
	// If a valid cached snapshot exists, returns the cached path without downloading.
	// Parameters:
	//   - url: Snapshot download URL
	//   - cacheKey: Cache key for organization (format: "plugin-network", e.g., "stable-mainnet", "ault-testnet")
	//   - noCache: If true, ignores cache and always downloads
	// Returns:
	//   - snapshotPath: Path to the snapshot file (cached or newly downloaded)
	//   - fromCache: True if the snapshot was served from cache
	//   - error: Any error that occurred
	DownloadWithCache(ctx context.Context, url, cacheKey string, noCache bool) (snapshotPath string, fromCache bool, err error)

	// Extract extracts a compressed snapshot.
	Extract(ctx context.Context, archivePath, destPath string) error

	// GetLatestSnapshotURL retrieves the latest snapshot URL for a cache key.
	GetLatestSnapshotURL(ctx context.Context, cacheKey string) (string, error)
}

// GenesisFetcher defines operations for fetching genesis data.
type GenesisFetcher interface {
	// ExportFromChain exports genesis from a running chain.
	ExportFromChain(ctx context.Context, homeDir string) ([]byte, error)

	// FetchFromRPC fetches genesis from an RPC endpoint.
	FetchFromRPC(ctx context.Context, endpoint string) ([]byte, error)

	// ModifyGenesis applies modifications to a genesis file.
	ModifyGenesis(genesis []byte, opts GenesisModifyOptions) ([]byte, error)
}

// AccountKeyInfo contains information about an account key.
type AccountKeyInfo struct {
	Name     string
	Address  string
	PubKey   string
	Mnemonic string
}

// NodeInitializer defines operations for initializing blockchain nodes.
type NodeInitializer interface {
	// Initialize runs the chain init command for a node.
	Initialize(ctx context.Context, nodeDir, moniker, chainID string) error

	// GetNodeID retrieves the node ID from an initialized node.
	GetNodeID(ctx context.Context, nodeDir string) (string, error)

	// CreateAccountKey creates a secp256k1 account key for transaction signing.
	// Keys are stored in keyringDir with the test backend.
	CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*AccountKeyInfo, error)

	// CreateAccountKeyFromMnemonic creates/recovers an account key from a specific mnemonic.
	// This is used for deterministic testing with well-known mnemonics.
	CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*AccountKeyInfo, error)

	// GetAccountKey retrieves information about an existing account key.
	GetAccountKey(ctx context.Context, keyringDir, keyName string) (*AccountKeyInfo, error)

	// GetTestMnemonic returns a deterministic test mnemonic for the given validator index.
	// These are well-known BIP39 mnemonics for reproducible testing.
	GetTestMnemonic(validatorIndex int) string
}

// GenesisModifyOptions holds options for modifying genesis.
type GenesisModifyOptions struct {
	ChainID       string
	NumValidators int // Number of validators to configure for
	VotingPeriod  time.Duration
	UnbondingTime time.Duration
	InflationRate string
	MinGasPrice   string
	AddValidators []ValidatorInfo
	AddAccounts   []AccountInfo
}

// ValidatorInfo represents validator information for genesis.
type ValidatorInfo struct {
	Moniker         string
	ConsPubKey      string // Base64 encoded Ed25519 consensus pubkey
	OperatorAddress string // Bech32 valoper address
	SelfDelegation  string // Amount of tokens to self-delegate
}

// AccountInfo represents account information for genesis.
type AccountInfo struct {
	Name    string
	Address string
	Balance string
}

// KeyManager defines operations for managing cryptographic keys.
type KeyManager interface {
	// CreateKey creates a new key with the given name.
	CreateKey(name string) (*KeyInfo, error)

	// GetKey retrieves a key by name.
	GetKey(name string) (*KeyInfo, error)

	// ListKeys returns all keys.
	ListKeys() ([]*KeyInfo, error)

	// DeleteKey removes a key.
	DeleteKey(name string) error

	// Sign signs data with a key.
	Sign(keyName string, data []byte) ([]byte, error)
}

// KeyInfo represents key information.
type KeyInfo struct {
	Name       string
	Address    string
	HexAddress string
	PubKey     string
	Mnemonic   string
}

// ValidatorKeyLoader defines operations for loading validator keys from the devnet.
type ValidatorKeyLoader interface {
	// LoadValidatorKeys loads all validator keys from the devnet.
	LoadValidatorKeys(ctx context.Context, opts ValidatorKeyOptions) ([]ValidatorKey, error)
}

// ValidatorKeyOptions holds options for loading validator keys.
type ValidatorKeyOptions struct {
	HomeDir       string
	NumValidators int
	ExecutionMode types.ExecutionMode
	Version       string // For docker mode, the image version
	BinaryName    string // Name of the chain binary
}

// ValidatorKey represents a validator's keys for governance operations.
type ValidatorKey struct {
	Index         int
	Name          string
	Bech32Address string
	HexAddress    string
	PrivateKey    string // EVM private key (hex, no 0x prefix)
}

// Builder defines operations for building binaries from source.
type Builder interface {
	// Build builds a binary from source.
	Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)

	// BuildToCache builds and stores in cache without activating.
	BuildToCache(ctx context.Context, opts BuildOptions) (*BuildResult, error)
}

// BuildOptions holds options for building.
type BuildOptions struct {
	Ref       string // Git ref (branch, tag, commit)
	Network   string // Network type for build tags
	OutputDir string // Where to place the binary
	UseCache  bool   // Whether to check cache first
}

// BuildResult holds the result of a build.
type BuildResult struct {
	BinaryPath string
	Ref        string
	CommitHash string
	CachedPath string
	CacheRef   string // Cache key for SetActive (format: commitHash-buildTagsHash)
}

// StateExportService defines operations for exporting genesis from snapshot state.
// This is essential for creating devnets that mirror production state:
// - Fork mainnet/testnet state for testing
// - Create realistic test environments with actual balances and contracts
// - Test upgrades against real state
type StateExportService interface {
	// ExportFromSnapshot exports genesis from a snapshot's application state.
	// This is the main entry point that orchestrates the full export flow:
	// 1. Prepares the home directory with config and snapshot data
	// 2. Runs the chain's export command
	// 3. Validates the exported genesis
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - opts: Export options including paths and settings
	//
	// Returns: Exported genesis JSON bytes, or error
	ExportFromSnapshot(ctx context.Context, opts StateExportOptions) ([]byte, error)

	// PrepareForExport prepares the node home directory for export.
	// This is called after extracting the snapshot and before running export.
	// It ensures:
	// - config directory exists with genesis.json
	// - data/priv_validator_state.json exists
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - homeDir: Path to the node home directory
	//   - rpcGenesis: Genesis from RPC (needed for chain params)
	PrepareForExport(ctx context.Context, homeDir string, rpcGenesis []byte) error

	// ValidateExportedGenesis validates the exported genesis.
	// Checks for required modules and proper structure.
	ValidateExportedGenesis(genesis []byte) error

	// DefaultExportOptions returns the default export options for devnet.
	DefaultExportOptions() *ExportOptions
}

// StateExportOptions holds options for state export.
type StateExportOptions struct {
	// HomeDir is the path to the node home directory containing snapshot data
	HomeDir string

	// BinaryPath is the path to the chain binary for running export
	BinaryPath string

	// RpcGenesis is the genesis fetched from RPC (for chain params)
	RpcGenesis []byte

	// ExportOpts are the export command options
	ExportOpts *ExportOptions

	// Network is the network type (e.g., "mainnet", "testnet")
	//
	// Deprecated: Use CacheKey instead
	Network string

	// CacheKey is the cache key for genesis export caching
	// Format: "plugin-network" (e.g., "stable-mainnet", "ault-testnet")
	CacheKey string

	// SnapshotURL is the URL of the snapshot this genesis is being exported from
	// Used to associate the cached genesis with its source snapshot
	SnapshotURL string

	// SnapshotFromCache indicates whether the snapshot was loaded from cache
	// When true, genesis export will also attempt to use cached genesis
	SnapshotFromCache bool
}

// ExportOptions defines options for genesis state export.
type ExportOptions struct {
	// ForZeroHeight resets the chain height to 0 for the exported genesis.
	// When true:
	// - Block height is reset to 0
	// - All delegation heights are reset
	// - Validator signing info is cleared
	ForZeroHeight bool

	// JailWhitelist is a list of validator operator addresses that should
	// not be jailed in the exported genesis.
	JailWhitelist []string

	// ModulesToSkip is a list of module names to skip during export.
	ModulesToSkip []string

	// Height specifies a specific height to export from.
	// If 0, exports from the latest height.
	Height int64

	// OutputPath is the path to write the exported genesis.
	// If empty, genesis is written to stdout.
	OutputPath string
}

// NewExportOptions creates default export options suitable for devnet.
func NewExportOptions() *ExportOptions {
	return &ExportOptions{
		ForZeroHeight: true,
		JailWhitelist: nil,
		ModulesToSkip: nil,
		Height:        0,
		OutputPath:    "",
	}
}

// SnapshotInfo contains information about a snapshot.
type SnapshotInfo struct {
	// URL is the download URL for the snapshot archive.
	URL string

	// Format is the archive format (e.g., "tar.lz4", "tar.zst", "tar.gz").
	Format string

	// Height is the block height of the snapshot.
	Height int64

	// Hash is the hash of the snapshot archive for verification.
	Hash string

	// Size is the size of the snapshot in bytes.
	Size int64
}
