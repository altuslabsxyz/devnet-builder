package export

import (
	"fmt"
	"regexp"

	"github.com/altuslabsxyz/devnet-builder/types"
)

// BinaryInfo contains information about the blockchain binary used for export
type BinaryInfo struct {
	Path          string              // Full path to binary (local mode)
	DockerImage   string              // Docker image name (docker mode)
	Hash          string              // SHA256 hash of binary/image
	HashPrefix    string              // First 8 characters of hash
	Version       string              // Binary version identifier
	ExecutionMode types.ExecutionMode // local or docker
}

// NewBinaryInfo creates a new BinaryInfo instance
func NewBinaryInfo(path, dockerImage, hash, version string, mode types.ExecutionMode) (*BinaryInfo, error) {
	bi := &BinaryInfo{
		Path:          path,
		DockerImage:   dockerImage,
		Hash:          hash,
		Version:       version,
		ExecutionMode: mode,
	}

	// Generate hash prefix
	if len(hash) >= 8 {
		bi.HashPrefix = hash[0:8]
	}

	if err := bi.Validate(); err != nil {
		return nil, err
	}

	return bi, nil
}

// Validate checks if the BinaryInfo is valid
func (b *BinaryInfo) Validate() error {
	// Validate execution mode
	if b.ExecutionMode != types.ExecutionModeLocal && b.ExecutionMode != types.ExecutionModeDocker {
		return NewValidationError("ExecutionMode", "must be 'local' or 'docker'")
	}

	// Exactly one of Path or DockerImage must be set
	if b.Path == "" && b.DockerImage == "" {
		return NewValidationError("Path/DockerImage", "exactly one of Path or DockerImage must be set")
	}
	if b.Path != "" && b.DockerImage != "" {
		return NewValidationError("Path/DockerImage", "cannot set both Path and DockerImage")
	}

	// Validate execution mode matches path/image
	if b.ExecutionMode == types.ExecutionModeLocal && b.Path == "" {
		return NewValidationError("Path", "Path must be set when ExecutionMode is 'local'")
	}
	if b.ExecutionMode == types.ExecutionModeDocker && b.DockerImage == "" {
		return NewValidationError("DockerImage", "DockerImage must be set when ExecutionMode is 'docker'")
	}

	// Validate hash format (64 hex characters)
	if b.Hash != "" {
		hashRegex := regexp.MustCompile(`^[a-f0-9]{64}$`)
		if !hashRegex.MatchString(b.Hash) {
			return NewValidationError("Hash", "must be 64 hexadecimal characters")
		}
	}

	// Validate hash prefix
	if b.Hash != "" && len(b.Hash) >= 8 {
		if b.HashPrefix != b.Hash[0:8] {
			return NewValidationError("HashPrefix", "must match first 8 characters of Hash")
		}
	}

	// Validate version is not empty
	if b.Version == "" {
		return NewValidationError("Version", "Version cannot be empty")
	}

	return nil
}

// GetIdentifier returns a unique identifier for this binary (for naming)
func (b *BinaryInfo) GetIdentifier() string {
	if b.HashPrefix != "" {
		return b.HashPrefix
	}
	// Fallback to version if hash prefix not available
	return fmt.Sprintf("v%s", b.Version)
}
