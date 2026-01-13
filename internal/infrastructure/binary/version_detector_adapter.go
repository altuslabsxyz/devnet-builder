package binary

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/executor"
)

// VersionDetectorAdapter implements ports.BinaryVersionDetector using command execution.
// This adapter follows Clean Architecture by implementing an application port with
// infrastructure-level concerns (command execution).
//
// Design Decision: Uses CommandExecutor abstraction for testability.
// The detector can work with any binary following Cosmos SDK version output format.
type VersionDetectorAdapter struct {
	executor executor.CommandExecutor
	timeout  time.Duration
}

// NewVersionDetectorAdapter creates a new version detector with the given executor and timeout.
//
// Parameters:
//   - executor: Command executor (use executor.NewOSCommandExecutor() for production)
//   - timeout: Maximum time to wait for version command (recommended: 5 seconds per spec)
//
// Example:
//
//	detector := NewVersionDetectorAdapter(
//	    executor.NewOSCommandExecutor(),
//	    5 * time.Second,
//	)
func NewVersionDetectorAdapter(exec executor.CommandExecutor, timeout time.Duration) *VersionDetectorAdapter {
	return &VersionDetectorAdapter{
		executor: exec,
		timeout:  timeout,
	}
}

// DetectVersion executes the binary with "version" command and parses output.
//
// Expected Binary Output Format (Cosmos SDK standard):
//
//	version: v1.0.0
//	commit: 80ad31b1234567890abcdef1234567890abcdef
//
// Or alternate format:
//
//	v1.0.0
//	80ad31b1234567890abcdef1234567890abcdef
//
// Parameters:
//   - ctx: Context for cancellation/timeout
//   - binaryPath: Absolute path to binary executable
//
// Returns:
//   - BinaryVersionInfo with parsed version and commit hash
//   - Error if execution fails, times out, or output cannot be parsed
//
// Error Handling:
//   - Command not found: "binary not executable or not found"
//   - Timeout: "version detection timed out after 5s"
//   - Parse failure: "failed to parse version output"
//   - Exit code != 0: "version command failed: <output>"
func (d *VersionDetectorAdapter) DetectVersion(ctx context.Context, binaryPath string) (*ports.BinaryVersionInfo, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Execute: {binaryPath} version
	output, err := d.executor.ExecuteWithTimeout(ctx, binaryPath, "version")
	if err != nil {
		// Check if timeout occurred
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("version detection timed out after %v", d.timeout)
		}
		// Command execution failed (not found, permission denied, or non-zero exit)
		return nil, fmt.Errorf("version command failed: %w", err)
	}

	// Parse output
	version, commit, err := parseVersionOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse version output: %w\nOutput: %s", err, string(output))
	}

	return &ports.BinaryVersionInfo{
		Version:    version,
		CommitHash: commit,
		GitCommit:  commit, // Backward compatibility alias
	}, nil
}

// parseVersionOutput extracts version and commit from binary output.
//
// Supported Formats:
//
//  1. Key-value format (Cosmos SDK standard):
//     version: v1.0.0
//     commit: 80ad31b...
//
//  2. Simple format:
//     v1.0.0
//     80ad31b...
//
//  3. Devnet format (custom builds):
//     devnet-20bc50b
//     This format is used by custom devnet builds where version is derived from commit hash.
//
// Returns:
//   - version string (e.g., "v1.0.0" or "devnet-20bc50b")
//   - commit hash (e.g., "80ad31b1234567890abcdef1234567890abcdef" or short hash "20bc50b")
//   - error if parsing fails
func parseVersionOutput(output string) (version string, commit string, err error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Pattern 1: Key-value format with "version:" and "commit:" labels
	versionRegex := regexp.MustCompile(`(?i)version:\s*(.+)`)
	commitRegex := regexp.MustCompile(`(?i)commit:\s*([a-f0-9]{7,40})`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Try to extract version
		if version == "" {
			if match := versionRegex.FindStringSubmatch(line); len(match) > 1 {
				version = strings.TrimSpace(match[1])
			}
		}

		// Try to extract commit
		if commit == "" {
			if match := commitRegex.FindStringSubmatch(line); len(match) > 1 {
				commit = strings.TrimSpace(match[1])
			}
		}
	}

	// Pattern 2: Simple format (first line is version, second line is commit)
	if version == "" && len(lines) >= 1 {
		// First line might be version (starts with 'v' or looks like semantic version)
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "v") || regexp.MustCompile(`^\d+\.\d+`).MatchString(firstLine) {
			version = firstLine
		}
	}

	if commit == "" && len(lines) >= 2 {
		// Second line might be commit hash (7-40 hex characters)
		secondLine := strings.TrimSpace(lines[1])
		if regexp.MustCompile(`^[a-f0-9]{7,40}$`).MatchString(secondLine) {
			commit = secondLine
		}
	}

	// Pattern 3: Devnet format "devnet-HASH" (custom builds without standard versioning)
	// This handles binaries built with ldflags that set version to "devnet-{shortcommit}"
	if version == "" && len(lines) >= 1 {
		firstLine := strings.TrimSpace(lines[0])
		devnetRegex := regexp.MustCompile(`^devnet-([a-f0-9]{7,40})$`)
		if match := devnetRegex.FindStringSubmatch(firstLine); len(match) > 1 {
			// Use full string as version and extracted hash as commit
			version = firstLine // "devnet-20bc50b"
			commit = match[1]   // "20bc50b"
		}
	}

	// Validate we found both version and commit
	if version == "" {
		return "", "", fmt.Errorf("version not found in output")
	}
	if commit == "" {
		return "", "", fmt.Errorf("commit hash not found in output")
	}

	return version, commit, nil
}
