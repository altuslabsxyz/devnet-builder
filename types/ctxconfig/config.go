// Package ctxconfig provides context-based configuration for devnet-builder.
package ctxconfig

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/types"
)

type configKey struct{}

// Config holds all configuration values passed through context.
type Config struct {
	homeDir           string
	configPath        string
	jsonMode          bool
	noColor           bool
	verbose           bool
	chainID           string
	networkVersion    string
	blockchainNetwork string
	networkName       string
	executionMode     types.ExecutionMode
	numValidators     int
	numAccounts       int
	noCache           bool
	cacheTTL          string
	githubToken       string
	dockerImage       string
}

// Option is a functional option for Config.
type Option func(*Config)

// New creates a new Config with the given options.
func New(opts ...Option) *Config {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// Clone creates a copy with optional modifications.
func (c *Config) Clone(opts ...Option) *Config {
	if c == nil {
		return New(opts...)
	}
	clone := *c
	for _, opt := range opts {
		opt(&clone)
	}
	return &clone
}

// ─────────────────────────────────────────────────────────────────────────────
// Context Operations
// ─────────────────────────────────────────────────────────────────────────────

// WithConfig attaches config to context.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

// FromContext retrieves config from context (nil if not found).
func FromContext(ctx context.Context) *Config {
	if ctx == nil {
		return nil
	}
	cfg, _ := ctx.Value(configKey{}).(*Config)
	return cfg
}

// MustFromContext retrieves config or panics.
func MustFromContext(ctx context.Context) *Config {
	cfg := FromContext(ctx)
	if cfg == nil {
		panic("ctxconfig: no config in context")
	}
	return cfg
}

// ─────────────────────────────────────────────────────────────────────────────
// Accessors
// ─────────────────────────────────────────────────────────────────────────────

func (c *Config) HomeDir() string {
	if c == nil {
		return ""
	}
	return c.homeDir
}

func (c *Config) ConfigPath() string {
	if c == nil {
		return ""
	}
	return c.configPath
}

func (c *Config) JSONMode() bool {
	if c == nil {
		return false
	}
	return c.jsonMode
}

func (c *Config) NoColor() bool {
	if c == nil {
		return false
	}
	return c.noColor
}

func (c *Config) Verbose() bool {
	if c == nil {
		return false
	}
	return c.verbose
}

func (c *Config) ChainID() string {
	if c == nil {
		return ""
	}
	return c.chainID
}

func (c *Config) NetworkVersion() string {
	if c == nil {
		return ""
	}
	return c.networkVersion
}

func (c *Config) BlockchainNetwork() string {
	if c == nil {
		return ""
	}
	return c.blockchainNetwork
}

func (c *Config) NetworkName() string {
	if c == nil {
		return ""
	}
	return c.networkName
}

func (c *Config) ExecutionMode() types.ExecutionMode {
	if c == nil {
		return ""
	}
	return c.executionMode
}

func (c *Config) NumValidators() int {
	if c == nil {
		return 0
	}
	return c.numValidators
}

func (c *Config) NumAccounts() int {
	if c == nil {
		return 0
	}
	return c.numAccounts
}

func (c *Config) NoCache() bool {
	if c == nil {
		return false
	}
	return c.noCache
}

func (c *Config) CacheTTL() string {
	if c == nil {
		return ""
	}
	return c.cacheTTL
}

func (c *Config) GitHubToken() string {
	if c == nil {
		return ""
	}
	return c.githubToken
}

func (c *Config) DockerImage() string {
	if c == nil {
		return ""
	}
	return c.dockerImage
}

// ─────────────────────────────────────────────────────────────────────────────
// Options
// ─────────────────────────────────────────────────────────────────────────────

func WithHomeDir(v string) Option                    { return func(c *Config) { c.homeDir = v } }
func WithConfigPath(v string) Option                 { return func(c *Config) { c.configPath = v } }
func WithJSONMode(v bool) Option                     { return func(c *Config) { c.jsonMode = v } }
func WithNoColor(v bool) Option                      { return func(c *Config) { c.noColor = v } }
func WithVerbose(v bool) Option                      { return func(c *Config) { c.verbose = v } }
func WithChainID(v string) Option                    { return func(c *Config) { c.chainID = v } }
func WithNetworkVersion(v string) Option             { return func(c *Config) { c.networkVersion = v } }
func WithBlockchainNetwork(v string) Option          { return func(c *Config) { c.blockchainNetwork = v } }
func WithNetworkName(v string) Option                { return func(c *Config) { c.networkName = v } }
func WithExecutionMode(v types.ExecutionMode) Option { return func(c *Config) { c.executionMode = v } }
func WithNumValidators(v int) Option                 { return func(c *Config) { c.numValidators = v } }
func WithNumAccounts(v int) Option                   { return func(c *Config) { c.numAccounts = v } }
func WithNoCache(v bool) Option                      { return func(c *Config) { c.noCache = v } }
func WithCacheTTL(v string) Option                   { return func(c *Config) { c.cacheTTL = v } }
func WithGitHubToken(v string) Option                { return func(c *Config) { c.githubToken = v } }
func WithDockerImage(v string) Option                { return func(c *Config) { c.dockerImage = v } }

// FromFileConfig applies FileConfig values (only non-nil values).
func FromFileConfig(fc *config.FileConfig) Option {
	return func(c *Config) {
		if fc == nil {
			return
		}
		if fc.Home != nil {
			c.homeDir = *fc.Home
		}
		if fc.NoColor != nil {
			c.noColor = *fc.NoColor
		}
		if fc.Verbose != nil {
			c.verbose = *fc.Verbose
		}
		if fc.JSON != nil {
			c.jsonMode = *fc.JSON
		}
		if fc.Network != nil {
			c.networkName = *fc.Network
		}
		if fc.BlockchainNetwork != nil {
			c.blockchainNetwork = *fc.BlockchainNetwork
		}
		if fc.Validators != nil {
			c.numValidators = *fc.Validators
		}
		if fc.ExecutionMode != nil {
			c.executionMode = *fc.ExecutionMode
		}
		if fc.NetworkVersion != nil {
			c.networkVersion = *fc.NetworkVersion
		}
		if fc.NoCache != nil {
			c.noCache = *fc.NoCache
		}
		if fc.Accounts != nil {
			c.numAccounts = *fc.Accounts
		}
		if fc.GitHubToken != nil {
			c.githubToken = *fc.GitHubToken
		}
		if fc.CacheTTL != nil {
			c.cacheTTL = *fc.CacheTTL
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Context Helpers with Fallback (for use cases)
// ─────────────────────────────────────────────────────────────────────────────

// HomeDir returns homeDir from context, or fallback if not found/empty.
func HomeDir(ctx context.Context, fallback string) string {
	if cfg := FromContext(ctx); cfg != nil && cfg.homeDir != "" {
		return cfg.homeDir
	}
	return fallback
}

// Verbose returns verbose flag from context.
func Verbose(ctx context.Context) bool {
	if cfg := FromContext(ctx); cfg != nil {
		return cfg.verbose
	}
	return false
}

// JSONMode returns jsonMode flag from context.
func JSONMode(ctx context.Context) bool {
	if cfg := FromContext(ctx); cfg != nil {
		return cfg.jsonMode
	}
	return false
}

// ExecutionMode returns executionMode from context.
func ExecutionMode(ctx context.Context) types.ExecutionMode {
	if cfg := FromContext(ctx); cfg != nil {
		return cfg.executionMode
	}
	return ""
}
