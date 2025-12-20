package cache

import "fmt"

// CacheError is returned when cache operations fail.
type CacheError struct {
	Operation string
	Message   string
}

func (e *CacheError) Error() string {
	return fmt.Sprintf("cache %s failed: %s", e.Operation, e.Message)
}

// NotActiveError is returned when no active binary is set.
type NotActiveError struct {
	BinaryName string
}

func (e *NotActiveError) Error() string {
	return fmt.Sprintf("no active binary set for %s", e.BinaryName)
}

// IsNotActive returns true if the error is a NotActiveError.
func IsNotActive(err error) bool {
	_, ok := err.(*NotActiveError)
	return ok
}
