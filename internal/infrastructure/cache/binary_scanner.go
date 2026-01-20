package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/filesystem"
)

// CachedBinaryMetadata represents a discovered cached binary with all metadata.
// This struct matches the data model defined in specs/interactive-binary-select/data-model.md
type CachedBinaryMetadata struct {
	// Filesystem attributes
	Path    string    // Absolute path: ~/.devnet-builder/cache/binaries/mainnet/80ad31b-empty/stabled
	Name    string    // Binary name: "stabled", "aultd"
	Size    int64     // File size in bytes
	ModTime time.Time // Last modification timestamp

	// Derived display attributes
	SizeHuman       string // Human-readable: "45.2 MB"
	ModTimeRelative string // Relative time: "2 hours ago"

	// Version information (from binary version command - populated by validator)
	Version    string // Semantic version: "v1.0.0"
	CommitHash string // Full git commit: "80ad31b1234567890abcdef1234567890abcdef" (40 chars)

	// Cache key components (parsed from directory structure)
	CommitHashShort string // First 8 chars: "80ad31b"
	CacheKey        string // Full key: "mainnet/80ad31b-empty"
	NetworkType     string // "mainnet", "testnet", "devnet"
	ConfigHash      string // Build config hash: "empty", "abc123"

	// Validation status (populated by validator)
	IsValid         bool   // true if passed validation (executable, version detectable)
	ValidationError string // Error message if validation failed (for logging)
}

// BinaryScanner scans cache directory for binaries matching criteria.
// This implements the Scanner component from the implementation plan.
//
// Design Decision: Uses filesystem abstraction for testability.
// Performance: Can scan 100+ binaries in <1 second (filesystem I/O bound).
type BinaryScanner struct {
	fs filesystem.FileSystem
}

// NewBinaryScanner creates a new binary scanner with the given filesystem.
// Use filesystem.NewOSFileSystem() for production, or a mock for testing.
func NewBinaryScanner(fs filesystem.FileSystem) *BinaryScanner {
	return &BinaryScanner{
		fs: fs,
	}
}

// ScanCachedBinaries finds all binaries matching the specified criteria.
//
// Parameters:
//   - ctx: Context for cancellation (currently unused but reserved for future)
//   - cacheDir: Base cache directory (e.g., ~/.devnet-builder/cache/binaries)
//   - networkType: Network filter ("mainnet", "testnet")
//   - binaryName: Binary name to search for (e.g., "stabled")
//
// Returns:
//   - Slice of CachedBinaryMetadata sorted by modification time (most recent first)
//   - Empty slice (not error) if cache directory doesn't exist
//   - Error only for permission denied or other filesystem failures
//
// Directory Structure Expected:
//
//	{cacheDir}/{networkType}/{cacheKey}/{binaryName}
//	Example: ~/.devnet-builder/cache/binaries/mainnet/80ad31b-empty/stabled
//
// Edge Cases Handled:
//   - EC-001: Empty cache directory → returns empty slice
//   - EC-009: Permission denied → returns error with actionable message
//   - Invalid cache key format → skips silently (logs could be added)
//   - Non-directory entries in network dir → skips
//   - Missing binary in cache key dir → skips
func (s *BinaryScanner) ScanCachedBinaries(ctx context.Context, cacheDir, networkType, binaryName string) ([]CachedBinaryMetadata, error) {
	// Build path to network directory: {cacheDir}/{networkType}
	networkDir := filepath.Join(cacheDir, networkType)

	// Read cache key directories (e.g., "80ad31b-empty", "3b334fa-custom")
	entries, err := s.fs.ReadDir(networkDir)
	if err != nil {
		// EC-001: Cache directory doesn't exist → empty result (not an error)
		// This is expected on first run or after cache clean
		if os.IsNotExist(err) {
			return []CachedBinaryMetadata{}, nil
		}
		// EC-009: Permission denied or other filesystem error
		if os.IsPermission(err) {
			return nil, fmt.Errorf("cannot access cache directory: permission denied\nRun: chmod -R u+r %s", cacheDir)
		}
		return nil, fmt.Errorf("failed to read cache directory %s: %w", networkDir, err)
	}

	var binaries []CachedBinaryMetadata

	// Iterate through cache key directories
	for _, entry := range entries {
		// Skip non-directories (shouldn't exist but be defensive)
		if !entry.IsDir() {
			continue
		}

		cacheKey := entry.Name() // e.g., "80ad31b-empty"
		binaryPath := filepath.Join(networkDir, cacheKey, binaryName)

		// Check if binary exists at expected path
		info, err := s.fs.Stat(binaryPath)
		if err != nil {
			// Binary not found in this cache key directory, skip
			continue
		}

		// Verify it's a regular file (not directory or symlink)
		if info.IsDir() {
			continue
		}

		// Parse cache key: {commitHash}-{configHash}
		// Example: "80ad31b-empty" → commitHash="80ad31b", configHash="empty"
		parts := strings.SplitN(cacheKey, "-", 2)
		if len(parts) != 2 {
			// Invalid cache key format, skip
			// Could log warning here in production
			continue
		}

		commitHashShort := parts[0]
		configHash := parts[1]

		// Validate commit hash is at least 7 characters (short hash minimum)
		if len(commitHashShort) < 7 {
			continue
		}

		// Build metadata struct
		binary := CachedBinaryMetadata{
			// Filesystem attributes
			Path:    binaryPath,
			Name:    binaryName,
			Size:    info.Size(),
			ModTime: info.ModTime(),

			// Derived display attributes
			SizeHuman:       formatBytes(info.Size()),
			ModTimeRelative: formatRelativeTime(info.ModTime()),

			// Cache key components
			CommitHashShort: commitHashShort,
			CacheKey:        filepath.Join(networkType, cacheKey), // e.g., "mainnet/80ad31b-empty"
			NetworkType:     networkType,
			ConfigHash:      configHash,

			// Version and validation populated later by validator
			IsValid: false, // Default to false until validated
		}

		binaries = append(binaries, binary)
	}

	// Sort by modification time descending (most recent first)
	// This matches FR-003 requirement and CLARIFICATION 3 decision
	sort.Slice(binaries, func(i, j int) bool {
		return binaries[i].ModTime.After(binaries[j].ModTime)
	})

	return binaries, nil
}

