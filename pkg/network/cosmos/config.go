// pkg/network/cosmos/config.go
package cosmos

import (
	"fmt"
	"sync"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// sdkConfigMu protects concurrent access to the SDK global config.
var sdkConfigMu sync.Mutex

// NewTxConfig creates a new TxConfig for Cosmos SDK v0.50+ transaction encoding.
// It sets up an interface registry with standard types and module interfaces,
// creates a proto codec, and returns a properly configured TxConfig.
func NewTxConfig() client.TxConfig {
	// Create interface registry
	interfaceRegistry := codectypes.NewInterfaceRegistry()

	// Register standard types
	std.RegisterInterfaces(interfaceRegistry)

	// Register module interfaces
	authtypes.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)
	govtypes.RegisterInterfaces(interfaceRegistry)
	stakingtypes.RegisterInterfaces(interfaceRegistry)

	// Create proto codec with the registry
	protoCodec := codec.NewProtoCodec(interfaceRegistry)

	// Create TxConfig with default sign modes using the new options-based API
	txConfig, err := tx.NewTxConfigWithOptions(protoCodec, tx.ConfigOptions{
		EnabledSignModes: tx.DefaultSignModes,
	})
	if err != nil {
		// This should not happen with default options, but handle gracefully
		panic("failed to create tx config: " + err.Error())
	}

	return txConfig
}

// SetupSDKConfig configures the Cosmos SDK with the given bech32 prefix.
// It sets up prefixes for account addresses, validator addresses, and consensus node addresses.
// This function is thread-safe and returns an error if the prefix is empty.
// Note: This function does NOT seal the config to allow reconfiguration during testing.
func SetupSDKConfig(bech32Prefix string) error {
	if bech32Prefix == "" {
		return fmt.Errorf("bech32 prefix cannot be empty")
	}

	sdkConfigMu.Lock()
	defer sdkConfigMu.Unlock()

	config := sdk.GetConfig()

	// Set bech32 prefixes for account addresses
	config.SetBech32PrefixForAccount(
		bech32Prefix,                    // account address prefix
		bech32Prefix+"pub",              // account public key prefix
	)

	// Set bech32 prefixes for validator addresses
	config.SetBech32PrefixForValidator(
		bech32Prefix+"valoper",          // validator operator address prefix
		bech32Prefix+"valoperpub",       // validator operator public key prefix
	)

	// Set bech32 prefixes for consensus node addresses
	config.SetBech32PrefixForConsensusNode(
		bech32Prefix+"valcons",          // consensus node address prefix
		bech32Prefix+"valconspub",       // consensus node public key prefix
	)

	// Note: We intentionally do NOT call config.Seal() to allow reconfiguration
	// This is useful for testing and for applications that need to switch between chains

	return nil
}
