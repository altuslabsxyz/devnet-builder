package ctxconfig

import (
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/types"
)

// Builder provides a fluent interface for constructing Config from various sources.
type Builder struct {
	cfg *Config
}

// NewBuilder creates a new config builder.
func NewBuilder() *Builder {
	return &Builder{
		cfg: &Config{},
	}
}

// FromFileConfig initializes the builder from a FileConfig.
// Only non-nil values from FileConfig are applied.
func (b *Builder) FromFileConfig(fc *config.FileConfig) *Builder {
	if fc == nil {
		return b
	}

	if fc.Home != nil {
		b.cfg.homeDir = *fc.Home
	}
	if fc.NoColor != nil {
		b.cfg.noColor = *fc.NoColor
	}
	if fc.Verbose != nil {
		b.cfg.verbose = *fc.Verbose
	}
	if fc.JSON != nil {
		b.cfg.jsonMode = *fc.JSON
	}
	if fc.Network != nil {
		b.cfg.networkName = *fc.Network
	}
	if fc.BlockchainNetwork != nil {
		b.cfg.blockchainNetwork = *fc.BlockchainNetwork
	}
	if fc.Validators != nil {
		b.cfg.numValidators = *fc.Validators
	}
	if fc.ExecutionMode != nil {
		b.cfg.executionMode = *fc.ExecutionMode
	}
	if fc.NetworkVersion != nil {
		b.cfg.networkVersion = *fc.NetworkVersion
	}
	if fc.NoCache != nil {
		b.cfg.noCache = *fc.NoCache
	}
	if fc.Accounts != nil {
		b.cfg.numAccounts = *fc.Accounts
	}
	if fc.GitHubToken != nil {
		b.cfg.githubToken = *fc.GitHubToken
	}
	if fc.CacheTTL != nil {
		b.cfg.cacheTTL = *fc.CacheTTL
	}

	return b
}

// WithHomeDir sets the home directory (overrides FileConfig value).
func (b *Builder) WithHomeDir(dir string) *Builder {
	b.cfg.homeDir = dir
	return b
}

// WithConfigPath sets the config file path.
func (b *Builder) WithConfigPath(path string) *Builder {
	b.cfg.configPath = path
	return b
}

// WithJSONMode sets JSON output mode.
func (b *Builder) WithJSONMode(enabled bool) *Builder {
	b.cfg.jsonMode = enabled
	return b
}

// WithNoColor sets color output disable flag.
func (b *Builder) WithNoColor(disabled bool) *Builder {
	b.cfg.noColor = disabled
	return b
}

// WithVerbose sets verbose output mode.
func (b *Builder) WithVerbose(enabled bool) *Builder {
	b.cfg.verbose = enabled
	return b
}

// WithChainID sets the chain identifier.
func (b *Builder) WithChainID(id string) *Builder {
	b.cfg.chainID = id
	return b
}

// WithNetworkVersion sets the network version.
func (b *Builder) WithNetworkVersion(version string) *Builder {
	b.cfg.networkVersion = version
	return b
}

// WithBlockchainNetwork sets the blockchain network module name.
func (b *Builder) WithBlockchainNetwork(network string) *Builder {
	b.cfg.blockchainNetwork = network
	return b
}

// WithNetworkName sets the network source name.
func (b *Builder) WithNetworkName(name string) *Builder {
	b.cfg.networkName = name
	return b
}

// WithExecutionMode sets the execution mode.
func (b *Builder) WithExecutionMode(mode string) *Builder {
	b.cfg.executionMode = parseExecutionMode(mode)
	return b
}

// WithExecutionModeType sets the execution mode from a types.ExecutionMode value.
func (b *Builder) WithExecutionModeType(mode types.ExecutionMode) *Builder {
	b.cfg.executionMode = mode
	return b
}

// WithNumValidators sets the number of validators.
func (b *Builder) WithNumValidators(n int) *Builder {
	b.cfg.numValidators = n
	return b
}

// WithNumAccounts sets the number of accounts.
func (b *Builder) WithNumAccounts(n int) *Builder {
	b.cfg.numAccounts = n
	return b
}

// WithNoCache sets the cache disable flag.
func (b *Builder) WithNoCache(disabled bool) *Builder {
	b.cfg.noCache = disabled
	return b
}

// WithCacheTTL sets the cache TTL.
func (b *Builder) WithCacheTTL(ttl string) *Builder {
	b.cfg.cacheTTL = ttl
	return b
}

// WithGitHubToken sets the GitHub API token.
func (b *Builder) WithGitHubToken(token string) *Builder {
	b.cfg.githubToken = token
	return b
}

// WithDockerImage sets the Docker image reference.
func (b *Builder) WithDockerImage(image string) *Builder {
	b.cfg.dockerImage = image
	return b
}

// Build returns the constructed Config.
func (b *Builder) Build() *Config {
	return b.cfg
}

// parseExecutionMode converts string to ExecutionMode.
func parseExecutionMode(mode string) types.ExecutionMode {
	switch mode {
	case "docker":
		return types.ExecutionModeDocker
	case "local":
		return types.ExecutionModeLocal
	default:
		return types.ExecutionMode(mode)
	}
}
