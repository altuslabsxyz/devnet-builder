package dto

import (
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ProposeInput contains the input for proposing an upgrade.
type ProposeInput struct {
	HomeDir       string
	UpgradeName   string
	UpgradeHeight int64 // 0 for auto-calculate
	VotingPeriod  time.Duration
	HeightBuffer  int
	DepositAmount string
	DepositDenom  string
}

// ProposeOutput contains the result of proposing.
type ProposeOutput struct {
	ProposalID    uint64
	UpgradeHeight int64
	TxHash        string
	VotingEndTime time.Time
}

// VoteInput contains the input for voting on a proposal.
type VoteInput struct {
	HomeDir    string
	ProposalID uint64
	VoteOption string // "yes", "no", "abstain", "no_with_veto"
	FromAll    bool   // Vote from all validators
}

// VoteOutput contains the result of voting.
type VoteOutput struct {
	VotesCast   int
	TotalVoters int
	TxHashes    []string
	Errors      []string
}

// SwitchBinaryInput contains the input for switching binaries.
type SwitchBinaryInput struct {
	HomeDir       string
	TargetBinary  string // Local binary path
	TargetImage   string // Docker image
	TargetVersion string // Version string
	CachePath     string // Pre-built cached binary
	CommitHash    string // Commit hash of cached binary (deprecated, use CacheRef)
	CacheRef      string // Cache key for SetActive
	Mode          ports.ExecutionMode
	UpgradeHeight int64 // Height to skip via --unsafe-skip-upgrades
}

// SwitchBinaryOutput contains the result of switching.
type SwitchBinaryOutput struct {
	OldBinary      string
	NewBinary      string
	NodesRestarted int
}

// MonitorInput contains the input for monitoring an upgrade.
type MonitorInput struct {
	HomeDir      string
	ProposalID   uint64
	TargetHeight int64
}

// MonitorOutput is a channel-based output for progress updates.
type MonitorProgress struct {
	Stage         ports.UpgradeStage
	CurrentHeight int64
	TargetHeight  int64
	VotesCast     int
	TotalVoters   int
	Message       string
	IsComplete    bool
	Error         error
}

// ExecuteUpgradeInput contains the full upgrade workflow input.
type ExecuteUpgradeInput struct {
	HomeDir     string
	UpgradeName string

	// Binary target (one of these)
	TargetBinary string
	TargetImage  string
	CachePath    string
	CommitHash   string // Deprecated, use CacheRef
	CacheRef     string // Cache key for SetActive

	// Options
	TargetVersion string
	VotingPeriod  time.Duration
	HeightBuffer  int
	UpgradeHeight int64
	WithExport    bool
	GenesisDir    string
	Mode          ports.ExecutionMode
}

// ExecuteUpgradeOutput contains the result of the full workflow.
type ExecuteUpgradeOutput struct {
	Success           bool
	ProposalID        uint64
	UpgradeHeight     int64
	PostUpgradeHeight int64
	NewBinary         string
	PreGenesisPath    string
	PostGenesisPath   string
	Duration          time.Duration
	Error             error
}

// GenesisExportInput contains the input for exporting genesis.
type GenesisExportInput struct {
	HomeDir     string
	OutputDir   string
	UpgradeName string
	PreUpgrade  bool
}

// GenesisExportOutput contains the result of genesis export.
type GenesisExportOutput struct {
	FilePath   string
	Height     int64
	ChainID    string
	ExportedAt time.Time
	SizeBytes  int64
}
