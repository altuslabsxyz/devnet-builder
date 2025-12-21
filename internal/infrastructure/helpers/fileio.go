package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// JSONLoadError represents an error that occurred while loading JSON.
type JSONLoadError struct {
	Path    string
	Reason  string
	Wrapped error
}

func (e *JSONLoadError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.Reason, e.Path, e.Wrapped)
	}
	return fmt.Sprintf("%s: %s", e.Reason, e.Path)
}

func (e *JSONLoadError) Unwrap() error {
	return e.Wrapped
}

// LoadJSON reads and unmarshals a JSON file into the provided type.
// Returns JSONLoadError with specific reason (not_found, read_error, parse_error).
func LoadJSON[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &JSONLoadError{
				Path:   path,
				Reason: "file not found",
			}
		}
		return nil, &JSONLoadError{
			Path:    path,
			Reason:  "failed to read",
			Wrapped: err,
		}
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &JSONLoadError{
			Path:    path,
			Reason:  "failed to parse JSON in",
			Wrapped: err,
		}
	}

	return &result, nil
}

// SaveJSON marshals data to JSON and writes to path.
// Creates parent directories if they don't exist.
// Uses indented JSON format for readability.
func SaveJSON(path string, data any, perm os.FileMode) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := EnsureDir(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal with indentation for readability
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, jsonData, perm); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

// FileExists checks if a file exists at the given path.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists at the given path.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir creates a directory and all parent directories if they don't exist.
func EnsureDir(path string, perm os.FileMode) error {
	if DirExists(path) {
		return nil
	}
	return os.MkdirAll(path, perm)
}
