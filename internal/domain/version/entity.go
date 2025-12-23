// Package version provides domain entities for version management.
package version

import (
	"fmt"
	"time"
)

// Version represents the application version stored in the home directory.
type Version struct {
	// Current semantic version (e.g., "1.1.0")
	Current string

	// LastMigrated is the timestamp of the last successful migration
	LastMigrated time.Time

	// MigrationHistory tracks all applied migrations
	MigrationHistory []MigrationRecord
}

// MigrationRecord represents a single migration that was applied.
type MigrationRecord struct {
	// FromVersion is the version before migration
	FromVersion string

	// ToVersion is the version after migration
	ToVersion string

	// AppliedAt is when the migration was applied
	AppliedAt time.Time

	// Success indicates if migration completed successfully
	Success bool

	// Error message if migration failed
	Error string
}

// NewVersion creates a new Version with the given current version.
func NewVersion(current string) *Version {
	return &Version{
		Current:          current,
		LastMigrated:     time.Now(),
		MigrationHistory: []MigrationRecord{},
	}
}

// AddMigrationRecord adds a migration record to the history.
func (v *Version) AddMigrationRecord(from, to string, success bool, err error) {
	record := MigrationRecord{
		FromVersion: from,
		ToVersion:   to,
		AppliedAt:   time.Now(),
		Success:     success,
	}
	if err != nil {
		record.Error = err.Error()
	}
	v.MigrationHistory = append(v.MigrationHistory, record)

	if success {
		v.Current = to
		v.LastMigrated = record.AppliedAt
	}
}

// WasMigrationApplied checks if a migration from -> to was already applied successfully.
func (v *Version) WasMigrationApplied(from, to string) bool {
	for _, record := range v.MigrationHistory {
		if record.FromVersion == from && record.ToVersion == to && record.Success {
			return true
		}
	}
	return false
}

// Validate checks if the version is valid.
func (v *Version) Validate() error {
	if v.Current == "" {
		return fmt.Errorf("version cannot be empty")
	}
	return nil
}
