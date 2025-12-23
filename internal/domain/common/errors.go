// Package common provides shared domain types and value objects.
package common

// =============================================================================
// Error Behavior Interfaces (Interface Segregation Principle)
// =============================================================================
// These interfaces define error behaviors that can be checked by the presentation
// layer to determine how to handle errors. This follows Clean Architecture by
// keeping error handling logic in the domain layer while allowing the presentation
// layer to make decisions based on error characteristics.

// SilenceUsageError is implemented by errors that should NOT trigger
// CLI usage information. This is useful for operational errors where
// the user's command syntax was correct, but something else failed.
//
// Examples:
//   - Authentication failures (user knows the command, just needs a token)
//   - Network connectivity issues
//   - Resource not found (but the command was used correctly)
//
// Usage in Cobra command:
//
//	if _, ok := err.(SilenceUsageError); ok && err.ShouldSilenceUsage() {
//	    cmd.SilenceUsage = true
//	}
type SilenceUsageError interface {
	error
	ShouldSilenceUsage() bool
}

// UserFacingError is implemented by errors that have a user-friendly
// message that should be displayed directly to the user.
//
// This allows infrastructure errors to provide guidance on how to fix
// the issue (e.g., "run this command to configure authentication").
type UserFacingError interface {
	error
	UserMessage() string
}

// RecoverableError is implemented by errors that suggest a recovery action.
// The presentation layer can use this to provide actionable guidance.
type RecoverableError interface {
	error
	RecoveryHint() string
}

// =============================================================================
// Error Checking Utilities
// =============================================================================

// ShouldSilenceUsage checks if an error should silence CLI usage output.
// Returns true if the error implements SilenceUsageError and returns true.
func ShouldSilenceUsage(err error) bool {
	if err == nil {
		return false
	}
	if sue, ok := err.(SilenceUsageError); ok {
		return sue.ShouldSilenceUsage()
	}
	// Check wrapped errors
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		return ShouldSilenceUsage(unwrapper.Unwrap())
	}
	return false
}

// GetUserMessage extracts a user-friendly message from an error.
// Returns the UserMessage if the error implements UserFacingError,
// otherwise returns the standard Error() message.
func GetUserMessage(err error) string {
	if err == nil {
		return ""
	}
	if ufe, ok := err.(UserFacingError); ok {
		return ufe.UserMessage()
	}
	// Check wrapped errors
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		if msg := GetUserMessage(unwrapper.Unwrap()); msg != "" {
			return msg
		}
	}
	return err.Error()
}

// GetRecoveryHint extracts a recovery hint from an error.
// Returns empty string if no hint is available.
func GetRecoveryHint(err error) string {
	if err == nil {
		return ""
	}
	if re, ok := err.(RecoverableError); ok {
		return re.RecoveryHint()
	}
	// Check wrapped errors
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		return GetRecoveryHint(unwrapper.Unwrap())
	}
	return ""
}
