package di

import (
	"context"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	appversion "github.com/b-harvest/devnet-builder/internal/application/version"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/binary"
	infrabuilder "github.com/b-harvest/devnet-builder/internal/infrastructure/builder"
	infracache "github.com/b-harvest/devnet-builder/internal/infrastructure/cache"
	infraevm "github.com/b-harvest/devnet-builder/internal/infrastructure/evm"
	infraexport "github.com/b-harvest/devnet-builder/internal/infrastructure/export"
	infragenesis "github.com/b-harvest/devnet-builder/internal/infrastructure/genesis"
	infragithub "github.com/b-harvest/devnet-builder/internal/infrastructure/github"
	infrainteractive "github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
	infrakeyring "github.com/b-harvest/devnet-builder/internal/infrastructure/keyring"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
	infranode "github.com/b-harvest/devnet-builder/internal/infrastructure/node"
	infranodeconfig "github.com/b-harvest/devnet-builder/internal/infrastructure/nodeconfig"
	infrapersistence "github.com/b-harvest/devnet-builder/internal/infrastructure/persistence"
	infraprocess "github.com/b-harvest/devnet-builder/internal/infrastructure/process"
	infrarpc "github.com/b-harvest/devnet-builder/internal/infrastructure/rpc"
	infrasnapshot "github.com/b-harvest/devnet-builder/internal/infrastructure/snapshot"
	infrastateexport "github.com/b-harvest/devnet-builder/internal/infrastructure/stateexport"
	infraversion "github.com/b-harvest/devnet-builder/internal/infrastructure/version"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

// InfrastructureFactory creates infrastructure implementations.
type InfrastructureFactory struct {
	homeDir     string
	logger      *output.Logger
	module      network.NetworkModule
	dockerMode  bool
	githubToken string
	githubOwner string
	githubRepo  string
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

// WithGitHubConfig sets GitHub configuration.
func (f *InfrastructureFactory) WithGitHubConfig(token, owner, repo string) *InfrastructureFactory {
	f.githubToken = token
	f.githubOwner = owner
	f.githubRepo = repo
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

// CreateExportRepository creates an ExportRepository implementation.
func (f *InfrastructureFactory) CreateExportRepository() ports.ExportRepository {
	return infraexport.NewRepository(f.homeDir)
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

// CreateBinaryVersionDetector creates a BinaryVersionDetector implementation.
func (f *InfrastructureFactory) CreateBinaryVersionDetector() ports.BinaryVersionDetector {
	return binary.NewBinaryVersionDetector()
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

// CreateStateExportService creates a StateExportService implementation.
func (f *InfrastructureFactory) CreateStateExportService() ports.StateExportService {
	return infrastateexport.NewAdapter(f.homeDir, f.logger)
}

// CreateNodeInitializer creates a NodeInitializer implementation.
func (f *InfrastructureFactory) CreateNodeInitializer() ports.NodeInitializer {
	mode := infranodeconfig.ModeLocal
	dockerImage := ""
	binaryPath := ""

	if f.dockerMode {
		mode = infranodeconfig.ModeDocker
	}
	if f.module != nil {
		dockerImage = f.module.DockerImage()
		binaryPath = f.homeDir + "/bin/" + f.module.BinaryName()
	}

	return &nodeInitializerAdapter{
		inner: infranodeconfig.NewNodeInitializerWithBinary(mode, dockerImage, binaryPath, f.logger),
	}
}

// nodeInitializerAdapter adapts infranodeconfig.NodeInitializer to ports.NodeInitializer.
type nodeInitializerAdapter struct {
	inner *infranodeconfig.NodeInitializer
}

func (a *nodeInitializerAdapter) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	return a.inner.Initialize(ctx, nodeDir, moniker, chainID)
}

func (a *nodeInitializerAdapter) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	return a.inner.GetNodeID(ctx, nodeDir)
}

func (a *nodeInitializerAdapter) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return a.inner.CreateAccountKey(ctx, keyringDir, keyName)
}

func (a *nodeInitializerAdapter) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	return a.inner.GetAccountKey(ctx, keyringDir, keyName)
}

func (a *nodeInitializerAdapter) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	return a.inner.CreateAccountKeyFromMnemonic(ctx, keyringDir, keyName, mnemonic)
}

func (a *nodeInitializerAdapter) GetTestMnemonic(validatorIndex int) string {
	return a.inner.GetTestMnemonic(validatorIndex)
}

