// Package version provides domain entities for version management.
package version

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
)

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
// Versions are normalized before comparison (e.g., v1.0.0-rc0-1-gdff0895-dirty -> 1.0.0).
func (mc MigrationChain) FindPath(from, to string) ([]Migration, error) {
	// Normalize versions by extracting core version
	normalizedFrom := normalizeVersion(from)
	normalizedTo := normalizeVersion(to)

	// Parse target version to determine the migration endpoint
	targetVer, err := version.NewVersion(normalizedTo)
	if err != nil {
		return nil, &MigrationError{
			Operation: "find_path",
			Message:   "invalid target version " + to + ": " + err.Error(),
		}
	}

	// Build a graph of possible migrations
	// For now, we'll do a simple linear search
	var path []Migration
	current := normalizedFrom

	// Try to find a path to the exact target or any compatible version
	for current != normalizedTo {
		found := false
		for _, m := range mc {
			fromVer := normalizeVersion(m.FromVersion())
			toVer := normalizeVersion(m.ToVersion())

			if fromVer == current {
				// Check if this migration gets us closer to the target
				migrToVer, _ := version.NewVersion(toVer)
				if migrToVer != nil && (migrToVer.Equal(targetVer) || migrToVer.LessThan(targetVer)) {
					path = append(path, m)
					current = toVer
					found = true
					break
				}
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

// normalizeVersion extracts the core semantic version from a git describe string.
// It removes git metadata (commit count, hash, dirty flag) but preserves pre-release tags.
// Examples:
//   - v1.0.0-rc0-1-gdff0895-dirty -> 1.0.0
//   - v1.0.0-rc0 -> 1.0.0
//   - 1.0.0 -> 1.0.0
//   - 0.1.0-dev -> 0.1.0
func normalizeVersion(v string) string {
	// Remove 'v' prefix
	v = strings.TrimPrefix(v, "v")

	// Try to parse with go-version to get the core version
	parsedVer, err := version.NewVersion(v)
	if err != nil {
		// If parsing fails, do manual extraction
		// Split on dash and take only the first part (major.minor.patch)
		parts := strings.Split(v, "-")
		if len(parts) > 0 {
			return parts[0]
		}
		return v
	}

	// Extract segments (major.minor.patch)
	segments := parsedVer.Segments()
	if len(segments) >= 3 {
		return fmt.Sprintf("%d.%d.%d", segments[0], segments[1], segments[2])
	} else if len(segments) == 2 {
		return fmt.Sprintf("%d.%d.0", segments[0], segments[1])
	} else if len(segments) == 1 {
		return fmt.Sprintf("%d.0.0", segments[0])
	}

	return v
}

// MigrationError represents an error during migration.
type MigrationError struct {
	Operation string
	Message   string
}

func (e *MigrationError) Error() string {
	return "migration " + e.Operation + " error: " + e.Message
}
