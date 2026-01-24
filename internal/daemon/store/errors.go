// internal/daemon/store/errors.go
package store

import (
	"errors"
	"fmt"
)

// Sentinel errors for simple checks.
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
)

// NotFoundError is returned when a resource is not found.
type NotFoundError struct {
	Resource string
	Name     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.Name)
}

// IsNotFound returns true if err is a NotFoundError or the sentinel ErrNotFound.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var notFound *NotFoundError
	return errors.As(err, &notFound)
}

// ConflictError is returned on optimistic concurrency conflicts.
type ConflictError struct {
	Resource string
	Name     string
	Message  string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict updating %s %q: %s", e.Resource, e.Name, e.Message)
}

// IsConflict returns true if err is a ConflictError.
func IsConflict(err error) bool {
	var conflict *ConflictError
	return errors.As(err, &conflict)
}

// AlreadyExistsError is returned when creating a resource that already exists.
type AlreadyExistsError struct {
	Resource string
	Name     string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s %q already exists", e.Resource, e.Name)
}

// IsAlreadyExists returns true if err is an AlreadyExistsError or the sentinel ErrAlreadyExists.
func IsAlreadyExists(err error) bool {
	if errors.Is(err, ErrAlreadyExists) {
		return true
	}
	var alreadyExists *AlreadyExistsError
	return errors.As(err, &alreadyExists)
}
