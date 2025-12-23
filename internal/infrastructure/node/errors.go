package node

import "fmt"

// NodeError is returned when node operations fail.
type NodeError struct {
	NodeIndex int
	Operation string
	Message   string
}

func (e *NodeError) Error() string {
	return fmt.Sprintf("node %d %s failed: %s", e.NodeIndex, e.Operation, e.Message)
}

// NotFoundError is returned when a node is not found.
type NotFoundError struct {
	NodeIndex int
	HomeDir   string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("node %d not found at %s", e.NodeIndex, e.HomeDir)
}

// IsNotFound returns true if the error is a NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// AlreadyRunningError is returned when trying to start an already running node.
type AlreadyRunningError struct {
	NodeIndex int
}

func (e *AlreadyRunningError) Error() string {
	return fmt.Sprintf("node %d is already running", e.NodeIndex)
}
