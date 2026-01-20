package config

import (
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/types"
)

// Validate validates the EffectiveConfig values against allowed ranges and types.
func (c *EffectiveConfig) Validate() error {
	// Validate network source using canonical type validation
	if !types.NetworkSource(c.Network.Value).IsValid() {
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", c.Network.Value)
	}

	// Validate blockchain network (network module selection)
	if c.BlockchainNetwork.Value != "" {
		if !network.Has(c.BlockchainNetwork.Value) {
			return fmt.Errorf("invalid blockchain_network: %s (available: %v)", c.BlockchainNetwork.Value, network.List())
		}
	}

	// Validate validators
	if c.Validators.Value < 1 || c.Validators.Value > 4 {
		return fmt.Errorf("invalid validators: %d (must be 1-4)", c.Validators.Value)
	}

	// Validate mode using canonical type validation
	if !types.ExecutionMode(c.Mode.Value).IsValid() {
		return fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", c.Mode.Value)
	}

	// Validate accounts
	if c.Accounts.Value < 0 || c.Accounts.Value > 100 {
		return fmt.Errorf("invalid accounts: %d (must be 0-100)", c.Accounts.Value)
	}

	return nil
}

// ValidateFileConfig validates the FileConfig values before merging.
// This is called when loading the config file to provide early error messages.
func ValidateFileConfig(cfg *FileConfig) error {
	if cfg == nil {
		return nil
	}

	// Validate network source if set using canonical type validation
	if cfg.Network != nil {
		if !types.NetworkSource(*cfg.Network).IsValid() {
			return fmt.Errorf("invalid network in config file: %s (must be 'mainnet' or 'testnet')", *cfg.Network)
		}
	}

	// Note: BlockchainNetwork validation is deferred until modules are registered.
	// This allows config loading to happen before network module init() calls.

	// Validate validators if set
	if cfg.Validators != nil {
		if *cfg.Validators < 1 || *cfg.Validators > 4 {
			return fmt.Errorf("invalid validators in config file: %d (must be 1-4)", *cfg.Validators)
		}
	}

	// Validate mode if set using canonical type validation
	if cfg.ExecutionMode != nil {
		if !types.ExecutionMode(*cfg.ExecutionMode).IsValid() {
			return fmt.Errorf("invalid mode in config file: %s (must be 'docker' or 'local')", *cfg.ExecutionMode)
		}
	}

	// Validate accounts if set
	if cfg.Accounts != nil {
		if *cfg.Accounts < 0 || *cfg.Accounts > 100 {
			return fmt.Errorf("invalid accounts in config file: %d (must be 0-100)", *cfg.Accounts)
		}
	}

	return nil
}

// ValidateBlockchainNetwork validates the blockchain network selection against registered modules.
// This should be called after network modules have been registered (after init phase).
func ValidateBlockchainNetwork(networkName string) error {
	if networkName == "" {
		return nil // Empty means use default
	}
	if !network.Has(networkName) {
		return fmt.Errorf("invalid blockchain_network: %s (available: %v)", networkName, network.List())
	}
	return nil
}
