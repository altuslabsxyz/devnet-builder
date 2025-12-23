package rpc

import "fmt"

// RPCError is returned when RPC operations fail.
type RPCError struct {
	Operation string
	Message   string
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC %s failed: %s", e.Operation, e.Message)
}

// NotFoundError is returned when a resource is not found.
type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("resource not found: %s", e.Resource)
}

// IsNotFound returns true if the error is a NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// ConnectionError is returned when connection fails.
type ConnectionError struct {
	Endpoint string
	Message  string
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("failed to connect to %s: %s", e.Endpoint, e.Message)
}

// TimeoutError is returned when an operation times out.
type TimeoutError struct {
	Operation string
	Duration  string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s timed out after %s", e.Operation, e.Duration)
}
