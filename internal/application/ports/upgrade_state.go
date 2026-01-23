// Package ports defines interfaces for the application layer.
// This file contains types and interfaces for upgrade state management.
package ports

import (
	"context"
	"time"
)

// ResumableStage represents the current stage in the resumable upgrade workflow.
// This is distinct from UpgradeStage which is used for progress callbacks.
type ResumableStage string

const (
	// ResumableStageInitialized indicates the upgrade has been started but no proposal submitted yet.
	ResumableStageInitialized ResumableStage = "Initialized"
	// ResumableStageProposalSubmitted indicates the proposal has been submitted and is in deposit period.
	ResumableStageProposalSubmitted ResumableStage = "ProposalSubmitted"
	// ResumableStageVoting indicates validators are voting on the proposal.
	ResumableStageVoting ResumableStage = "Voting"
	// ResumableStageWaitingForHeight indicates the proposal passed and we're waiting for upgrade height.
	ResumableStageWaitingForHeight ResumableStage = "WaitingForHeight"
	// ResumableStageChainHalted indicates the chain has halted at upgrade height.
	ResumableStageChainHalted ResumableStage = "ChainHalted"
	// ResumableStageSwitchingBinary indicates binary switch is in progress.
	ResumableStageSwitchingBinary ResumableStage = "SwitchingBinary"
	// ResumableStageVerifyingResume indicates we're verifying the chain resumed with new binary.
	ResumableStageVerifyingResume ResumableStage = "VerifyingResume"
	// ResumableStageCompleted indicates the upgrade completed successfully.
	ResumableStageCompleted ResumableStage = "Completed"
	// ResumableStageFailed indicates the upgrade failed.
	ResumableStageFailed ResumableStage = "Failed"
	// ResumableStageProposalRejected indicates the governance proposal was rejected.
	ResumableStageProposalRejected ResumableStage = "ProposalRejected"
)

// String returns the string representation of the stage.
func (s ResumableStage) String() string {
	return string(s)
}

// IsTerminal returns true if this is a terminal stage (Completed, Failed, or ProposalRejected).
func (s ResumableStage) IsTerminal() bool {
	return s == ResumableStageCompleted || s == ResumableStageFailed || s == ResumableStageProposalRejected
}

