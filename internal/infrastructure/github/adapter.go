// Package github provides infrastructure adapter for GitHub API operations.
package github

import (
	"context"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// Adapter implements ports.GitHubClient using the github Client.
type Adapter struct {
	client *Client
}

// AdapterOption is a function that configures an Adapter.
type AdapterOption func(*Adapter)

// NewAdapter creates a new GitHub adapter.
func NewAdapter(token, owner, repo, cacheDir string) *Adapter {
	opts := []ClientOption{
		WithOwnerRepo(owner, repo),
	}

	if token != "" {
		opts = append(opts, WithToken(token))
	}

	if cacheDir != "" {
		cache := NewCacheManager(cacheDir, DefaultCacheTTL)
		opts = append(opts, WithCache(cache))
	}

	return &Adapter{
		client: NewClient(opts...),
	}
}

// FetchReleases fetches all releases from the repository.
func (a *Adapter) FetchReleases(ctx context.Context) ([]ports.GitHubRelease, *ports.RateLimitInfo, error) {
	releases, rateLimit, err := a.client.FetchReleases(ctx)
	if err != nil {
		return nil, convertRateLimitInfo(rateLimit), err
	}

	return convertReleases(releases), convertRateLimitInfo(rateLimit), nil
}

// FetchReleasesWithCache fetches releases with caching support.
func (a *Adapter) FetchReleasesWithCache(ctx context.Context) ([]ports.GitHubRelease, bool, error) {
	releases, fromCache, err := a.client.FetchReleasesWithCache(ctx)
	if err != nil {
		return nil, fromCache, err
	}

	return convertReleases(releases), fromCache, nil
}

// GetImageVersions returns container image versions.
func (a *Adapter) GetImageVersions(ctx context.Context, packageName string) ([]ports.ImageVersion, error) {
	versions, err := a.client.GetImageVersions(ctx, packageName)
	if err != nil {
		return nil, err
	}

	return convertImageVersions(versions), nil
}

// GetImageVersionsWithCache returns image versions with caching.
func (a *Adapter) GetImageVersionsWithCache(ctx context.Context, packageName string) ([]ports.ImageVersion, bool, error) {
	versions, fromCache, err := a.client.GetImageVersionsWithCache(ctx, packageName)
	if err != nil {
		return nil, fromCache, err
	}

	return convertImageVersions(versions), fromCache, nil
}

// convertReleases converts internal releases to port releases.
func convertReleases(releases []GitHubRelease) []ports.GitHubRelease {
	result := make([]ports.GitHubRelease, len(releases))
	for i, r := range releases {
		result[i] = ports.GitHubRelease{
			TagName:     r.TagName,
			Name:        r.Name,
			Draft:       r.Draft,
			Prerelease:  r.Prerelease,
			PublishedAt: r.PublishedAt,
			HTMLURL:     r.HTMLURL,
		}
	}
	return result
}

// convertImageVersions converts internal image versions to port versions.
func convertImageVersions(versions []ImageVersion) []ports.ImageVersion {
	result := make([]ports.ImageVersion, len(versions))
	for i, v := range versions {
		result[i] = ports.ImageVersion{
			Tag:       v.Tag,
			CreatedAt: v.CreatedAt,
			IsLatest:  v.IsLatest,
		}
	}
	return result
}

// convertRateLimitInfo converts internal rate limit info to port info.
func convertRateLimitInfo(info *RateLimitInfo) *ports.RateLimitInfo {
	if info == nil {
		return nil
	}
	return &ports.RateLimitInfo{
		Limit:     info.Limit,
		Remaining: info.Remaining,
		Reset:     info.Reset,
	}
}
