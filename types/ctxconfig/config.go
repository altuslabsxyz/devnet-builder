// Package ctxconfig provides context-based configuration management for devnet-builder.
// This package enables type-safe configuration propagation through context.Context,
// replacing global variables with a more idiomatic Go pattern.
//
// Usage:
//
//	// Create config and add to context
//	cfg := ctxconfig.New(
//	    ctxconfig.WithHomeDir("/path/to/home"),
//	    ctxconfig.WithChainID("devnet-1"),
//	)
//	ctx = ctxconfig.WithConfig(ctx, cfg)
//
//	// Retrieve config from context
//	cfg := ctxconfig.FromContext(ctx)
//	chainID := cfg.ChainID()
//
//	// Or use field-specific accessors
//	chainID := ctxconfig.ChainID(ctx)
package ctxconfig

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/types"
)

// configKey is the unexported key type for storing config in context.
// Using a struct type instead of string prevents collisions with other packages.
type configKey struct{}

// Config holds all configuration values that can be passed through context.
// All fields are private and accessed via methods to ensure immutability.
type Config struct {
	// Core settings
	homeDir    string
	configPath string

	// Output settings
	jsonMode bool
	noColor  bool
	verbose  bool

	// Chain settings
	chainID           string
	networkVersion    string
	blockchainNetwork string
	networkName       string // mainnet/testnet

	// Execution settings
	executionMode types.ExecutionMode

	// Node settings
	numValidators int
	numAccounts   int

	// Cache settings
	noCache  bool
	cacheTTL string

	// Auth settings
	githubToken string

	// Docker settings
	dockerImage string
}

// Option is a functional option for configuring Config.
type Option func(*Config)

