// Package version provides use cases for version management.
package version

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-version"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	domainVersion "github.com/altuslabsxyz/devnet-builder/internal/domain/version"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

const (
	// DefaultVersion is the version assigned to fresh installations.
	DefaultVersion = "0.0.1"
)

// Service implements ports.MigrationService.
// It manages version tracking and migration execution.
type Service struct {
	repository ports.VersionRepository
	migrations domainVersion.MigrationChain
	logger     *output.Logger
}

// NewService creates a new migration service.
func NewService(repository ports.VersionRepository, logger *output.Logger) *Service {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &Service{
		repository: repository,
		migrations: make(domainVersion.MigrationChain, 0),
		logger:     logger,
	}
}

// GetCurrentVersion returns the current version stored in homeDir.
func (s *Service) GetCurrentVersion(homeDir string) (*domainVersion.Version, error) {
	v, err := s.repository.Load(homeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load version: %w", err)
	}

	// First run - create default version
	if v == nil {
		v = domainVersion.NewVersion(DefaultVersion)
		s.logger.Debug("First run detected, initializing version %s", DefaultVersion)
	}

	return v, nil
}

// isSameMajorMinor checks if two versions have the same major and minor version.
// Returns true if major.minor are identical, ignoring patch, prerelease, and git metadata.
// For example: 1.2.0 and 1.2.3 -> true, 1.2.0-rc0-7-g... and 1.2.0-rc0-12-g... -> true
func isSameMajorMinor(current, target *version.Version) bool {
	currentSegs := current.Segments()
	targetSegs := target.Segments()

	// Ensure we have at least major.minor for both versions
	if len(currentSegs) < 2 || len(targetSegs) < 2 {
		return false
	}

	// Check if major and minor are the same
	return currentSegs[0] == targetSegs[0] && currentSegs[1] == targetSegs[1]
}

// CheckAndMigrate checks if migration is needed and applies migrations.
// Note: If major.minor are the same, skip migration entirely and only update the version.
// This avoids issues with prerelease/git-describe version comparisons.
func (s *Service) CheckAndMigrate(ctx context.Context, homeDir string, target string) (*domainVersion.Version, error) {
	// Get current version
	current, err := s.GetCurrentVersion(homeDir)
	if err != nil {
		return nil, err
	}

	// Parse versions for comparison
	currentVer, err := version.NewVersion(current.Current)
	if err != nil {
		return nil, fmt.Errorf("invalid current version %s: %w", current.Current, err)
	}

	targetVer, err := version.NewVersion(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target version %s: %w", target, err)
	}

	// Check major.minor first - if same, skip migration entirely (just update version)
	// This avoids prerelease comparison issues (e.g., v1.0.0-rc0-7-g... vs v1.0.0-rc0-12-g...)
	if isSameMajorMinor(currentVer, targetVer) {
		if current.Current == target {
			s.logger.Debug("Version %s is current, no migration needed", current.Current)
			return current, nil
		}
		s.logger.Debug("Same major.minor version %s -> %s, skipping migration", current.Current, target)
		current.Current = target
		if err := s.repository.Save(homeDir, current); err != nil {
			return nil, fmt.Errorf("failed to save version: %w", err)
		}
		return current, nil
	}

	// Different major.minor - check for downgrade
	currentSegs := currentVer.Segments()
	targetSegs := targetVer.Segments()
	if len(currentSegs) >= 2 && len(targetSegs) >= 2 {
		if currentSegs[0] > targetSegs[0] || (currentSegs[0] == targetSegs[0] && currentSegs[1] > targetSegs[1]) {
			return nil, fmt.Errorf("downgrade from %s to %s is not supported", current.Current, target)
		}
	}

	// Find migration path
	s.logger.Info("Migration needed: %s -> %s", current.Current, target)
	migrationPath, err := s.migrations.FindPath(current.Current, target)
	if err != nil {
		return nil, fmt.Errorf("failed to find migration path: %w", err)
	}

	// Apply migrations
	for _, migration := range migrationPath {
		if err := s.applyMigration(ctx, homeDir, current, migration); err != nil {
			return nil, fmt.Errorf("migration %s -> %s failed: %w",
				migration.FromVersion(), migration.ToVersion(), err)
		}
	}

	// Update current version to target
	current.Current = target

	// Save final version
	if err := s.repository.Save(homeDir, current); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	s.logger.Success("Migration completed: %s -> %s", currentVer, targetVer)
	return current, nil
}

// applyMigration applies a single migration.
func (s *Service) applyMigration(ctx context.Context, homeDir string, v *domainVersion.Version, m domainVersion.Migration) error {
	// Check if already applied
	if v.WasMigrationApplied(m.FromVersion(), m.ToVersion()) {
		s.logger.Debug("Migration %s -> %s already applied, skipping", m.FromVersion(), m.ToVersion())
		return nil
	}

	s.logger.Info("Applying migration: %s -> %s", m.FromVersion(), m.ToVersion())
	s.logger.Info("  %s", m.Description())

	// Execute migration
	err := m.Migrate(ctx, homeDir)

	// Record migration attempt
	v.AddMigrationRecord(m.FromVersion(), m.ToVersion(), err == nil, err)

	// Save version after each migration
	if saveErr := s.repository.Save(homeDir, v); saveErr != nil {
		s.logger.Warn("Failed to save version after migration: %v", saveErr)
	}

	if err != nil {
		return err
	}

	s.logger.Success("Migration %s -> %s completed successfully", m.FromVersion(), m.ToVersion())
	return nil
}

// RegisterMigration registers a migration to the service.
func (s *Service) RegisterMigration(migration domainVersion.Migration) {
	s.migrations = append(s.migrations, migration)
	s.logger.Debug("Registered migration: %s -> %s", migration.FromVersion(), migration.ToVersion())
}

// ListMigrations returns all registered migrations.
func (s *Service) ListMigrations() []domainVersion.Migration {
	return s.migrations
}

// Ensure Service implements MigrationService.
var _ ports.MigrationService = (*Service)(nil)
