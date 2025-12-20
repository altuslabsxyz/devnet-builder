package upgrade

import (
	"time"

	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// UpgradeStage represents the current stage of the upgrade process.
type UpgradeStage string

const (
	StageVerifying       UpgradeStage = "verifying"
	StageSubmitting      UpgradeStage = "submitting"
	StageVoting          UpgradeStage = "voting"
	StageWaiting         UpgradeStage = "waiting"
	StageSwitching       UpgradeStage = "switching"
	StageVerifyingResume UpgradeStage = "verifying_resume"
	StageCompleted       UpgradeStage = "completed"
	StageFailed          UpgradeStage = "failed"
)

// String returns a human-readable description of the stage.
func (s UpgradeStage) String() string {
	switch s {
	case StageVerifying:
		return "Verifying devnet status"
	case StageSubmitting:
		return "Submitting upgrade proposal"
	case StageVoting:
		return "Voting from validators"
	case StageWaiting:
		return "Waiting for upgrade height"
	case StageSwitching:
		return "Switching to new binary"
	case StageVerifyingResume:
		return "Verifying chain resumed"
	case StageCompleted:
		return "Upgrade completed"
	case StageFailed:
		return "Upgrade failed"
	default:
		return string(s)
	}
}

// StageNumber returns the stage number (1-6) for progress display.
func (s UpgradeStage) StageNumber() int {
	switch s {
	case StageVerifying:
		return 1
	case StageSubmitting:
		return 2
	case StageVoting:
		return 3
	case StageWaiting:
		return 4
	case StageSwitching:
		return 5
	case StageVerifyingResume:
		return 6
	case StageCompleted, StageFailed:
		return 6
	default:
		return 0
	}
}

// ProposalStatus represents the status of an upgrade proposal.
type ProposalStatus string

const (
	ProposalPending  ProposalStatus = "pending"
	ProposalPassed   ProposalStatus = "passed"
	ProposalRejected ProposalStatus = "rejected"
	ProposalExecuted ProposalStatus = "executed"
)

// SnapshotType represents the type of genesis snapshot.
type SnapshotType string

const (
	SnapshotPreUpgrade  SnapshotType = "pre_upgrade"
	SnapshotPostUpgrade SnapshotType = "post_upgrade"
)

// UpgradeConfig holds configuration for an upgrade operation.
type UpgradeConfig struct {
	Name          string               // Upgrade handler name (required)
	Mode          devnet.ExecutionMode // Explicit execution mode (docker/local); empty = use metadata
	TargetImage   string               // Docker image for upgrade (mutually exclusive with TargetBinary)
	TargetBinary  string               // Local binary path for upgrade
	TargetVersion string               // Version string for the upgrade (e.g., "v1.2.0")
	CachePath     string               // Path to pre-built cached binary (for symlink switch)
	CommitHash    string               // Commit hash of the cached binary
	VotingPeriod  time.Duration        // Expedited voting period (default: 60s)
	HeightBuffer  int                  // Blocks to add after voting (default: 10)
	UpgradeHeight int64                // Explicit upgrade height (0 = auto-calculate)
	ExportGenesis bool                 // Export genesis before/after upgrade
	GenesisDir    string               // Directory for genesis exports
}

// Validate checks if the config is valid.
func (c *UpgradeConfig) Validate() error {
	if c.Name == "" {
		return ErrInvalidConfig
	}
	// Need at least one of: TargetImage, TargetBinary, or CachePath
	if c.TargetImage == "" && c.TargetBinary == "" && c.CachePath == "" {
		return ErrNoTargetBinary
	}
	// Can't have both Docker image and local binary
	if c.TargetImage != "" && (c.TargetBinary != "" || c.CachePath != "") {
		return ErrBothTargetsDefined
	}
	if c.VotingPeriod < MinVotingPeriod {
		return ErrVotingPeriodTooShort
	}
	if c.HeightBuffer < MinHeightBuffer {
		return ErrHeightBufferTooSmall
	}
	return nil
}

// IsCacheMode returns true if the upgrade uses a pre-cached binary.
func (c *UpgradeConfig) IsCacheMode() bool {
	return c.CachePath != "" && c.CommitHash != ""
}

// IsDockerMode returns true if the upgrade targets a Docker image.
func (c *UpgradeConfig) IsDockerMode() bool {
	return c.TargetImage != ""
}

// UpgradeProposal represents a submitted upgrade proposal.
type UpgradeProposal struct {
	ID            uint64         // Proposal ID from governance module
	TxHash        string         // Transaction hash of submission
	UpgradeName   string         // Handler name from config
	UpgradeHeight int64          // Target block height
	SubmittedAt   time.Time      // Submission timestamp
	VotingEndTime time.Time      // Voting period end time
	Status        ProposalStatus // Current proposal status
}

// UpgradeProgress tracks the overall upgrade process state.
type UpgradeProgress struct {
	Stage         UpgradeStage     // Current stage of upgrade
	Proposal      *UpgradeProposal // Submitted proposal (nil before submission)
	VotesCast     int              // Number of successful votes
	TotalVoters   int              // Total validators to vote
	CurrentHeight int64            // Latest observed block height
	TargetHeight  int64            // Upgrade target height
	VotingEndTime time.Time        // Voting period end time (for display)
	Error         error            // Error if stage failed
	StartedAt     time.Time        // Upgrade start timestamp
	CompletedAt   *time.Time       // Completion timestamp (nil if in progress)
}

// ProgressCallback is called when upgrade progress changes.
type ProgressCallback func(progress UpgradeProgress)

// ValidatorAccount represents a validator's keys for voting.
type ValidatorAccount struct {
	Index         int    // Validator index (0-3)
	Name          string // Account name (e.g., "validator0")
	Bech32Address string // Cosmos address
	HexAddress    string // EVM address (0x prefixed)
	PrivateKey    string // EVM private key (hex, no 0x prefix)
}

// UpgradeResult contains the final result of an upgrade operation.
type UpgradeResult struct {
	Success           bool          // Whether upgrade succeeded
	ProposalID        uint64        // Proposal ID
	UpgradeHeight     int64         // Height where upgrade occurred
	PostUpgradeHeight int64         // First height after upgrade
	NewBinary         string        // New binary (image or path)
	PreGenesisPath    string        // Path to pre-upgrade genesis (empty if not exported)
	PostGenesisPath   string        // Path to post-upgrade genesis (empty if not exported)
	Duration          time.Duration // Total upgrade duration
	Error             error         // Error if failed
}

// GenesisSnapshot represents an exported genesis state.
type GenesisSnapshot struct {
	Type       SnapshotType // Pre or post upgrade
	FilePath   string       // Path to exported file
	Height     int64        // Block height at export
	ChainID    string       // Chain ID from genesis
	ExportedAt time.Time    // Export timestamp
	SizeBytes  int64        // File size
}

// ExecuteOptions contains options for ExecuteUpgrade.
type ExecuteOptions struct {
	HomeDir          string                 // Base directory for devnet
	Metadata         *devnet.DevnetMetadata // Loaded devnet metadata
	Logger           *output.Logger         // Logger for output
	ProgressCallback ProgressCallback       // Progress callback
}
