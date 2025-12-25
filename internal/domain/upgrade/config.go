package upgrade

import (
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain/common"
)

const (
	// MinVotingPeriod is the minimum allowed voting period.
	MinVotingPeriod = 30 * time.Second

	// MinHeightBuffer is the minimum blocks to add after voting.
	MinHeightBuffer = 5

	// DefaultVotingPeriod is the default voting period.
	DefaultVotingPeriod = 60 * time.Second

	// DefaultHeightBuffer is the default height buffer.
	DefaultHeightBuffer = 10
)

// Config holds configuration for an upgrade operation.
type Config struct {
	// Required
	Name string `json:"name"` // Upgrade handler name

	// Execution mode
	Mode common.ExecutionMode `json:"mode,omitempty"`

	// Target (one of these must be set)
	TargetImage  string `json:"target_image,omitempty"`  // Docker image
	TargetBinary string `json:"target_binary,omitempty"` // Local binary path
	CachePath    string `json:"cache_path,omitempty"`    // Pre-built cached binary

	// Version info
	TargetVersion string `json:"target_version,omitempty"`
	CommitHash    string `json:"commit_hash,omitempty"`

	// Timing
	VotingPeriod  time.Duration `json:"voting_period"`
	HeightBuffer  int           `json:"height_buffer"`
	UpgradeHeight int64         `json:"upgrade_height,omitempty"` // 0 = auto-calculate

	// State export
	ExportEnabled bool   `json:"export_enabled"`
	GenesisDir    string `json:"genesis_dir,omitempty"`
}

// NewConfig creates a new Config with default values.
func NewConfig(name string) Config {
	return Config{
		Name:         name,
		VotingPeriod: DefaultVotingPeriod,
		HeightBuffer: DefaultHeightBuffer,
	}
}

// Validate checks if the config is valid.
func (c Config) Validate() error {
	if c.Name == "" {
		return &ValidationError{Field: "name", Message: "upgrade name is required"}
	}

	// Need at least one target
	if c.TargetImage == "" && c.TargetBinary == "" && c.CachePath == "" {
		return &ValidationError{Field: "target", Message: "one of target_image, target_binary, or cache_path is required"}
	}

	// Can't have both Docker image and local binary
	if c.TargetImage != "" && (c.TargetBinary != "" || c.CachePath != "") {
		return &ValidationError{Field: "target", Message: "cannot specify both docker image and local binary"}
	}

	if c.VotingPeriod < MinVotingPeriod {
		return &ValidationError{
			Field:   "voting_period",
			Message: fmt.Sprintf("must be at least %s", MinVotingPeriod),
		}
	}

	if c.HeightBuffer < MinHeightBuffer {
		return &ValidationError{
			Field:   "height_buffer",
			Message: fmt.Sprintf("must be at least %d", MinHeightBuffer),
		}
	}

	return nil
}

// IsCacheMode returns true if using a pre-cached binary.
func (c Config) IsCacheMode() bool {
	return c.CachePath != "" && c.CommitHash != ""
}

// IsDockerMode returns true if targeting a Docker image.
func (c Config) IsDockerMode() bool {
	return c.TargetImage != ""
}

// IsLocalMode returns true if targeting a local binary.
func (c Config) IsLocalMode() bool {
	return c.TargetBinary != "" || c.CachePath != ""
}

// WithMode returns a new Config with the specified mode.
func (c Config) WithMode(mode common.ExecutionMode) Config {
	c.Mode = mode
	return c
}

// WithDockerImage returns a new Config with the specified Docker image.
func (c Config) WithDockerImage(image string) Config {
	c.TargetImage = image
	c.TargetBinary = ""
	c.CachePath = ""
	return c
}

// WithBinary returns a new Config with the specified binary path.
func (c Config) WithBinary(path string) Config {
	c.TargetBinary = path
	c.TargetImage = ""
	return c
}

// WithCache returns a new Config with the specified cache path and commit.
func (c Config) WithCache(path, commitHash string) Config {
	c.CachePath = path
	c.CommitHash = commitHash
	c.TargetImage = ""
	return c
}

// WithVotingPeriod returns a new Config with the specified voting period.
func (c Config) WithVotingPeriod(period time.Duration) Config {
	c.VotingPeriod = period
	return c
}

// WithHeightBuffer returns a new Config with the specified height buffer.
func (c Config) WithHeightBuffer(buffer int) Config {
	c.HeightBuffer = buffer
	return c
}

// WithExport returns a new Config with state export enabled.
func (c Config) WithExport(dir string) Config {
	c.ExportEnabled = true
	c.GenesisDir = dir
	return c
}
