// internal/daemon/builder/types.go
package builder

import (
	"context"
	"time"
)

// BuildSpec specifies what binary to build
type BuildSpec struct {
	GitRepo    string            // e.g., "github.com/cosmos/gaia"
	GitRef     string            // commit hash, branch, or tag
	PluginName string            // plugin handles build logic
	BuildFlags map[string]string // plugin-specific flags (ldflags, tags, etc.)
	GoVersion  string            // optional Go version constraint
}

// BuildResult contains the result of a successful build
type BuildResult struct {
	BinaryPath string    // path to built binary
	GitCommit  string    // resolved commit hash
	GitRef     string    // original ref (branch/tag)
	BuiltAt    time.Time // when the build completed
	CacheKey   string    // for cache lookups
	BuildLog   string    // build output (for debugging)
}

// BinaryBuilder builds binaries from git sources
type BinaryBuilder interface {
	// Build builds a binary from source and returns the path to the built binary
	Build(ctx context.Context, spec BuildSpec) (*BuildResult, error)

	// GetCached returns a cached build if available and valid
	GetCached(ctx context.Context, spec BuildSpec) (*BuildResult, bool)

	// Clean removes cached builds older than maxAge
	Clean(ctx context.Context, maxAge time.Duration) error
}
