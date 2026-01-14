package export

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Export represents a complete blockchain state export with metadata
type Export struct {
	DirectoryPath   string
	ExportTimestamp time.Time
	BlockHeight     int64
	NetworkSource   string
	BinaryInfo      *BinaryInfo
	Metadata        *ExportMetadata
	GenesisFilePath string
}

// NewExport creates a new Export instance
func NewExport(
	directoryPath string,
	timestamp time.Time,
	height int64,
	network string,
	binaryInfo *BinaryInfo,
	metadata *ExportMetadata,
	genesisPath string,
) (*Export, error) {
	e := &Export{
		DirectoryPath:   directoryPath,
		ExportTimestamp: timestamp,
		BlockHeight:     height,
		NetworkSource:   network,
		BinaryInfo:      binaryInfo,
		Metadata:        metadata,
		GenesisFilePath: genesisPath,
	}

	if err := e.Validate(); err != nil {
		return nil, err
	}

	return e, nil
}

// Validate checks if the Export is complete and valid
func (e *Export) Validate() error {
	if e.DirectoryPath == "" {
		return NewValidationError("DirectoryPath", "cannot be empty")
	}

	dirName := filepath.Base(e.DirectoryPath)
	if !isValidDirectoryName(dirName) {
		return NewValidationError("DirectoryPath", fmt.Sprintf("directory name '%s' does not follow naming convention", dirName))
	}

	if e.BlockHeight <= 0 {
		return NewValidationError("BlockHeight", "must be greater than 0")
	}

	if e.NetworkSource != "mainnet" && e.NetworkSource != "testnet" {
		return NewValidationError("NetworkSource", "must be 'mainnet' or 'testnet'")
	}

	if e.BinaryInfo == nil {
		return NewValidationError("BinaryInfo", "cannot be nil")
	}
	if err := e.BinaryInfo.Validate(); err != nil {
		return fmt.Errorf("BinaryInfo validation failed: %w", err)
	}

	if e.Metadata == nil {
		return NewValidationError("Metadata", "cannot be nil")
	}
	if err := e.Metadata.Validate(); err != nil {
		return fmt.Errorf("Metadata validation failed: %w", err)
	}

	if e.GenesisFilePath == "" {
		return NewValidationError("GenesisFilePath", "cannot be empty")
	}

	return nil
}

// DirectoryName returns the formatted directory name
func (e *Export) DirectoryName() string {
	return GenerateDirectoryName(e.NetworkSource, e.BinaryInfo.GetIdentifier(), e.BlockHeight, e.ExportTimestamp)
}

// GenerateDirectoryName creates a directory name from the given components
// This can be called without creating a full Export object
func GenerateDirectoryName(network string, binaryIdentifier string, height int64, timestamp time.Time) string {
	ts := timestamp.Format("20060102150405")
	return fmt.Sprintf("%s-%s-%d-%s", network, binaryIdentifier, height, ts)
}

// IsComplete checks if all required files exist
func (e *Export) IsComplete() bool {
	if _, err := os.Stat(e.DirectoryPath); os.IsNotExist(err) {
		return false
	}

	metadataPath := filepath.Join(e.DirectoryPath, "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return false
	}

	if _, err := os.Stat(e.GenesisFilePath); os.IsNotExist(err) {
		return false
	}

	return true
}

// GetMetadataPath returns the path to the metadata.json file
func (e *Export) GetMetadataPath() string {
	return filepath.Join(e.DirectoryPath, "metadata.json")
}

// isValidDirectoryName validates export directory name format
func isValidDirectoryName(name string) bool {
	pattern := `^(mainnet|testnet)-[a-f0-9]{8}-[1-9][0-9]*-\d{14}(-[1-9][0-9]*)?$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

// GetGenesisFileName returns the expected genesis file name
func (e *Export) GetGenesisFileName() string {
	return GenerateGenesisFileName(e.BlockHeight, e.BinaryInfo.GetIdentifier())
}

// GenerateGenesisFileName creates a genesis file name from the given components
// This can be called without creating a full Export object
func GenerateGenesisFileName(height int64, binaryIdentifier string) string {
	return fmt.Sprintf("genesis-%d-%s.json", height, binaryIdentifier)
}
