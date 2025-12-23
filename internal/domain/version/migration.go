// Package version provides domain entities for version management.
package version

import "context"

// Migration represents a single version migration.
// Each migration is responsible for upgrading from one version to another.
//
// SOLID Principles:
// - Single Responsibility: Each migration handles one version upgrade
// - Open/Closed: New migrations can be added without modifying existing code
// - Liskov Substitution: All migrations are interchangeable
// - Interface Segregation: Minimal interface with only necessary methods
// - Dependency Inversion: High-level code depends on this interface, not concrete implementations
type Migration interface {
	// FromVersion returns the version this migration upgrades from.
	FromVersion() string

	// ToVersion returns the version this migration upgrades to.
	ToVersion() string

	// Description returns a human-readable description of what this migration does.
	Description() string

	// Migrate performs the migration.
	// It should be idempotent - running it multiple times should be safe.
	// Returns error if migration fails.
	Migrate(ctx context.Context, homeDir string) error

	// Rollback attempts to reverse the migration.
	// Returns error if rollback fails or is not supported.
	Rollback(ctx context.Context, homeDir string) error

	// CanRollback indicates whether this migration supports rollback.
	CanRollback() bool
}

// MigrationChain represents an ordered sequence of migrations.
type MigrationChain []Migration

// FindPath finds the migration path from current version to target version.
// Returns the migrations to apply in order, or error if no path exists.
func (mc MigrationChain) FindPath(from, to string) ([]Migration, error) {
	// Build a graph of possible migrations
	// For now, we'll do a simple linear search
	var path []Migration
	current := from

	for current != to {
		found := false
		for _, m := range mc {
			if m.FromVersion() == current {
				path = append(path, m)
				current = m.ToVersion()
				found = true
				break
			}
		}
		if !found {
			return nil, &MigrationError{
				Operation: "find_path",
				Message:   "no migration path found from " + from + " to " + to,
			}
		}
	}

	return path, nil
}

// MigrationError represents an error during migration.
type MigrationError struct {
	Operation string
	Message   string
}

func (e *MigrationError) Error() string {
	return "migration " + e.Operation + " error: " + e.Message
}
