package domain

import "fmt"

// SourceType represents the type of binary source selected by the user.
// This enum follows the Type-Safe Enum pattern for Go.
type SourceType int

const (
	// SourceTypeLocal indicates the user selected a local filesystem binary.
	// When this type is selected, SelectedPath MUST be non-empty and valid.
	SourceTypeLocal SourceType = iota

	// SourceTypeGitHubRelease indicates the user selected a GitHub release binary.
	// When this type is selected, the system will fetch releases from GitHub cache.
	SourceTypeGitHubRelease
)

// String returns the string representation of the SourceType.
// This implements the fmt.Stringer interface for readable logging.
func (s SourceType) String() string {
	switch s {
	case SourceTypeLocal:
		return "local"
	case SourceTypeGitHubRelease:
		return "github-release"
	default:
		return "unknown"
	}
}

// BinarySource represents the user's choice of binary origin.
// This is a domain entity that encapsulates all information about the selected binary source.
//
// Design Decision: This is a pure domain entity with no infrastructure dependencies.
// It follows Single Responsibility Principle - it only manages binary source selection state.
//
// State Transitions:
//
//	[Initial] → [SourceSelected] → [PathEntered] → [Validated] → [Ready]
//	                                              ↓
//	                                         [Invalid] → [PathEntered] (retry)
//
// Invariants:
//   - If SourceType == Local, SelectedPath MUST be non-empty
//   - If SourceType == Local, ValidationStatus MUST be true before use
//   - SelectedPath MUST be absolute path (starts with "/" or "C:\")
//   - SelectedPath MUST point to a file, not a directory
type BinarySource struct {
	// SourceType indicates whether the binary is from local filesystem or GitHub release.
	SourceType SourceType

	// SelectedPath is the absolute path to the binary file (only for SourceTypeLocal).
	// This field is empty for GitHub releases.
	SelectedPath string

	// ValidationStatus indicates whether the binary passed all validation checks.
	// This includes: file exists, is executable, has correct permissions, not corrupted.
	ValidationStatus bool

	// ValidationError contains the error message if validation failed.
	// This field is empty when ValidationStatus is true.
	ValidationError string
}

// NewBinarySource creates a new BinarySource instance with the given type.
//
// Parameters:
//   - sourceType: The type of binary source (Local or GitHubRelease)
//
// Returns:
//   - *BinarySource: A new instance with SourceType set, other fields at zero values
//
// Example:
//
//	source := NewBinarySource(domain.SourceTypeLocal)
//	source.SelectedPath = "/usr/local/bin/stabled"
//	if err := source.Validate(); err != nil {
//	    // Handle validation error
//	}
func NewBinarySource(sourceType SourceType) *BinarySource {
	return &BinarySource{
		SourceType:       sourceType,
		ValidationStatus: false,
	}
}

// Validate checks if the BinarySource is in a valid state according to business rules.
//
// Validation Rules:
//  1. If SourceType == Local, SelectedPath MUST be non-empty
//  2. If SourceType == Local, SelectedPath MUST be absolute (starts with "/" or drive letter)
//  3. If SourceType == GitHub, SelectedPath SHOULD be empty (will be set after download)
//  4. ValidationStatus consistency: if ValidationError is set, ValidationStatus should be false
//
// Returns:
//   - error: Validation error describing the first validation failure, or nil if valid
//
// Note: This method validates the domain entity state, not the actual binary file.
// Binary file validation (executable check, architecture check) is done by infrastructure layer.
func (b *BinarySource) Validate() error {
	// Rule 1: SourceType == Local requires SelectedPath
	if b.SourceType == SourceTypeLocal && b.SelectedPath == "" {
		return fmt.Errorf("local binary source requires a selected path")
	}

	// Rule 2: Local paths must be absolute
	if b.SourceType == SourceTypeLocal && !isAbsolutePath(b.SelectedPath) {
		return fmt.Errorf("selected path must be absolute: %s", b.SelectedPath)
	}

	// Rule 3: ValidationStatus consistency
	if b.ValidationError != "" && b.ValidationStatus {
		return fmt.Errorf("inconsistent state: ValidationError set but ValidationStatus is true")
	}

	return nil
}

// MarkValid marks the binary source as validated successfully.
// This should be called after infrastructure layer verifies the binary file.
func (b *BinarySource) MarkValid() {
	b.ValidationStatus = true
	b.ValidationError = ""
}

// MarkInvalid marks the binary source as invalid with the given error message.
// This should be called when infrastructure layer validation fails.
//
// Parameters:
//   - err: The validation error that occurred
func (b *BinarySource) MarkInvalid(err error) {
	b.ValidationStatus = false
	if err != nil {
		b.ValidationError = err.Error()
	}
}

// IsValid returns true if the binary source has been validated successfully.
//
// Returns:
//   - bool: true if ValidationStatus is true and ValidationError is empty
func (b *BinarySource) IsValid() bool {
	return b.ValidationStatus && b.ValidationError == ""
}

// IsLocal returns true if this is a local filesystem binary source.
func (b *BinarySource) IsLocal() bool {
	return b.SourceType == SourceTypeLocal
}

// IsGitHubRelease returns true if this is a GitHub release binary source.
func (b *BinarySource) IsGitHubRelease() bool {
	return b.SourceType == SourceTypeGitHubRelease
}

// isAbsolutePath checks if the given path is absolute.
// This is a helper function for cross-platform path validation.
//
// Returns:
//   - true for Unix absolute paths (start with "/")
//   - true for Windows absolute paths (start with drive letter like "C:\")
//   - false otherwise
func isAbsolutePath(path string) bool {
	if path == "" {
		return false
	}

	// Unix absolute path
	if path[0] == '/' {
		return true
	}

	// Windows absolute path (e.g., "C:\", "D:\")
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}

	return false
}
