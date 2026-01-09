package binary

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// versionDetector implements the BinaryVersionDetector port by executing
// binaries and parsing their version output using regular expressions.
// This is the adapter implementation for the infrastructure layer.
type versionDetector struct {
	timeout time.Duration
}

// NewBinaryVersionDetector creates a new version detector with a default timeout.
// The timeout prevents hanging if a binary doesn't respond or gets stuck.
func NewBinaryVersionDetector() ports.BinaryVersionDetector {
	return &versionDetector{
		timeout: 10 * time.Second, // Reasonable timeout for version command
	}
}

// NewBinaryVersionDetectorWithTimeout creates a new version detector with a custom timeout.
// This is useful for testing or when dealing with slow binaries.
func NewBinaryVersionDetectorWithTimeout(timeout time.Duration) ports.BinaryVersionDetector {
	return &versionDetector{
		timeout: timeout,
	}
}

// DetectVersion executes the binary with the "version" command and parses the output.
// It follows the Cosmos SDK standard output format:
//
//	version: v1.0.0
//	commit: 80ad31b1234567890abcdef1234567890abcdef
//
// The function is resilient to output variations:
//   - Case-insensitive matching
//   - Handles extra whitespace
//   - Accepts version with or without 'v' prefix
//   - Validates commit hash format (40 hex characters)
func (d *versionDetector) DetectVersion(ctx context.Context, binaryPath string) (*ports.BinaryVersionInfo, error) {
	// Create context with timeout to prevent hanging
	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Execute binary with version command
	cmd := exec.CommandContext(timeoutCtx, binaryPath, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute binary version command: %w (output: %s)", err, string(output))
	}

	outputStr := string(output)

	// Parse version from output
	version, err := d.parseVersion(outputStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version from output: %w\nOutput was:\n%s", err, outputStr)
	}

	// Parse commit hash from output
	commitHash, err := d.parseCommitHash(outputStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commit hash from output: %w\nOutput was:\n%s", err, outputStr)
	}

	return &ports.BinaryVersionInfo{
		Version:    version,
		CommitHash: commitHash,
		GitCommit:  commitHash, // Maintain backward compatibility
	}, nil
}

// parseVersion extracts the semantic version from the binary output.
// It looks for patterns like "version: v1.0.0" or "version: 1.0.0" (case-insensitive).
// Returns the version with 'v' prefix normalized (always includes 'v').
func (d *versionDetector) parseVersion(output string) (string, error) {
	// Regex pattern explanation:
	// (?i) - case insensitive
	// version:\s* - matches "version:" followed by optional whitespace
	// (v?\d+\.\d+\.\d+[^\s]*) - captures version with optional 'v', digits, and optional suffix
	versionRegex := regexp.MustCompile(`(?i)version:\s*(v?\d+\.\d+\.\d+[^\s]*)`)
	matches := versionRegex.FindStringSubmatch(output)

	if len(matches) < 2 {
		return "", fmt.Errorf("version not found in output (expected format: 'version: v1.0.0')")
	}

	version := strings.TrimSpace(matches[1])

	// Normalize version to always have 'v' prefix
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	return version, nil
}

// parseCommitHash extracts the git commit hash from the binary output.
// It looks for patterns like "commit: 80ad31b..." (case-insensitive).
// Validates that the hash is exactly 40 hexadecimal characters.
func (d *versionDetector) parseCommitHash(output string) (string, error) {
	// Regex pattern explanation:
	// (?i) - case insensitive
	// commit:\s* - matches "commit:" followed by optional whitespace
	// ([a-f0-9]{40}) - captures exactly 40 hexadecimal characters (git SHA-1 hash)
	commitRegex := regexp.MustCompile(`(?i)commit:\s*([a-f0-9]{40})`)
	matches := commitRegex.FindStringSubmatch(output)

	if len(matches) < 2 {
		return "", fmt.Errorf("commit hash not found in output (expected format: 'commit: <40-char-hex-hash>')")
	}

	commitHash := strings.TrimSpace(matches[1])

	// Validate hash length (should be exactly 40 characters for SHA-1)
	if len(commitHash) != 40 {
		return "", fmt.Errorf("invalid commit hash length: expected 40 characters, got %d", len(commitHash))
	}

	return commitHash, nil
}
