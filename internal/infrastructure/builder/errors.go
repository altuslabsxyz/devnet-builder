package builder

import "fmt"

// BuilderError is returned when builder operations fail.
type BuilderError struct {
	Operation string
	Message   string
}

func (e *BuilderError) Error() string {
	return fmt.Sprintf("builder %s failed: %s", e.Operation, e.Message)
}

// NoModuleError is returned when no network module is configured.
type NoModuleError struct{}

func (e *NoModuleError) Error() string {
	return "no network module configured"
}
