package ports

import "context"

// BinaryVersionDetector defines the interface for detecting version information from compiled binaries.
// This port allows the application layer to remain agnostic of how version detection is implemented,
// following the Dependency Inversion Principle (DIP).
type BinaryVersionDetector interface {
	// DetectVersion executes the binary with the "version" command and parses the output
	// to extract version and commit information.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - binaryPath: Absolute path to the binary executable
	//
	// Returns:
	//   - BinaryVersionInfo containing parsed version data
	//   - Error if binary execution fails or output cannot be parsed
	//
	// Expected binary output format (Cosmos SDK standard):
	//   version: v1.0.0
	//   commit: 80ad31b1234567890abcdef1234567890abcdef
	DetectVersion(ctx context.Context, binaryPath string) (*BinaryVersionInfo, error)
}

// BinaryVersionInfo contains version metadata extracted from a binary.
// This struct is designed to be compatible with the existing cache metadata format.
type BinaryVersionInfo struct {
	// Version is the semantic version string (e.g., "v1.0.0", "v2.1.3")
	Version string

	// CommitHash is the full git commit hash (40 characters)
	// This is used as part of the cache key: {networkType}/{commitHash}-{configHash}
	CommitHash string

	// GitCommit is an alias for CommitHash, maintained for backward compatibility
	// with existing code that may use this field name
	GitCommit string
}
