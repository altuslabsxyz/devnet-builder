package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// GitHubAPIBaseURL is the base URL for GitHub API.
	GitHubAPIBaseURL = "https://api.github.com"

	// DefaultOwner is the default repository owner.
	DefaultOwner = "stablelabs"

	// DefaultRepo is the default repository name.
	DefaultRepo = "stable"

	// DefaultPerPage is the default number of results per page.
	DefaultPerPage = 100
)

// RateLimitInfo contains GitHub API rate limit information.
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// Client is a GitHub API client for fetching releases.
type Client struct {
	httpClient *http.Client
	token      string
	owner      string
	repo       string
	cache      *CacheManager
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithToken sets the GitHub token for authentication.
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

// WithOwnerRepo sets the repository owner and name.
func WithOwnerRepo(owner, repo string) ClientOption {
	return func(c *Client) {
		c.owner = owner
		c.repo = repo
	}
}

// WithCache sets the cache manager.
func WithCache(cache *CacheManager) ClientOption {
	return func(c *Client) {
		c.cache = cache
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient creates a new GitHub API client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		owner:      DefaultOwner,
		repo:       DefaultRepo,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// FetchReleases fetches all releases from the GitHub repository.
func (c *Client) FetchReleases(ctx context.Context) ([]GitHubRelease, *RateLimitInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d", GitHubAPIBaseURL, c.owner, c.repo, DefaultPerPage)

	var allReleases []GitHubRelease
	var rateLimitInfo *RateLimitInfo

	for url != "" {
		releases, nextURL, rateLimit, err := c.fetchPage(ctx, url)
		if err != nil {
			return nil, rateLimit, err
		}

		allReleases = append(allReleases, releases...)
		rateLimitInfo = rateLimit
		url = nextURL
	}

	// Filter out draft releases
	filtered := make([]GitHubRelease, 0, len(allReleases))
	for _, r := range allReleases {
		if !r.Draft {
			filtered = append(filtered, r)
		}
	}

	return filtered, rateLimitInfo, nil
}

// fetchPage fetches a single page of releases.
func (c *Client) fetchPage(ctx context.Context, url string) ([]GitHubRelease, string, *RateLimitInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	// Parse rate limit info
	rateLimitInfo := parseRateLimitHeaders(resp)

	// Check for rate limiting
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, "", rateLimitInfo, &RateLimitError{
			Limit:     rateLimitInfo.Limit,
			Remaining: rateLimitInfo.Remaining,
			Reset:     rateLimitInfo.Reset,
		}
	}

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, "", rateLimitInfo, &AuthenticationError{
			Message: "GitHub authentication failed. Check your token.",
		}
	}

	// Check for 404 Not Found - often means private repo without proper token
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", rateLimitInfo, &NotFoundError{
			Message: `Repository not found. This usually means the repository is private and requires authentication.

To set up GitHub authentication:
  1. Create a Personal Access Token at https://github.com/settings/tokens
     - For classic tokens: select 'repo' scope
     - For fine-grained tokens: select 'Contents' read access
  2. Configure the token using one of these methods:
     - Run: devnet-builder config set github-token <your-token>
     - Or set environment variable: export GITHUB_TOKEN=<your-token>`,
		}
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", rateLimitInfo, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", rateLimitInfo, fmt.Errorf("failed to read response: %w", err)
	}

	var releases []GitHubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, "", rateLimitInfo, fmt.Errorf("failed to parse releases: %w", err)
	}

	// Parse Link header for pagination
	nextURL := parseNextPageURL(resp.Header.Get("Link"))

	return releases, nextURL, rateLimitInfo, nil
}

// parseRateLimitHeaders extracts rate limit info from response headers.
func parseRateLimitHeaders(resp *http.Response) *RateLimitInfo {
	info := &RateLimitInfo{}

	if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
		info.Limit, _ = strconv.Atoi(limit)
	}

	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		info.Remaining, _ = strconv.Atoi(remaining)
	}

	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		resetUnix, _ := strconv.ParseInt(reset, 10, 64)
		info.Reset = time.Unix(resetUnix, 0)
	}

	return info
}

// parseNextPageURL extracts the next page URL from the Link header.
func parseNextPageURL(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	// Link header format: <url>; rel="next", <url>; rel="last"
	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			// Extract URL between < and >
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start != -1 && end != -1 && start < end {
				return part[start+1 : end]
			}
		}
	}

	return ""
}

// FetchReleasesWithCache fetches releases from API (always fresh).
// Cache is only used as fallback when API request fails.
// Returns (releases, fromCache, error).
func (c *Client) FetchReleasesWithCache(ctx context.Context) ([]GitHubRelease, bool, error) {
	// Always fetch fresh data from API for releases
	releases, _, err := c.FetchReleases(ctx)
	if err != nil {
		// If we have cache, use it as fallback with warning
		if c.cache != nil {
			cache, loadErr := c.cache.Load()
			if loadErr == nil && cache != nil {
				return cache.Releases, true, &StaleDataWarning{
					Message: fmt.Sprintf("Using cached data (fetched %s ago): %v",
						time.Since(cache.FetchedAt).Round(time.Minute), err),
				}
			}
		}
		return nil, false, err
	}

	// Save to cache for fallback use
	if c.cache != nil {
		cache := &VersionCache{
			Version:   CacheSchemaVersion,
			FetchedAt: time.Now(),
			ExpiresAt: time.Now().Add(c.cache.TTL()),
			Releases:  releases,
		}
		_ = c.cache.Save(cache) // Ignore save errors
	}

	return releases, false, nil
}

