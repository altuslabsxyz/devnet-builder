package devnet

import (
	"github.com/b-harvest/devnet-builder/internal/domain/common"
)

// Config holds the configuration for a devnet.
// This is an immutable value object set at creation time.
type Config struct {
	// Network settings
	ChainID       common.ChainID       `json:"chain_id"`
	NetworkSource common.NetworkSource `json:"network_source"`
	NetworkName   string               `json:"network_name"` // "stable", "ault", etc.

	// Execution settings
	ExecutionMode common.ExecutionMode `json:"execution_mode"`
	DockerImage   string               `json:"docker_image,omitempty"`

	// Validator settings
	NumValidators int `json:"num_validators"`
	NumAccounts   int `json:"num_accounts"`
}

// NewConfig creates a new Config with default values.
func NewConfig() Config {
	return Config{
		ChainID:       "stable-devnet-1",
		NetworkSource: common.NetworkSourceMainnet,
		NetworkName:   "stable",
		ExecutionMode: common.ModeDocker,
		NumValidators: 4,
		NumAccounts:   0,
	}
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if !c.ChainID.IsValid() {
		return &ValidationError{
			Field:   "chain_id",
			Message: "must match Cosmos chain ID format",
		}
	}

	if !c.NetworkSource.IsValid() {
		return &ValidationError{
			Field:   "network_source",
			Message: "must be 'mainnet' or 'testnet'",
		}
	}

	if !c.ExecutionMode.IsValid() {
		return &ValidationError{
			Field:   "execution_mode",
			Message: "must be 'docker' or 'local'",
		}
	}

	if c.NumValidators < 1 || c.NumValidators > 4 {
		return &ValidationError{
			Field:   "num_validators",
			Message: "must be between 1 and 4",
		}
	}

	if c.NumAccounts < 0 {
		return &ValidationError{
			Field:   "num_accounts",
			Message: "must be non-negative",
		}
	}

	return nil
}

// WithChainID returns a new Config with the specified chain ID.
func (c Config) WithChainID(id common.ChainID) Config {
	c.ChainID = id
	return c
}

// WithNetworkSource returns a new Config with the specified network source.
func (c Config) WithNetworkSource(source common.NetworkSource) Config {
	c.NetworkSource = source
	return c
}

// WithNetworkName returns a new Config with the specified network name.
func (c Config) WithNetworkName(name string) Config {
	c.NetworkName = name
	return c
}

// WithExecutionMode returns a new Config with the specified execution mode.
func (c Config) WithExecutionMode(mode common.ExecutionMode) Config {
	c.ExecutionMode = mode
	return c
}

// WithDockerImage returns a new Config with the specified docker image.
func (c Config) WithDockerImage(image string) Config {
	c.DockerImage = image
	return c
}

// WithNumValidators returns a new Config with the specified number of validators.
func (c Config) WithNumValidators(n int) Config {
	c.NumValidators = n
	return c
}

// WithNumAccounts returns a new Config with the specified number of accounts.
func (c Config) WithNumAccounts(n int) Config {
	c.NumAccounts = n
	return c
}
