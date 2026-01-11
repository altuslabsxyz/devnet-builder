package binary

import (
	"context"
	"fmt"
	"os"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/cache"
)

// BinaryValidator validates cached binaries for usability.
// This implements the Validator component from the implementation plan.
//
// Validation Checks:
//  1. File exists and is a regular file (not directory)
//  2. File has executable permissions (mode & 0111 != 0)
//  3. Binary runs successfully with "version" command
//  4. Version output can be parsed
//
// Design Decision: Uses BinaryVersionDetector port to maintain Clean Architecture.
// The validator focuses on structural validation, delegating version detection to the adapter.
type BinaryValidator struct {
	detector ports.BinaryVersionDetector
}

// NewBinaryValidator creates a new binary validator.
//
// Parameters:
//   - detector: Version detector implementation (e.g., VersionDetectorAdapter)
//
// Example:
//
//	detector := NewVersionDetectorAdapter(
//	    executor.NewOSCommandExecutor(),
//	    5 * time.Second,
//	)
//	validator := NewBinaryValidator(detector)
func NewBinaryValidator(detector ports.BinaryVersionDetector) *BinaryValidator {
	return &BinaryValidator{
		detector: detector,
	}
}

// ValidateBinary checks if a binary is usable for deployment.
//
// This performs all validation checks required by the spec:
//   - FR-004: Binary must be executable and version-detectable
//   - EC-007: Corrupted binaries must be skipped with warning
//   - EC-009: Permission issues must produce actionable error messages
//
// Parameters:
//   - ctx: Context for cancellation/timeout
//   - binaryPath: Absolute path to binary file
//
// Returns:
//   - nil if binary is valid and usable
//   - Error with actionable message if validation fails
//
// Error Messages (user-facing):
//   - "binary not found: <path>"
//   - "path is a directory, not a binary"
//   - "binary is not executable (use chmod +x to fix)"
//   - "version detection failed: <reason>"
func (v *BinaryValidator) ValidateBinary(ctx context.Context, binaryPath string) error {
	// Check 1: File exists
	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary not found: %s", binaryPath)
		}
		// Permission denied or other OS error
		return fmt.Errorf("cannot access binary: %w", err)
	}

	// Check 2: Is regular file (not directory or device)
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a binary: %s", binaryPath)
	}

	// Check 3: Has executable permissions
	// Unix permission bits: owner(rwx) group(rwx) other(rwx) = 0111 checks execute bits
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable: %s (use chmod +x to fix)", binaryPath)
	}

	// Check 4: Binary runs and version is detectable
	// This validates the binary is not corrupted and follows expected output format
	_, err = v.detector.DetectVersion(ctx, binaryPath)
	if err != nil {
		return fmt.Errorf("version detection failed: %w", err)
	}

	// All checks passed
	return nil
}

// ValidateAndEnrichMetadata validates a binary and enriches metadata with version info.
//
// This is a convenience method that combines validation and version detection
// in a single call, updating the CachedBinaryMetadata struct in place.
//
// Parameters:
//   - ctx: Context for cancellation/timeout
//   - metadata: Pointer to CachedBinaryMetadata to validate and enrich
//
// Side Effects:
//   - Sets metadata.IsValid = true if validation succeeds
//   - Sets metadata.ValidationError if validation fails
//   - Populates metadata.Version and metadata.CommitHash on success
//
// Returns:
//   - The enriched metadata with updated fields
//   - Error if validation fails (also stored in metadata.ValidationError)
//
// Usage:
//
//	enriched, err := validator.ValidateAndEnrichMetadata(ctx, &binary)
//	if err != nil {
//	    logger.Warn("Skipping binary %s: %v", binary.Path, err)
//	}
func (v *BinaryValidator) ValidateAndEnrichMetadata(ctx context.Context, metadata *cache.CachedBinaryMetadata) (*cache.CachedBinaryMetadata, error) {
	// Validate binary
	err := v.ValidateBinary(ctx, metadata.Path)
	if err != nil {
		// Mark as invalid and store error
		metadata.IsValid = false
		metadata.ValidationError = err.Error()
		return metadata, err
	}

	// Detect version (already done in ValidateBinary, but need the result)
	versionInfo, err := v.detector.DetectVersion(ctx, metadata.Path)
	if err != nil {
		// Should not happen since ValidateBinary succeeded, but handle defensively
		metadata.IsValid = false
		metadata.ValidationError = fmt.Sprintf("version detection failed after validation: %v", err)
		return metadata, err
	}

	// Enrich metadata with version information
	metadata.Version = versionInfo.Version
	metadata.CommitHash = versionInfo.CommitHash

	// If CommitHashShort is empty or doesn't match, update it from full hash
	// (Scanner sets it from directory name, but we validate against actual binary)
	if len(metadata.CommitHash) >= 8 {
		metadata.CommitHashShort = metadata.CommitHash[:8]
	}

	// Mark as valid
	metadata.IsValid = true
	metadata.ValidationError = ""

	return metadata, nil
}
