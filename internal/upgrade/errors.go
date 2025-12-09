package upgrade

import (
	"errors"
	"fmt"
)

// Base errors for upgrade operations.
var (
	ErrDevnetNotRunning     = errors.New("devnet is not running")
	ErrNoValidators         = errors.New("no validators found")
	ErrInsufficientBalance  = errors.New("insufficient balance for deposit")
	ErrProposalFailed       = errors.New("proposal submission failed")
	ErrVotingFailed         = errors.New("voting failed")
	ErrUpgradeTimeout       = errors.New("upgrade timeout")
	ErrChainNotResumed      = errors.New("chain did not resume after upgrade")
	ErrInvalidConfig        = errors.New("invalid upgrade configuration: name is required")
	ErrNoTargetBinary       = errors.New("either --image or --binary must be specified")
	ErrBothTargetsDefined   = errors.New("--image and --binary are mutually exclusive")
	ErrVotingPeriodTooShort = errors.New("voting period must be at least 30 seconds")
	ErrHeightBufferTooSmall = errors.New("height buffer must be at least 5 blocks")
	ErrDockerNotAvailable   = errors.New("docker is not available")
	ErrBinaryNotFound       = errors.New("binary not found")
	ErrKeyExportFailed      = errors.New("failed to export validator keys")
	ErrRPCError             = errors.New("RPC request failed")
	ErrGenesisExportFailed  = errors.New("genesis export failed")
)

// UpgradeError wraps an error with upgrade context.
type UpgradeError struct {
	Stage      UpgradeStage
	Operation  string
	Err        error
	Suggestion string
}

func (e *UpgradeError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("[%s] %s: %v\nHint: %s", e.Stage, e.Operation, e.Err, e.Suggestion)
	}
	return fmt.Sprintf("[%s] %s: %v", e.Stage, e.Operation, e.Err)
}

func (e *UpgradeError) Unwrap() error {
	return e.Err
}

// WrapError creates an UpgradeError with context.
func WrapError(stage UpgradeStage, operation string, err error, suggestion string) *UpgradeError {
	return &UpgradeError{
		Stage:      stage,
		Operation:  operation,
		Err:        err,
		Suggestion: suggestion,
	}
}

// ErrorWithSuggestion returns common errors with recovery suggestions.
func ErrorWithSuggestion(err error) string {
	switch {
	case errors.Is(err, ErrDevnetNotRunning):
		return "Start devnet with 'devnet-builder start' first"
	case errors.Is(err, ErrInsufficientBalance):
		return "Restart devnet with higher validator balance or fund validators"
	case errors.Is(err, ErrDockerNotAvailable):
		return "Install and start Docker, or use --binary for local binary mode"
	case errors.Is(err, ErrBinaryNotFound):
		return "Verify the binary path exists and is executable"
	case errors.Is(err, ErrUpgradeTimeout):
		return "Check node logs with 'devnet-builder logs -f' for issues"
	case errors.Is(err, ErrChainNotResumed):
		return "The upgrade handler may not exist in the new binary. Verify upgrade name matches."
	case errors.Is(err, ErrProposalFailed):
		return "Check validator balance and network connectivity"
	case errors.Is(err, ErrVotingFailed):
		return "Some votes may have failed. Check individual validator status."
	default:
		return ""
	}
}
