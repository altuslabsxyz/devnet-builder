// Package di provides dependency injection container for the application.
// It centralizes the creation and management of service dependencies,
// following Clean Architecture principles for loose coupling and testability.
package di

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/b-harvest/devnet-builder/internal/application/binary"
	"github.com/b-harvest/devnet-builder/internal/application/build"
	appdevnet "github.com/b-harvest/devnet-builder/internal/application/devnet"
	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/application/upgrade"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/plugin"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// exportUseCaseAdapter adapts the concrete ExportUseCase to the ports interface.
type exportUseCaseAdapter struct {
	concrete interface {
		Execute(ctx context.Context, input dto.ExportInput) (*dto.ExportOutput, error)
	}
}

func (a *exportUseCaseAdapter) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	exportInput, ok := input.(dto.ExportInput)
	if !ok {
		return nil, fmt.Errorf("invalid input type for export: expected dto.ExportInput, got %T", input)
	}
	return a.concrete.Execute(ctx, exportInput)
}

// Container holds all application dependencies.
// It provides lazy initialization of UseCases and ensures
// thread-safe access to shared dependencies.
type Container struct {
	mu sync.RWMutex

	// Configuration
	config *Config

	// Core dependencies
	logger     *output.Logger
	networkReg *NetworkRegistry
	pluginMgr  *plugin.PluginManager

	// Infrastructure implementations (injected)
	devnetRepo          ports.DevnetRepository
	nodeRepo            ports.NodeRepository
	binaryCache         ports.BinaryCache
	executor            ports.ProcessExecutor
	rpcClient           ports.RPCClient
	evmClient           ports.EVMClient
	snapshotSvc         ports.SnapshotFetcher
	genesisSvc          ports.GenesisFetcher
	stateExportSvc      ports.StateExportService
	nodeInitializer     ports.NodeInitializer
	keyManager          ports.KeyManager
	validatorKeyLoader  ports.ValidatorKeyLoader
	healthChecker       ports.HealthChecker
	builder             ports.Builder
	networkModule       ports.NetworkModule
	githubClient        ports.GitHubClient
	interactiveSelector ports.InteractiveSelector
	binaryResolver      ports.BinaryResolver
	binaryExecutor      ports.BinaryExecutor
	exportRepo          ports.ExportRepository

	// Lazy-initialized UseCases
	provisionUC      *appdevnet.ProvisionUseCase
	runUC            *appdevnet.RunUseCase
	stopUC           *appdevnet.StopUseCase
	healthUC         *appdevnet.HealthUseCase
	resetUC          *appdevnet.ResetUseCase
	destroyUC        *appdevnet.DestroyUseCase
	proposeUC        *upgrade.ProposeUseCase
	voteUC           *upgrade.VoteUseCase
	switchUC         *upgrade.SwitchBinaryUseCase
	executeUpgradeUC *upgrade.ExecuteUpgradeUseCase
	monitorUC        *upgrade.MonitorUseCase
	buildUC          *build.BuildUseCase
	cacheListUC      *build.CacheListUseCase
	cacheCleanUC     *build.CacheCleanUseCase
	passthroughUC    *binary.PassthroughUseCase
	exportUC         *appdevnet.ExportUseCase
}

// Config holds configuration for the container.
type Config struct {
	HomeDir       string
	PluginDir     string
	Verbose       bool
	NoColor       bool
	JSONMode      bool
	ExecutionMode ports.ExecutionMode
}

// NetworkRegistry wraps network registration operations.
// This provides an injectable alternative to the global registry.
type NetworkRegistry struct{}

// Get retrieves a network module by name.
func (r *NetworkRegistry) Get(name string) (network.NetworkModule, error) {
	return network.Get(name)
}

// Has checks if a network is registered.
func (r *NetworkRegistry) Has(name string) bool {
	return network.Has(name)
}

// List returns all registered network names.
func (r *NetworkRegistry) List() []string {
	return network.List()
}

// ListModules returns all registered network modules.
func (r *NetworkRegistry) ListModules() []network.NetworkModule {
	return network.ListModules()
}

