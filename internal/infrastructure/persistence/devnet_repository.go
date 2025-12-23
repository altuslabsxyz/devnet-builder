// Package persistence provides file-based storage implementations.
package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// DevnetFileRepository implements DevnetRepository using the filesystem.
type DevnetFileRepository struct {
	metadataFilename string
}

// NewDevnetFileRepository creates a new DevnetFileRepository.
func NewDevnetFileRepository() *DevnetFileRepository {
	return &DevnetFileRepository{
		metadataFilename: "metadata.json",
	}
}

// devnetDir returns the devnet directory path.
func (r *DevnetFileRepository) devnetDir(homeDir string) string {
	return filepath.Join(homeDir, "devnet")
}

// metadataPath returns the path to the metadata file.
func (r *DevnetFileRepository) metadataPath(homeDir string) string {
	return filepath.Join(r.devnetDir(homeDir), r.metadataFilename)
}

// Save persists the devnet metadata to storage.
func (r *DevnetFileRepository) Save(ctx context.Context, metadata *ports.DevnetMetadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata is nil")
	}

	dir := r.devnetDir(metadata.HomeDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create devnet directory: %w", err)
	}

	// Convert to storage format
	stored := r.toStoredFormat(metadata)

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	path := r.metadataPath(metadata.HomeDir)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// Load retrieves devnet metadata from storage.
func (r *DevnetFileRepository) Load(ctx context.Context, homeDir string) (*ports.DevnetMetadata, error) {
	path := r.metadataPath(homeDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Path: homeDir}
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var stored storedDevnetMetadata
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return r.fromStoredFormat(&stored), nil
}

// Delete removes all devnet data from storage.
func (r *DevnetFileRepository) Delete(ctx context.Context, homeDir string) error {
	dir := r.devnetDir(homeDir)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to delete devnet directory: %w", err)
	}
	return nil
}

// Exists checks if a devnet exists at the given path.
func (r *DevnetFileRepository) Exists(homeDir string) bool {
	path := r.metadataPath(homeDir)
	_, err := os.Stat(path)
	return err == nil
}

// storedDevnetMetadata is the JSON storage format.
type storedDevnetMetadata struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	HomeDir           string  `json:"home_dir"`
	ChainID           string  `json:"chain_id"`
	NetworkName       string  `json:"network_name,omitempty"`
	NetworkSource     string  `json:"network_source,omitempty"`
	BlockchainNetwork string  `json:"blockchain_network,omitempty"`
	NetworkVersion    string  `json:"network_version,omitempty"`
	NumValidators     int     `json:"num_validators"`
	NumAccounts       int     `json:"num_accounts"`
	ExecutionMode     string  `json:"execution_mode"`
	Status            string  `json:"status"`
	ProvisionState    string  `json:"provision_state,omitempty"`
	DockerImage       string  `json:"docker_image,omitempty"`
	CustomBinaryPath  string  `json:"custom_binary_path,omitempty"`
	BinaryName        string  `json:"binary_name,omitempty"`
	GenesisPath       string  `json:"genesis_path,omitempty"`
	InitialVersion    string  `json:"initial_version,omitempty"`
	CurrentVersion    string  `json:"current_version,omitempty"`
	CreatedAt         string  `json:"created_at"`
	LastProvisioned   *string `json:"last_provisioned,omitempty"`
	LastStarted       *string `json:"last_started,omitempty"`
	LastStopped       *string `json:"last_stopped,omitempty"`
}

// toStoredFormat converts ports.DevnetMetadata to storage format.
func (r *DevnetFileRepository) toStoredFormat(m *ports.DevnetMetadata) *storedDevnetMetadata {
	stored := &storedDevnetMetadata{
		HomeDir:           m.HomeDir,
		ChainID:           m.ChainID,
		NetworkName:       m.NetworkName,
		NetworkSource:     m.NetworkName, // Backward compatibility
		BlockchainNetwork: m.BlockchainNetwork,
		NetworkVersion:    m.NetworkVersion,
		NumValidators:     m.NumValidators,
		NumAccounts:       m.NumAccounts,
		ExecutionMode:     string(m.ExecutionMode),
		Status:            string(m.Status),
		DockerImage:       m.DockerImage,
		CustomBinaryPath:  m.CustomBinaryPath,
		BinaryName:        m.BinaryName,
		GenesisPath:       m.GenesisPath,
		InitialVersion:    m.InitialVersion,
		CurrentVersion:    m.CurrentVersion,
		CreatedAt:         m.CreatedAt.Format(time.RFC3339),
	}

	if m.LastProvisioned != nil {
		t := m.LastProvisioned.Format(time.RFC3339)
		stored.LastProvisioned = &t
	}
	if m.LastStarted != nil {
		t := m.LastStarted.Format(time.RFC3339)
		stored.LastStarted = &t
	}
	if m.LastStopped != nil {
		t := m.LastStopped.Format(time.RFC3339)
		stored.LastStopped = &t
	}

	return stored
}

// fromStoredFormat converts storage format to ports.DevnetMetadata.
func (r *DevnetFileRepository) fromStoredFormat(s *storedDevnetMetadata) *ports.DevnetMetadata {
	// Handle backward compatibility: NetworkSource -> NetworkName
	networkName := s.NetworkName
	if networkName == "" && s.NetworkSource != "" {
		networkName = s.NetworkSource
	}

	m := &ports.DevnetMetadata{
		HomeDir:           s.HomeDir,
		ChainID:           s.ChainID,
		NetworkName:       networkName,
		BlockchainNetwork: s.BlockchainNetwork,
		NetworkVersion:    s.NetworkVersion,
		NumValidators:     s.NumValidators,
		NumAccounts:       s.NumAccounts,
		ExecutionMode:     ports.ExecutionMode(s.ExecutionMode),
		Status:            ports.DevnetState(s.Status),
		DockerImage:       s.DockerImage,
		CustomBinaryPath:  s.CustomBinaryPath,
		BinaryName:        s.BinaryName,
		GenesisPath:       s.GenesisPath,
		InitialVersion:    s.InitialVersion,
		CurrentVersion:    s.CurrentVersion,
	}

	if t, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
		m.CreatedAt = t
	}
	if s.LastProvisioned != nil {
		if t, err := time.Parse(time.RFC3339, *s.LastProvisioned); err == nil {
			m.LastProvisioned = &t
		}
	}
	if s.LastStarted != nil {
		if t, err := time.Parse(time.RFC3339, *s.LastStarted); err == nil {
			m.LastStarted = &t
		}
	}
	if s.LastStopped != nil {
		if t, err := time.Parse(time.RFC3339, *s.LastStopped); err == nil {
			m.LastStopped = &t
		}
	}

	return m
}

// Ensure DevnetFileRepository implements DevnetRepository.
var _ ports.DevnetRepository = (*DevnetFileRepository)(nil)
