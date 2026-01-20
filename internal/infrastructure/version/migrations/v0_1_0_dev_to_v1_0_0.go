// Package migrations contains all version migrations.
package migrations

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/domain/version"
)

// V010DevToV100Migration is a no-op migration from 0.1.0-dev to 1.0.0.
// This migration exists to establish a migration path for development builds.
type V010DevToV100Migration struct{}

// NewV010DevToV100Migration creates a new v0.1.0-dev -> v1.0.0 migration.
func NewV010DevToV100Migration() *V010DevToV100Migration {
	return &V010DevToV100Migration{}
}

// FromVersion returns the version this migration upgrades from.
func (m *V010DevToV100Migration) FromVersion() string {
	return "0.1.0-dev"
}

// ToVersion returns the version this migration upgrades to.
func (m *V010DevToV100Migration) ToVersion() string {
	return "1.0.0"
}

// Description returns a human-readable description.
func (m *V010DevToV100Migration) Description() string {
	return "Upgrade from dev build to v1.0.0 release (no schema changes)"
}

// Migrate performs the migration (no-op).
func (m *V010DevToV100Migration) Migrate(ctx context.Context, homeDir string) error {
	// No changes needed - this is a version bump only
	return nil
}

// Rollback reverses the migration (no-op).
func (m *V010DevToV100Migration) Rollback(ctx context.Context, homeDir string) error {
	// No changes needed
	return nil
}

// CanRollback indicates this migration supports rollback.
func (m *V010DevToV100Migration) CanRollback() bool {
	return true
}

// Ensure V010DevToV100Migration implements Migration.
var _ version.Migration = (*V010DevToV100Migration)(nil)
