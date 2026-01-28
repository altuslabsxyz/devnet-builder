// internal/daemon/server/ante/errors.go
package ante

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error codes for validation failures.
const (
	CodeRequired          = "required"
	CodeInvalidRange      = "invalid_range"
	CodeInvalidValue      = "invalid_value"
	CodeInvalidFormat     = "invalid_format"
	CodeNotFound          = "not_found"
	CodeMutuallyExclusive = "mutually_exclusive"
	CodeValidationFailed  = "validation_failed"
)

// ValidationError represents a single validation failure.
type ValidationError struct {
	Field   string
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// GRPCCode returns the appropriate gRPC status code.
func (e *ValidationError) GRPCCode() codes.Code {
	switch e.Code {
	case CodeNotFound:
		return codes.NotFound
	default:
		return codes.InvalidArgument
	}
}

// MultiValidationError collects multiple validation failures.
type MultiValidationError struct {
	Errors []*ValidationError
}

func (e *MultiValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("  - %s", err.Error()))
	}
	return fmt.Sprintf("validation failed:\n%s", strings.Join(msgs, "\n"))
}

// GRPCCode returns InvalidArgument for multiple errors.
func (e *MultiValidationError) GRPCCode() codes.Code {
	return codes.InvalidArgument
}

// ToGRPCError converts validation errors to gRPC status errors.
func ToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	switch e := err.(type) {
	case *ValidationError:
		return status.Error(e.GRPCCode(), e.Error())
	case *MultiValidationError:
		return status.Error(e.GRPCCode(), e.Error())
	default:
		return status.Error(codes.InvalidArgument, err.Error())
	}
}

// toError converts a slice of validation errors to a single error.
func toError(errs []*ValidationError) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return &MultiValidationError{Errors: errs}
}
