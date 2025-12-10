package github

import "time"

// GitHubRelease represents a release from the GitHub repository.
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`     // e.g., "v1.0.0"
	Name        string    `json:"name"`         // Release title
	Draft       bool      `json:"draft"`        // Is this a draft release?
	Prerelease  bool      `json:"prerelease"`   // Is this a pre-release?
	PublishedAt time.Time `json:"published_at"` // When was it published?
	HTMLURL     string    `json:"html_url"`     // Link to release page
}

// VersionCache represents cached version data with metadata.
type VersionCache struct {
	Version   int             `json:"version"`    // Cache schema version
	FetchedAt time.Time       `json:"fetched_at"` // When was data fetched?
	ExpiresAt time.Time       `json:"expires_at"` // When does cache expire?
	Releases  []GitHubRelease `json:"releases"`   // Cached releases
}

// CacheSchemaVersion is the current cache schema version.
const CacheSchemaVersion = 1

// DefaultCacheTTL is the default time-to-live for cached data.
const DefaultCacheTTL = 1 * time.Hour
