package config

import "fmt"

// Validate validates the EffectiveConfig values against allowed ranges and types.
func (c *EffectiveConfig) Validate() error {
	// Validate network
	if c.Network.Value != "mainnet" && c.Network.Value != "testnet" {
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", c.Network.Value)
	}

	// Validate validators
	if c.Validators.Value < 1 || c.Validators.Value > 4 {
		return fmt.Errorf("invalid validators: %d (must be 1-4)", c.Validators.Value)
	}

	// Validate mode
	if c.Mode.Value != "docker" && c.Mode.Value != "local" {
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

	// Validate network if set
	if cfg.Network != nil {
		if *cfg.Network != "mainnet" && *cfg.Network != "testnet" {
			return fmt.Errorf("invalid network in config file: %s (must be 'mainnet' or 'testnet')", *cfg.Network)
		}
	}

	// Validate validators if set
	if cfg.Validators != nil {
		if *cfg.Validators < 1 || *cfg.Validators > 4 {
			return fmt.Errorf("invalid validators in config file: %d (must be 1-4)", *cfg.Validators)
		}
	}

	// Validate mode if set
	if cfg.Mode != nil {
		if *cfg.Mode != "docker" && *cfg.Mode != "local" {
			return fmt.Errorf("invalid mode in config file: %s (must be 'docker' or 'local')", *cfg.Mode)
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
