package dto

// BuildInput contains the input for building a binary.
type BuildInput struct {
	Ref      string // Git ref (branch, tag, commit)
	Network  string // Network type for build tags
	UseCache bool   // Check cache first
	ToCache  bool   // Store in cache without activating
}

// BuildOutput contains the result of building.
type BuildOutput struct {
	BinaryPath string
	Ref        string
	CommitHash string
	CachedPath string
	CacheRef   string // Cache key for SetActive
	FromCache  bool
}

// CacheListInput contains the input for listing cached binaries.
type CacheListInput struct {
	ShowDetails bool
}

// CacheBinary represents a cached binary entry.
type CacheBinary struct {
	Ref        string
	Path       string
	CommitHash string
	IsActive   bool
	Size       int64
	BuildTime  string // RFC3339 formatted time
	Network    string
}

// CacheListOutput contains the list of cached binaries.
type CacheListOutput struct {
	Binaries  []CacheBinary
	ActiveRef string
	TotalSize int64
}

// CacheCleanInput contains the input for cleaning cache.
type CacheCleanInput struct {
	KeepActive bool     // Keep the active binary
	KeepRecent int      // Keep N most recent binaries
	Refs       []string // Specific refs to remove (if empty, clean all)
}

// CacheCleanOutput contains the result of cache cleaning.
type CacheCleanOutput struct {
	Removed    []string
	SpaceFreed int64
	Kept       []string
}

// VersionsInput contains the input for listing versions.
type VersionsInput struct {
	Network string
	ShowAll bool
	Limit   int
}

// VersionInfo represents a version entry.
type VersionInfo struct {
	Tag        string
	CommitHash string
	Date       string
	IsCached   bool
	IsActive   bool
	IsLatest   bool
}

// VersionsOutput contains available versions.
type VersionsOutput struct {
	Versions   []VersionInfo
	CurrentTag string
	LatestTag  string
}
