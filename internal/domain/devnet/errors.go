package devnet

import "fmt"

// InvalidTransitionError is returned when a state transition is not allowed.
type InvalidTransitionError struct {
	From Status
	To   Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid state transition from %s to %s", e.From, e.To)
}

// ValidationError is returned when devnet configuration is invalid.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

// NotFoundError is returned when a devnet is not found.
type NotFoundError struct {
	HomeDir string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("devnet not found at %s", e.HomeDir)
}

// AlreadyExistsError is returned when trying to create a devnet that already exists.
type AlreadyExistsError struct {
	HomeDir string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("devnet already exists at %s", e.HomeDir)
}

// NotProvisionedError is returned when trying to run an unprovisioned devnet.
type NotProvisionedError struct {
	State ProvisionState
}

func (e *NotProvisionedError) Error() string {
	return fmt.Sprintf("devnet is not provisioned (current state: %s)", e.State)
}
