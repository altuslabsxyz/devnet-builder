package devnet

// ProvisionState represents the provision state of a devnet.
type ProvisionState string

const (
	// ProvisionStateNone indicates no provision has been performed.
	ProvisionStateNone ProvisionState = "none"
	// ProvisionStateSyncing indicates provision is in progress.
	ProvisionStateSyncing ProvisionState = "syncing"
	// ProvisionStateProvisioned indicates provision completed successfully.
	ProvisionStateProvisioned ProvisionState = "provisioned"
	// ProvisionStateFailed indicates provision failed.
	ProvisionStateFailed ProvisionState = "failed"
)

// IsValid returns true if the provision state is a valid value.
func (s ProvisionState) IsValid() bool {
	switch s {
	case ProvisionStateNone, ProvisionStateSyncing, ProvisionStateProvisioned, ProvisionStateFailed:
		return true
	default:
		return false
	}
}

// CanTransitionTo returns true if transitioning to the target state is allowed.
func (s ProvisionState) CanTransitionTo(target ProvisionState) bool {
	switch s {
	case ProvisionStateNone:
		return target == ProvisionStateSyncing
	case ProvisionStateSyncing:
		return target == ProvisionStateProvisioned || target == ProvisionStateFailed
	case ProvisionStateFailed:
		return target == ProvisionStateSyncing
	case ProvisionStateProvisioned:
		return false // Terminal state, cannot transition
	default:
		return false
	}
}

// CanRun returns true if the devnet can be started from this state.
func (s ProvisionState) CanRun() bool {
	return s == ProvisionStateProvisioned
}

// String returns the string representation of the provision state.
func (s ProvisionState) String() string {
	return string(s)
}
