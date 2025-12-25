package export

import (
	"errors"
	"fmt"
)

// Domain errors for export operations
var (
	ErrExportNotFound       = errors.New("export not found")
	ErrExportIncomplete     = errors.New("export is incomplete")
	ErrInvalidMetadata      = errors.New("invalid export metadata")
	ErrDirectoryExists      = errors.New("export directory already exists")
	ErrInvalidDirectoryName = errors.New("export directory name is invalid")
)

// ValidationError represents a field-specific validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
