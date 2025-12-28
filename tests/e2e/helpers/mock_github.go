package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockGitHubAPI provides a mock GitHub API server for testing
// This eliminates dependency on actual GitHub API and allows testing version/release logic
type MockGitHubAPI struct {
	t        *testing.T
	server   *httptest.Server
	releases []GitHubRelease // Mock release data
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	CreatedAt   string        `json:"created_at"`
	PublishedAt string        `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a release asset
type GitHubAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int    `json:"size"`
}

// NewMockGitHubAPI creates a new mock GitHub API server
func NewMockGitHubAPI(t *testing.T) *MockGitHubAPI {
	t.Helper()

	mock := &MockGitHubAPI{
		t:        t,
		releases: make([]GitHubRelease, 0),
	}

	// Create HTTP server first (needed by AddDefaultReleases)
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))

	// Add default releases (now server.URL is available)
	mock.AddDefaultReleases()

	// Register cleanup
	t.Cleanup(func() {
		mock.server.Close()
	})

	t.Logf("Mock GitHub API server started at %s", mock.server.URL)
	return mock
}

// URL returns the base URL of the mock GitHub API server
func (m *MockGitHubAPI) URL() string {
	return m.server.URL
}

// AddRelease adds a mock release to the API
func (m *MockGitHubAPI) AddRelease(release GitHubRelease) {
	m.releases = append(m.releases, release)
}

// AddDefaultReleases adds common test releases
func (m *MockGitHubAPI) AddDefaultReleases() {
	// Add v1.0.0 release (latest stable)
	m.AddRelease(GitHubRelease{
		TagName:     "v1.0.0",
		Name:        "Release v1.0.0",
		Draft:       false,
		Prerelease:  false,
		CreatedAt:   "2024-01-01T00:00:00Z",
		PublishedAt: "2024-01-01T00:00:00Z",
		Assets: []GitHubAsset{
			{
				Name:        "stable-linux-amd64",
				DownloadURL: m.server.URL + "/download/v1.0.0/stable-linux-amd64",
				Size:        10485760, // 10MB
			},
		},
	})

	// Add v1.1.0-rc1 release (prerelease)
	m.AddRelease(GitHubRelease{
		TagName:     "v1.1.0-rc1",
		Name:        "Release v1.1.0-rc1",
		Draft:       false,
		Prerelease:  true,
		CreatedAt:   "2024-02-01T00:00:00Z",
		PublishedAt: "2024-02-01T00:00:00Z",
		Assets: []GitHubAsset{
			{
				Name:        "stable-linux-amd64",
				DownloadURL: m.server.URL + "/download/v1.1.0-rc1/stable-linux-amd64",
				Size:        10485760,
			},
		},
	})

	// Add v0.9.0 release (older stable)
	m.AddRelease(GitHubRelease{
		TagName:     "v0.9.0",
		Name:        "Release v0.9.0",
		Draft:       false,
		Prerelease:  false,
		CreatedAt:   "2023-12-01T00:00:00Z",
		PublishedAt: "2023-12-01T00:00:00Z",
		Assets: []GitHubAsset{
			{
				Name:        "stable-linux-amd64",
				DownloadURL: m.server.URL + "/download/v0.9.0/stable-linux-amd64",
				Size:        9437184, // 9MB
			},
		},
	})
}

// ClearReleases removes all mock releases
func (m *MockGitHubAPI) ClearReleases() {
	m.releases = make([]GitHubRelease, 0)
}

// handleRequest handles HTTP requests to the mock GitHub API
func (m *MockGitHubAPI) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.t.Logf("Mock GitHub API received request: %s %s", r.Method, r.URL.Path)

	// Route requests
	switch {
	case strings.HasPrefix(r.URL.Path, "/repos/") && strings.HasSuffix(r.URL.Path, "/releases"):
		m.handleListReleases(w, r)
	case strings.HasPrefix(r.URL.Path, "/repos/") && strings.Contains(r.URL.Path, "/releases/tags/"):
		m.handleGetRelease(w, r)
	case strings.HasPrefix(r.URL.Path, "/repos/") && strings.Contains(r.URL.Path, "/releases/latest"):
		m.handleGetLatestRelease(w, r)
	case strings.HasPrefix(r.URL.Path, "/download/"):
		m.handleDownloadAsset(w, r)
	default:
		m.t.Logf("Unknown GitHub API path: %s", r.URL.Path)
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// handleListReleases handles GET /repos/:owner/:repo/releases
func (m *MockGitHubAPI) handleListReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return all releases
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(m.releases); err != nil {
		m.t.Logf("Failed to encode releases: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleGetRelease handles GET /repos/:owner/:repo/releases/tags/:tag
func (m *MockGitHubAPI) handleGetRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract tag from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	tag := parts[len(parts)-1]

	// Find release by tag
	for _, release := range m.releases {
		if release.TagName == tag {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(release); err != nil {
				m.t.Logf("Failed to encode release: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}
	}

	http.Error(w, fmt.Sprintf("Release not found: %s", tag), http.StatusNotFound)
}

// handleGetLatestRelease handles GET /repos/:owner/:repo/releases/latest
func (m *MockGitHubAPI) handleGetLatestRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Find latest non-prerelease release
	var latest *GitHubRelease
	for i := range m.releases {
		release := &m.releases[i]
		if !release.Draft && !release.Prerelease {
			if latest == nil {
				latest = release
			} else if release.TagName > latest.TagName {
				// Compare versions (simple string comparison for testing)
				latest = release
			}
		}
	}

	if latest == nil {
		http.Error(w, "No releases found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(latest); err != nil {
		m.t.Logf("Failed to encode latest release: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleDownloadAsset handles GET /download/:tag/:asset
func (m *MockGitHubAPI) handleDownloadAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For testing, just return a small binary blob
	// Real implementation would serve actual binary
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")

	// Write mock binary data (small for fast tests)
	mockBinary := []byte("MOCK_BINARY_DATA")
	if _, err := w.Write(mockBinary); err != nil {
		m.t.Logf("Failed to write asset: %v", err)
	}

	m.t.Logf("Served mock asset download: %s", r.URL.Path)
}

// SimulateAPIError simulates a GitHub API error response
func (m *MockGitHubAPI) SimulateAPIError(statusCode int, message string) {
	// Replace handler temporarily
	originalHandler := m.server.Config.Handler
	m.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, statusCode)
	})

	// Restore original handler after first request
	m.t.Cleanup(func() {
		m.server.Config.Handler = originalHandler
	})
}

// SimulateRateLimit simulates GitHub API rate limiting
func (m *MockGitHubAPI) SimulateRateLimit() {
	m.SimulateAPIError(http.StatusTooManyRequests, "API rate limit exceeded")
}

// GetReleaseByTag returns a release by tag name
func (m *MockGitHubAPI) GetReleaseByTag(tag string) *GitHubRelease {
	for i := range m.releases {
		if m.releases[i].TagName == tag {
			return &m.releases[i]
		}
	}
	return nil
}

// GetLatestRelease returns the latest non-prerelease release
func (m *MockGitHubAPI) GetLatestRelease() *GitHubRelease {
	var latest *GitHubRelease
	for i := range m.releases {
		release := &m.releases[i]
		if !release.Draft && !release.Prerelease {
			if latest == nil {
				latest = release
			} else if release.TagName > latest.TagName {
				latest = release
			}
		}
	}
	return latest
}
