package di

import (
	"context"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	infrabuilder "github.com/b-harvest/devnet-builder/internal/infrastructure/builder"
	infracache "github.com/b-harvest/devnet-builder/internal/infrastructure/cache"
	infragenesis "github.com/b-harvest/devnet-builder/internal/infrastructure/genesis"
	infranode "github.com/b-harvest/devnet-builder/internal/infrastructure/node"
	infrapersistence "github.com/b-harvest/devnet-builder/internal/infrastructure/persistence"
	infraprocess "github.com/b-harvest/devnet-builder/internal/infrastructure/process"
	infrarpc "github.com/b-harvest/devnet-builder/internal/infrastructure/rpc"
	infrasnapshot "github.com/b-harvest/devnet-builder/internal/infrastructure/snapshot"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// InfrastructureFactory creates infrastructure implementations.
type InfrastructureFactory struct {
	homeDir    string
	logger     *output.Logger
	module     network.NetworkModule
	dockerMode bool
}

// NewInfrastructureFactory creates a new infrastructure factory.
func NewInfrastructureFactory(homeDir string, logger *output.Logger) *InfrastructureFactory {
	return &InfrastructureFactory{
		homeDir: homeDir,
		logger:  logger,
	}
}

// WithNetworkModule sets the network module.
func (f *InfrastructureFactory) WithNetworkModule(module network.NetworkModule) *InfrastructureFactory {
	f.module = module
	return f
}

// WithDockerMode sets whether to use Docker execution.
func (f *InfrastructureFactory) WithDockerMode(useDocker bool) *InfrastructureFactory {
	f.dockerMode = useDocker
	return f
}

// CreateDevnetRepository creates a DevnetRepository implementation.
func (f *InfrastructureFactory) CreateDevnetRepository() ports.DevnetRepository {
	return infrapersistence.NewDevnetFileRepository()
}

// CreateNodeRepository creates a NodeRepository implementation.
func (f *InfrastructureFactory) CreateNodeRepository() ports.NodeRepository {
	return infrapersistence.NewNodeFileRepository()
}

// CreateProcessExecutor creates a ProcessExecutor implementation.
func (f *InfrastructureFactory) CreateProcessExecutor() ports.ProcessExecutor {
	if f.dockerMode {
		return infraprocess.NewDockerExecutor()
	}
	return infraprocess.NewLocalExecutor()
}

// CreateDockerExecutor creates a DockerExecutor implementation.
func (f *InfrastructureFactory) CreateDockerExecutor() ports.DockerExecutor {
	return infraprocess.NewDockerExecutor()
}

// CreateRPCClient creates an RPCClient for the given host and port.
func (f *InfrastructureFactory) CreateRPCClient(host string, port int) ports.RPCClient {
	return infrarpc.NewCosmosRPCClient(host, port)
}

// CreateBinaryCache creates a BinaryCache implementation.
func (f *InfrastructureFactory) CreateBinaryCache() (ports.BinaryCache, error) {
	binaryName := "binary"
	if f.module != nil {
		binaryName = f.module.BinaryName()
	}
	return infracache.NewBinaryCacheAdapter(f.homeDir, binaryName, f.logger)
}

// CreateBuilder creates a Builder implementation.
func (f *InfrastructureFactory) CreateBuilder() ports.Builder {
	return infrabuilder.NewBuilderAdapter(f.homeDir, f.logger, f.module)
}

// CreateSnapshotFetcher creates a SnapshotFetcher implementation.
func (f *InfrastructureFactory) CreateSnapshotFetcher() ports.SnapshotFetcher {
	return infrasnapshot.NewFetcherAdapter(f.homeDir, f.logger)
}

// CreateGenesisFetcher creates a GenesisFetcher implementation.
func (f *InfrastructureFactory) CreateGenesisFetcher() ports.GenesisFetcher {
	binaryPath := ""
	dockerImage := ""
	if f.module != nil {
		binaryPath = f.homeDir + "/bin/" + f.module.BinaryName()
		// Use module's default docker image if available
	}
	return infragenesis.NewFetcherAdapter(f.homeDir, binaryPath, dockerImage, f.dockerMode, f.logger)
}

// CreateNodeManagerFactory creates a NodeManagerFactory.
func (f *InfrastructureFactory) CreateNodeManagerFactory() *infranode.NodeManagerFactory {
	return infranode.NewNodeManagerFactory(f.logger)
}

// CreateHealthChecker creates a HealthChecker implementation.
func (f *InfrastructureFactory) CreateHealthChecker(rpcPort int) ports.HealthChecker {
	return &healthCheckerAdapter{
		factory: f,
	}
}

// healthCheckerAdapter adapts RPCClient to HealthChecker interface.
type healthCheckerAdapter struct {
	factory *InfrastructureFactory
}

func (h *healthCheckerAdapter) CheckNode(ctx context.Context, rpcEndpoint string) (*ports.HealthStatus, error) {
	// Parse endpoint to get host and port (simplified - assumes http://host:port format)
	client := infrarpc.NewCosmosRPCClientWithURL(rpcEndpoint)

	height, err := client.GetBlockHeight(ctx)
	if err != nil {
		return &ports.HealthStatus{
			IsRunning: false,
			Status:    ports.NodeStatusError,
			Error:     err,
		}, nil
	}

	isRunning := client.IsChainRunning(ctx)
	status := ports.NodeStatusStopped
	if isRunning {
		status = ports.NodeStatusRunning
	}

	return &ports.HealthStatus{
		IsRunning:   isRunning,
		Status:      status,
		BlockHeight: height,
	}, nil
}

func (h *healthCheckerAdapter) CheckAllNodes(ctx context.Context, nodes []*ports.NodeMetadata) ([]*ports.HealthStatus, error) {
	results := make([]*ports.HealthStatus, len(nodes))
	for i, node := range nodes {
		endpoint := fmt.Sprintf("http://localhost:%d", node.Ports.RPC)
		status, err := h.CheckNode(ctx, endpoint)
		if err != nil {
			results[i] = &ports.HealthStatus{
				NodeIndex: node.Index,
				NodeName:  node.Name,
				Status:    ports.NodeStatusError,
				Error:     err,
			}
			continue
		}
		status.NodeIndex = node.Index
		status.NodeName = node.Name
		results[i] = status
	}
	return results, nil
}

// WireContainer wires all infrastructure components into a Container.
func (f *InfrastructureFactory) WireContainer(opts ...Option) (*Container, error) {
	// Create all infrastructure implementations
	devnetRepo := f.CreateDevnetRepository()
	nodeRepo := f.CreateNodeRepository()
	executor := f.CreateProcessExecutor()
	snapshotFetcher := f.CreateSnapshotFetcher()
	genesisFetcher := f.CreateGenesisFetcher()
	builder := f.CreateBuilder()

	binaryCache, err := f.CreateBinaryCache()
	if err != nil {
		return nil, err
	}

	// Default RPC client (node0)
	rpcClient := f.CreateRPCClient("localhost", 26657)
	healthChecker := f.CreateHealthChecker(26657)

	// Create container with all dependencies
	allOpts := []Option{
		WithLogger(f.logger),
		WithDevnetRepository(devnetRepo),
		WithNodeRepository(nodeRepo),
		WithExecutor(executor),
		WithBinaryCache(binaryCache),
		WithRPCClient(rpcClient),
		WithSnapshotFetcher(snapshotFetcher),
		WithGenesisFetcher(genesisFetcher),
		WithHealthChecker(healthChecker),
		WithBuilder(builder),
	}

	// Add network module adapter if available
	if f.module != nil {
		allOpts = append(allOpts, WithNetworkModule(&networkModuleAdapter{module: f.module}))
	}

	// Append user-provided options
	allOpts = append(allOpts, opts...)

	return New(allOpts...), nil
}

// networkModuleAdapter adapts network.NetworkModule to ports.NetworkModule.
type networkModuleAdapter struct {
	module network.NetworkModule
}

func (a *networkModuleAdapter) Name() string {
	return a.module.Name()
}

func (a *networkModuleAdapter) DisplayName() string {
	return a.module.DisplayName()
}

func (a *networkModuleAdapter) Version() string {
	return a.module.Version()
}

func (a *networkModuleAdapter) BinaryName() string {
	return a.module.BinaryName()
}

func (a *networkModuleAdapter) DefaultBinaryVersion() string {
	return a.module.DefaultBinaryVersion()
}

func (a *networkModuleAdapter) DefaultChainID() string {
	return a.module.DefaultChainID()
}

func (a *networkModuleAdapter) Bech32Prefix() string {
	return a.module.Bech32Prefix()
}

func (a *networkModuleAdapter) BaseDenom() string {
	return a.module.BaseDenom()
}

func (a *networkModuleAdapter) InitCommand(homeDir, chainID, moniker string) []string {
	return a.module.InitCommand(homeDir, chainID, moniker)
}

func (a *networkModuleAdapter) StartCommand(homeDir string) []string {
	return a.module.StartCommand(homeDir)
}

func (a *networkModuleAdapter) ExportCommand(homeDir string) []string {
	return a.module.ExportCommand(homeDir)
}

func (a *networkModuleAdapter) DefaultNodeHome() string {
	return a.module.DefaultNodeHome()
}

func (a *networkModuleAdapter) PIDFileName() string {
	return a.module.PIDFileName()
}

func (a *networkModuleAdapter) LogFileName() string {
	return a.module.LogFileName()
}

func (a *networkModuleAdapter) DockerImage() string {
	return a.module.DockerImage()
}

func (a *networkModuleAdapter) DockerImageTag(version string) string {
	return a.module.DockerImageTag(version)
}

func (a *networkModuleAdapter) DockerHomeDir() string {
	return a.module.DockerHomeDir()
}

func (a *networkModuleAdapter) DefaultPorts() ports.PortConfig {
	np := a.module.DefaultPorts()
	return ports.PortConfig{
		RPC:   np.RPC,
		P2P:   np.P2P,
		GRPC:  np.GRPC,
		API:   np.API,
		EVM:   np.EVMRPC,
		EVMWS: np.EVMWS,
	}
}

func (a *networkModuleAdapter) SnapshotURL(networkType string) string {
	return a.module.SnapshotURL(networkType)
}

func (a *networkModuleAdapter) RPCEndpoint(networkType string) string {
	return a.module.RPCEndpoint(networkType)
}

func (a *networkModuleAdapter) AvailableNetworks() []string {
	return a.module.AvailableNetworks()
}
