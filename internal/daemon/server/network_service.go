package server

import (
	"context"
	"log/slog"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NetworkService implements the gRPC NetworkServiceServer.
type NetworkService struct {
	v1.UnimplementedNetworkServiceServer
	githubFactory GitHubClientFactory
	logger        *slog.Logger
}

// NewNetworkService creates a new NetworkService.
func NewNetworkService(githubFactory GitHubClientFactory) *NetworkService {
	return &NetworkService{
		githubFactory: githubFactory,
		logger:        slog.Default(),
	}
}

// SetLogger sets the logger for the service.
func (s *NetworkService) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// ListNetworks returns all registered network modules.
func (s *NetworkService) ListNetworks(ctx context.Context, req *v1.ListNetworksRequest) (*v1.ListNetworksResponse, error) {
	modules := network.ListModules()

	summaries := make([]*v1.NetworkSummary, 0, len(modules))
	for _, module := range modules {
		summaries = append(summaries, moduleToSummary(module))
	}

	return &v1.ListNetworksResponse{
		Networks: summaries,
	}, nil
}

// GetNetworkInfo returns detailed information about a specific network module.
func (s *NetworkService) GetNetworkInfo(ctx context.Context, req *v1.GetNetworkInfoRequest) (*v1.GetNetworkInfoResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	module, err := network.Get(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "network %q not found: %v", req.Name, err)
	}

	return &v1.GetNetworkInfoResponse{
		Network: moduleToNetworkInfo(module),
	}, nil
}

// moduleToSummary converts a NetworkModule to a NetworkSummary proto.
func moduleToSummary(module network.NetworkModule) *v1.NetworkSummary {
	return &v1.NetworkSummary{
		Name:                 module.Name(),
		DisplayName:          module.DisplayName(),
		Version:              module.Version(),
		BinaryName:           module.BinaryName(),
		AvailableNetworks:    module.AvailableNetworks(),
		DefaultBinaryVersion: module.DefaultBinaryVersion(),
	}
}

// moduleToNetworkInfo converts a NetworkModule to a NetworkInfo proto.
func moduleToNetworkInfo(module network.NetworkModule) *v1.NetworkInfo {
	// Get binary source
	binarySource := module.BinarySource()
	pbBinarySource := &v1.NetworkBinarySource{
		Type:      string(binarySource.Type),
		Owner:     binarySource.Owner,
		Repo:      binarySource.Repo,
		LocalPath: binarySource.LocalPath,
	}

	// Build endpoints map
	endpoints := make(map[string]*v1.EndpointInfo)
	for _, networkType := range module.AvailableNetworks() {
		endpoints[networkType] = &v1.EndpointInfo{
			RpcEndpoint: module.RPCEndpoint(networkType),
			SnapshotUrl: module.SnapshotURL(networkType),
		}
	}

	// Get default ports
	defaultPorts := module.DefaultPorts()
	pbPorts := &v1.NetworkPortConfig{
		Rpc:       int32(defaultPorts.RPC),
		P2P:       int32(defaultPorts.P2P),
		Grpc:      int32(defaultPorts.GRPC),
		GrpcWeb:   int32(defaultPorts.GRPCWeb),
		Api:       int32(defaultPorts.API),
		EvmRpc:    int32(defaultPorts.EVMRPC),
		EvmSocket: int32(defaultPorts.EVMWS),
	}

	return &v1.NetworkInfo{
		Name:                 module.Name(),
		DisplayName:          module.DisplayName(),
		Version:              module.Version(),
		BinaryName:           module.BinaryName(),
		DefaultBinaryVersion: module.DefaultBinaryVersion(),
		BinarySource:         pbBinarySource,
		Bech32Prefix:         module.Bech32Prefix(),
		BaseDenom:            module.BaseDenom(),
		DefaultChainId:       module.DefaultChainID(),
		Endpoints:            endpoints,
		DockerImage:          module.DockerImage(),
		DockerHomeDir:        module.DockerHomeDir(),
		DefaultPorts:         pbPorts,
	}
}

// ListBinaryVersions returns available binary versions for a network.
func (s *NetworkService) ListBinaryVersions(ctx context.Context, req *v1.ListBinaryVersionsRequest) (*v1.ListBinaryVersionsResponse, error) {
	if req.NetworkName == "" {
		return nil, status.Error(codes.InvalidArgument, "network_name is required")
	}

	// Get network module
	module, err := network.Get(req.NetworkName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "network %q not found: %v", req.NetworkName, err)
	}

	// Get binary source info
	binarySource := module.BinarySource()
	sourceType := string(binarySource.Type)

	// For local binary sources, versions cannot be listed
	if binarySource.IsLocal() {
		s.logger.Debug("binary source is local, returning empty version list",
			"network", req.NetworkName,
			"localPath", binarySource.LocalPath)
		return &v1.ListBinaryVersionsResponse{
			NetworkName:    req.NetworkName,
			Versions:       nil,
			DefaultVersion: module.DefaultBinaryVersion(),
			SourceType:     sourceType,
		}, nil
	}

	// For GitHub sources, fetch releases
	if !binarySource.IsGitHub() {
		return nil, status.Errorf(codes.Internal, "unsupported binary source type: %s", sourceType)
	}

	// Defensive check: ensure factory is configured
	if s.githubFactory == nil {
		return nil, status.Error(codes.Internal, "github client factory not configured")
	}

	// Create GitHub client for this network
	client := s.githubFactory.CreateClient(req.NetworkName, binarySource.Owner, binarySource.Repo)

	// Fetch releases (with cache fallback)
	releases, fromCache, err := client.FetchReleasesWithCache(ctx)
	if err != nil {
		s.logger.Error("failed to fetch releases",
			"network", req.NetworkName,
			"owner", binarySource.Owner,
			"repo", binarySource.Repo,
			"error", err)
		return nil, status.Errorf(codes.Internal, "failed to fetch releases: %v", err)
	}

	if fromCache {
		s.logger.Debug("using cached release data",
			"network", req.NetworkName)
	}

	// Convert to proto messages
	versions := make([]*v1.BinaryVersionInfo, 0, len(releases))
	for _, r := range releases {
		// Skip prereleases unless requested
		if r.Prerelease && !req.IncludePrerelease {
			continue
		}

		versions = append(versions, &v1.BinaryVersionInfo{
			Tag:         r.TagName,
			Name:        r.Name,
			Prerelease:  r.Prerelease,
			PublishedAt: timestamppb.New(r.PublishedAt),
			HtmlUrl:     r.HTMLURL,
		})
	}

	s.logger.Debug("returning binary versions",
		"network", req.NetworkName,
		"count", len(versions),
		"includePrerelease", req.IncludePrerelease)

	return &v1.ListBinaryVersionsResponse{
		NetworkName:    req.NetworkName,
		Versions:       versions,
		DefaultVersion: module.DefaultBinaryVersion(),
		SourceType:     sourceType,
	}, nil
}