// Default returns the default network module.
func (r *NetworkRegistry) Default() (network.NetworkModule, error) {
	return network.Default()
}

// SetDefault changes the default network name.
func (r *NetworkRegistry) SetDefault(name string) error {
	return network.SetDefault(name)
}

// Option is a function that configures the container.
type Option func(*Container)

// WithLogger sets a custom logger.
func WithLogger(logger *output.Logger) Option {
	return func(c *Container) {
		c.logger = logger
	}
}

// WithConfig sets the configuration.
func WithConfig(config *Config) Option {
	return func(c *Container) {
		c.config = config
	}
}

// WithPluginManager sets a custom plugin manager.
func WithPluginManager(pm *plugin.PluginManager) Option {
	return func(c *Container) {
		c.pluginMgr = pm
	}
}

// WithDevnetRepository sets the devnet repository.
func WithDevnetRepository(repo ports.DevnetRepository) Option {
	return func(c *Container) {
		c.devnetRepo = repo
	}
}

// WithNodeRepository sets the node repository.
func WithNodeRepository(repo ports.NodeRepository) Option {
	return func(c *Container) {
		c.nodeRepo = repo
	}
}

// WithBinaryCache sets the binary cache.
func WithBinaryCache(cache ports.BinaryCache) Option {
	return func(c *Container) {
		c.binaryCache = cache
	}
}

// WithExecutor sets the process executor.
func WithExecutor(executor ports.ProcessExecutor) Option {
	return func(c *Container) {
		c.executor = executor
	}
}

// WithRPCClient sets the RPC client.
func WithRPCClient(client ports.RPCClient) Option {
	return func(c *Container) {
		c.rpcClient = client
	}
}

// WithEVMClient sets the EVM client.
func WithEVMClient(client ports.EVMClient) Option {
	return func(c *Container) {
		c.evmClient = client
	}
}

// WithSnapshotFetcher sets the snapshot fetcher.
func WithSnapshotFetcher(svc ports.SnapshotFetcher) Option {
	return func(c *Container) {
		c.snapshotSvc = svc
	}
}

// WithGenesisFetcher sets the genesis fetcher.
func WithGenesisFetcher(svc ports.GenesisFetcher) Option {
	return func(c *Container) {
		c.genesisSvc = svc
	}
}

// WithStateExportService sets the state export service.
func WithStateExportService(svc ports.StateExportService) Option {
	return func(c *Container) {
		c.stateExportSvc = svc
	}
}

// WithNodeInitializer sets the node initializer.
func WithNodeInitializer(ni ports.NodeInitializer) Option {
	return func(c *Container) {
		c.nodeInitializer = ni
	}
}

// WithKeyManager sets the key manager.
func WithKeyManager(km ports.KeyManager) Option {
	return func(c *Container) {
		c.keyManager = km
	}
}

// WithHealthChecker sets the health checker.
func WithHealthChecker(hc ports.HealthChecker) Option {
	return func(c *Container) {
		c.healthChecker = hc
	}
}

// WithValidatorKeyLoader sets the validator key loader.
func WithValidatorKeyLoader(loader ports.ValidatorKeyLoader) Option {
	return func(c *Container) {
		c.validatorKeyLoader = loader
	}
}

// WithBuilder sets the builder.
func WithBuilder(builder ports.Builder) Option {
	return func(c *Container) {
		c.builder = builder
	}
}

// WithNetworkModule sets the network module.
func WithNetworkModule(nm ports.NetworkModule) Option {
	return func(c *Container) {
		c.networkModule = nm
	}
}

// WithGitHubClient sets the GitHub client.
func WithGitHubClient(client ports.GitHubClient) Option {
	return func(c *Container) {
		c.githubClient = client
	}
}

// WithInteractiveSelector sets the interactive selector.
func WithInteractiveSelector(selector ports.InteractiveSelector) Option {
	return func(c *Container) {
		c.interactiveSelector = selector
	}
}

