package devnet

import (
	"path/filepath"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain/common"
	"github.com/google/uuid"
)

// Devnet represents a local development network instance.
// This is the aggregate root for devnet domain.
type Devnet struct {
	// Identity
	ID   string `json:"id"`
	Name string `json:"name"`

	// Configuration (immutable after creation)
	Config Config `json:"config"`

	// Paths
	HomeDir string `json:"home_dir"`

	// Version tracking
	Version VersionInfo `json:"version"`

	// State (mutable)
	State     State         `json:"state"`
	Provision ProvisionInfo `json:"provision"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
}

// VersionInfo holds version-related information.
type VersionInfo struct {
	// Requested version (branch, tag, or commit)
	Requested common.Version `json:"requested"`

	// Initial version from genesis
	Initial common.Version `json:"initial,omitempty"`

	// Current running version (after upgrades)
	Current common.Version `json:"current,omitempty"`

	// True if built from custom branch/commit
	IsCustomRef bool `json:"is_custom_ref,omitempty"`

	// Path to custom-built binary
	CustomBinaryPath string `json:"custom_binary_path,omitempty"`
}

// New creates a new Devnet with the given home directory.
func New(homeDir string) *Devnet {
	return &Devnet{
		ID:        uuid.New().String(),
		Name:      "devnet",
		Config:    NewConfig(),
		HomeDir:   homeDir,
		Version:   VersionInfo{Requested: "latest"},
		State:     NewState(),
		Provision: NewProvisionInfo(),
		CreatedAt: time.Now(),
	}
}

// NewWithConfig creates a new Devnet with the given configuration.
func NewWithConfig(homeDir string, config Config) (*Devnet, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	d := New(homeDir)
	d.Config = config
	return d, nil
}

// DevnetDir returns the path to the devnet directory.
func (d *Devnet) DevnetDir() string {
	return filepath.Join(d.HomeDir, "devnet")
}

// NodeDir returns the path to a specific node's directory.
func (d *Devnet) NodeDir(index int) string {
	return filepath.Join(d.DevnetDir(), "node"+string(rune('0'+index)))
}

// GenesisPath returns the path to the genesis file.
func (d *Devnet) GenesisPath() string {
	return filepath.Join(d.DevnetDir(), "genesis.json")
}

// MetadataPath returns the path to the metadata file.
func (d *Devnet) MetadataPath() string {
	return filepath.Join(d.DevnetDir(), "metadata.json")
}

// Start starts the devnet.
func (d *Devnet) Start() error {
	if !d.Provision.CanRun() {
		return &NotProvisionedError{State: d.Provision.State}
	}
	return d.State.SetRunning()
}

// Stop stops the devnet.
func (d *Devnet) Stop() error {
	return d.State.SetStopped()
}

// IsRunning returns true if the devnet is running.
func (d *Devnet) IsRunning() bool {
	return d.State.IsRunning()
}

// IsProvisioned returns true if the devnet is provisioned.
func (d *Devnet) IsProvisioned() bool {
	return d.Provision.State.IsProvisioned()
}

// StartProvisioning marks the devnet as provisioning.
func (d *Devnet) StartProvisioning() {
	d.Provision.Start()
}

// CompleteProvisioning marks provisioning as complete.
func (d *Devnet) CompleteProvisioning() {
	d.Provision.Complete()
}

// FailProvisioning marks provisioning as failed.
func (d *Devnet) FailProvisioning(err error) {
	d.Provision.Fail(err)
}

// SetInitialVersion sets the initial version from genesis.
func (d *Devnet) SetInitialVersion(version string) {
	d.Version.Initial = common.Version(version)
	if d.Version.Current.IsEmpty() {
		d.Version.Current = d.Version.Initial
	}
}

// SetCurrentVersion sets the current version.
func (d *Devnet) SetCurrentVersion(version string) {
	d.Version.Current = common.Version(version)
}

// GetEffectiveVersion returns the effective version to use.
func (d *Devnet) GetEffectiveVersion() common.Version {
	if !d.Version.Requested.IsEmpty() {
		return d.Version.Requested
	}
	return "latest"
}

// Validate validates the devnet configuration.
func (d *Devnet) Validate() error {
	return d.Config.Validate()
}
