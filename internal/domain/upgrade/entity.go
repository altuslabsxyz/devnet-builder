// Package upgrade provides domain entities for upgrade management.
package upgrade

import "time"

// Stage represents the current stage of the upgrade process.
type Stage string

const (
	StageVerifying       Stage = "verifying"
	StageSubmitting      Stage = "submitting"
	StageVoting          Stage = "voting"
	StageWaiting         Stage = "waiting"
	StageSwitching       Stage = "switching"
	StageVerifyingResume Stage = "verifying_resume"
	StageCompleted       Stage = "completed"
	StageFailed          Stage = "failed"
)

// String returns a human-readable description of the stage.
func (s Stage) String() string {
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
func (s Stage) StageNumber() int {
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
	case StageVerifyingResume, StageCompleted, StageFailed:
		return 6
	default:
		return 0
	}
}

// IsTerminal returns true if this is a terminal stage.
func (s Stage) IsTerminal() bool {
	return s == StageCompleted || s == StageFailed
}

// ProposalStatus represents the status of an upgrade proposal.
type ProposalStatus string

const (
	ProposalPending  ProposalStatus = "pending"
	ProposalPassed   ProposalStatus = "passed"
	ProposalRejected ProposalStatus = "rejected"
	ProposalExecuted ProposalStatus = "executed"
)

// Proposal represents a submitted upgrade proposal.
type Proposal struct {
	ID            uint64         `json:"id"`
	TxHash        string         `json:"tx_hash"`
	UpgradeName   string         `json:"upgrade_name"`
	UpgradeHeight int64          `json:"upgrade_height"`
	SubmittedAt   time.Time      `json:"submitted_at"`
	VotingEndTime time.Time      `json:"voting_end_time"`
	Status        ProposalStatus `json:"status"`
}

// NewProposal creates a new Proposal.
func NewProposal(id uint64, txHash, upgradeName string, height int64) *Proposal {
	return &Proposal{
		ID:            id,
		TxHash:        txHash,
		UpgradeName:   upgradeName,
		UpgradeHeight: height,
		SubmittedAt:   time.Now(),
		Status:        ProposalPending,
	}
}

// SetPassed marks the proposal as passed.
func (p *Proposal) SetPassed() {
	p.Status = ProposalPassed
}

// SetRejected marks the proposal as rejected.
func (p *Proposal) SetRejected() {
	p.Status = ProposalRejected
}

// SetExecuted marks the proposal as executed.
func (p *Proposal) SetExecuted() {
	p.Status = ProposalExecuted
}

// IsPassed returns true if the proposal passed.
func (p *Proposal) IsPassed() bool {
	return p.Status == ProposalPassed || p.Status == ProposalExecuted
}

// Vote represents a vote on an upgrade proposal.
type Vote struct {
	ValidatorIndex int       `json:"validator_index"`
	ProposalID     uint64    `json:"proposal_id"`
	Option         VoteOption `json:"option"`
	TxHash         string    `json:"tx_hash"`
	VotedAt        time.Time `json:"voted_at"`
}

// VoteOption represents a voting option.
type VoteOption string

const (
	VoteYes        VoteOption = "yes"
	VoteNo         VoteOption = "no"
	VoteAbstain    VoteOption = "abstain"
	VoteNoWithVeto VoteOption = "no_with_veto"
)

// Plan represents an upgrade plan from the chain.
type Plan struct {
	Name   string `json:"name"`
	Height int64  `json:"height"`
	Info   string `json:"info,omitempty"`
}

// Progress tracks the overall upgrade process state.
type Progress struct {
	Stage         Stage      `json:"stage"`
	Proposal      *Proposal  `json:"proposal,omitempty"`
	VotesCast     int        `json:"votes_cast"`
	TotalVoters   int        `json:"total_voters"`
	CurrentHeight int64      `json:"current_height"`
	TargetHeight  int64      `json:"target_height"`
	VotingEndTime time.Time  `json:"voting_end_time,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// NewProgress creates a new Progress.
func NewProgress(totalVoters int) *Progress {
	return &Progress{
		Stage:       StageVerifying,
		TotalVoters: totalVoters,
		StartedAt:   time.Now(),
	}
}

// SetStage updates the current stage.
func (p *Progress) SetStage(stage Stage) {
	p.Stage = stage
	if stage.IsTerminal() {
		now := time.Now()
		p.CompletedAt = &now
	}
}

// SetProposal sets the proposal information.
func (p *Progress) SetProposal(proposal *Proposal) {
	p.Proposal = proposal
	p.TargetHeight = proposal.UpgradeHeight
	p.VotingEndTime = proposal.VotingEndTime
}

// IncrementVotes increments the vote count.
func (p *Progress) IncrementVotes() {
	p.VotesCast++
}

// SetHeight updates the current height.
func (p *Progress) SetHeight(height int64) {
	p.CurrentHeight = height
}

// SetError sets the error and marks as failed.
func (p *Progress) SetError(err error) {
	p.Stage = StageFailed
	if err != nil {
		p.Error = err.Error()
	}
	now := time.Now()
	p.CompletedAt = &now
}

// Duration returns the duration of the upgrade process.
func (p *Progress) Duration() time.Duration {
	if p.CompletedAt != nil {
		return p.CompletedAt.Sub(p.StartedAt)
	}
	return time.Since(p.StartedAt)
}

// Result contains the final result of an upgrade operation.
type Result struct {
	Success           bool          `json:"success"`
	ProposalID        uint64        `json:"proposal_id"`
	UpgradeHeight     int64         `json:"upgrade_height"`
	PostUpgradeHeight int64         `json:"post_upgrade_height"`
	NewBinary         string        `json:"new_binary"`
	PreGenesisPath    string        `json:"pre_genesis_path,omitempty"`
	PostGenesisPath   string        `json:"post_genesis_path,omitempty"`
	Duration          time.Duration `json:"duration"`
	Error             string        `json:"error,omitempty"`
}