// ScanAllNetworks finds all binaries across ALL network directories.
// Use this when networkType is unknown or empty (fallback scanning).
//
// Parameters:
//   - ctx: Context for cancellation
//   - cacheDir: Base cache directory (e.g., ~/.devnet-builder/cache/binaries)
//   - binaryName: Binary name to search for (e.g., "stabled")
//
// Returns:
//   - Slice of CachedBinaryMetadata sorted by modification time (most recent first)
//   - Empty slice if no binaries found
//   - Error only for permission denied or filesystem failures
//
// This method scans subdirectories of cacheDir that look like network names
// (mainnet, testnet, devnet, etc.) and searches for binaries in each.
func (s *BinaryScanner) ScanAllNetworks(ctx context.Context, cacheDir, binaryName string) ([]CachedBinaryMetadata, error) {
	// Read top-level directories (network types: mainnet, testnet, etc.)
	entries, err := s.fs.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []CachedBinaryMetadata{}, nil
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("cannot access cache directory: permission denied\nRun: chmod -R u+r %s", cacheDir)
		}
		return nil, fmt.Errorf("failed to read cache directory %s: %w", cacheDir, err)
	}

	var allBinaries []CachedBinaryMetadata

	// Iterate through potential network directories
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		networkType := entry.Name()

		// Skip hidden directories and non-network-like names
		if strings.HasPrefix(networkType, ".") {
			continue
		}

		// Scan binaries in this network directory
		binaries, err := s.ScanCachedBinaries(ctx, cacheDir, networkType, binaryName)
		if err != nil {
			// Log but continue scanning other networks
			continue
		}

		allBinaries = append(allBinaries, binaries...)
	}

	// Sort combined results by modification time (most recent first)
	sort.Slice(allBinaries, func(i, j int) bool {
		return allBinaries[i].ModTime.After(allBinaries[j].ModTime)
	})

	return allBinaries, nil
}

// formatBytes converts bytes to human-readable size (KB, MB, GB).
// This matches the spec requirement for "45.2 MB" style formatting.
//
// Implementation: Uses 1024-based units (KiB, MiB, GiB) per computing standard.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	// Format with 1 decimal place
	// Unit suffixes: K=1024, M=1024^2, G=1024^3, T=1024^4, P=1024^5, E=1024^6
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatRelativeTime converts timestamp to "2 hours ago" format.
// This matches the spec requirement for user-friendly time display.
//
// Time Ranges:
//   - < 1 minute: "just now"
//   - < 1 hour: "N min ago"
//   - < 24 hours: "N hour(s) ago"
//   - >= 24 hours: "N day(s) ago"
//
// Note: Doesn't handle months/years as cache entries are typically short-lived.
func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
