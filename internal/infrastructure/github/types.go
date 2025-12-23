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

// ContainerVersion represents a container package version from GHCR.
type ContainerVersion struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"` // SHA digest
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

// ImageVersion represents a simplified version for display.
type ImageVersion struct {
	Tag       string
	CreatedAt time.Time
	IsLatest  bool
}

// ContainerVersionCache represents cached container version data.
type ContainerVersionCache struct {
	Version   int                `json:"version"`
	FetchedAt time.Time          `json:"fetched_at"`
	ExpiresAt time.Time          `json:"expires_at"`
	Versions  []ContainerVersion `json:"versions"`
}
