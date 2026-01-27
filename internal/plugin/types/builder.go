// internal/plugin/types/builder.go
package types

import (
	"context"
	"log/slog"
)

// BuildOptions contains options for building a binary
type BuildOptions struct {
	SourceDir string            // git repo root after checkout
	OutputDir string            // where to place built binary
	Flags     map[string]string // merged: plugin defaults + user overrides
	GoVersion string            // requested Go version (empty = any)
	GitCommit string            // resolved commit hash (for version injection)
	GitRef    string            // original ref (branch/tag)
	Logger    *slog.Logger
}

// PluginBuilder handles binary compilation for a network type
type PluginBuilder interface {
	// DefaultGitRepo returns the default git repository for this network
	DefaultGitRepo() string

	// DefaultBuildFlags returns default build flags
	DefaultBuildFlags() map[string]string

	// BinaryName returns the expected binary name (e.g., "stabled", "gaiad")
	BinaryName() string

	// BuildBinary compiles the binary from source
	// Called after git checkout is complete
	BuildBinary(ctx context.Context, opts BuildOptions) error

	// ValidateBinary checks if the built binary is valid and runnable
	ValidateBinary(ctx context.Context, binaryPath string) error
}
