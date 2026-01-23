// internal/daemon/types/upgrade.go
package types

// Upgrade phase constants.
const (
	UpgradePhasePending   = "Pending"
	UpgradePhaseProposing = "Proposing"
	UpgradePhaseVoting    = "Voting"
	UpgradePhaseWaiting   = "Waiting"
	UpgradePhaseSwitching = "Switching"
	UpgradePhaseVerifying = "Verifying"
	UpgradePhaseCompleted = "Completed"
	UpgradePhaseFailed    = "Failed"
)

// Upgrade represents a chain upgrade operation.
type Upgrade struct {
	Metadata ResourceMeta  `json:"metadata"`
	Spec     UpgradeSpec   `json:"spec"`
	Status   UpgradeStatus `json:"status"`
}

// UpgradeSpec defines the desired upgrade configuration.
type UpgradeSpec struct {
	// DevnetRef is the name of the target Devnet.
	DevnetRef string `json:"devnetRef"`

	// UpgradeName is the on-chain upgrade name.
	UpgradeName string `json:"upgradeName"`

	// TargetHeight is the block height for the upgrade.
	// 0 means auto-calculate (current + default offset).
	TargetHeight int64 `json:"targetHeight"`

	// NewBinary specifies the upgraded binary source.
	NewBinary BinarySource `json:"newBinary"`

	// WithExport enables state export before/after upgrade.
	WithExport bool `json:"withExport"`

	// AutoVote automatically votes yes with all validators.
	AutoVote bool `json:"autoVote"`
}

// UpgradeStatus defines the observed state of an Upgrade.
type UpgradeStatus struct {
	// Phase is the current upgrade phase.
	Phase string `json:"phase"`

	// ProposalID is the governance proposal ID.
	ProposalID uint64 `json:"proposalId,omitempty"`

	// VotesReceived is the number of validator votes.
	VotesReceived int `json:"votesReceived"`

	// VotesRequired is the number of votes needed.
	VotesRequired int `json:"votesRequired"`

	// CurrentHeight is the chain's current height.
	CurrentHeight int64 `json:"currentHeight"`

	// PreExportPath is the path to pre-upgrade state export.
	PreExportPath string `json:"preExportPath,omitempty"`

	// PostExportPath is the path to post-upgrade state export.
	PostExportPath string `json:"postExportPath,omitempty"`

	// Message provides additional status information.
	Message string `json:"message,omitempty"`

	// Error contains error details if phase is Failed.
	Error string `json:"error,omitempty"`
}
