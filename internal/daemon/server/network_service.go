package server

import (
	"context"
	"log/slog"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NetworkService implements the gRPC NetworkServiceServer.
type NetworkService struct {
	v1.UnimplementedNetworkServiceServer
	logger *slog.Logger
}

// NewNetworkService creates a new NetworkService.
func NewNetworkService() *NetworkService {
	return &NetworkService{
		logger: slog.Default(),
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
