package network

// BinarySourceType enumerates binary source types.
type BinarySourceType string

const (
	// BinarySourceGitHub indicates the binary is downloaded from GitHub releases.
	BinarySourceGitHub BinarySourceType = "github"

	// BinarySourceLocal indicates the binary is sourced from a local path.
	BinarySourceLocal BinarySourceType = "local"
)

// BinarySource defines how to acquire a network binary.
type BinarySource struct {
	// Type specifies the source type: "github" or "local"
	Type BinarySourceType

	// Owner is the GitHub organization/user (required if Type="github")
	Owner string

	// Repo is the GitHub repository name (required if Type="github")
	Repo string

	// LocalPath is the path to a local binary (required if Type="local")
	LocalPath string

	// MaxRetries is the number of download retry attempts (default: 3)
	MaxRetries int

	// AssetNameFunc generates the download asset filename for a release.
	// Parameters: version, goos (e.g., "linux"), goarch (e.g., "amd64")
	// Returns: Asset filename (e.g., "stabled_v1.0.0_linux_amd64.tar.gz")
	// If nil, a default naming pattern is used.
	AssetNameFunc func(version, goos, goarch string) string

	// BuildTags contains Go build tags required when building from source.
	// Example: ["no_dynamic_precompiles"] for disabling dynamic EVM precompiles.
	BuildTags []string
}

// GetMaxRetries returns the configured max retries, defaulting to 3 if not set.
func (b BinarySource) GetMaxRetries() int {
	if b.MaxRetries <= 0 {
		return 3
	}
	return b.MaxRetries
}

// IsGitHub returns true if this is a GitHub source.
func (b BinarySource) IsGitHub() bool {
	return b.Type == BinarySourceGitHub
}

// IsLocal returns true if this is a local source.
func (b BinarySource) IsLocal() bool {
	return b.Type == BinarySourceLocal
}