// FetchContainerVersions fetches container package versions from GHCR.
func (c *Client) FetchContainerVersions(ctx context.Context, packageName string) ([]ContainerVersion, *RateLimitInfo, error) {
	url := fmt.Sprintf("%s/orgs/%s/packages/container/%s/versions?per_page=%d&state=active",
		GitHubAPIBaseURL, c.owner, packageName, DefaultPerPage)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch container versions: %w", err)
	}
	defer resp.Body.Close()

	// Parse rate limit info
	rateLimitInfo := parseRateLimitHeaders(resp)

	// Check for rate limiting
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimitInfo, &RateLimitError{
			Limit:     rateLimitInfo.Limit,
			Remaining: rateLimitInfo.Remaining,
			Reset:     rateLimitInfo.Reset,
		}
	}

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, rateLimitInfo, &AuthenticationError{
			Message: "GitHub authentication failed. Check your token.",
		}
	}

	// Check for 404 Not Found - often means private repo without proper token
	if resp.StatusCode == http.StatusNotFound {
		return nil, rateLimitInfo, &NotFoundError{
			Message: `Repository not found. This usually means the repository is private and requires authentication.

To set up GitHub authentication:
  1. Create a Personal Access Token at https://github.com/settings/tokens
     - For classic tokens: select 'repo' scope
     - For fine-grained tokens: select 'Contents' read access
  2. Configure the token using one of these methods:
     - Run: devnet-builder config set github-token <your-token>
     - Or set environment variable: export GITHUB_TOKEN=<your-token>`,
		}
	}

	// Check for other errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, rateLimitInfo, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, rateLimitInfo, fmt.Errorf("failed to read response: %w", err)
	}

	var versions []ContainerVersion
	if err := json.Unmarshal(body, &versions); err != nil {
		return nil, rateLimitInfo, fmt.Errorf("failed to parse container versions: %w", err)
	}

	return versions, rateLimitInfo, nil
}

// GetImageVersions returns simplified version list for UI display.
func (c *Client) GetImageVersions(ctx context.Context, packageName string) ([]ImageVersion, error) {
	versions, _, err := c.FetchContainerVersions(ctx, packageName)
	if err != nil {
		return nil, err
	}

	// Extract unique tags and create ImageVersion list
	tagMap := make(map[string]ImageVersion)
	for _, v := range versions {
		for _, tag := range v.Metadata.Container.Tags {
			// Skip sha256 digest tags
			if strings.HasPrefix(tag, "sha256:") {
				continue
			}
			// Use the most recent created_at for each tag
			existing, exists := tagMap[tag]
			if !exists || v.CreatedAt.After(existing.CreatedAt) {
				tagMap[tag] = ImageVersion{
					Tag:       tag,
					CreatedAt: v.CreatedAt,
				}
			}
		}
	}

	// Convert map to slice and sort by date (newest first)
	result := make([]ImageVersion, 0, len(tagMap))
	for _, iv := range tagMap {
		result = append(result, iv)
	}

	// Sort by CreatedAt descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	// Mark the first one as latest
	if len(result) > 0 {
		result[0].IsLatest = true
	}

	return result, nil
}

// GetImageVersionsWithCache returns image versions from API (always fresh).
// Cache is only used as fallback when API request fails.
// Returns (versions, fromCache, error).
func (c *Client) GetImageVersionsWithCache(ctx context.Context, packageName string) ([]ImageVersion, bool, error) {
	// Always fetch fresh data from API
	containerVersions, _, err := c.FetchContainerVersions(ctx, packageName)
	if err != nil {
		// If we have cache, use it as fallback with warning
		if c.cache != nil {
			cache, loadErr := c.cache.LoadContainerCache()
			if loadErr == nil && cache != nil {
				versions := c.convertToImageVersions(cache.Versions)
				return versions, true, &StaleDataWarning{
					Message: fmt.Sprintf("Using cached data (fetched %s ago): %v",
						time.Since(cache.FetchedAt).Round(time.Minute), err),
				}
			}
		}
		return nil, false, err
	}

	// Save to cache for fallback use
	if c.cache != nil {
		cache := &ContainerVersionCache{
			Version:   CacheSchemaVersion,
			FetchedAt: time.Now(),
			ExpiresAt: time.Now().Add(c.cache.TTL()),
			Versions:  containerVersions,
		}
		_ = c.cache.SaveContainerCache(cache) // Ignore save errors
	}

	// Convert to ImageVersion list
	versions := c.convertToImageVersions(containerVersions)
	return versions, false, nil
}

// convertToImageVersions converts ContainerVersion list to ImageVersion list.
func (c *Client) convertToImageVersions(versions []ContainerVersion) []ImageVersion {
	tagMap := make(map[string]ImageVersion)
	for _, v := range versions {
		for _, tag := range v.Metadata.Container.Tags {
			// Skip sha256 digest tags
			if strings.HasPrefix(tag, "sha256:") {
				continue
			}
			// Use the most recent created_at for each tag
			existing, exists := tagMap[tag]
			if !exists || v.CreatedAt.After(existing.CreatedAt) {
				tagMap[tag] = ImageVersion{
					Tag:       tag,
					CreatedAt: v.CreatedAt,
				}
			}
		}
	}

	// Convert map to slice and sort by date (newest first)
	result := make([]ImageVersion, 0, len(tagMap))
	for _, iv := range tagMap {
		result = append(result, iv)
	}

	// Sort by CreatedAt descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	// Mark the first one as latest
	if len(result) > 0 {
		result[0].IsLatest = true
	}

	return result
}
