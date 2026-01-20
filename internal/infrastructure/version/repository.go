// Package version provides infrastructure implementations for version management.
package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	domainVersion "github.com/altuslabsxyz/devnet-builder/internal/domain/version"
)

const (
	// VersionFileName is the name of the version metadata file.
	VersionFileName = ".version"
)

// FilesystemVersionRepository implements ports.VersionRepository.
// It stores version information in a JSON file in the home directory.
type FilesystemVersionRepository struct{}

// NewFilesystemVersionRepository creates a new FilesystemVersionRepository.
func NewFilesystemVersionRepository() *FilesystemVersionRepository {
	return &FilesystemVersionRepository{}
}

// Load reads the version from storage.
func (r *FilesystemVersionRepository) Load(homeDir string) (*domainVersion.Version, error) {
	versionPath := r.versionPath(homeDir)

	data, err := os.ReadFile(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // First run, no version file
		}
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	var v domainVersion.Version
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("failed to parse version file: %w", err)
	}

	return &v, nil
}

// Save persists the version to storage.
func (r *FilesystemVersionRepository) Save(homeDir string, v *domainVersion.Version) error {
	if err := v.Validate(); err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}

	versionPath := r.versionPath(homeDir)

	// Ensure home directory exists
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		return fmt.Errorf("failed to create home directory: %w", err)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	if err := os.WriteFile(versionPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	return nil
}

// Exists checks if version file exists.
func (r *FilesystemVersionRepository) Exists(homeDir string) bool {
	versionPath := r.versionPath(homeDir)
	_, err := os.Stat(versionPath)
	return err == nil
}

// versionPath returns the full path to the version file.
func (r *FilesystemVersionRepository) versionPath(homeDir string) string {
	return filepath.Join(homeDir, VersionFileName)
}

// Ensure FilesystemVersionRepository implements VersionRepository.
var _ ports.VersionRepository = (*FilesystemVersionRepository)(nil)
