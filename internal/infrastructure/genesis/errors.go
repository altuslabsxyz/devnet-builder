package genesis

import "fmt"

// GenesisError is returned when genesis operations fail.
type GenesisError struct {
	Operation string
	Message   string
}

func (e *GenesisError) Error() string {
	return fmt.Sprintf("genesis %s failed: %s", e.Operation, e.Message)
}

// FetchError is returned when fetching genesis fails.
type FetchError struct {
	Source  string
	Message string
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("failed to fetch genesis from %s: %s", e.Source, e.Message)
}

// ExportError is returned when exporting genesis fails.
type ExportError struct {
	HomeDir string
	Message string
}

func (e *ExportError) Error() string {
	return fmt.Sprintf("failed to export genesis from %s: %s", e.HomeDir, e.Message)
}
