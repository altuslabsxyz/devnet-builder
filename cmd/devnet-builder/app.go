// Package main provides the CLI entry point for devnet-builder.
// app.go contains application-level initialization and dependency injection wiring.
package main

import (
	"sync"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
	"github.com/altuslabsxyz/devnet-builder/internal/di"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types"
)

var (
	// appContainer holds the global DI container instance.
	appContainer *di.Container
	// containerOnce ensures container is initialized only once.
	containerOnce sync.Once
	// containerErr stores any initialization error.
	containerErr error
)

// AppConfig holds configuration for the application container.
type AppConfig struct {
	HomeDir           string
	BlockchainNetwork string
	ExecutionMode     string // "docker" or "local"
	Verbose           bool
	NoColor           bool
	JSONMode          bool
}

// InitContainer initializes the global DI container with the given configuration.
// This should be called once during application startup, typically in PersistentPreRunE.
// It is safe to call multiple times - only the first call initializes the container.
func InitContainer(cfg AppConfig) (*di.Container, error) {
	containerOnce.Do(func() {
		appContainer, containerErr = initContainerInternal(cfg)
	})
	return appContainer, containerErr
}

// GetContainer returns the global DI container.
// Returns nil if InitContainer has not been called.
func GetContainer() *di.Container {
	return appContainer
}

// ResetContainer resets the container for testing purposes.
// This allows re-initialization with different configuration.
func ResetContainer() {
	appContainer = nil
	containerErr = nil
	containerOnce = sync.Once{}
}

// initContainerInternal creates and wires the DI container.
func initContainerInternal(cfg AppConfig) (*di.Container, error) {
	logger := output.DefaultLogger
	logger.SetVerbose(cfg.Verbose)
	logger.SetNoColor(cfg.NoColor)
	logger.SetJSONMode(cfg.JSONMode)

	// Create infrastructure factory
	factory := di.NewInfrastructureFactory(cfg.HomeDir, logger)

	// Set network module if specified
	if cfg.BlockchainNetwork != "" {
		module, err := network.Get(cfg.BlockchainNetwork)
		if err == nil {
			factory.WithNetworkModule(module)
		}
	}

	// Set execution mode
	factory.WithDockerMode(cfg.ExecutionMode == string(types.ExecutionModeDocker))

	// Wire container with all infrastructure components
	container, err := factory.WireContainer(
		di.WithConfig(&di.Config{
			HomeDir:  cfg.HomeDir,
			Verbose:  cfg.Verbose,
			NoColor:  cfg.NoColor,
			JSONMode: cfg.JSONMode,
		}),
	)
	if err != nil {
		return nil, err
	}

	// Initialize binary resolver with plugin loader
	// The plugin loader is global and created in main.go
	if pluginLoader := GetPluginLoader(); pluginLoader != nil {
		binaryCache := container.BinaryCache()
		binaryResolver := factory.CreateBinaryResolver(pluginLoader, binaryCache)
		container.SetBinaryResolver(binaryResolver)
	}

	return container, nil
}

// InitContainerForCommand initializes the container for a specific command.
// This is a convenience function that extracts common parameters from global flags.
func InitContainerForCommand(blockchainNetwork, executionMode string) (*di.Container, error) {
	return InitContainer(AppConfig{
		HomeDir:           shared.GetHomeDir(),
		BlockchainNetwork: blockchainNetwork,
		ExecutionMode:     executionMode,
		Verbose:           shared.GetVerbose(),
		NoColor:           shared.GetNoColor(),
		JSONMode:          shared.GetJSONMode(),
	})
}

// MustGetContainer returns the global container or panics if not initialized.
// Use this only when you're certain the container has been initialized.
func MustGetContainer() *di.Container {
	if appContainer == nil {
		panic("DI container not initialized. Call InitContainer first.")
	}
	return appContainer
}