// WithBinaryResolver sets the binary resolver.
func WithBinaryResolver(resolver ports.BinaryResolver) Option {
	return func(c *Container) {
		c.binaryResolver = resolver
	}
}

// WithBinaryExecutor sets the binary executor.
func WithBinaryExecutor(executor ports.BinaryExecutor) Option {
	return func(c *Container) {
		c.binaryExecutor = executor
	}
}

// WithExportRepository sets the export repository.
func WithExportRepository(repo ports.ExportRepository) Option {
	return func(c *Container) {
		c.exportRepo = repo
	}
}

// New creates a new dependency injection container with the given options.
func New(opts ...Option) *Container {
	c := &Container{
		logger:     output.NewLogger(),
		networkReg: &NetworkRegistry{},
		config:     &Config{},
	}

	for _, opt := range opts {
		opt(c)
	}

	// Apply config to logger
	if c.config != nil {
		c.logger.SetVerbose(c.config.Verbose)
		c.logger.SetNoColor(c.config.NoColor)
		c.logger.SetJSONMode(c.config.JSONMode)
	}

	// Initialize plugin manager if not provided
	if c.pluginMgr == nil && c.config.PluginDir != "" {
		c.pluginMgr = plugin.NewPluginManager(c.config.PluginDir)
	}

	return c
}

// Logger returns the logger instance.
func (c *Container) Logger() *output.Logger {
	return c.logger
}

// LoggerPort returns the logger as a port interface.
func (c *Container) LoggerPort() ports.Logger {
	return &loggerAdapter{logger: c.logger}
}

// NetworkRegistry returns the network registry wrapper.
func (c *Container) NetworkRegistry() *NetworkRegistry {
	return c.networkReg
}

// PluginManager returns the plugin manager.
func (c *Container) PluginManager() *plugin.PluginManager {
	return c.pluginMgr
}

// Config returns the configuration.
func (c *Container) Config() *Config {
	return c.config
}

// DevnetRepository returns the devnet repository.
func (c *Container) DevnetRepository() ports.DevnetRepository {
	return c.devnetRepo
}

// NodeRepository returns the node repository.
func (c *Container) NodeRepository() ports.NodeRepository {
	return c.nodeRepo
}

// ExportRepository returns the export repository.
func (c *Container) ExportRepository() ports.ExportRepository {
	return c.exportRepo
}

// SetNetworkModule sets the network module at runtime.
func (c *Container) SetNetworkModule(module ports.NetworkModule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.networkModule = module
}

// Executor returns the process executor.
func (c *Container) Executor() ports.ProcessExecutor {
	return c.executor
}

// HealthChecker returns the health checker.
func (c *Container) HealthChecker() ports.HealthChecker {
	return c.healthChecker
}

// NetworkModule returns the network module.
func (c *Container) NetworkModule() ports.NetworkModule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkModule
}

// GitHubClient returns the GitHub client.
func (c *Container) GitHubClient() ports.GitHubClient {
	return c.githubClient
}

// InteractiveSelector returns the interactive selector.
func (c *Container) InteractiveSelector() ports.InteractiveSelector {
	return c.interactiveSelector
}

// BinaryCache returns the binary cache.
func (c *Container) BinaryCache() ports.BinaryCache {
	return c.binaryCache
}

// SetBinaryResolver sets the binary resolver.
// This is used to inject the resolver after container creation,
// since it depends on the global plugin loader.
func (c *Container) SetBinaryResolver(resolver ports.BinaryResolver) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.binaryResolver = resolver
}

// Builder returns the builder.
func (c *Container) Builder() ports.Builder {
	return c.builder
}

// ValidatorKeyLoader returns the validator key loader.
func (c *Container) ValidatorKeyLoader() ports.ValidatorKeyLoader {
	return c.validatorKeyLoader
}

// EVMClient returns the EVM client.
func (c *Container) EVMClient() ports.EVMClient {
	return c.evmClient
}

// RPCClient returns the RPC client.
func (c *Container) RPCClient() ports.RPCClient {
	return c.rpcClient
}

