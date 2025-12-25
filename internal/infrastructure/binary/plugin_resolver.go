package binary

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/pkg/network"
	"github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

// PluginBinaryResolver implements ports.BinaryResolver using the plugin system.
// It resolves plugin names to binary paths by:
//  1. Loading the plugin to get its Module interface
//  2. Querying the binary name from Module.BinaryName()
//  3. Checking the BinaryCache for the active binary
//
// This adapter follows the Adapter pattern to bridge the plugin system
// with the binary passthrough use cases.
//
// Thread-safety: This implementation is thread-safe for concurrent use.
type PluginBinaryResolver struct {
	loader *plugin.Loader
	cache  ports.BinaryCache

	// pluginModules caches loaded plugin modules to avoid repeated RPC calls
	pluginModules map[string]network.Module
	mu            sync.RWMutex
}

// NewPluginBinaryResolver creates a new PluginBinaryResolver.
//
// Parameters:
//   - loader: Plugin loader for discovering and loading plugins
//   - cache: Binary cache for resolving cached binary paths
//
// Returns:
//   - *PluginBinaryResolver: Configured resolver instance
func NewPluginBinaryResolver(loader *plugin.Loader, cache ports.BinaryCache) *PluginBinaryResolver {
	return &PluginBinaryResolver{
		loader:        loader,
		cache:         cache,
		pluginModules: make(map[string]network.Module),
	}
}

// ResolveBinary resolves a plugin name to its binary path.
// This implementation:
//  1. Loads the plugin (or retrieves from cache)
//  2. Gets the binary name from the Module interface
//  3. Checks for custom binary in devnet metadata (future enhancement)
//  4. Falls back to active binary from cache
//
// Thread-safe: Uses RWMutex for concurrent access to plugin cache.
func (r *PluginBinaryResolver) ResolveBinary(ctx context.Context, pluginName string) (string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Load plugin module (cached or fresh load)
	module, err := r.getOrLoadModule(ctx, pluginName)
	if err != nil {
		return "", fmt.Errorf("failed to load plugin %q: %w", pluginName, err)
	}

	// Get binary name from module
	binaryName := module.BinaryName()
	if binaryName == "" {
		return "", fmt.Errorf("plugin %q does not specify a binary name", pluginName)
	}

	// Try to get active binary from cache
	binaryPath, err := r.cache.GetActive()
	if err != nil {
		return "", fmt.Errorf("no active binary set for plugin %q: %w", pluginName, err)
	}

	// Verify binary exists and is executable
	if err := validateBinary(binaryPath); err != nil {
		return "", fmt.Errorf("active binary for plugin %q is invalid: %w", pluginName, err)
	}

	return binaryPath, nil
}

// GetActiveBinary returns the currently active binary path and plugin name.
// This queries the BinaryCache for the active symlink and resolves it.
func (r *PluginBinaryResolver) GetActiveBinary(ctx context.Context) (string, string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	default:
	}

	// Get symlink info from cache
	symlinkInfo, err := r.cache.SymlinkInfo()
	if err != nil {
		return "", "", fmt.Errorf("failed to get active binary info: %w", err)
	}

	if !symlinkInfo.Exists {
		return "", "", fmt.Errorf("no active binary set (symlink does not exist)")
	}

	binaryPath := symlinkInfo.Target
	if symlinkInfo.IsRegular {
		// If it's a regular file (not a symlink), use the symlink path itself
		binaryPath = symlinkInfo.Path
	}

	// Verify binary exists
	if err := validateBinary(binaryPath); err != nil {
		return "", "", fmt.Errorf("active binary is invalid: %w", err)
	}

	// Try to determine plugin name from the binary path
	// The cache path structure is: <cache-dir>/<network>/<ref>/binary
	// We extract the network name from the path
	pluginName := extractPluginNameFromPath(binaryPath)

	return binaryPath, pluginName, nil
}

// GetBinaryName returns the binary name for a plugin.
// This loads the plugin and queries its Module.BinaryName() method.
func (r *PluginBinaryResolver) GetBinaryName(ctx context.Context, pluginName string) (string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	module, err := r.getOrLoadModule(ctx, pluginName)
	if err != nil {
		return "", fmt.Errorf("failed to load plugin %q: %w", pluginName, err)
	}

	binaryName := module.BinaryName()
	if binaryName == "" {
		return "", fmt.Errorf("plugin %q does not specify a binary name", pluginName)
	}

	return binaryName, nil
}

// ListAvailablePlugins returns all available plugin names.
// This uses the plugin loader's Discover() method.
func (r *PluginBinaryResolver) ListAvailablePlugins(ctx context.Context) ([]string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	plugins, err := r.loader.Discover()
	if err != nil {
		return nil, fmt.Errorf("failed to discover plugins: %w", err)
	}

	return plugins, nil
}

// getOrLoadModule retrieves a plugin module from cache or loads it.
// Thread-safe with RWMutex for concurrent access.
func (r *PluginBinaryResolver) getOrLoadModule(ctx context.Context, pluginName string) (network.Module, error) {
	// Fast path: check cache with read lock
	r.mu.RLock()
	if module, ok := r.pluginModules[pluginName]; ok {
		r.mu.RUnlock()
		return module, nil
	}
	r.mu.RUnlock()

	// Slow path: load plugin with write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if module, ok := r.pluginModules[pluginName]; ok {
		return module, nil
	}

	// Check for context cancellation before expensive operation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Load plugin
	pluginClient, err := r.loader.Load(pluginName)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin: %w", err)
	}

	// Get module interface
	module := pluginClient.Module()
	if module == nil {
		return nil, fmt.Errorf("plugin %q returned nil module", pluginName)
	}

	// Cache for future use
	r.pluginModules[pluginName] = module

	return module, nil
}

// validateBinary checks if a binary path exists and is executable.
func validateBinary(binaryPath string) error {
	// Check if file exists
	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary not found at %s", binaryPath)
		}
		return fmt.Errorf("failed to stat binary: %w", err)
	}

	// Check if it's a regular file
	if info.IsDir() {
		return fmt.Errorf("path %s is a directory, not a binary", binaryPath)
	}

	// Check if executable (Unix permission bits)
	// For portability, we check any of the executable bits
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary at %s is not executable", binaryPath)
	}

	return nil
}

// extractPluginNameFromPath attempts to extract the plugin name from a binary path.
// Cache structure: <cache-dir>/<network>/<ref>/binary
// Returns empty string if unable to determine.
func extractPluginNameFromPath(binaryPath string) string {
	// Clean the path
	cleanPath := filepath.Clean(binaryPath)

	// Get the directory containing the binary
	dir := filepath.Dir(cleanPath)

	// Go up one level to get the ref directory
	refDir := filepath.Dir(dir)

	// Go up one more level to get the network directory
	networkDir := filepath.Dir(refDir)

	// The network name is the base of this directory
	networkName := filepath.Base(networkDir)

	// Sanity check: if the path is too short, return empty
	if networkName == "." || networkName == "/" {
		return ""
	}

	return networkName
}

// Ensure PluginBinaryResolver implements ports.BinaryResolver
var _ ports.BinaryResolver = (*PluginBinaryResolver)(nil)
