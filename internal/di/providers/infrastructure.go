// Package providers contains domain-specific dependency providers that
// follow the Interface Segregation Principle by grouping related dependencies.
package providers

import (
	"io"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// InfrastructureProvider provides access to infrastructure implementations.
// This interface groups all infrastructure dependencies needed by use cases,
// allowing for easy mocking in tests and clear dependency boundaries.
type InfrastructureProvider interface {
	// Repositories
	DevnetRepository() ports.DevnetRepository
	NodeRepository() ports.NodeRepository
	ExportRepository() ports.ExportRepository

	// Process Management
	ProcessExecutor() ports.ProcessExecutor
	HealthChecker() ports.HealthChecker

	// Network Communication
	RPCClient() ports.RPCClient
	EVMClient() ports.EVMClient

	// Data Fetching
	SnapshotFetcher() ports.SnapshotFetcher
	GenesisFetcher() ports.GenesisFetcher
	StateExportService() ports.StateExportService

	// Node Operations
	NodeInitializer() ports.NodeInitializer
	NetworkModule() ports.NetworkModule
	ValidatorKeyLoader() ports.ValidatorKeyLoader
	KeyManager() ports.KeyManager

	// Build and Binary
	Builder() ports.Builder
	BinaryCache() ports.BinaryCache
	BinaryResolver() ports.BinaryResolver
	BinaryExecutor() ports.BinaryExecutor
	BinaryVersionDetector() ports.BinaryVersionDetector

	// External Services
	GitHubClient() ports.GitHubClient
	InteractiveSelector() ports.InteractiveSelector

	// Logging
	Logger() ports.Logger
}

// infrastructure is the concrete implementation of InfrastructureProvider.
type infrastructure struct {
	// Repositories
	devnetRepo ports.DevnetRepository
	nodeRepo   ports.NodeRepository
	exportRepo ports.ExportRepository

	// Process Management
	executor      ports.ProcessExecutor
	healthChecker ports.HealthChecker

	// Network Communication
	rpcClient ports.RPCClient
	evmClient ports.EVMClient

	// Data Fetching
	snapshotSvc    ports.SnapshotFetcher
	genesisSvc     ports.GenesisFetcher
	stateExportSvc ports.StateExportService

	// Node Operations
	nodeInitializer    ports.NodeInitializer
	networkModule      ports.NetworkModule
	validatorKeyLoader ports.ValidatorKeyLoader
	keyManager         ports.KeyManager

	// Build and Binary
	builder               ports.Builder
	binaryCache           ports.BinaryCache
	binaryResolver        ports.BinaryResolver
	binaryExecutor        ports.BinaryExecutor
	binaryVersionDetector ports.BinaryVersionDetector

	// External Services
	githubClient        ports.GitHubClient
	interactiveSelector ports.InteractiveSelector

	// Logging
	logger ports.Logger
}

// InfrastructureConfig holds all infrastructure dependencies for construction.
type InfrastructureConfig struct {
	DevnetRepo            ports.DevnetRepository
	NodeRepo              ports.NodeRepository
	ExportRepo            ports.ExportRepository
	Executor              ports.ProcessExecutor
	HealthChecker         ports.HealthChecker
	RPCClient             ports.RPCClient
	EVMClient             ports.EVMClient
	SnapshotSvc           ports.SnapshotFetcher
	GenesisSvc            ports.GenesisFetcher
	StateExportSvc        ports.StateExportService
	NodeInitializer       ports.NodeInitializer
	NetworkModule         ports.NetworkModule
	ValidatorKeyLoader    ports.ValidatorKeyLoader
	KeyManager            ports.KeyManager
	Builder               ports.Builder
	BinaryCache           ports.BinaryCache
	BinaryResolver        ports.BinaryResolver
	BinaryExecutor        ports.BinaryExecutor
	BinaryVersionDetector ports.BinaryVersionDetector
	GitHubClient          ports.GitHubClient
	InteractiveSelector   ports.InteractiveSelector
	Logger                ports.Logger
}

// NewInfrastructure creates a new InfrastructureProvider from configuration.
func NewInfrastructure(cfg InfrastructureConfig) InfrastructureProvider {
	return &infrastructure{
		devnetRepo:            cfg.DevnetRepo,
		nodeRepo:              cfg.NodeRepo,
		exportRepo:            cfg.ExportRepo,
		executor:              cfg.Executor,
		healthChecker:         cfg.HealthChecker,
		rpcClient:             cfg.RPCClient,
		evmClient:             cfg.EVMClient,
		snapshotSvc:           cfg.SnapshotSvc,
		genesisSvc:            cfg.GenesisSvc,
		stateExportSvc:        cfg.StateExportSvc,
		nodeInitializer:       cfg.NodeInitializer,
		networkModule:         cfg.NetworkModule,
		validatorKeyLoader:    cfg.ValidatorKeyLoader,
		keyManager:            cfg.KeyManager,
		builder:               cfg.Builder,
		binaryCache:           cfg.BinaryCache,
		binaryResolver:        cfg.BinaryResolver,
		binaryExecutor:        cfg.BinaryExecutor,
		binaryVersionDetector: cfg.BinaryVersionDetector,
		githubClient:          cfg.GitHubClient,
		interactiveSelector:   cfg.InteractiveSelector,
		logger:                cfg.Logger,
	}
}

// Implementation of InfrastructureProvider interface

func (i *infrastructure) DevnetRepository() ports.DevnetRepository     { return i.devnetRepo }
func (i *infrastructure) NodeRepository() ports.NodeRepository         { return i.nodeRepo }
func (i *infrastructure) ExportRepository() ports.ExportRepository     { return i.exportRepo }
func (i *infrastructure) ProcessExecutor() ports.ProcessExecutor       { return i.executor }
func (i *infrastructure) HealthChecker() ports.HealthChecker           { return i.healthChecker }
func (i *infrastructure) RPCClient() ports.RPCClient                   { return i.rpcClient }
func (i *infrastructure) EVMClient() ports.EVMClient                   { return i.evmClient }
func (i *infrastructure) SnapshotFetcher() ports.SnapshotFetcher       { return i.snapshotSvc }
func (i *infrastructure) GenesisFetcher() ports.GenesisFetcher         { return i.genesisSvc }
func (i *infrastructure) StateExportService() ports.StateExportService { return i.stateExportSvc }
func (i *infrastructure) NodeInitializer() ports.NodeInitializer       { return i.nodeInitializer }
func (i *infrastructure) NetworkModule() ports.NetworkModule           { return i.networkModule }
func (i *infrastructure) ValidatorKeyLoader() ports.ValidatorKeyLoader { return i.validatorKeyLoader }
func (i *infrastructure) KeyManager() ports.KeyManager                 { return i.keyManager }
func (i *infrastructure) Builder() ports.Builder                       { return i.builder }
func (i *infrastructure) BinaryCache() ports.BinaryCache               { return i.binaryCache }
func (i *infrastructure) BinaryResolver() ports.BinaryResolver         { return i.binaryResolver }
func (i *infrastructure) BinaryExecutor() ports.BinaryExecutor         { return i.binaryExecutor }
func (i *infrastructure) BinaryVersionDetector() ports.BinaryVersionDetector {
	return i.binaryVersionDetector
}
func (i *infrastructure) GitHubClient() ports.GitHubClient { return i.githubClient }
func (i *infrastructure) InteractiveSelector() ports.InteractiveSelector {
	return i.interactiveSelector
}
func (i *infrastructure) Logger() ports.Logger { return i.logger }

// LoggerAdapter adapts output.Logger to ports.Logger interface.
// This adapter enables the use of the concrete logger implementation
// while maintaining loose coupling through the ports interface.
type LoggerAdapter struct {
	logger *output.Logger
}

// NewLoggerAdapter creates a new LoggerAdapter.
func NewLoggerAdapter(logger *output.Logger) *LoggerAdapter {
	return &LoggerAdapter{logger: logger}
}

func (a *LoggerAdapter) Info(format string, args ...interface{})  { a.logger.Info(format, args...) }
func (a *LoggerAdapter) Warn(format string, args ...interface{})  { a.logger.Warn(format, args...) }
func (a *LoggerAdapter) Error(format string, args ...interface{}) { a.logger.Error(format, args...) }
func (a *LoggerAdapter) Debug(format string, args ...interface{}) { a.logger.Debug(format, args...) }
func (a *LoggerAdapter) Success(format string, args ...interface{}) {
	a.logger.Success(format, args...)
}
func (a *LoggerAdapter) Print(format string, args ...interface{}) { a.logger.Print(format, args...) }
func (a *LoggerAdapter) Println(format string, args ...interface{}) {
	a.logger.Println(format, args...)
}
func (a *LoggerAdapter) SetVerbose(verbose bool) { a.logger.SetVerbose(verbose) }
func (a *LoggerAdapter) IsVerbose() bool         { return a.logger.IsVerbose() }
func (a *LoggerAdapter) Writer() io.Writer       { return a.logger.Writer() }
func (a *LoggerAdapter) ErrWriter() io.Writer    { return a.logger.ErrWriter() }
