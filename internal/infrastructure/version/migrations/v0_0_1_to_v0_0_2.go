// Package migrations contains all version migrations.
package migrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/domain/version"
)

// CacheKeyMigration migrates from network-based cache keys to composite cache keys.
//
// Before (v0.0.1):
//   ~/.devnet-builder/snapshots/mainnet/...
//   ~/.devnet-builder/snapshots/testnet/...
//
// After (v0.0.2):
//   ~/.devnet-builder/snapshots/stable-mainnet/...
//   ~/.devnet-builder/snapshots/stable-testnet/...
//
// This migration assumes the default plugin is "stable" for existing caches.
type CacheKeyMigration struct{}

// NewCacheKeyMigration creates a new cache key migration.
func NewCacheKeyMigration() *CacheKeyMigration {
	return &CacheKeyMigration{}
}

// FromVersion returns the version this migration upgrades from.
func (m *CacheKeyMigration) FromVersion() string {
	return "0.0.1"
}

// ToVersion returns the version this migration upgrades to.
func (m *CacheKeyMigration) ToVersion() string {
	return "0.0.2"
}

// Description returns a human-readable description.
func (m *CacheKeyMigration) Description() string {
	return "Migrating snapshot cache from network-based keys to composite plugin-network keys"
}

// Migrate performs the migration.
func (m *CacheKeyMigration) Migrate(ctx context.Context, homeDir string) error {
	snapshotsDir := filepath.Join(homeDir, "snapshots")

	// Check if snapshots directory exists
	if _, err := os.Stat(snapshotsDir); os.IsNotExist(err) {
		// No snapshots directory, nothing to migrate
		return nil
	}

	// Migrate known network directories
	networks := []string{"mainnet", "testnet"}
	defaultPlugin := "stable"

	for _, network := range networks {
		oldPath := filepath.Join(snapshotsDir, network)
		newPath := filepath.Join(snapshotsDir, fmt.Sprintf("%s-%s", defaultPlugin, network))

		// Check if old path exists
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue // This network doesn't exist, skip
		}

		// Check if new path already exists
		if _, err := os.Stat(newPath); err == nil {
			// New path already exists, skip to avoid conflicts
			// This could happen if migration was partially applied before
			continue
		}

		// Rename old path to new path
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to migrate %s to %s: %w", oldPath, newPath, err)
		}
	}

	return nil
}

// Rollback reverses the migration.
func (m *CacheKeyMigration) Rollback(ctx context.Context, homeDir string) error {
	snapshotsDir := filepath.Join(homeDir, "snapshots")

	// Rollback: rename stable-{network} back to {network}
	networks := []string{"mainnet", "testnet"}
	defaultPlugin := "stable"

	for _, network := range networks {
		newPath := filepath.Join(snapshotsDir, fmt.Sprintf("%s-%s", defaultPlugin, network))
		oldPath := filepath.Join(snapshotsDir, network)

		// Check if new path exists
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			continue
		}

		// Check if old path already exists
		if _, err := os.Stat(oldPath); err == nil {
			// Old path already exists, cannot rollback safely
			return fmt.Errorf("cannot rollback: %s already exists", oldPath)
		}

		// Rename new path back to old path
		if err := os.Rename(newPath, oldPath); err != nil {
			return fmt.Errorf("failed to rollback %s to %s: %w", newPath, oldPath, err)
		}
	}

	return nil
}

// CanRollback indicates this migration supports rollback.
func (m *CacheKeyMigration) CanRollback() bool {
	return true
}

// Ensure CacheKeyMigration implements Migration.
var _ version.Migration = (*CacheKeyMigration)(nil)
