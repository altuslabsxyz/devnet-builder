// internal/daemon/server/plugins.go
package server

import (
	"fmt"
	"log/slog"

	hclog "github.com/hashicorp/go-hclog"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

// PluginManager handles plugin discovery, loading, and registration.
type PluginManager struct {
	loader *plugin.Loader
	logger *slog.Logger
}

// PluginManagerConfig configures the PluginManager.
type PluginManagerConfig struct {
	// PluginDirs are additional directories to search for plugins.
	// Default directories (~/.devnet-builder/plugins, ./plugins) are always included.
	PluginDirs []string

	// Logger for logging plugin operations.
	Logger *slog.Logger
}

// NewPluginManager creates a new PluginManager.
func NewPluginManager(config PluginManagerConfig) *PluginManager {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Create hashicorp logger adapter
	hcLogger := hclog.New(&hclog.LoggerOptions{
		Name:  "plugin-loader",
		Level: hclog.Warn,
	})

	// Create loader with options
	opts := []plugin.LoaderOption{
		plugin.WithLogger(hcLogger),
	}
	if len(config.PluginDirs) > 0 {
		opts = append(opts, plugin.WithPluginDirs(config.PluginDirs...))
	}

	return &PluginManager{
		loader: plugin.NewLoader(opts...),
		logger: logger,
	}
}

// LoadResult contains the results of loading plugins.
type LoadResult struct {
	// Loaded contains the names of successfully loaded plugins.
	Loaded []string

	// Errors contains any plugin loading errors.
	Errors []PluginLoadError
}

// PluginLoadError represents a plugin that failed to load.
type PluginLoadError struct {
	Name  string
	Error error
}

// LoadAndRegister discovers plugins, loads them, and registers them with the global network registry.
// Returns the load results including any errors encountered.
func (pm *PluginManager) LoadAndRegister() (*LoadResult, error) {
	result := &LoadResult{
		Loaded: make([]string, 0),
		Errors: make([]PluginLoadError, 0),
	}

	// Discover available plugins
	discovered, err := pm.loader.Discover()
	if err != nil {
		return nil, fmt.Errorf("failed to discover plugins: %w", err)
	}

	if len(discovered) == 0 {
		pm.logger.Info("no plugins discovered in plugin directories",
			"dirs", pm.loader.PluginDirs())
		return result, nil
	}

	pm.logger.Info("discovered plugins", "count", len(discovered), "plugins", discovered)

	// Load each plugin and register with global registry
	for _, name := range discovered {
		if err := pm.loadAndRegisterPlugin(name); err != nil {
			pm.logger.Warn("failed to load plugin",
				"plugin", name,
				"error", err)
			result.Errors = append(result.Errors, PluginLoadError{
				Name:  name,
				Error: err,
			})
			continue
		}

		pm.logger.Info("plugin loaded and registered", "plugin", name)
		result.Loaded = append(result.Loaded, name)
	}

	return result, nil
}

// loadAndRegisterPlugin loads a single plugin and registers it with the global registry.
func (pm *PluginManager) loadAndRegisterPlugin(name string) error {
	// Load the plugin
	client, err := pm.loader.Load(name)
	if err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	// Get the module from the plugin
	module := client.Module()

	// Wrap with adapter to convert pkg/network.Module to internal/network.NetworkModule
	adapter := network.NewPluginAdapter(module)

	// Register with global registry
	if err := network.MustRegister(adapter, false); err != nil {
		return fmt.Errorf("failed to register plugin module: %w", err)
	}

	return nil
}

// Close closes all loaded plugins.
func (pm *PluginManager) Close() {
	pm.loader.Close()
}

// ListLoaded returns the names of currently loaded plugins.
func (pm *PluginManager) ListLoaded() []string {
	return pm.loader.LoadedPlugins()
}

// Reload reloads a specific plugin.
func (pm *PluginManager) Reload(name string) error {
	// First, try to reload the plugin in the loader
	client, err := pm.loader.Reload(name)
	if err != nil {
		return fmt.Errorf("failed to reload plugin: %w", err)
	}

	// Get the module from the reloaded plugin
	module := client.Module()

	// Wrap with adapter
	adapter := network.NewPluginAdapter(module)

	// Re-register with global registry
	// Note: The registry doesn't support re-registration, so this may fail
	// if the module was previously registered. In production, you'd want
	// a mechanism to update existing registrations.
	if err := network.MustRegister(adapter, false); err != nil {
		pm.logger.Warn("plugin reloaded but registry update failed (already registered)",
			"plugin", name,
			"error", err)
	}

	return nil
}

// DiscoverAvailable returns the names of available (but not necessarily loaded) plugins.
func (pm *PluginManager) DiscoverAvailable() ([]string, error) {
	return pm.loader.Discover()
}
