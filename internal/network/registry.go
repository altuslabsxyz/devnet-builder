package network

import (
	"sort"
	"sync"
)

// DefaultNetworkName is the name of the default network for backward compatibility.
const DefaultNetworkName = "stable"

// Global registry instance
var (
	globalRegistry = newRegistry()
)

// registry holds registered network modules.
type registry struct {
	mu       sync.RWMutex
	modules  map[string]NetworkModule
	defaults string // default network name
}

// newRegistry creates a new registry instance.
func newRegistry() *registry {
	return &registry{
		modules:  make(map[string]NetworkModule),
		defaults: DefaultNetworkName,
	}
}

// Register adds a network module to the global registry.
// This function is typically called from init() in network module packages.
// It panics if:
//   - A module with the same name is already registered
//   - The module fails validation
func Register(module NetworkModule) {
	if err := globalRegistry.register(module); err != nil {
		panic(err)
	}
}

// MustRegister is like Register but allows specifying whether to panic on error.
// If panicOnError is false, errors are silently ignored.
func MustRegister(module NetworkModule, panicOnError bool) error {
	err := globalRegistry.register(module)
	if err != nil && panicOnError {
		panic(err)
	}
	return err
}

// Get retrieves a network module by name from the global registry.
// Returns an error if the network is not registered.
func Get(name string) (NetworkModule, error) {
	return globalRegistry.get(name)
}

// MustGet retrieves a network module by name, panicking if not found.
func MustGet(name string) NetworkModule {
	m, err := Get(name)
	if err != nil {
		panic(err)
	}
	return m
}

// Has checks if a network is registered.
func Has(name string) bool {
	return globalRegistry.has(name)
}

// List returns all registered network names in sorted order.
func List() []string {
	return globalRegistry.list()
}

// ListModules returns all registered network modules.
func ListModules() []NetworkModule {
	return globalRegistry.listModules()
}

// Default returns the default network module ("stable").
// Returns an error if the default network is not registered.
func Default() (NetworkModule, error) {
	return globalRegistry.defaults_()
}

// SetDefault changes the default network name.
// Returns an error if the network is not registered.
func SetDefault(name string) error {
	return globalRegistry.setDefault(name)
}

// Registry methods

func (r *registry) register(module NetworkModule) error {
	if module == nil {
		return &ModuleValidationError{
			ModuleName: "<nil>",
			Reason:     "cannot register nil module",
		}
	}

	// Validate module before registration
	if err := ValidateModuleCompatibility(module); err != nil {
		return err
	}

	name := module.Name()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[name]; exists {
		return &DuplicateNetworkError{NetworkName: name}
	}

	r.modules[name] = module
	return nil
}

func (r *registry) get(name string) (NetworkModule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	module, ok := r.modules[name]
	if !ok {
		return nil, &UnknownNetworkError{
			RequestedNetwork:  name,
			AvailableNetworks: r.listLocked(),
		}
	}
	return module, nil
}

func (r *registry) has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.modules[name]
	return ok
}

func (r *registry) list() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listLocked()
}

func (r *registry) listLocked() []string {
	names := make([]string, 0, len(r.modules))
	for name := range r.modules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *registry) listModules() []NetworkModule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modules := make([]NetworkModule, 0, len(r.modules))
	names := r.listLocked()
	for _, name := range names {
		modules = append(modules, r.modules[name])
	}
	return modules
}

func (r *registry) defaults_() (NetworkModule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaults == "" {
		return nil, ErrNoDefaultNetwork
	}

	module, ok := r.modules[r.defaults]
	if !ok {
		// Default is not registered yet - this is not an error during init
		// as modules may be registered in any order
		return nil, &UnknownNetworkError{
			RequestedNetwork:  r.defaults,
			AvailableNetworks: r.listLocked(),
		}
	}
	return module, nil
}

func (r *registry) setDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.modules[name]; !ok {
		return &UnknownNetworkError{
			RequestedNetwork:  name,
			AvailableNetworks: r.listLocked(),
		}
	}
	r.defaults = name
	return nil
}

// ResetRegistry clears all registered modules. This is primarily for testing.
func ResetRegistry() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.modules = make(map[string]NetworkModule)
	globalRegistry.defaults = DefaultNetworkName
}
