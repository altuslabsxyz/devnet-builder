package internal

// Build-time variables injected via ldflags
// These are set by GoReleaser during the build process:
//
//	-X github.com/b-harvest/devnet-builder/internal.Version={{.Version}}
//	-X github.com/b-harvest/devnet-builder/internal.GitCommit={{.FullCommit}}
//	-X github.com/b-harvest/devnet-builder/internal.BuildDate={{.Date}}
var (
	// Version is the semantic version of the application.
	// Set at build time via ldflags, defaults to "0.1.0-dev" for local builds.
	Version = "0.1.0-dev"

	// GitCommit is the git commit hash of the build.
	// Set at build time via ldflags.
	GitCommit = "unknown"

	// BuildDate is the date when the binary was built.
	// Set at build time via ldflags.
	BuildDate = "unknown"
)

// BuildInfo contains all build-time information.
type BuildInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// GetBuildInfo returns the build information for the application.
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}
