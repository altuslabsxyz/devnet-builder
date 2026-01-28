package server

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
)

// mockGitHubClient is a mock implementation of ports.GitHubClient.
type mockGitHubClient struct {
	releases []ports.GitHubRelease
	err      error
}

func (m *mockGitHubClient) FetchReleases(ctx context.Context) ([]ports.GitHubRelease, *ports.RateLimitInfo, error) {
	return m.releases, nil, m.err
}

func (m *mockGitHubClient) FetchReleasesWithCache(ctx context.Context) ([]ports.GitHubRelease, bool, error) {
	return m.releases, false, m.err
}

func (m *mockGitHubClient) GetImageVersions(ctx context.Context, packageName string) ([]ports.ImageVersion, error) {
	return nil, nil
}

func (m *mockGitHubClient) GetImageVersionsWithCache(ctx context.Context, packageName string) ([]ports.ImageVersion, bool, error) {
	return nil, false, nil
}

// mockGitHubClientFactory is a mock implementation of GitHubClientFactory.
type mockGitHubClientFactory struct {
	client            *mockGitHubClient
	lastNetworkName   string
	lastOwner         string
	lastRepo          string
	createClientCalls int
}

func (f *mockGitHubClientFactory) CreateClient(networkName, owner, repo string) ports.GitHubClient {
	f.lastNetworkName = networkName
	f.lastOwner = owner
	f.lastRepo = repo
	f.createClientCalls++
	return f.client
}

func TestNetworkService_ListBinaryVersions_MissingNetworkName(t *testing.T) {
	factory := &mockGitHubClientFactory{}
	svc := NewNetworkService(factory)

	_, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName: "",
	})

	if err == nil {
		t.Fatal("Expected error for missing network_name")
	}
}

func TestNetworkService_ListBinaryVersions_NetworkNotFound(t *testing.T) {
	factory := &mockGitHubClientFactory{}
	svc := NewNetworkService(factory)

	_, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName: "nonexistent-network",
	})

	if err == nil {
		t.Fatal("Expected error for non-existent network")
	}
}

func TestNetworkService_ListBinaryVersions_NilFactory(t *testing.T) {
	// Test that nil factory is handled gracefully
	svc := NewNetworkService(nil)

	// This will fail at network lookup stage for non-existent network
	// but if we had a registered network with GitHub source, it should catch nil factory
	_, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName: "test-network",
	})

	// Network not found is expected since no modules registered
	if err == nil {
		t.Fatal("Expected error")
	}
}

// Helper to find a GitHub-sourced network module for testing
func findGitHubSourcedModule() network.NetworkModule {
	modules := network.ListModules()
	for _, m := range modules {
		if m.BinarySource().IsGitHub() {
			return m
		}
	}
	return nil
}

func TestNetworkService_ListBinaryVersions_Success(t *testing.T) {
	testModule := findGitHubSourcedModule()
	if testModule == nil {
		t.Skip("No GitHub-sourced network modules registered for testing")
	}

	publishedAt := time.Now().Add(-24 * time.Hour)
	mockClient := &mockGitHubClient{
		releases: []ports.GitHubRelease{
			{
				TagName:     "v1.0.0",
				Name:        "Release v1.0.0",
				Prerelease:  false,
				PublishedAt: publishedAt,
				HTMLURL:     "https://github.com/example/repo/releases/tag/v1.0.0",
			},
			{
				TagName:     "v1.1.0-rc1",
				Name:        "Release v1.1.0-rc1",
				Prerelease:  true,
				PublishedAt: publishedAt.Add(time.Hour),
				HTMLURL:     "https://github.com/example/repo/releases/tag/v1.1.0-rc1",
			},
		},
	}

	factory := &mockGitHubClientFactory{client: mockClient}
	svc := NewNetworkService(factory)

	// Test without prereleases
	resp, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName:       testModule.Name(),
		IncludePrerelease: false,
	})
	if err != nil {
		t.Fatalf("ListBinaryVersions failed: %v", err)
	}

	if resp.NetworkName != testModule.Name() {
		t.Errorf("Expected NetworkName %q, got %q", testModule.Name(), resp.NetworkName)
	}
	if resp.SourceType != "github" {
		t.Errorf("Expected SourceType 'github', got %q", resp.SourceType)
	}
	if len(resp.Versions) != 1 {
		t.Errorf("Expected 1 version (no prereleases), got %d", len(resp.Versions))
	}
	if len(resp.Versions) > 0 && resp.Versions[0].Tag != "v1.0.0" {
		t.Errorf("Expected version v1.0.0, got %q", resp.Versions[0].Tag)
	}

	// Verify factory was called correctly
	if factory.createClientCalls != 1 {
		t.Errorf("Expected CreateClient to be called once, got %d", factory.createClientCalls)
	}
	binarySource := testModule.BinarySource()
	if factory.lastOwner != binarySource.Owner {
		t.Errorf("Expected owner %q, got %q", binarySource.Owner, factory.lastOwner)
	}
	if factory.lastRepo != binarySource.Repo {
		t.Errorf("Expected repo %q, got %q", binarySource.Repo, factory.lastRepo)
	}
}

