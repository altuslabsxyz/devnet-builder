package process

import "fmt"

// ExecutionError is returned when process execution fails.
type ExecutionError struct {
	Operation string
	Message   string
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("process %s failed: %s", e.Operation, e.Message)
}

// NotRunningError is returned when a process is not running.
type NotRunningError struct {
	PID int
}

func (e *NotRunningError) Error() string {
	return fmt.Sprintf("process %d is not running", e.PID)
}

// ContainerError is returned when Docker container operations fail.
type ContainerError struct {
	ContainerID string
	Operation   string
	Message     string
}

func (e *ContainerError) Error() string {
	return fmt.Sprintf("container %s %s failed: %s", e.ContainerID, e.Operation, e.Message)
}

// ImageError is returned when Docker image operations fail.
type ImageError struct {
	Image   string
	Message string
}

func (e *ImageError) Error() string {
	return fmt.Sprintf("docker image %s: %s", e.Image, e.Message)
}
