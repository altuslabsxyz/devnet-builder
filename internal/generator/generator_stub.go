//go:build !private

package generator

import (
	"errors"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ErrPrivateBuildRequired is returned when private build tag is required.
var ErrPrivateBuildRequired = errors.New("this feature requires building with '-tags=private' and access to stablelabs private repositories")

// ValidatorInfo contains information about a generated validator.
type ValidatorInfo struct {
	Moniker         string
	OperatorAddress sdk.ValAddress
	AccountAddress  sdk.AccAddress
	Tokens          math.Int
}

// AccountInfo contains information about a generated account.
type AccountInfo struct {
	Name    string
	Address sdk.AccAddress
}

// DevnetGenerator is a stub implementation for non-private builds.
type DevnetGenerator struct {
	config     *Config
	validators []ValidatorInfo
	accounts   []AccountInfo
}

// NewDevnetGenerator creates a new DevnetGenerator.
// This is a stub implementation that returns an error when Build is called.
func NewDevnetGenerator(config *Config, logger log.Logger) *DevnetGenerator {
	return &DevnetGenerator{
		config:     config,
		validators: make([]ValidatorInfo, 0),
		accounts:   make([]AccountInfo, 0),
	}
}

// Build is a stub implementation that returns an error.
func (g *DevnetGenerator) Build(genesisFile string) error {
	return ErrPrivateBuildRequired
}

// GetValidators returns the generated validators info.
func (g *DevnetGenerator) GetValidators() []ValidatorInfo {
	return g.validators
}

// GetAccounts returns the generated accounts info.
func (g *DevnetGenerator) GetAccounts() []AccountInfo {
	return g.accounts
}
