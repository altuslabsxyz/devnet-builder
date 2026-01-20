package export

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/altuslabsxyz/devnet-builder/types"
)

// ExportMetadata contains comprehensive information about an exported state
type ExportMetadata struct {
	ExportTimestamp  time.Time `json:"export_timestamp"`
	BlockHeight      int64     `json:"block_height"`
	NetworkSource    string    `json:"network_source"`
	ForkHeight       int64     `json:"fork_height"`
	BinaryPath       string    `json:"binary_path"`
	BinaryHash       string    `json:"binary_hash"`
	BinaryHashPrefix string    `json:"binary_hash_prefix"`
	BinaryVersion    string    `json:"binary_version"`
	DockerImage      string    `json:"docker_image"`
	ChainID          string    `json:"chain_id"`
	NumValidators    int       `json:"num_validators"`
	NumAccounts      int       `json:"num_accounts"`
	ExecutionMode    string    `json:"execution_mode"`
	DevnetHomeDir    string    `json:"devnet_home_dir"`
}

// NewExportMetadata creates a new ExportMetadata instance
func NewExportMetadata(
	timestamp time.Time,
	height int64,
	network string,
	forkHeight int64,
	binaryPath string,
	binaryHash string,
	binaryVersion string,
	dockerImage string,
	chainID string,
	numValidators int,
	numAccounts int,
	executionMode string,
	homeDir string,
) (*ExportMetadata, error) {
	hashPrefix := ""
	if len(binaryHash) >= 8 {
		hashPrefix = binaryHash[0:8]
	}

	em := &ExportMetadata{
		ExportTimestamp:  timestamp,
		BlockHeight:      height,
		NetworkSource:    network,
		ForkHeight:       forkHeight,
		BinaryPath:       binaryPath,
		BinaryHash:       binaryHash,
		BinaryHashPrefix: hashPrefix,
		BinaryVersion:    binaryVersion,
		DockerImage:      dockerImage,
		ChainID:          chainID,
		NumValidators:    numValidators,
		NumAccounts:      numAccounts,
		ExecutionMode:    executionMode,
		DevnetHomeDir:    homeDir,
	}

	if err := em.Validate(); err != nil {
		return nil, err
	}

	return em, nil
}

// Validate checks if the ExportMetadata is valid
func (em *ExportMetadata) Validate() error {
	if em.BlockHeight <= 0 {
		return NewValidationError("BlockHeight", "must be greater than 0")
	}

	// Use canonical type validation instead of string literals
	networkSource := types.NetworkSource(em.NetworkSource)
	if !networkSource.IsValid() {
		return NewValidationError("NetworkSource", "must be 'mainnet' or 'testnet'")
	}

	execMode := types.ExecutionMode(em.ExecutionMode)
	if !execMode.IsValid() {
		return NewValidationError("ExecutionMode", "must be 'local' or 'docker'")
	}

	if em.BinaryPath == "" && em.DockerImage == "" {
		return NewValidationError("BinaryPath/DockerImage", "exactly one must be set")
	}
	if em.BinaryPath != "" && em.DockerImage != "" {
		return NewValidationError("BinaryPath/DockerImage", "cannot set both")
	}

	if execMode == types.ExecutionModeLocal && em.BinaryPath == "" {
		return NewValidationError("BinaryPath", "must be set when ExecutionMode is 'local'")
	}
	if execMode == types.ExecutionModeDocker && em.DockerImage == "" {
		return NewValidationError("DockerImage", "must be set when ExecutionMode is 'docker'")
	}

	if em.BinaryHash != "" {
		hashRegex := regexp.MustCompile(`^[a-f0-9]{64}$`)
		if !hashRegex.MatchString(em.BinaryHash) {
			return NewValidationError("BinaryHash", "must be 64 hexadecimal characters")
		}
	}

	if em.BinaryHash != "" && len(em.BinaryHash) >= 8 {
		if em.BinaryHashPrefix != em.BinaryHash[0:8] {
			return NewValidationError("BinaryHashPrefix", "must match first 8 characters of BinaryHash")
		}
	}

	if em.BinaryVersion == "" {
		return NewValidationError("BinaryVersion", "cannot be empty")
	}
	if em.ChainID == "" {
		return NewValidationError("ChainID", "cannot be empty")
	}
	if em.NumValidators <= 0 {
		return NewValidationError("NumValidators", "must be greater than 0")
	}
	if em.NumAccounts < 0 {
		return NewValidationError("NumAccounts", "cannot be negative")
	}
	if em.DevnetHomeDir == "" {
		return NewValidationError("DevnetHomeDir", "cannot be empty")
	}

	return nil
}

// ToJSON serializes the metadata to JSON
func (em *ExportMetadata) ToJSON() ([]byte, error) {
	return json.MarshalIndent(em, "", "  ")
}

// FromJSON deserializes metadata from JSON
func FromJSON(data []byte) (*ExportMetadata, error) {
	var em ExportMetadata
	if err := json.Unmarshal(data, &em); err != nil {
		return nil, err
	}

	if err := em.Validate(); err != nil {
		return nil, err
	}

	return &em, nil
}