// ProvisionUseCase returns the provision use case (lazy init).
func (c *Container) ProvisionUseCase() *appdevnet.ProvisionUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.provisionUC == nil {
		c.provisionUC = appdevnet.NewProvisionUseCase(
			c.devnetRepo,
			c.nodeRepo,
			c.snapshotSvc,
			c.genesisSvc,
			c.stateExportSvc,
			c.nodeInitializer,
			c.networkModule,
			c.LoggerPort(),
		)
	}
	return c.provisionUC
}

// NodeInitializer returns the node initializer.
func (c *Container) NodeInitializer() ports.NodeInitializer {
	return c.nodeInitializer
}

// RunUseCase returns the run use case (lazy init).
func (c *Container) RunUseCase() *appdevnet.RunUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.runUC == nil {
		c.runUC = appdevnet.NewRunUseCase(
			c.devnetRepo,
			c.nodeRepo,
			c.executor,
			c.healthChecker,
			c.networkModule,
			c.LoggerPort(),
		)
	}
	return c.runUC
}

// StopUseCase returns the stop use case (lazy init).
func (c *Container) StopUseCase() *appdevnet.StopUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopUC == nil {
		c.stopUC = appdevnet.NewStopUseCase(
			c.devnetRepo,
			c.nodeRepo,
			c.executor,
			c.LoggerPort(),
		)
	}
	return c.stopUC
}

// HealthUseCase returns the health use case (lazy init).
func (c *Container) HealthUseCase() *appdevnet.HealthUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.healthUC == nil {
		c.healthUC = appdevnet.NewHealthUseCase(
			c.devnetRepo,
			c.nodeRepo,
			c.healthChecker,
			c.LoggerPort(),
		)
	}
	return c.healthUC
}

// ResetUseCase returns the reset use case (lazy init).
func (c *Container) ResetUseCase() *appdevnet.ResetUseCase {
	// Get StopUseCase first without holding the lock to avoid deadlock
	stopUC := c.StopUseCase()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.resetUC == nil {
		c.resetUC = appdevnet.NewResetUseCase(
			c.devnetRepo,
			c.nodeRepo,
			stopUC,
			c.LoggerPort(),
		)
	}
	return c.resetUC
}

// DestroyUseCase returns the destroy use case (lazy init).
func (c *Container) DestroyUseCase() *appdevnet.DestroyUseCase {
	// Get StopUseCase first without holding the lock to avoid deadlock
	stopUC := c.StopUseCase()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.destroyUC == nil {
		c.destroyUC = appdevnet.NewDestroyUseCase(
			c.devnetRepo,
			stopUC,
			c.LoggerPort(),
		)
	}
	return c.destroyUC
}

// ProposeUseCase returns the propose use case (lazy init).
func (c *Container) ProposeUseCase() *upgrade.ProposeUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.proposeUC == nil {
		c.proposeUC = upgrade.NewProposeUseCase(
			c.devnetRepo,
			c.rpcClient,
			c.validatorKeyLoader,
			c.LoggerPort(),
		)
	}
	return c.proposeUC
}

// VoteUseCase returns the vote use case (lazy init).
func (c *Container) VoteUseCase() *upgrade.VoteUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.voteUC == nil {
		c.voteUC = upgrade.NewVoteUseCase(
			c.devnetRepo,
			c.rpcClient,
			c.validatorKeyLoader,
			c.LoggerPort(),
		)
	}
	return c.voteUC
}

// SwitchBinaryUseCase returns the switch binary use case (lazy init).
func (c *Container) SwitchBinaryUseCase() *upgrade.SwitchBinaryUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.switchUC == nil {
		c.switchUC = upgrade.NewSwitchBinaryUseCase(
			c.devnetRepo,
			c.nodeRepo,
			c.executor,
			c.binaryCache,
			c.LoggerPort(),
		)
	}
	return c.switchUC
}