// CreateNodeManagerFactory creates a NodeManagerFactory.
func (f *InfrastructureFactory) CreateNodeManagerFactory() *infranode.NodeManagerFactory {
	mode := infranode.ModeLocal
	if f.dockerMode {
		mode = infranode.ModeDocker
	}
	config := infranode.FactoryConfig{
		Mode:   mode,
		Logger: f.logger,
	}
	return infranode.NewNodeManagerFactory(config)
}

// CreateGitHubClient creates a GitHubClient implementation.
func (f *InfrastructureFactory) CreateGitHubClient() ports.GitHubClient {
	owner := f.githubOwner
	repo := f.githubRepo
	if owner == "" {
		owner = "stablelabs"
	}
	if repo == "" {
		repo = "stable"
	}
	return infragithub.NewAdapter(f.githubToken, owner, repo, f.homeDir)
}

// CreateInteractiveSelector creates an InteractiveSelector implementation.
func (f *InfrastructureFactory) CreateInteractiveSelector() ports.InteractiveSelector {
	return infrainteractive.NewAdapter()
}

// CreateHealthChecker creates a HealthChecker implementation.
func (f *InfrastructureFactory) CreateHealthChecker(rpcPort int) ports.HealthChecker {
	return &healthCheckerAdapter{
		factory: f,
	}
}

// CreateEVMClient creates an EVMClient implementation.
func (f *InfrastructureFactory) CreateEVMClient(evmRPCURL string) *infraevm.Client {
	return infraevm.NewClient(evmRPCURL)
}

// CreateValidatorKeyLoader creates a ValidatorKeyLoader implementation.
func (f *InfrastructureFactory) CreateValidatorKeyLoader() ports.ValidatorKeyLoader {
	dockerImage := ""
	if f.module != nil {
		dockerImage = f.module.DockerImage()
	}
	return infrakeyring.NewValidatorKeyLoader(dockerImage)
}

// CreateVersionRepository creates a VersionRepository implementation.
func (f *InfrastructureFactory) CreateVersionRepository() ports.VersionRepository {
	return infraversion.NewFilesystemVersionRepository()
}

