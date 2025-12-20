// Package persistence provides file-based storage implementations.
package persistence

import "fmt"

// NotFoundError is returned when a resource is not found.
type NotFoundError struct {
	Path string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("resource not found at path: %s", e.Path)
}

// IsNotFound returns true if the error is a NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// WriteError is returned when writing fails.
type WriteError struct {
	Path    string
	Message string
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("failed to write to %s: %s", e.Path, e.Message)
}

// ReadError is returned when reading fails.
type ReadError struct {
	Path    string
	Message string
}

func (e *ReadError) Error() string {
	return fmt.Sprintf("failed to read from %s: %s", e.Path, e.Message)
}