// New creates a new Config with the given options.
func New(opts ...Option) *Config {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// Clone creates a copy of the Config with optional modifications.
func (c *Config) Clone(opts ...Option) *Config {
	if c == nil {
		return New(opts...)
	}
	// Create a shallow copy
	clone := *c
	// Apply modifications
	for _, opt := range opts {
		opt(&clone)
	}
	return &clone
}

// WithConfig returns a new context with the given config attached.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

// FromContext retrieves the Config from context.
// Returns nil if no config is present.
func FromContext(ctx context.Context) *Config {
	if ctx == nil {
		return nil
	}
	cfg, _ := ctx.Value(configKey{}).(*Config)
	return cfg
}

// MustFromContext retrieves the Config from context.
// Panics if no config is present.
func MustFromContext(ctx context.Context) *Config {
	cfg := FromContext(ctx)
	if cfg == nil {
		panic("ctxconfig: no config in context")
	}
	return cfg
}

// FromContextOrDefault retrieves the Config from context or returns a default config.
func FromContextOrDefault(ctx context.Context) *Config {
	cfg := FromContext(ctx)
	if cfg == nil {
		return &Config{}
	}
	return cfg
}

// ─────────────────────────────────────────────────────────────────────────────
// Config Accessors (methods on Config)
// ─────────────────────────────────────────────────────────────────────────────

// HomeDir returns the home directory path.
func (c *Config) HomeDir() string {
	if c == nil {
		return ""
	}
	return c.homeDir
}

// ConfigPath returns the config file path.
func (c *Config) ConfigPath() string {
	if c == nil {
		return ""
	}
	return c.configPath
}

// JSONMode returns whether JSON output mode is enabled.
func (c *Config) JSONMode() bool {
	if c == nil {
		return false
	}
	return c.jsonMode
}

// NoColor returns whether color output is disabled.
func (c *Config) NoColor() bool {
	if c == nil {
		return false
	}
	return c.noColor
}

// Verbose returns whether verbose output is enabled.
func (c *Config) Verbose() bool {
	if c == nil {
		return false
	}
	return c.verbose
}

// ChainID returns the chain identifier.
func (c *Config) ChainID() string {
	if c == nil {
		return ""
	}
	return c.chainID
}

// NetworkVersion returns the network version (e.g., "v1.0.0").
func (c *Config) NetworkVersion() string {
	if c == nil {
		return ""
	}
	return c.networkVersion
}

// BlockchainNetwork returns the blockchain network module name (e.g., "stable", "ault").
func (c *Config) BlockchainNetwork() string {
	if c == nil {
		return ""
	}
	return c.blockchainNetwork
}

// NetworkName returns the network source name (e.g., "mainnet", "testnet").
func (c *Config) NetworkName() string {
	if c == nil {
		return ""
	}
	return c.networkName
}

// ExecutionMode returns the execution mode (local/docker).
func (c *Config) ExecutionMode() types.ExecutionMode {
	if c == nil {
		return ""
	}
	return c.executionMode
}

// NumValidators returns the number of validators.
func (c *Config) NumValidators() int {
	if c == nil {
		return 0
	}
	return c.numValidators
}

// NumAccounts returns the number of accounts.
func (c *Config) NumAccounts() int {
	if c == nil {
		return 0
	}
	return c.numAccounts
}

// NoCache returns whether caching is disabled.
func (c *Config) NoCache() bool {
	if c == nil {
		return false
	}
	return c.noCache
}

// CacheTTL returns the cache time-to-live setting.
func (c *Config) CacheTTL() string {
	if c == nil {
		return ""
	}
	return c.cacheTTL
}

// GitHubToken returns the GitHub API token.
func (c *Config) GitHubToken() string {
	if c == nil {
		return ""
	}
	return c.githubToken
}

// DockerImage returns the Docker image reference.
func (c *Config) DockerImage() string {
	if c == nil {
		return ""
	}
	return c.dockerImage
}

// ─────────────────────────────────────────────────────────────────────────────
// Functional Options
// ─────────────────────────────────────────────────────────────────────────────

// WithHomeDir sets the home directory.
func WithHomeDir(dir string) Option {
	return func(c *Config) {
		c.homeDir = dir
	}
}

// WithConfigPath sets the config file path.
func WithConfigPath(path string) Option {
	return func(c *Config) {
		c.configPath = path
	}
}

// WithJSONMode sets JSON output mode.
func WithJSONMode(enabled bool) Option {
	return func(c *Config) {
		c.jsonMode = enabled
	}
}

// WithNoColor sets color output disable flag.
func WithNoColor(disabled bool) Option {
	return func(c *Config) {
		c.noColor = disabled
	}
}

// WithVerbose sets verbose output mode.
func WithVerbose(enabled bool) Option {
	return func(c *Config) {
		c.verbose = enabled
	}
}

// WithChainID sets the chain identifier.
func WithChainID(id string) Option {
	return func(c *Config) {
		c.chainID = id
	}
}

// WithNetworkVersion sets the network version.
func WithNetworkVersion(version string) Option {
	return func(c *Config) {
		c.networkVersion = version
	}
}

// WithBlockchainNetwork sets the blockchain network module name.
func WithBlockchainNetwork(network string) Option {
	return func(c *Config) {
		c.blockchainNetwork = network
	}
}

// WithNetworkName sets the network source name.
func WithNetworkName(name string) Option {
	return func(c *Config) {
		c.networkName = name
	}
}

// WithExecutionMode sets the execution mode.
func WithExecutionMode(mode types.ExecutionMode) Option {
	return func(c *Config) {
		c.executionMode = mode
	}
}

// WithNumValidators sets the number of validators.
func WithNumValidators(n int) Option {
	return func(c *Config) {
		c.numValidators = n
	}
}

// WithNumAccounts sets the number of accounts.
func WithNumAccounts(n int) Option {
	return func(c *Config) {
		c.numAccounts = n
	}
}

// WithNoCache sets the cache disable flag.
func WithNoCache(disabled bool) Option {
	return func(c *Config) {
		c.noCache = disabled
	}
}

// WithCacheTTL sets the cache TTL.
func WithCacheTTL(ttl string) Option {
	return func(c *Config) {
		c.cacheTTL = ttl
	}
}

// WithGitHubToken sets the GitHub API token.
func WithGitHubToken(token string) Option {
	return func(c *Config) {
		c.githubToken = token
	}
}

// WithDockerImage sets the Docker image reference.
func WithDockerImage(image string) Option {
	return func(c *Config) {
		c.dockerImage = image
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Context-level Accessors (shortcuts for common operations)
// ─────────────────────────────────────────────────────────────────────────────

// ChainIDFromContext retrieves the ChainID directly from context.
// Returns empty string if not found.
func ChainIDFromContext(ctx context.Context) string {
	cfg := FromContext(ctx)
	if cfg == nil {
		return ""
	}
	return cfg.chainID
}

// HomeDirFromContext retrieves the HomeDir directly from context.
// Returns empty string if not found.
func HomeDirFromContext(ctx context.Context) string {
	cfg := FromContext(ctx)
	if cfg == nil {
		return ""
	}
	return cfg.homeDir
}

// ExecutionModeFromContext retrieves the ExecutionMode directly from context.
// Returns empty string if not found.
func ExecutionModeFromContext(ctx context.Context) types.ExecutionMode {
	cfg := FromContext(ctx)
	if cfg == nil {
		return ""
	}
	return cfg.executionMode
}

// VerboseFromContext retrieves the Verbose flag directly from context.
// Returns false if not found.
func VerboseFromContext(ctx context.Context) bool {
	cfg := FromContext(ctx)
	if cfg == nil {
		return false
	}
	return cfg.verbose
}

// JSONModeFromContext retrieves the JSONMode flag directly from context.
// Returns false if not found.
func JSONModeFromContext(ctx context.Context) bool {
	cfg := FromContext(ctx)
	if cfg == nil {
		return false
	}
	return cfg.jsonMode
}

// NoColorFromContext retrieves the NoColor flag directly from context.
// Returns false if not found.
func NoColorFromContext(ctx context.Context) bool {
	cfg := FromContext(ctx)
	if cfg == nil {
		return false
	}
	return cfg.noColor
}
