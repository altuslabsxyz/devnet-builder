// Package di provides dependency injection container for the application.
// It centralizes the creation and management of service dependencies.
package di

import (
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/plugin"
)

// Container holds all application dependencies.
// It provides a central place for dependency injection,
// making the application more testable and loosely coupled.
type Container struct {
	logger     *output.Logger
	networkReg *NetworkRegistry
	pluginMgr  *plugin.PluginManager
	config     *Config
}

// Config holds configuration for the container.
type Config struct {
	HomeDir   string
	PluginDir string
	Verbose   bool
	NoColor   bool
	JSONMode  bool
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
