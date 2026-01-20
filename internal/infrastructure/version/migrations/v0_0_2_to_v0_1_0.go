// Package migrations contains all version migrations.
package migrations

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/domain/version"
)

// NoOpMigration is a no-op migration from 0.0.2 to 0.1.0-dev.
// This migration exists to establish a migration path but doesn't perform
// any actual changes, as there are no breaking schema changes between these versions.
type NoOpMigration struct{}

// NewNoOpMigration creates a new no-op migration.
func NewNoOpMigration() *NoOpMigration {
	return &NoOpMigration{}
}

// FromVersion returns the version this migration upgrades from.
func (m *NoOpMigration) FromVersion() string {
	return "0.0.2"
}

// ToVersion returns the version this migration upgrades to.
func (m *NoOpMigration) ToVersion() string {
	return "0.1.0-dev"
}

// Description returns a human-readable description.
func (m *NoOpMigration) Description() string {
	return "No-op migration (no schema changes)"
}

// Migrate performs the migration (no-op).
func (m *NoOpMigration) Migrate(ctx context.Context, homeDir string) error {
	// No changes needed
	return nil
}

// Rollback reverses the migration (no-op).
func (m *NoOpMigration) Rollback(ctx context.Context, homeDir string) error {
	// No changes needed
	return nil
}

// CanRollback indicates this migration supports rollback.
func (m *NoOpMigration) CanRollback() bool {
	return true
}

// Ensure NoOpMigration implements Migration.
var _ version.Migration = (*NoOpMigration)(nil)