func TestNetworkService_ListBinaryVersions_IncludePrerelease(t *testing.T) {
	testModule := findGitHubSourcedModule()
	if testModule == nil {
		t.Skip("No GitHub-sourced network modules registered for testing")
	}

	publishedAt := time.Now().Add(-24 * time.Hour)
	mockClient := &mockGitHubClient{
		releases: []ports.GitHubRelease{
			{TagName: "v1.0.0", Prerelease: false, PublishedAt: publishedAt},
			{TagName: "v1.1.0-rc1", Prerelease: true, PublishedAt: publishedAt.Add(time.Hour)},
		},
	}

	factory := &mockGitHubClientFactory{client: mockClient}
	svc := NewNetworkService(factory)

	// Test with prereleases
	resp, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName:       testModule.Name(),
		IncludePrerelease: true,
	})
	if err != nil {
		t.Fatalf("ListBinaryVersions failed: %v", err)
	}

	if len(resp.Versions) != 2 {
		t.Errorf("Expected 2 versions (with prereleases), got %d", len(resp.Versions))
	}
}

func TestNetworkService_ListBinaryVersions_FetchError(t *testing.T) {
	testModule := findGitHubSourcedModule()
	if testModule == nil {
		t.Skip("No GitHub-sourced network modules registered for testing")
	}

	mockClient := &mockGitHubClient{
		err: errors.New("connection timeout"),
	}

	factory := &mockGitHubClientFactory{client: mockClient}
	svc := NewNetworkService(factory)

	_, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName: testModule.Name(),
	})

	if err == nil {
		t.Fatal("Expected error when fetch fails")
	}
}

func TestNetworkService_ListBinaryVersions_EmptyReleases(t *testing.T) {
	testModule := findGitHubSourcedModule()
	if testModule == nil {
		t.Skip("No GitHub-sourced network modules registered for testing")
	}

	mockClient := &mockGitHubClient{
		releases: []ports.GitHubRelease{},
	}

	factory := &mockGitHubClientFactory{client: mockClient}
	svc := NewNetworkService(factory)

	resp, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName: testModule.Name(),
	})
	if err != nil {
		t.Fatalf("ListBinaryVersions failed: %v", err)
	}

	if len(resp.Versions) != 0 {
		t.Errorf("Expected 0 versions, got %d", len(resp.Versions))
	}
	// Should still have default version from module
	if resp.DefaultVersion == "" {
		t.Error("Expected default version to be set")
	}
}

func TestNetworkService_ListBinaryVersions_OnlyPrereleases(t *testing.T) {
	testModule := findGitHubSourcedModule()
	if testModule == nil {
		t.Skip("No GitHub-sourced network modules registered for testing")
	}

	publishedAt := time.Now()
	mockClient := &mockGitHubClient{
		releases: []ports.GitHubRelease{
			{TagName: "v1.0.0-alpha", Prerelease: true, PublishedAt: publishedAt},
			{TagName: "v1.0.0-beta", Prerelease: true, PublishedAt: publishedAt.Add(time.Hour)},
		},
	}

	factory := &mockGitHubClientFactory{client: mockClient}
	svc := NewNetworkService(factory)

	// Without prereleases - should return empty
	resp, err := svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName:       testModule.Name(),
		IncludePrerelease: false,
	})
	if err != nil {
		t.Fatalf("ListBinaryVersions failed: %v", err)
	}
	if len(resp.Versions) != 0 {
		t.Errorf("Expected 0 versions without prereleases, got %d", len(resp.Versions))
	}

	// With prereleases - should return both
	resp, err = svc.ListBinaryVersions(context.Background(), &v1.ListBinaryVersionsRequest{
		NetworkName:       testModule.Name(),
		IncludePrerelease: true,
	})
	if err != nil {
		t.Fatalf("ListBinaryVersions failed: %v", err)
	}
	if len(resp.Versions) != 2 {
		t.Errorf("Expected 2 versions with prereleases, got %d", len(resp.Versions))
	}
}
