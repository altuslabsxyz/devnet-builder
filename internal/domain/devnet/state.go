// Package devnet provides domain entities for devnet management.
package devnet

import "time"

// Status represents the current state of a devnet.
type Status string

const (
	StatusCreated Status = "created"
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

// IsValid checks if the status is valid.
func (s Status) IsValid() bool {
	switch s {
	case StatusCreated, StatusRunning, StatusStopped, StatusError:
		return true
	}
	return false
}

// CanTransitionTo checks if a transition to the target status is valid.
func (s Status) CanTransitionTo(target Status) bool {
	switch s {
	case StatusCreated:
		return target == StatusRunning || target == StatusError
	case StatusRunning:
		return target == StatusStopped || target == StatusError
	case StatusStopped:
		return target == StatusRunning || target == StatusError
	case StatusError:
		return target == StatusStopped || target == StatusRunning
	}
	return false
}

// ProvisionState represents the provisioning state of a devnet.
type ProvisionState string

const (
	ProvisionStateNone        ProvisionState = ""
	ProvisionStateSyncing     ProvisionState = "syncing"
	ProvisionStateProvisioned ProvisionState = "provisioned"
	ProvisionStateFailed      ProvisionState = "failed"
)

// IsProvisioned returns true if provisioning is complete.
func (p ProvisionState) IsProvisioned() bool {
	return p == ProvisionStateProvisioned
}

// CanRun returns true if the devnet can be started.
func (p ProvisionState) CanRun() bool {
	return p == ProvisionStateProvisioned || p == ProvisionStateNone
}

// IsFailed returns true if provisioning failed.
func (p ProvisionState) IsFailed() bool {
	return p == ProvisionStateFailed
}

// State holds the runtime state of a devnet.
type State struct {
	Status    Status    `json:"status"`
	StartedAt time.Time `json:"started_at,omitempty"`
	StoppedAt time.Time `json:"stopped_at,omitempty"`
}

// NewState creates a new State with created status.
func NewState() State {
	return State{
		Status: StatusCreated,
	}
}

// SetRunning transitions to running state.
func (s *State) SetRunning() error {
	if !s.Status.CanTransitionTo(StatusRunning) {
		return &InvalidTransitionError{From: s.Status, To: StatusRunning}
	}
	s.Status = StatusRunning
	s.StartedAt = time.Now()
	return nil
}

// SetStopped transitions to stopped state.
func (s *State) SetStopped() error {
	if !s.Status.CanTransitionTo(StatusStopped) {
		return &InvalidTransitionError{From: s.Status, To: StatusStopped}
	}
	s.Status = StatusStopped
	s.StoppedAt = time.Now()
	return nil
}

// SetError transitions to error state.
func (s *State) SetError() {
	s.Status = StatusError
}

// IsRunning returns true if the devnet is running.
func (s *State) IsRunning() bool {
	return s.Status == StatusRunning
}

// ProvisionInfo holds provisioning-related state.
type ProvisionInfo struct {
	State       ProvisionState `json:"state,omitempty"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
	Error       string         `json:"error,omitempty"`
	RetryCount  int            `json:"retry_count,omitempty"`
}

// NewProvisionInfo creates a new ProvisionInfo.
func NewProvisionInfo() ProvisionInfo {
	return ProvisionInfo{
		State: ProvisionStateNone,
	}
}

// Start marks provisioning as started.
func (p *ProvisionInfo) Start() {
	p.State = ProvisionStateSyncing
	p.StartedAt = time.Now()
	p.CompletedAt = time.Time{}
	p.Error = ""
}

// Complete marks provisioning as completed successfully.
func (p *ProvisionInfo) Complete() {
	p.State = ProvisionStateProvisioned
	p.CompletedAt = time.Now()
	p.Error = ""
}

// Fail marks provisioning as failed.
func (p *ProvisionInfo) Fail(err error) {
	p.State = ProvisionStateFailed
	if err != nil {
		p.Error = err.Error()
	}
}

// IncrementRetry increments the retry counter.
func (p *ProvisionInfo) IncrementRetry() {
	p.RetryCount++
}

// ResetRetry resets the retry counter.
func (p *ProvisionInfo) ResetRetry() {
	p.RetryCount = 0
}

// CanRun returns true if the devnet can be started.
func (p *ProvisionInfo) CanRun() bool {
	return p.State.CanRun()
}