// ExecuteUpgradeUseCase returns the execute upgrade use case (lazy init).
func (c *Container) ExecuteUpgradeUseCase() *upgrade.ExecuteUpgradeUseCase {
	// Get dependencies first without holding the lock to avoid deadlock
	proposeUC := c.ProposeUseCase()
	voteUC := c.VoteUseCase()
	switchUC := c.SwitchBinaryUseCase()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.executeUpgradeUC == nil {
		// Create export adapter to match ports interface
		exportAdapter := &exportUseCaseAdapter{concrete: c.ExportUseCase()}

		c.executeUpgradeUC = upgrade.NewExecuteUpgradeUseCase(
			proposeUC,
			voteUC,
			switchUC,
			exportAdapter,
			c.rpcClient,
			c.devnetRepo,
			c.healthChecker,
			c.LoggerPort(),
		)
	}
	return c.executeUpgradeUC
}

// MonitorUseCase returns the monitor use case (lazy init).
func (c *Container) MonitorUseCase() *upgrade.MonitorUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.monitorUC == nil {
		c.monitorUC = upgrade.NewMonitorUseCase(
			c.rpcClient,
			c.LoggerPort(),
		)
	}
	return c.monitorUC
}

// BuildUseCase returns the build use case (lazy init).
func (c *Container) BuildUseCase() *build.BuildUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.buildUC == nil {
		c.buildUC = build.NewBuildUseCase(
			c.builder,
			c.binaryCache,
			c.LoggerPort(),
		)
	}
	return c.buildUC
}

// CacheListUseCase returns the cache list use case (lazy init).
func (c *Container) CacheListUseCase() *build.CacheListUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cacheListUC == nil {
		c.cacheListUC = build.NewCacheListUseCase(
			c.binaryCache,
			c.LoggerPort(),
		)
	}
	return c.cacheListUC
}

// CacheCleanUseCase returns the cache clean use case (lazy init).
func (c *Container) CacheCleanUseCase() *build.CacheCleanUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cacheCleanUC == nil {
		c.cacheCleanUC = build.NewCacheCleanUseCase(
			c.binaryCache,
			c.LoggerPort(),
		)
	}
	return c.cacheCleanUC
}

// PassthroughUseCase returns the binary passthrough use case (lazy init).
func (c *Container) PassthroughUseCase() *binary.PassthroughUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.passthroughUC == nil {
		c.passthroughUC = binary.NewPassthroughUseCase(
			c.binaryResolver,
			c.binaryExecutor,
		)
	}
	return c.passthroughUC
}

// ExportUseCase returns the export use case (lazy init).
func (c *Container) ExportUseCase() *appdevnet.ExportUseCase {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.exportUC == nil {
		c.exportUC = appdevnet.NewExportUseCase(
			c.devnetRepo,
			c.nodeRepo,
			c.exportRepo,
			c.LoggerPort(),
		)
	}
	return c.exportUC
}

// loggerAdapter adapts output.Logger to ports.Logger interface.
type loggerAdapter struct {
	logger *output.Logger
}

func (a *loggerAdapter) Info(format string, args ...interface{}) {
	a.logger.Info(format, args...)
}

func (a *loggerAdapter) Warn(format string, args ...interface{}) {
	a.logger.Warn(format, args...)
}

func (a *loggerAdapter) Error(format string, args ...interface{}) {
	a.logger.Error(format, args...)
}

func (a *loggerAdapter) Debug(format string, args ...interface{}) {
	a.logger.Debug(format, args...)
}

func (a *loggerAdapter) Success(format string, args ...interface{}) {
	a.logger.Success(format, args...)
}

func (a *loggerAdapter) Print(format string, args ...interface{}) {
	a.logger.Print(format, args...)
}

func (a *loggerAdapter) Println(format string, args ...interface{}) {
	a.logger.Println(format, args...)
}

func (a *loggerAdapter) SetVerbose(verbose bool) {
	a.logger.SetVerbose(verbose)
}

func (a *loggerAdapter) IsVerbose() bool {
	return a.logger.IsVerbose()
}

func (a *loggerAdapter) Writer() io.Writer {
	return a.logger.Writer()
}

func (a *loggerAdapter) ErrWriter() io.Writer {
	return a.logger.ErrWriter()
}
