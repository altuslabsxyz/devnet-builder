package interactive

import (
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/infrastructure/cache"
)

// formatBinaryForDisplay formats a CachedBinaryMetadata into a user-friendly display string.
//
// Format Template (per spec FR-003):
//
//	{binaryName} {version} ({shortCommit}) [{cacheKey}] - {size} - {relativeTime}
//
// Example Output:
//
//	stabled v1.0.0 (80ad31b) [mainnet/80ad31b-empty] - 45.2 MB - 2 hours ago
//	stabled v1.1.0 (3b334fa) [mainnet/3b334fa-custom] - 46.1 MB - 1 hour ago
//	stabled v2.0.0-rc1 (756936f) [mainnet/756936f-empty] - 47.0 MB - 30 min ago
//
// Design Decision: Single-line format with all key information visible.
// Matches the existing style used for version selection in selector.go.
//
// Parameters:
//   - binary: CachedBinaryMetadata with all fields populated by scanner and validator
//
// Returns:
//   - Formatted display string ready for promptui list
//
// Notes:
//   - Version and CommitHashShort are populated by the validator
//   - If version is empty (validation failed), shows "unknown" instead
//   - If size or time formatting fails, shows raw values as fallback
func formatBinaryForDisplay(binary cache.CachedBinaryMetadata) string {
	// Handle missing version (shouldn't happen for valid binaries, but be defensive)
	version := binary.Version
	if version == "" {
		version = "unknown"
	}

	// Format: binaryName version (shortCommit) [cacheKey] - size - relativeTime
	// Example: stabled v1.0.0 (80ad31b) [mainnet/80ad31b-empty] - 45.2 MB - 2 hours ago
	return fmt.Sprintf(
		"%s %s (%s) [%s] - %s - %s",
		binary.Name,
		version,
		binary.CommitHashShort,
		binary.CacheKey,
		binary.SizeHuman,
		binary.ModTimeRelative,
	)
}

// formatBuildFromSourceOption creates the "Build from source" option display string.
//
// This matches the spec requirement (FR-003) to include a build option in the list.
//
// Returns:
//   - Formatted display string for the build option
//
// Note: This is a separate constant-like function to ensure consistent formatting
// and allow easy localization in the future if needed.
func formatBuildFromSourceOption() string {
	return "Build from source (specify version)"
}
