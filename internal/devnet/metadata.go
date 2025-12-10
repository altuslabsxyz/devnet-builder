package devnet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// ExecutionMode defines how nodes are executed.
type ExecutionMode string

const (
	ModeDocker ExecutionMode = "docker"
	ModeLocal  ExecutionMode = "local"
)

// DevnetStatus represents the current state of a devnet.
type DevnetStatus string

const (
	StatusCreated DevnetStatus = "created"
	StatusRunning DevnetStatus = "running"
	StatusStopped DevnetStatus = "stopped"
	StatusError   DevnetStatus = "error"
)

// DevnetMetadata contains all configuration and state for a devnet instance.
type DevnetMetadata struct {
	// Identification
	ID   string `json:"id"`   // UUID for this devnet instance
	Name string `json:"name"` // Human-readable name (default: "devnet")

	// Network Configuration
	ChainID       string `json:"chain_id"`        // e.g., "stable-devnet-1"
	NetworkSource string `json:"network_source"`  // "mainnet" or "testnet"

	// Execution
	ExecutionMode    ExecutionMode `json:"execution_mode"`              // "docker" or "local"
	StableVersion    string        `json:"stable_version"`              // e.g., "v1.2.3" or "feat/branch"
	IsCustomRef      bool          `json:"is_custom_ref,omitempty"`     // True if built from custom branch/commit
	CustomBinaryPath string        `json:"custom_binary_path,omitempty"` // Path to custom-built binary

	// Validators
	NumValidators int `json:"num_validators"` // 1-4
	NumAccounts   int `json:"num_accounts"`   // Additional funded accounts

	// Timestamps
	CreatedAt time.Time  `json:"created_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`

	// Paths
	HomeDir     string `json:"home_dir"`     // Base directory for this devnet
	GenesisPath string `json:"genesis_path"` // Path to genesis.json

	// State
	Status DevnetStatus `json:"status"` // "created", "running", "stopped", "error"
}

// NewDevnetMetadata creates a new DevnetMetadata with default values.
func NewDevnetMetadata(homeDir string) *DevnetMetadata {
	return &DevnetMetadata{
		ID:            uuid.New().String(),
		Name:          "devnet",
		ChainID:       "stable-devnet-1",
		NetworkSource: "mainnet",
		ExecutionMode: ModeDocker,
		StableVersion: "latest",
		NumValidators: 4,
		NumAccounts:   0,
		CreatedAt:     time.Now(),
		HomeDir:       homeDir,
		Status:        StatusCreated,
	}
}

// Validate checks if the metadata is valid.
func (m *DevnetMetadata) Validate() error {
	// Validate NumValidators
	if m.NumValidators < 1 || m.NumValidators > 4 {
		return fmt.Errorf("num_validators must be between 1 and 4, got %d", m.NumValidators)
	}

	// Validate NetworkSource
	if m.NetworkSource != "mainnet" && m.NetworkSource != "testnet" {
		return fmt.Errorf("network_source must be 'mainnet' or 'testnet', got '%s'", m.NetworkSource)
	}

	// Validate ExecutionMode
	if m.ExecutionMode != ModeDocker && m.ExecutionMode != ModeLocal {
		return fmt.Errorf("execution_mode must be 'docker' or 'local', got '%s'", m.ExecutionMode)
	}

	// Validate ChainID pattern
	chainIDPattern := regexp.MustCompile(`^[a-z]+-devnet-\d+$`)
	if !chainIDPattern.MatchString(m.ChainID) {
		return fmt.Errorf("chain_id must match pattern '^[a-z]+-devnet-\\d+$', got '%s'", m.ChainID)
	}

	return nil
}

// DevnetDir returns the path to the devnet directory.
func (m *DevnetMetadata) DevnetDir() string {
	return filepath.Join(m.HomeDir, "devnet")
}

// NodeDir returns the path to a specific node's directory.
func (m *DevnetMetadata) NodeDir(index int) string {
	return filepath.Join(m.DevnetDir(), fmt.Sprintf("node%d", index))
}

// MetadataPath returns the path to the metadata.json file.
func (m *DevnetMetadata) MetadataPath() string {
	return filepath.Join(m.DevnetDir(), "metadata.json")
}

// Save persists the metadata to disk.
func (m *DevnetMetadata) Save() error {
	// Ensure directory exists
	if err := os.MkdirAll(m.DevnetDir(), 0755); err != nil {
		return fmt.Errorf("failed to create devnet directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(m.MetadataPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// LoadDevnetMetadata loads metadata from the specified home directory.
func LoadDevnetMetadata(homeDir string) (*DevnetMetadata, error) {
	metadataPath := filepath.Join(homeDir, "devnet", "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("devnet not found at %s", homeDir)
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata DevnetMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// DevnetExists checks if a devnet exists in the specified home directory.
func DevnetExists(homeDir string) bool {
	metadataPath := filepath.Join(homeDir, "devnet", "metadata.json")
	_, err := os.Stat(metadataPath)
	return err == nil
}

// SetRunning marks the devnet as running.
func (m *DevnetMetadata) SetRunning() {
	now := time.Now()
	m.StartedAt = &now
	m.StoppedAt = nil
	m.Status = StatusRunning
}

// SetStopped marks the devnet as stopped.
func (m *DevnetMetadata) SetStopped() {
	now := time.Now()
	m.StoppedAt = &now
	m.Status = StatusStopped
}

// SetError marks the devnet as having an error.
func (m *DevnetMetadata) SetError() {
	m.Status = StatusError
}

// IsRunning returns true if the devnet is in running state.
func (m *DevnetMetadata) IsRunning() bool {
	return m.Status == StatusRunning
}
