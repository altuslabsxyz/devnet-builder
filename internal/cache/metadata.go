package cache

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadMetadata reads cache metadata from a JSON file.
func ReadMetadata(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Validate required fields
	if metadata.CommitHash == "" {
		return nil, fmt.Errorf("metadata missing commit_hash")
	}

	return &metadata, nil
}

// WriteMetadata writes cache metadata to a JSON file.
func WriteMetadata(path string, metadata *Metadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}
