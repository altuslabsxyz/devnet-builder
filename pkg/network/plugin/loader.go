package plugin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/altuslabsxyz/devnet-builder/internal/paths"
	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// =============================================================================
// Plugin Loader Errors
// =============================================================================

var (
	// ErrPluginNotFound is returned when a plugin cannot be found.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrPluginNotLoaded is returned when operating on an unloaded plugin.
	ErrPluginNotLoaded = errors.New("plugin not loaded")

	// ErrPluginAlreadyLoaded is returned when attempting to load an already loaded plugin.
	ErrPluginAlreadyLoaded = errors.New("plugin already loaded")

	// ErrIncompatibleVersion is returned when a plugin version is incompatible.
	ErrIncompatibleVersion = errors.New("incompatible plugin version")
)

// PluginError provides detailed error information for plugin operations.
type PluginError struct {
	Op         string // Operation that failed (e.g., "load", "validate")
	PluginName string // Name of the plugin
	Err        error  // Underlying error
}

func (e *PluginError) Error() string {
	if e.PluginName != "" {
		return fmt.Sprintf("plugin %s: %s: %v", e.PluginName, e.Op, e.Err)
	}
	return fmt.Sprintf("plugin: %s: %v", e.Op, e.Err)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

// =============================================================================
// Version Compatibility
// =============================================================================

// VersionConstraint defines version compatibility requirements.
type VersionConstraint struct {
	MinVersion string // Minimum required version (semantic versioning)
	MaxVersion string // Maximum allowed version (empty = no upper limit)
}

// DefaultVersionConstraint returns the default version constraint for plugins.
func DefaultVersionConstraint() VersionConstraint {
	return VersionConstraint{
		MinVersion: "1.0.0",
		MaxVersion: "", // No upper limit
	}
}

// semverCompare compares two semantic versions.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b.
func semverCompare(a, b string) int {
	aParts := parseSemver(a)
	bParts := parseSemver(b)

	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

// parseSemver parses a semantic version string into [major, minor, patch].
func parseSemver(version string) [3]int {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	var parts [3]int
	re := regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?`)
	matches := re.FindStringSubmatch(version)

	if len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &parts[0])
	}
	if len(matches) > 2 && matches[2] != "" {
		fmt.Sscanf(matches[2], "%d", &parts[1])
	}
	if len(matches) > 3 && matches[3] != "" {
		fmt.Sscanf(matches[3], "%d", &parts[2])
	}

	return parts
}

// CheckVersion validates if a version satisfies the constraint.
func (c VersionConstraint) CheckVersion(version string) error {
	if c.MinVersion != "" && semverCompare(version, c.MinVersion) < 0 {
		return fmt.Errorf("%w: version %s is below minimum %s", ErrIncompatibleVersion, version, c.MinVersion)
	}
	if c.MaxVersion != "" && semverCompare(version, c.MaxVersion) > 0 {
		return fmt.Errorf("%w: version %s exceeds maximum %s", ErrIncompatibleVersion, version, c.MaxVersion)
	}
	return nil
}

// PluginClient represents a loaded plugin client.
type PluginClient struct {
	client *plugin.Client
	module network.Module
	name   string
}

// Module returns the network module implementation from the plugin.
func (p *PluginClient) Module() network.Module {
	return p.module
}

// Name returns the plugin name.
func (p *PluginClient) Name() string {
	return p.name
}

// Close cleanly shuts down the plugin.
func (p *PluginClient) Close() {
	if p.client != nil {
		p.client.Kill()
	}
}

// Loader discovers and loads network plugins.
type Loader struct {
	mu                sync.RWMutex
	pluginDirs        []string
	logger            hclog.Logger
	plugins           map[string]*PluginClient
	versionConstraint VersionConstraint
}

// LoaderOption is a functional option for configuring a Loader.
type LoaderOption func(*Loader)

// WithLogger sets a custom logger for the loader.
func WithLogger(logger hclog.Logger) LoaderOption {
	return func(l *Loader) {
		l.logger = logger
	}
}

// WithVersionConstraint sets a custom version constraint for plugin compatibility.
func WithVersionConstraint(constraint VersionConstraint) LoaderOption {
	return func(l *Loader) {
		l.versionConstraint = constraint
	}
}

// WithPluginDirs adds additional plugin directories.
func WithPluginDirs(dirs ...string) LoaderOption {
	return func(l *Loader) {
		l.pluginDirs = append(l.pluginDirs, dirs...)
	}
}

// NewLoader creates a new plugin loader with optional configuration.
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{
		pluginDirs: []string{
			"./plugins",
			paths.PluginsPath(paths.DefaultHomeDir()),
			"/usr/local/lib/devnet-builder/plugins",
		},
		logger:            hclog.New(&hclog.LoggerOptions{Name: "plugin-loader", Level: hclog.Warn}),
		plugins:           make(map[string]*PluginClient),
		versionConstraint: DefaultVersionConstraint(),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// NewLoaderWithDirs creates a new plugin loader with specific directories (legacy API).
//
// Deprecated: Use NewLoader with WithPluginDirs option instead.
func NewLoaderWithDirs(pluginDirs ...string) *Loader {
	if len(pluginDirs) == 0 {
		return NewLoader()
	}

	return &Loader{
		pluginDirs:        pluginDirs,
		logger:            hclog.New(&hclog.LoggerOptions{Name: "plugin-loader", Level: hclog.Warn}),
		plugins:           make(map[string]*PluginClient),
		versionConstraint: DefaultVersionConstraint(),
	}
}

// SetLogger sets the logger for the plugin loader.
func (l *Loader) SetLogger(logger hclog.Logger) {
	l.logger = logger
}

// PluginInfo contains metadata about a discovered plugin.
type PluginInfo struct {
	Name    string // Network name (e.g., "stable", "osmosis")
	Path    string // Full path to the plugin binary
	Size    int64  // File size in bytes
	ModTime int64  // Modification time (Unix timestamp)
}

// Discover finds all available plugins in the plugin directories.
func (l *Loader) Discover() ([]string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var plugins []string
	seen := make(map[string]bool)

	for _, dir := range l.pluginDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			l.logger.Warn("failed to read plugin directory", "dir", dir, "error", err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			// Plugin binaries should be named <network>-plugin
			if !strings.HasSuffix(name, "-plugin") {
				continue
			}

			// Extract network name
			networkName := strings.TrimSuffix(name, "-plugin")
			if seen[networkName] {
				continue
			}

			// Check if executable
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Mode()&0111 == 0 {
				continue
			}

			seen[networkName] = true
			plugins = append(plugins, networkName)
		}
	}

	return plugins, nil
}

// DiscoverWithInfo finds all available plugins with detailed metadata.
func (l *Loader) DiscoverWithInfo() ([]PluginInfo, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var plugins []PluginInfo
	seen := make(map[string]bool)

	for _, dir := range l.pluginDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			l.logger.Warn("failed to read plugin directory", "dir", dir, "error", err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, "-plugin") {
				continue
			}

			networkName := strings.TrimSuffix(name, "-plugin")
			if seen[networkName] {
				continue
			}

			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Mode()&0111 == 0 {
				continue
			}

			seen[networkName] = true
			plugins = append(plugins, PluginInfo{
				Name:    networkName,
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime().Unix(),
			})
		}
	}

	return plugins, nil
}

// Load loads a plugin by name and returns the network module.
func (l *Loader) Load(name string) (*PluginClient, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.loadLocked(name)
}

// loadLocked loads a plugin without acquiring the mutex (caller must hold lock).
func (l *Loader) loadLocked(name string) (*PluginClient, error) {
	// Check if already loaded
	if p, ok := l.plugins[name]; ok {
		return p, nil
	}

	// Find the plugin binary
	pluginPath, err := l.findPluginLocked(name)
	if err != nil {
		return nil, &PluginError{Op: "find", PluginName: name, Err: err}
	}

	// Create the plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"network": &NetworkModulePlugin{},
		},
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           l.logger,
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, &PluginError{Op: "connect", PluginName: name, Err: err}
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("network")
	if err != nil {
		client.Kill()
		return nil, &PluginError{Op: "dispense", PluginName: name, Err: err}
	}

	module, ok := raw.(network.Module)
	if !ok {
		client.Kill()
		return nil, &PluginError{
			Op:         "type-assertion",
			PluginName: name,
			Err:        fmt.Errorf("does not implement network.Module"),
		}
	}

	// Validate version compatibility
	version := module.Version()
	if err := l.versionConstraint.CheckVersion(version); err != nil {
		client.Kill()
		return nil, &PluginError{Op: "version-check", PluginName: name, Err: err}
	}

	// Validate the module
	if err := module.Validate(); err != nil {
		client.Kill()
		return nil, &PluginError{Op: "validate", PluginName: name, Err: err}
	}

	pc := &PluginClient{
		client: client,
		module: module,
		name:   name,
	}

	l.plugins[name] = pc
	l.logger.Info("plugin loaded successfully", "name", name, "version", version)
	return pc, nil
}

// LoadAll loads all discovered plugins.
// Returns successfully loaded plugins; errors for individual plugins are logged.
func (l *Loader) LoadAll() ([]*PluginClient, error) {
	// Discover without lock (it acquires its own)
	names, err := l.Discover()
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var clients []*PluginClient
	for _, name := range names {
		pc, err := l.loadLocked(name)
		if err != nil {
			l.logger.Warn("failed to load plugin", "name", name, "error", err)
			continue
		}
		clients = append(clients, pc)
	}

	return clients, nil
}

// LoadAllStrict loads all discovered plugins, failing if any plugin fails to load.
func (l *Loader) LoadAllStrict() ([]*PluginClient, error) {
	names, err := l.Discover()
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var clients []*PluginClient
	for _, name := range names {
		pc, err := l.loadLocked(name)
		if err != nil {
			// Unload any plugins we've loaded so far
			for _, loaded := range clients {
				loaded.Close()
				delete(l.plugins, loaded.name)
			}
			return nil, fmt.Errorf("failed to load all plugins: %w", err)
		}
		clients = append(clients, pc)
	}

	return clients, nil
}

// Get returns a loaded plugin by name.
func (l *Loader) Get(name string) (*PluginClient, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	p, ok := l.plugins[name]
	return p, ok
}

// IsLoaded checks if a plugin is currently loaded.
func (l *Loader) IsLoaded(name string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	_, ok := l.plugins[name]
	return ok
}

// LoadedPlugins returns a list of all loaded plugin names.
func (l *Loader) LoadedPlugins() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.plugins))
	for name := range l.plugins {
		names = append(names, name)
	}
	return names
}

// Close closes all loaded plugins.
func (l *Loader) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, p := range l.plugins {
		p.Close()
	}
	l.plugins = make(map[string]*PluginClient)
	l.logger.Info("all plugins closed")
}

// =============================================================================
// New Methods (A2 Task)
// =============================================================================

// ValidatePlugin validates a plugin without loading it permanently.
// This is useful for checking plugin compatibility before actual use.
func (l *Loader) ValidatePlugin(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// If already loaded, just re-validate
	if pc, ok := l.plugins[name]; ok {
		return pc.module.Validate()
	}

	// Find the plugin binary
	pluginPath, err := l.findPluginLocked(name)
	if err != nil {
		return &PluginError{Op: "validate-find", PluginName: name, Err: ErrPluginNotFound}
	}

	// Create a temporary client for validation
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"network": &NetworkModulePlugin{},
		},
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           l.logger,
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		return &PluginError{Op: "validate-connect", PluginName: name, Err: err}
	}

	raw, err := rpcClient.Dispense("network")
	if err != nil {
		return &PluginError{Op: "validate-dispense", PluginName: name, Err: err}
	}

	module, ok := raw.(network.Module)
	if !ok {
		return &PluginError{
			Op:         "validate-type",
			PluginName: name,
			Err:        fmt.Errorf("does not implement network.Module"),
		}
	}

	// Check version compatibility
	if err := l.versionConstraint.CheckVersion(module.Version()); err != nil {
		return &PluginError{Op: "validate-version", PluginName: name, Err: err}
	}

	// Run module's own validation
	if err := module.Validate(); err != nil {
		return &PluginError{Op: "validate-module", PluginName: name, Err: err}
	}

	l.logger.Debug("plugin validation successful", "name", name, "version", module.Version())
	return nil
}

// GetPluginVersion returns the version of a plugin.
// If the plugin is not loaded, it temporarily loads it to get the version.
func (l *Loader) GetPluginVersion(name string) (string, error) {
	l.mu.RLock()
	if pc, ok := l.plugins[name]; ok {
		l.mu.RUnlock()
		return pc.module.Version(), nil
	}
	l.mu.RUnlock()

	// Need to temporarily load the plugin to get version
	l.mu.Lock()
	defer l.mu.Unlock()

	pluginPath, err := l.findPluginLocked(name)
	if err != nil {
		return "", &PluginError{Op: "get-version", PluginName: name, Err: ErrPluginNotFound}
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"network": &NetworkModulePlugin{},
		},
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           l.logger,
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		return "", &PluginError{Op: "get-version-connect", PluginName: name, Err: err}
	}

	raw, err := rpcClient.Dispense("network")
	if err != nil {
		return "", &PluginError{Op: "get-version-dispense", PluginName: name, Err: err}
	}

	module, ok := raw.(network.Module)
	if !ok {
		return "", &PluginError{
			Op:         "get-version-type",
			PluginName: name,
			Err:        fmt.Errorf("does not implement network.Module"),
		}
	}

	return module.Version(), nil
}

// UnloadPlugin unloads a specific plugin.
func (l *Loader) UnloadPlugin(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	pc, ok := l.plugins[name]
	if !ok {
		return &PluginError{Op: "unload", PluginName: name, Err: ErrPluginNotLoaded}
	}

	pc.Close()
	delete(l.plugins, name)
	l.logger.Info("plugin unloaded", "name", name)
	return nil
}

// Reload unloads and reloads a plugin.
// Useful for hot-reloading plugin updates.
func (l *Loader) Reload(name string) (*PluginClient, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Unload if loaded
	if pc, ok := l.plugins[name]; ok {
		pc.Close()
		delete(l.plugins, name)
	}

	// Reload
	return l.loadLocked(name)
}

// =============================================================================
// Internal Methods
// =============================================================================

// findPluginLocked finds the plugin binary path (caller must hold lock).
func (l *Loader) findPluginLocked(name string) (string, error) {
	binaryName := name + "-plugin"

	for _, dir := range l.pluginDirs {
		path := filepath.Join(dir, binaryName)
		if info, err := os.Stat(path); err == nil && info.Mode()&0111 != 0 {
			return path, nil
		}
	}

	return "", fmt.Errorf("%w: %s not in %v", ErrPluginNotFound, name, l.pluginDirs)
}

// findPlugin finds the plugin binary path for the given network name.
//
// Deprecated: Use findPluginLocked with proper locking.
func (l *Loader) findPlugin(name string) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.findPluginLocked(name)
}

// PluginDirs returns the configured plugin directories.
func (l *Loader) PluginDirs() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	dirs := make([]string, len(l.pluginDirs))
	copy(dirs, l.pluginDirs)
	return dirs
}

// AddPluginDir adds a directory to the plugin search path.
func (l *Loader) AddPluginDir(dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pluginDirs = append(l.pluginDirs, dir)
}

// SetVersionConstraint updates the version constraint for future plugin loads.
func (l *Loader) SetVersionConstraint(constraint VersionConstraint) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.versionConstraint = constraint
}
