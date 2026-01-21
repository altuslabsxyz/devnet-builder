package di

import (
	"context"
	"sync"

	appbinary "github.com/altuslabsxyz/devnet-builder/internal/application/binary"
	"github.com/altuslabsxyz/devnet-builder/internal/application/build"
	appdevnet "github.com/altuslabsxyz/devnet-builder/internal/application/devnet"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/application/upgrade"
	"github.com/altuslabsxyz/devnet-builder/internal/di/providers"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/plugin"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// ContainerV2 is a refactored dependency injection container that composes
// domain-specific providers. This design follows Clean Architecture and SOLID
// principles by separating concerns into cohesive provider groups.
//
// Benefits over the original Container:
//   - Reduced complexity: 4 providers vs 43 direct fields
//   - Better testability: Each provider can be mocked independently
//   - Clearer boundaries: Domain-specific use cases are grouped together
//   - Easier maintenance: Changes to one domain don't affect others
type ContainerV2 struct {
	mu sync.RWMutex

	// Configuration
	config *Config

	// Core dependencies (always available)
	logger     *output.Logger
	networkReg *NetworkRegistry
	pluginMgr  *plugin.PluginManager

	// Infrastructure provider - shared by all use case providers
	infra providers.InfrastructureProvider

	// Domain-specific use case providers (lazy initialized)
	devnetProvider  providers.DevnetUseCasesProvider
	upgradeProvider providers.UpgradeUseCasesProvider
	buildProvider   providers.BuildUseCasesProvider
	binaryProvider  providers.BinaryUseCasesProvider
}

// OptionV2 is a function that configures ContainerV2.
type OptionV2 func(*ContainerV2)

// WithLoggerV2 sets a custom logger.
func WithLoggerV2(logger *output.Logger) OptionV2 {
	return func(c *ContainerV2) {
		c.logger = logger
	}
}

// WithConfigV2 sets the configuration.
func WithConfigV2(config *Config) OptionV2 {
	return func(c *ContainerV2) {
		c.config = config
	}
}

// WithPluginManagerV2 sets a custom plugin manager.
func WithPluginManagerV2(pm *plugin.PluginManager) OptionV2 {
	return func(c *ContainerV2) {
		c.pluginMgr = pm
	}
}

// WithInfrastructureV2 sets the infrastructure provider.
func WithInfrastructureV2(infra providers.InfrastructureProvider) OptionV2 {
	return func(c *ContainerV2) {
		c.infra = infra
	}
}

// NewV2 creates a new ContainerV2 with the given options.
func NewV2(opts ...OptionV2) *ContainerV2 {
	c := &ContainerV2{
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

// ensureProviders initializes the use case providers if not already set.
func (c *ContainerV2) ensureProviders() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.infra == nil {
		return // Cannot initialize providers without infrastructure
	}

	if c.devnetProvider == nil {
		c.devnetProvider = providers.NewDevnetUseCases(c.infra)
	}
	if c.upgradeProvider == nil {
		c.upgradeProvider = providers.NewUpgradeUseCases(c.infra)
	}
	if c.buildProvider == nil {
		c.buildProvider = providers.NewBuildUseCases(c.infra)
	}
	if c.binaryProvider == nil {
		c.binaryProvider = providers.NewBinaryUseCases(c.infra)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Core Accessors
// ─────────────────────────────────────────────────────────────────────────────

// Logger returns the logger instance.
func (c *ContainerV2) Logger() *output.Logger {
	return c.logger
}

// LoggerPort returns the logger as a port interface.
func (c *ContainerV2) LoggerPort() ports.Logger {
	return providers.NewLoggerAdapter(c.logger)
}

// NetworkRegistry returns the network registry wrapper.
func (c *ContainerV2) NetworkRegistry() *NetworkRegistry {
	return c.networkReg
}

// PluginManager returns the plugin manager.
func (c *ContainerV2) PluginManager() *plugin.PluginManager {
	return c.pluginMgr
}

// Config returns the configuration.
func (c *ContainerV2) Config() *Config {
	return c.config
}

// Infrastructure returns the infrastructure provider.
func (c *ContainerV2) Infrastructure() providers.InfrastructureProvider {
	return c.infra
}

// ─────────────────────────────────────────────────────────────────────────────
// Provider Accessors
// ─────────────────────────────────────────────────────────────────────────────

// DevnetProvider returns the devnet use cases provider.
func (c *ContainerV2) DevnetProvider() providers.DevnetUseCasesProvider {
	c.ensureProviders()
	return c.devnetProvider
}

// UpgradeProvider returns the upgrade use cases provider.
func (c *ContainerV2) UpgradeProvider() providers.UpgradeUseCasesProvider {
	c.ensureProviders()
	return c.upgradeProvider
}

// BuildProvider returns the build use cases provider.
func (c *ContainerV2) BuildProvider() providers.BuildUseCasesProvider {
	c.ensureProviders()
	return c.buildProvider
}

// BinaryProvider returns the binary use cases provider.
func (c *ContainerV2) BinaryProvider() providers.BinaryUseCasesProvider {
	c.ensureProviders()
	return c.binaryProvider
}

// ─────────────────────────────────────────────────────────────────────────────
// Direct Infrastructure Accessors (for backward compatibility)
// ─────────────────────────────────────────────────────────────────────────────

// DevnetRepository returns the devnet repository.
func (c *ContainerV2) DevnetRepository() ports.DevnetRepository {
	if c.infra == nil {
		return nil
	}
	return c.infra.DevnetRepository()
}

// NodeRepository returns the node repository.
func (c *ContainerV2) NodeRepository() ports.NodeRepository {
	if c.infra == nil {
		return nil
	}
	return c.infra.NodeRepository()
}

// ExportRepository returns the export repository.
func (c *ContainerV2) ExportRepository() ports.ExportRepository {
	if c.infra == nil {
		return nil
	}
	return c.infra.ExportRepository()
}

// Executor returns the process executor.
func (c *ContainerV2) Executor() ports.ProcessExecutor {
	if c.infra == nil {
		return nil
	}
	return c.infra.ProcessExecutor()
}

// HealthChecker returns the health checker.
func (c *ContainerV2) HealthChecker() ports.HealthChecker {
	if c.infra == nil {
		return nil
	}
	return c.infra.HealthChecker()
}

// NetworkModule returns the network module.
func (c *ContainerV2) NetworkModule() ports.NetworkModule {
	if c.infra == nil {
		return nil
	}
	return c.infra.NetworkModule()
}

// GitHubClient returns the GitHub client.
func (c *ContainerV2) GitHubClient() ports.GitHubClient {
	if c.infra == nil {
		return nil
	}
	return c.infra.GitHubClient()
}

// InteractiveSelector returns the interactive selector.
func (c *ContainerV2) InteractiveSelector() ports.InteractiveSelector {
	if c.infra == nil {
		return nil
	}
	return c.infra.InteractiveSelector()
}

// BinaryCache returns the binary cache.
func (c *ContainerV2) BinaryCache() ports.BinaryCache {
	if c.infra == nil {
		return nil
	}
	return c.infra.BinaryCache()
}

// Builder returns the builder.
func (c *ContainerV2) Builder() ports.Builder {
	if c.infra == nil {
		return nil
	}
	return c.infra.Builder()
}

// ValidatorKeyLoader returns the validator key loader.
func (c *ContainerV2) ValidatorKeyLoader() ports.ValidatorKeyLoader {
	if c.infra == nil {
		return nil
	}
	return c.infra.ValidatorKeyLoader()
}

// EVMClient returns the EVM client.
func (c *ContainerV2) EVMClient() ports.EVMClient {
	if c.infra == nil {
		return nil
	}
	return c.infra.EVMClient()
}

// RPCClient returns the RPC client.
func (c *ContainerV2) RPCClient() ports.RPCClient {
	if c.infra == nil {
		return nil
	}
	return c.infra.RPCClient()
}

// NodeInitializer returns the node initializer.
func (c *ContainerV2) NodeInitializer() ports.NodeInitializer {
	if c.infra == nil {
		return nil
	}
	return c.infra.NodeInitializer()
}

// ─────────────────────────────────────────────────────────────────────────────
// Use Case Accessors (for backward compatibility)
// These delegate to the domain-specific providers.
// ─────────────────────────────────────────────────────────────────────────────

// ProvisionUseCase returns the provision use case.
func (c *ContainerV2) ProvisionUseCase() *appdevnet.ProvisionUseCase {
	return c.DevnetProvider().ProvisionUseCase()
}

// RunUseCase returns the run use case.
func (c *ContainerV2) RunUseCase() *appdevnet.RunUseCase {
	return c.DevnetProvider().RunUseCase()
}

// StopUseCase returns the stop use case.
func (c *ContainerV2) StopUseCase() *appdevnet.StopUseCase {
	return c.DevnetProvider().StopUseCase()
}

// HealthUseCase returns the health use case.
func (c *ContainerV2) HealthUseCase() *appdevnet.HealthUseCase {
	return c.DevnetProvider().HealthUseCase()
}

// ResetUseCase returns the reset use case.
func (c *ContainerV2) ResetUseCase() *appdevnet.ResetUseCase {
	return c.DevnetProvider().ResetUseCase()
}

// DestroyUseCase returns the destroy use case.
func (c *ContainerV2) DestroyUseCase() *appdevnet.DestroyUseCase {
	return c.DevnetProvider().DestroyUseCase()
}

// ExportUseCase returns the export use case.
func (c *ContainerV2) ExportUseCase(ctx context.Context) *appdevnet.ExportUseCase {
	return c.DevnetProvider().ExportUseCase(ctx)
}

// NodeLifecycleManager returns the node lifecycle manager.
func (c *ContainerV2) NodeLifecycleManager() ports.NodeLifecycleManager {
	return c.DevnetProvider().NodeLifecycleManager()
}

// ProposeUseCase returns the propose use case.
func (c *ContainerV2) ProposeUseCase() *upgrade.ProposeUseCase {
	return c.UpgradeProvider().ProposeUseCase()
}

// VoteUseCase returns the vote use case.
func (c *ContainerV2) VoteUseCase() *upgrade.VoteUseCase {
	return c.UpgradeProvider().VoteUseCase()
}

// SwitchBinaryUseCase returns the switch binary use case.
func (c *ContainerV2) SwitchBinaryUseCase() *upgrade.SwitchBinaryUseCase {
	return c.UpgradeProvider().SwitchBinaryUseCase()
}

// ExecuteUpgradeUseCase returns the execute upgrade use case.
func (c *ContainerV2) ExecuteUpgradeUseCase() *upgrade.ExecuteUpgradeUseCase {
	return c.UpgradeProvider().ExecuteUpgradeUseCase(c.DevnetProvider())
}

// MonitorUseCase returns the monitor use case.
func (c *ContainerV2) MonitorUseCase() *upgrade.MonitorUseCase {
	return c.UpgradeProvider().MonitorUseCase()
}

// BuildUseCase returns the build use case.
func (c *ContainerV2) BuildUseCase() *build.BuildUseCase {
	return c.BuildProvider().BuildUseCase()
}

// CacheListUseCase returns the cache list use case.
func (c *ContainerV2) CacheListUseCase() *build.CacheListUseCase {
	return c.BuildProvider().CacheListUseCase()
}

// CacheCleanUseCase returns the cache clean use case.
func (c *ContainerV2) CacheCleanUseCase() *build.CacheCleanUseCase {
	return c.BuildProvider().CacheCleanUseCase()
}

// PassthroughUseCase returns the binary passthrough use case.
func (c *ContainerV2) PassthroughUseCase() *appbinary.PassthroughUseCase {
	return c.BinaryProvider().PassthroughUseCase()
}

// ImportCustomBinaryUseCase returns the import custom binary use case.
func (c *ContainerV2) ImportCustomBinaryUseCase() *appbinary.ImportCustomBinaryUseCase {
	return c.BinaryProvider().ImportCustomBinaryUseCase(c.config.HomeDir)
}

// ─────────────────────────────────────────────────────────────────────────────
// Setters (for runtime updates)
// ─────────────────────────────────────────────────────────────────────────────

// SetNetworkModule sets the network module at runtime.
// Note: This is a transitional method. In the new architecture, the network
// module should be set when creating the InfrastructureProvider.
func (c *ContainerV2) SetNetworkModule(module ports.NetworkModule) {
	// This method exists for backward compatibility but is a no-op in V2
	// since the network module is set through the infrastructure provider.
	c.logger.Warn("SetNetworkModule called on ContainerV2 - this is deprecated. " +
		"Set the network module when creating the InfrastructureProvider instead.")
}

// SetBinaryResolver sets the binary resolver.
// Note: This is a transitional method. In the new architecture, the binary
// resolver should be set when creating the InfrastructureProvider.
func (c *ContainerV2) SetBinaryResolver(resolver ports.BinaryResolver) {
	// This method exists for backward compatibility but is a no-op in V2
	c.logger.Warn("SetBinaryResolver called on ContainerV2 - this is deprecated. " +
		"Set the binary resolver when creating the InfrastructureProvider instead.")
}

// ─────────────────────────────────────────────────────────────────────────────
// NetworkRegistry Wrapper (unchanged from original)
// ─────────────────────────────────────────────────────────────────────────────

// NetworkRegistryV2 wraps network registration operations.
// This provides an injectable alternative to the global registry.
type NetworkRegistryV2 struct{}

// Get retrieves a network module by name.
func (r *NetworkRegistryV2) Get(name string) (network.NetworkModule, error) {
	return network.Get(name)
}

// Has checks if a network is registered.
func (r *NetworkRegistryV2) Has(name string) bool {
	return network.Has(name)
}

// List returns all registered network names.
func (r *NetworkRegistryV2) List() []string {
	return network.List()
}

// ListModules returns all registered network modules.
func (r *NetworkRegistryV2) ListModules() []network.NetworkModule {
	return network.ListModules()
}

// Default returns the default network module.
func (r *NetworkRegistryV2) Default() (network.NetworkModule, error) {
	return network.Default()
}

// SetDefault changes the default network name.
func (r *NetworkRegistryV2) SetDefault(name string) error {
	return network.SetDefault(name)
}

// ─────────────────────────────────────────────────────────────────────────────
// Compatibility Layer
// ─────────────────────────────────────────────────────────────────────────────

// ToLegacyContainer creates a legacy Container from ContainerV2.
// This is provided for backward compatibility during migration.
//
// Deprecated: Use ContainerV2 directly instead.
func (c *ContainerV2) ToLegacyContainer() *Container {
	opts := []Option{
		WithLogger(c.logger),
		WithConfig(c.config),
	}

	if c.pluginMgr != nil {
		opts = append(opts, WithPluginManager(c.pluginMgr))
	}

	if c.infra != nil {
		opts = append(opts,
			WithDevnetRepository(c.infra.DevnetRepository()),
			WithNodeRepository(c.infra.NodeRepository()),
			WithExportRepository(c.infra.ExportRepository()),
			WithExecutor(c.infra.ProcessExecutor()),
			WithBinaryCache(c.infra.BinaryCache()),
			WithBinaryVersionDetector(c.infra.BinaryVersionDetector()),
			WithRPCClient(c.infra.RPCClient()),
			WithEVMClient(c.infra.EVMClient()),
			WithSnapshotFetcher(c.infra.SnapshotFetcher()),
			WithGenesisFetcher(c.infra.GenesisFetcher()),
			WithStateExportService(c.infra.StateExportService()),
			WithNodeInitializer(c.infra.NodeInitializer()),
			WithHealthChecker(c.infra.HealthChecker()),
			WithBuilder(c.infra.Builder()),
			WithValidatorKeyLoader(c.infra.ValidatorKeyLoader()),
			WithGitHubClient(c.infra.GitHubClient()),
			WithInteractiveSelector(c.infra.InteractiveSelector()),
			WithBinaryExecutor(c.infra.BinaryExecutor()),
			WithBinaryResolver(c.infra.BinaryResolver()),
			WithNetworkModule(c.infra.NetworkModule()),
		)
	}

	return New(opts...)
}
