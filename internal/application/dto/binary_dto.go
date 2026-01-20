package dto

import (
	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// CustomBinaryImportInput contains the input parameters for importing a custom binary
// into the cache system. This DTO is used by the ImportCustomBinary use case.
type CustomBinaryImportInput struct {
	// BinaryPath is the absolute path to the custom binary file to import.
	// This path must point to a valid, executable file that has already been validated.
	BinaryPath string

	// NetworkType specifies the target network (e.g., "mainnet", "testnet", "devnet").
	// This is used to organize binaries in the cache: cache/binaries/{networkType}/
	NetworkType string

	// BuildConfig contains the build configuration used to create this binary.
	// This is used to generate the cache key: {commitHash}-{configHash}
	// For custom binaries, this may be nil or a default configuration.
	BuildConfig *network.BuildConfig

	// Ref is an optional reference name for this import (e.g., "custom", "debug-build").
	// This is stored in metadata but not used for the cache key.
	// If empty, defaults to "custom".
	Ref string
}

// CustomBinaryImportResult contains the output from importing a custom binary.
// It provides all necessary information to use the cached binary.
type CustomBinaryImportResult struct {
	// CacheKey is the full cache key where the binary is stored.
	// Format: {networkType}/{commitHash}-{configHash}
	// Example: "mainnet/80ad31b123...-790d3b6d"
	CacheKey string

	// CachedBinaryPath is the absolute path to the binary in the cache directory.
	// Example: ~/.devnet-builder/cache/binaries/mainnet/80ad31b123...-790d3b6d/stabled
	CachedBinaryPath string

	// SymlinkPath is the absolute path to the symlink pointing to the cached binary.
	// This is the "current" symlink that should be used to execute the binary.
	// Example: ~/.devnet-builder/bin/stabled
	SymlinkPath string

	// Version is the semantic version detected from the binary (e.g., "v1.0.0").
	Version string

	// CommitHash is the full git commit hash detected from the binary (40 chars).
	CommitHash string

	// Size is the file size of the cached binary in bytes.
	Size int64
}
