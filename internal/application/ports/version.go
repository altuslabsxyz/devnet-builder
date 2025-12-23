package ports

import (
	"context"

	"github.com/b-harvest/devnet-builder/internal/domain/version"
)

// VersionRepository defines operations for persisting version information.
//
// Dependency Inversion Principle: Application layer defines the interface,
// infrastructure layer provides the implementation.
type VersionRepository interface {
	// Load reads the version from storage.
	// Returns nil if version file doesn't exist (first run).
	Load(homeDir string) (*version.Version, error)

	// Save persists the version to storage.
	Save(homeDir string, v *version.Version) error

	// Exists checks if version file exists.
	Exists(homeDir string) bool
}

// MigrationService defines operations for version management and migration.
type MigrationService interface {
	// GetCurrentVersion returns the current version stored in homeDir.
	// Returns default version if not found.
	GetCurrentVersion(homeDir string) (*version.Version, error)

	// CheckAndMigrate checks if migration is needed and applies migrations.
	// target is the desired version (usually the app's current version).
	// Returns the version after migration and any error.
	CheckAndMigrate(ctx context.Context, homeDir string, target string) (*version.Version, error)

	// RegisterMigration registers a migration to the service.
	RegisterMigration(migration version.Migration)

	// ListMigrations returns all registered migrations.
	ListMigrations() []version.Migration
}