// CreateMigrationService creates a MigrationService implementation.
func (f *InfrastructureFactory) CreateMigrationService() ports.MigrationService {
	repo := f.CreateVersionRepository()
	return appversion.NewService(repo, f.logger)
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

	// Get app version from /abci_info
	appVersion, _ := client.GetAppVersion(ctx) // Ignore error, empty string if not available

	return &ports.HealthStatus{
		IsRunning:   isRunning,
		Status:      status,
		BlockHeight: height,
		AppVersion:  appVersion,
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
	exportRepo := f.CreateExportRepository()
	executor := f.CreateProcessExecutor()
	snapshotFetcher := f.CreateSnapshotFetcher()
	genesisFetcher := f.CreateGenesisFetcher()
	stateExportSvc := f.CreateStateExportService()
	nodeInitializer := f.CreateNodeInitializer()
	builder := f.CreateBuilder()

	binaryCache, err := f.CreateBinaryCache()
	if err != nil {
		return nil, err
	}

	// Binary version detector for custom binary imports
	binaryVersionDetector := f.CreateBinaryVersionDetector()

	// Default RPC client (node0)
	rpcClient := f.CreateRPCClient("localhost", 26657)
	healthChecker := f.CreateHealthChecker(26657)

	// Default EVM client (node0 EVM port)
	evmClient := f.CreateEVMClient("http://localhost:8545")

	// Validator key loader
	validatorKeyLoader := f.CreateValidatorKeyLoader()

	// GitHub and Interactive adapters
	githubClient := f.CreateGitHubClient()
	interactiveSelector := f.CreateInteractiveSelector()

	// Binary passthrough components
	// Note: The plugin loader is passed separately as it's created in main.go
	// We'll set a placeholder here and update it later via WithBinaryResolver
	binaryExecutor := f.CreateBinaryExecutor()

	// Create container with all dependencies
	allOpts := []Option{
		WithLogger(f.logger),
		WithDevnetRepository(devnetRepo),
		WithNodeRepository(nodeRepo),
		WithExportRepository(exportRepo),
		WithExecutor(executor),
		WithBinaryCache(binaryCache),
		WithBinaryVersionDetector(binaryVersionDetector),
		WithRPCClient(rpcClient),
		WithEVMClient(evmClient),
		WithSnapshotFetcher(snapshotFetcher),
		WithGenesisFetcher(genesisFetcher),
		WithStateExportService(stateExportSvc),
		WithNodeInitializer(nodeInitializer),
		WithHealthChecker(healthChecker),
		WithBuilder(builder),
		WithValidatorKeyLoader(validatorKeyLoader),
		WithGitHubClient(githubClient),
		WithInteractiveSelector(interactiveSelector),
		WithBinaryExecutor(binaryExecutor),
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

// Compile-time interface checks.
var (
	_ ports.NetworkModule            = (*networkModuleAdapter)(nil)
	_ ports.FileBasedGenesisModifier = (*networkModuleAdapter)(nil)
)

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
		RPC:     np.RPC,
		P2P:     np.P2P,
		GRPC:    np.GRPC,
		GRPCWeb: np.GRPCWeb,
		API:     np.API,
		EVM:     np.EVMRPC,
		EVMWS:   np.EVMWS,
		PProf:   6060, // Default pprof port
		Rosetta: 8080, // Default Rosetta API port
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

func (a *networkModuleAdapter) ModifyGenesis(genesis []byte, opts ports.GenesisModifyOptions) ([]byte, error) {
	// Convert ports.ValidatorInfo to network.GenesisValidatorInfo
	validators := make([]network.GenesisValidatorInfo, len(opts.AddValidators))
	for i, v := range opts.AddValidators {
		validators[i] = network.GenesisValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	// Convert ports.GenesisModifyOptions to network.GenesisOptions
	networkOpts := network.GenesisOptions{
		ChainID:       opts.ChainID,
		NumValidators: opts.NumValidators,
		Validators:    validators,
	}
	return a.module.ModifyGenesis(genesis, networkOpts)
}

// ModifyGenesisFile implements ports.FileBasedGenesisModifier.
// This method handles large genesis files that exceed gRPC message size limits (4MB).
func (a *networkModuleAdapter) ModifyGenesisFile(inputPath, outputPath string, opts ports.GenesisModifyOptions) (int64, error) {
	// Check if underlying module supports file-based modification
	fileModifier, ok := a.module.(network.FileBasedGenesisModifier)
	if !ok {
		return 0, fmt.Errorf("network module does not support file-based genesis modification")
	}

	// Convert ports.ValidatorInfo to network.GenesisValidatorInfo
	validators := make([]network.GenesisValidatorInfo, len(opts.AddValidators))
	for i, v := range opts.AddValidators {
		validators[i] = network.GenesisValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	// Convert ports.GenesisModifyOptions to network.GenesisOptions
	networkOpts := network.GenesisOptions{
		ChainID:       opts.ChainID,
		NumValidators: opts.NumValidators,
		Validators:    validators,
	}

	return fileModifier.ModifyGenesisFile(inputPath, outputPath, networkOpts)
}

func (a *networkModuleAdapter) GenesisConfig() ports.GenesisConfig {
	cfg := a.module.GenesisConfig()
	return ports.GenesisConfig{
		UnbondingTime:    cfg.UnbondingTime,
		VotingPeriod:     cfg.VotingPeriod,
		MaxDepositPeriod: cfg.MaxDepositPeriod,
		MinDeposit:       cfg.MinDeposit,
		MaxValidators:    cfg.MaxValidators,
		BaseDenom:        cfg.BaseDenom,
		BondDenom:        cfg.BondDenom,
	}
}

func (a *networkModuleAdapter) GetConfigOverrides(nodeIndex int, opts ports.NodeConfigOptions) ([]byte, []byte, error) {
	// Convert ports.NodeConfigOptions to network.NodeConfigOptions
	networkOpts := network.NodeConfigOptions{
		ChainID:         opts.ChainID,
		PersistentPeers: opts.PersistentPeers,
		NumValidators:   opts.NumValidators,
		IsValidator:     opts.IsValidator,
		Moniker:         opts.Moniker,
		Ports: network.PortConfig{
			RPC:     opts.Ports.RPC,
			P2P:     opts.Ports.P2P,
			GRPC:    opts.Ports.GRPC,
			GRPCWeb: opts.Ports.GRPCWeb,
			API:     opts.Ports.API,
			EVMRPC:  opts.Ports.EVM,
			EVMWS:   opts.Ports.EVMWS,
		},
	}
	return a.module.GetConfigOverrides(nodeIndex, networkOpts)
}

// CreateBinaryResolver creates a BinaryResolver implementation.
// Requires plugin loader and binary cache to be initialized.
func (f *InfrastructureFactory) CreateBinaryResolver(loader *plugin.Loader, cache ports.BinaryCache) ports.BinaryResolver {
	return binary.NewPluginBinaryResolver(loader, cache)
}

// CreateBinaryExecutor creates a BinaryExecutor implementation.
func (f *InfrastructureFactory) CreateBinaryExecutor() ports.BinaryExecutor {
	return binary.NewPassthroughExecutor()
}