// ValidatorVoteState tracks voting status for a single validator.
type ValidatorVoteState struct {
	Address   string     `json:"address"`
	Moniker   string     `json:"moniker,omitempty"`
	Voted     bool       `json:"voted"`
	TxHash    string     `json:"txHash,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

// NodeSwitchState tracks binary switch status for a single node.
type NodeSwitchState struct {
	NodeName  string     `json:"nodeName"`
	Switched  bool       `json:"switched"`
	Stopped   bool       `json:"stopped"`
	Started   bool       `json:"started"`
	OldBinary string     `json:"oldBinary,omitempty"`
	NewBinary string     `json:"newBinary,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

// StageTransition records a state transition for audit purposes.
type StageTransition struct {
	From      ResumableStage `json:"from"`
	To        ResumableStage `json:"to"`
	Timestamp time.Time      `json:"timestamp"`
	Reason    string         `json:"reason,omitempty"`
}

// UpgradeState represents the complete persistent state of an upgrade.
type UpgradeState struct {
	// Version is the schema version for forward compatibility (start at 1).
	Version int `json:"version"`
	// Checksum is SHA256 of JSON content (excluding this field) for integrity validation.
	Checksum string `json:"checksum"`
	// UpgradeName is the upgrade handler name (e.g., "v2.0.0").
	UpgradeName string `json:"upgradeName"`
	// Stage is the current stage in the upgrade workflow.
	Stage ResumableStage `json:"stage"`
	// Mode is the execution mode: "docker" or "local".
	Mode string `json:"mode"`
	// SkipGovernance indicates whether governance was skipped.
	SkipGovernance bool `json:"skipGovernance"`
	// ProposalID is the governance proposal ID (0 if skip-gov).
	ProposalID uint64 `json:"proposalID,omitempty"`
	// UpgradeHeight is the target block height for upgrade.
	UpgradeHeight int64 `json:"upgradeHeight,omitempty"`
	// TargetBinary is the path to target binary (local mode).
	TargetBinary string `json:"targetBinary,omitempty"`
	// TargetImage is the Docker image reference (docker mode).
	TargetImage string `json:"targetImage,omitempty"`
	// TargetVersion is the target version string.
	TargetVersion string `json:"targetVersion,omitempty"`
	// ValidatorVotes tracks vote status per validator.
	ValidatorVotes []ValidatorVoteState `json:"validatorVotes,omitempty"`
	// NodeSwitches tracks switch status per node.
	NodeSwitches []NodeSwitchState `json:"nodeSwitches,omitempty"`
	// Error contains the error message if stage is Failed.
	Error string `json:"error,omitempty"`
	// CreatedAt is when the upgrade was initiated.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is the last state update timestamp.
	UpdatedAt time.Time `json:"updatedAt"`
	// StageHistory is the audit log of stage transitions.
	StageHistory []StageTransition `json:"stageHistory"`
}

// CurrentStateVersion is the current schema version for UpgradeState.
const CurrentStateVersion = 1

// NewUpgradeState creates a new UpgradeState with initialized fields.
func NewUpgradeState(upgradeName, mode string, skipGovernance bool) *UpgradeState {
	now := time.Now()
	return &UpgradeState{
		Version:        CurrentStateVersion,
		UpgradeName:    upgradeName,
		Stage:          ResumableStageInitialized,
		Mode:           mode,
		SkipGovernance: skipGovernance,
		CreatedAt:      now,
		UpdatedAt:      now,
		StageHistory: []StageTransition{
			{
				From:      "",
				To:        ResumableStageInitialized,
				Timestamp: now,
				Reason:    "upgrade initiated",
			},
		},
	}
}

// UpgradeStateManager defines the contract for managing upgrade state persistence.
type UpgradeStateManager interface {
	// LoadState loads the current upgrade state from disk.
	// Returns nil, nil if no state exists.
	// Returns error if state file is corrupted or unreadable.
	LoadState(ctx context.Context) (*UpgradeState, error)

	// SaveState persists the upgrade state to disk atomically.
	// Uses atomic write (temp file + rename) to prevent corruption.
	SaveState(ctx context.Context, state *UpgradeState) error

	// DeleteState removes the upgrade state file.
	// Used after successful completion or when clearing state.
	DeleteState(ctx context.Context) error

	// StateExists checks if an upgrade state file exists.
	StateExists(ctx context.Context) (bool, error)

	// ValidateState checks state integrity (schema + checksum).
	// Returns validation errors if state is corrupted.
	ValidateState(state *UpgradeState) error

	// AcquireLock acquires an exclusive lock to prevent concurrent upgrades.
	// Returns error if another upgrade is in progress.
	AcquireLock(ctx context.Context) error

	// ReleaseLock releases the exclusive lock.
	ReleaseLock(ctx context.Context) error
}

// UpgradeStateTransitioner defines valid state transitions.
type UpgradeStateTransitioner interface {
	// TransitionTo attempts to transition to the target stage.
	// Returns error if transition is invalid.
	TransitionTo(state *UpgradeState, target ResumableStage, reason string) error

	// CanTransition checks if a transition is valid without performing it.
	CanTransition(from, to ResumableStage) bool

	// GetValidTransitions returns all valid target stages from current stage.
	GetValidTransitions(from ResumableStage) []ResumableStage
}

// UpgradeStateDetector defines detection logic for resume scenarios.
type UpgradeStateDetector interface {
	// DetectCurrentStage queries chain state to determine actual stage.
	// Used on resume to reconcile saved state with chain reality.
	DetectCurrentStage(ctx context.Context, state *UpgradeState) (ResumableStage, error)

	// DetectProposalStatus queries governance for proposal status.
	// Returns: "voting", "passed", "rejected", "failed", or "unknown".
	DetectProposalStatus(ctx context.Context, proposalID uint64) (string, error)

	// DetectChainStatus checks if chain is running, halted, or unreachable.
	// Returns: "running", "halted", or "unreachable".
	DetectChainStatus(ctx context.Context) (string, error)

	// DetectValidatorVotes queries which validators have voted on proposal.
	DetectValidatorVotes(ctx context.Context, proposalID uint64) ([]ValidatorVoteState, error)
}

// ResumeOptions defines options for resuming an upgrade.
type ResumeOptions struct {
	// ForceRestart ignores saved state and starts fresh.
	ForceRestart bool
	// ResumeFrom overrides detected stage with a specific stage.
	ResumeFrom ResumableStage
	// ClearState clears the state file and exits.
	ClearState bool
	// ShowStatus displays current state without running.
	ShowStatus bool
}

// StateCorruptionError indicates the state file is corrupted.
type StateCorruptionError struct {
	Reason string
}

func (e *StateCorruptionError) Error() string {
	return "upgrade state file is corrupted: " + e.Reason
}

// UpgradeInProgressError indicates another upgrade is already in progress.
type UpgradeInProgressError struct {
	UpgradeName string
	Stage       ResumableStage
}

func (e *UpgradeInProgressError) Error() string {
	return "another upgrade is in progress: " + e.UpgradeName + " (stage: " + e.Stage.String() + ")"
}

// InvalidTransitionError indicates an invalid state transition was attempted.
type InvalidTransitionError struct {
	From ResumableStage
	To   ResumableStage
}

func (e *InvalidTransitionError) Error() string {
	return "invalid state transition from " + e.From.String() + " to " + e.To.String()
}
