package snapshot

import "fmt"

// SnapshotError is returned when snapshot operations fail.
type SnapshotError struct {
	Operation string
	Message   string
}

func (e *SnapshotError) Error() string {
	return fmt.Sprintf("snapshot %s failed: %s", e.Operation, e.Message)
}

// DownloadError is returned when downloading fails.
type DownloadError struct {
	URL     string
	Message string
}

func (e *DownloadError) Error() string {
	return fmt.Sprintf("failed to download %s: %s", e.URL, e.Message)
}

// ExtractError is returned when extraction fails.
type ExtractError struct {
	Path    string
	Message string
}

func (e *ExtractError) Error() string {
	return fmt.Sprintf("failed to extract %s: %s", e.Path, e.Message)
}
