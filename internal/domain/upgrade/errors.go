package upgrade

import "fmt"

// ValidationError is returned when upgrade configuration is invalid.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("upgrade validation error for %s: %s", e.Field, e.Message)
}

// ProposalError is returned when proposal operations fail.
type ProposalError struct {
	ProposalID uint64
	Operation  string
	Message    string
}

func (e *ProposalError) Error() string {
	return fmt.Sprintf("proposal %d %s failed: %s", e.ProposalID, e.Operation, e.Message)
}

// VoteError is returned when voting fails.
type VoteError struct {
	ValidatorIndex int
	ProposalID     uint64
	Message        string
}

func (e *VoteError) Error() string {
	return fmt.Sprintf("validator %d vote on proposal %d failed: %s", e.ValidatorIndex, e.ProposalID, e.Message)
}

// HeightError is returned for height-related errors.
type HeightError struct {
	CurrentHeight int64
	TargetHeight  int64
	Message       string
}

func (e *HeightError) Error() string {
	return fmt.Sprintf("height error (current: %d, target: %d): %s", e.CurrentHeight, e.TargetHeight, e.Message)
}

// BinarySwitchError is returned when binary switching fails.
type BinarySwitchError struct {
	OldBinary string
	NewBinary string
	Message   string
}

func (e *BinarySwitchError) Error() string {
	return fmt.Sprintf("failed to switch binary from %s to %s: %s", e.OldBinary, e.NewBinary, e.Message)
}
