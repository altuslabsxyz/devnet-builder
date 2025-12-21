package ports

import (
	"context"
	"time"
)

// GitHubRelease represents a release from a GitHub repository.
type GitHubRelease struct {
	TagName     string
	Name        string
	Draft       bool
	Prerelease  bool
	PublishedAt time.Time
	HTMLURL     string
}

// ImageVersion represents a container image version.
type ImageVersion struct {
	Tag       string
	CreatedAt time.Time
	IsLatest  bool
}

// RateLimitInfo contains GitHub API rate limit information.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// GitHubClient defines operations for interacting with GitHub API.
type GitHubClient interface {
	// FetchReleases fetches all releases from the repository.
	FetchReleases(ctx context.Context) ([]GitHubRelease, *RateLimitInfo, error)

	// FetchReleasesWithCache fetches releases with caching support.
	// Returns releases, whether from cache, and any error.
	FetchReleasesWithCache(ctx context.Context) ([]GitHubRelease, bool, error)

	// GetImageVersions returns container image versions.
	GetImageVersions(ctx context.Context, packageName string) ([]ImageVersion, error)

	// GetImageVersionsWithCache returns image versions with caching.
	GetImageVersionsWithCache(ctx context.Context, packageName string) ([]ImageVersion, bool, error)
}
