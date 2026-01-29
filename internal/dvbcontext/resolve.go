package dvbcontext

import "errors"

// ErrNoDevnet is returned when no devnet is specified and no context is set.
var ErrNoDevnet = errors.New("no devnet specified and no context set. Run 'dvb use <devnet>' to set context")

// NoDevnetError is an error that can carry suggestions for available devnets.
// It satisfies errors.Is(err, ErrNoDevnet) for backward compatibility.
type NoDevnetError struct {
	Suggestion string
}

// Error returns the error message with optional suggestions.
func (e *NoDevnetError) Error() string {
	msg := "no devnet specified and no context set"
	if e.Suggestion != "" {
		return msg + "\n\n" + e.Suggestion
	}
	return msg + ". Run 'dvb use <devnet>' to set context"
}

// Is allows errors.Is(err, ErrNoDevnet) to return true.
func (e *NoDevnetError) Is(target error) bool {
	return target == ErrNoDevnet
}

// NewNoDevnetError creates a NoDevnetError with the given suggestion.
func NewNoDevnetError(suggestion string) *NoDevnetError {
	return &NoDevnetError{Suggestion: suggestion}
}

// Resolve determines the namespace and devnet to use based on:
// 1. Explicit arguments (highest priority)
// 2. Context file (fallback)
// 3. Default namespace "default" if only devnet is known
//
// Returns ErrNoDevnet if devnet cannot be determined.
func Resolve(explicitDevnet, explicitNamespace string, ctx *Context) (namespace, devnet string, err error) {
	namespace = explicitNamespace
	devnet = explicitDevnet

	// Fill from context if not explicitly provided
	if ctx != nil {
		if namespace == "" {
			namespace = ctx.Namespace
		}
		if devnet == "" {
			devnet = ctx.Devnet
		}
	}

	// Apply default namespace if still empty
	if namespace == "" {
		namespace = "default"
	}

	// Error if devnet still not known
	if devnet == "" {
		return "", "", ErrNoDevnet
	}

	return namespace, devnet, nil
}
