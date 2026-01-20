// Package migrations contains all version migrations.
package migrations

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/domain/version"
)

// V001ToV100Migration is a no-op migration from 0.0.1 to 1.0.0.
// This migration exists for users upgrading from very early versions.
type V001ToV100Migration struct{}

// NewV001ToV100Migration creates a new v0.0.1 -> v1.0.0 migration.
func NewV001ToV100Migration() *V001ToV100Migration {
	return &V001ToV100Migration{}
}

// FromVersion returns the version this migration upgrades from.
func (m *V001ToV100Migration) FromVersion() string {
	return "0.0.1"
}

// ToVersion returns the version this migration upgrades to.
func (m *V001ToV100Migration) ToVersion() string {
	return "1.0.0"
}

// Description returns a human-readable description.
func (m *V001ToV100Migration) Description() string {
	return "Direct upgrade from v0.0.1 to v1.0.0 (no schema changes)"
}

// Migrate performs the migration (no-op).
func (m *V001ToV100Migration) Migrate(ctx context.Context, homeDir string) error {
	// No changes needed - this is a version bump only
	return nil
}

// Rollback reverses the migration (no-op).
func (m *V001ToV100Migration) Rollback(ctx context.Context, homeDir string) error {
	// No changes needed
	return nil
}

// CanRollback indicates this migration supports rollback.
func (m *V001ToV100Migration) CanRollback() bool {
	return true
}

// Ensure V001ToV100Migration implements Migration.
var _ version.Migration = (*V001ToV100Migration)(nil)
