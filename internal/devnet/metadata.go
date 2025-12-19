package devnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/stablelabs/stable-devnet/internal/helpers"
	"github.com/stablelabs/stable-devnet/internal/network"
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
	ChainID           string `json:"chain_id"`                     // e.g., "stable-devnet-1"
	NetworkSource     string `json:"network_source"`               // "mainnet" or "testnet" (snapshot source)
	BlockchainNetwork string `json:"blockchain_network,omitempty"` // Network module: "stable", "ault", etc. (default: "stable")

	// Execution
	ExecutionMode    ExecutionMode `json:"execution_mode"`               // "docker" or "local"
	StableVersion    string        `json:"stable_version"`               // e.g., "v1.2.3" or "feat/branch" (deprecated: use NetworkVersion)
	NetworkVersion   string        `json:"network_version,omitempty"`    // Version for the blockchain network
	IsCustomRef      bool          `json:"is_custom_ref,omitempty"`      // True if built from custom branch/commit
	CustomBinaryPath string        `json:"custom_binary_path,omitempty"` // Path to custom-built binary
	DockerImage      string        `json:"docker_image,omitempty"`       // Docker image used (only for docker mode)

	// Version tracking
	InitialVersion string `json:"initial_version,omitempty"` // Version from exported genesis (e.g., "1.1.3")
	CurrentVersion string `json:"current_version,omitempty"` // Current running version after upgrades

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

	// Provision State (for staged provision → run workflow)
	ProvisionState       ProvisionState `json:"provision_state,omitempty"`        // none, syncing, provisioned, failed
	ProvisionStartedAt   *time.Time     `json:"provision_started_at,omitempty"`   // When provision started
	ProvisionCompletedAt *time.Time     `json:"provision_completed_at,omitempty"` // When provision completed
	ProvisionError       string         `json:"provision_error,omitempty"`        // Error message if failed
	RetryCount           int            `json:"retry_count,omitempty"`            // Number of retries attempted
}

// NewDevnetMetadata creates a new DevnetMetadata with default values.
func NewDevnetMetadata(homeDir string) *DevnetMetadata {
	return &DevnetMetadata{
		ID:                uuid.New().String(),
		Name:              "devnet",
		ChainID:           "stable-devnet-1",
		NetworkSource:     "mainnet",
		BlockchainNetwork: "stable", // Default for backward compatibility
		ExecutionMode:     ModeDocker,
		StableVersion:     "latest",
		NumValidators:     4,
		NumAccounts:       0,
		CreatedAt:         time.Now(),
		HomeDir:           homeDir,
		Status:            StatusCreated,
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

	// Validate ChainID pattern - accepts both devnet format (stable-devnet-1) and
	// forked network format (stable_988-1, stabletestnet_2201-1)
	chainIDPattern := regexp.MustCompile(`^[a-z]+(_\d+-\d+|-devnet-\d+)$`)
	if !chainIDPattern.MatchString(m.ChainID) {
		return fmt.Errorf("chain_id must match Cosmos chain ID format, got '%s'", m.ChainID)
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
// Uses helpers.SaveJSON for consistent file I/O with automatic directory creation.
func (m *DevnetMetadata) Save() error {
	if err := helpers.SaveJSON(m.MetadataPath(), m, 0644); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}
	return nil
}

// LoadDevnetMetadata loads metadata from the specified home directory.
// Uses helpers.LoadJSON for consistent file I/O with structured error handling.
func LoadDevnetMetadata(homeDir string) (*DevnetMetadata, error) {
	metadataPath := filepath.Join(homeDir, "devnet", "metadata.json")

	metadata, err := helpers.LoadJSON[DevnetMetadata](metadataPath)
	if err != nil {
		// Preserve backward-compatible error messages
		var jsonErr *helpers.JSONLoadError
		if errors.As(err, &jsonErr) {
			if jsonErr.Reason == "file not found" {
				return nil, fmt.Errorf("devnet not found at %s", homeDir)
			}
			if jsonErr.Reason == "failed to parse JSON in" {
				return nil, fmt.Errorf("failed to parse metadata: %w", jsonErr.Wrapped)
			}
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	return metadata, nil
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

// SetProvisionStarted marks the devnet as provisioning in progress.
func (m *DevnetMetadata) SetProvisionStarted() {
	now := time.Now()
	m.ProvisionState = ProvisionStateSyncing
	m.ProvisionStartedAt = &now
	m.ProvisionCompletedAt = nil
	m.ProvisionError = ""
}

// SetProvisionComplete marks the devnet as provisioned successfully.
func (m *DevnetMetadata) SetProvisionComplete() {
	now := time.Now()
	m.ProvisionState = ProvisionStateProvisioned
	m.ProvisionCompletedAt = &now
	m.ProvisionError = ""
}

// SetProvisionFailed marks the devnet provisioning as failed.
func (m *DevnetMetadata) SetProvisionFailed(err error) {
	m.ProvisionState = ProvisionStateFailed
	if err != nil {
		m.ProvisionError = err.Error()
	}
}

// IncrementRetryCount increments the retry counter.
func (m *DevnetMetadata) IncrementRetryCount() {
	m.RetryCount++
}

// ResetRetryCount resets the retry counter.
func (m *DevnetMetadata) ResetRetryCount() {
	m.RetryCount = 0
}

// IsProvisioned returns true if the devnet has been provisioned.
func (m *DevnetMetadata) IsProvisioned() bool {
	return m.ProvisionState == ProvisionStateProvisioned
}

// CanRun returns true if the devnet can be started.
func (m *DevnetMetadata) CanRun() bool {
	return m.ProvisionState.CanRun()
}

// GetVersionFromGenesis reads the app_version from genesis.json file.
func GetVersionFromGenesis(genesisPath string) (string, error) {
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		return "", fmt.Errorf("failed to read genesis file: %w", err)
	}

	var genesis struct {
		AppVersion string `json:"app_version"`
	}
	if err := json.Unmarshal(data, &genesis); err != nil {
		return "", fmt.Errorf("failed to parse genesis: %w", err)
	}

	return genesis.AppVersion, nil
}

// SetInitialVersionFromGenesis sets InitialVersion from genesis file.
func (m *DevnetMetadata) SetInitialVersionFromGenesis() error {
	genesisPath := filepath.Join(m.DevnetDir(), "genesis.json")
	version, err := GetVersionFromGenesis(genesisPath)
	if err != nil {
		return err
	}
	m.InitialVersion = version
	m.CurrentVersion = version // Initially same as initial
	return nil
}

// SetCurrentVersion updates the current running version.
func (m *DevnetMetadata) SetCurrentVersion(version string) {
	m.CurrentVersion = version
}

// GetNetworkModule returns the network module for this devnet.
// Returns the default (stable) module if BlockchainNetwork is empty.
func (m *DevnetMetadata) GetNetworkModule() (network.NetworkModule, error) {
	networkName := m.BlockchainNetwork
	if networkName == "" {
		networkName = "stable" // Default for backward compatibility
	}
	return network.Get(networkName)
}

// GetEffectiveVersion returns the effective binary version to use.
// Prefers NetworkVersion if set, otherwise falls back to StableVersion.
func (m *DevnetMetadata) GetEffectiveVersion() string {
	if m.NetworkVersion != "" {
		return m.NetworkVersion
	}
	return m.StableVersion
}
