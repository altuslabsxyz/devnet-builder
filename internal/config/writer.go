package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigWriter handles writing configuration to homeDir/config.toml.
type ConfigWriter struct {
	homeDir string
}

// NewConfigWriter creates a new ConfigWriter for the given home directory.
func NewConfigWriter(homeDir string) *ConfigWriter {
	return &ConfigWriter{
		homeDir: homeDir,
	}
}

// Path returns the full path to config.toml in homeDir.
func (w *ConfigWriter) Path() string {
	return filepath.Join(w.homeDir, "config.toml")
}

// Exists returns true if config.toml already exists in homeDir.
func (w *ConfigWriter) Exists() bool {
	_, err := os.Stat(w.Path())
	return err == nil
}

// Write saves the FileConfig to homeDir/config.toml.
// Creates homeDir if it doesn't exist.
func (w *ConfigWriter) Write(cfg *FileConfig) error {
	// Create homeDir if needed
	if err := os.MkdirAll(w.homeDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", w.homeDir, err)
	}

	// Generate TOML content with comments
	content := w.generateTOMLWithComments(cfg)

	// Write to file
	if err := os.WriteFile(w.Path(), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// generateTOMLWithComments creates TOML content with section comments.
func (w *ConfigWriter) generateTOMLWithComments(cfg *FileConfig) string {
	var content string

	content += "# devnet-builder configuration file\n"
	content += "# Priority: default < config.toml < environment < CLI flag\n"
	content += "#\n"
	content += fmt.Sprintf("# Location: %s\n", w.Path())
	content += "# Override with: --config /path/to/config.toml\n"
	content += "\n"

	// =============================================================================
	// Global Settings
	// =============================================================================
	content += "# =============================================================================\n"
	content += "# Global Settings (apply to all commands)\n"
	content += "# =============================================================================\n\n"

	if cfg.Home != nil {
		content += fmt.Sprintf("home = %q\n", *cfg.Home)
	} else {
		content += "# home = \"~/.devnet-builder\"\n"
	}

	if cfg.Verbose != nil && *cfg.Verbose {
		content += "verbose = true\n"
	} else {
		content += "# verbose = false\n"
	}

	if cfg.JSON != nil && *cfg.JSON {
		content += "json = true\n"
	} else {
		content += "# json = false\n"
	}

	if cfg.NoColor != nil && *cfg.NoColor {
		content += "no_color = true\n"
	} else {
		content += "# no_color = false\n"
	}

	content += "\n"

	// =============================================================================
	// Network Settings
	// =============================================================================
	content += "# =============================================================================\n"
	content += "# Network Settings\n"
	content += "# =============================================================================\n\n"

	if cfg.BlockchainNetwork != nil {
		content += fmt.Sprintf("blockchain_network = %q\n", *cfg.BlockchainNetwork)
	} else {
		content += "# blockchain_network = \"stable\"\n"
	}

	if cfg.Network != nil {
		content += fmt.Sprintf("network = %q\n", *cfg.Network)
	} else {
		content += "# network = \"mainnet\"\n"
	}

	if cfg.NetworkVersion != nil {
		content += fmt.Sprintf("network_version = %q\n", *cfg.NetworkVersion)
	} else {
		content += "# network_version = \"latest\"\n"
	}

	content += "\n"

	// =============================================================================
	// Devnet Settings
	// =============================================================================
	content += "# =============================================================================\n"
	content += "# Devnet Settings\n"
	content += "# =============================================================================\n\n"

	if cfg.Validators != nil {
		content += fmt.Sprintf("validators = %d\n", *cfg.Validators)
	} else {
		content += "# validators = 4\n"
	}

	if cfg.Mode != nil {
		content += fmt.Sprintf("mode = %q\n", *cfg.Mode)
	} else {
		content += "# mode = \"docker\"\n"
	}

	if cfg.Accounts != nil && *cfg.Accounts > 0 {
		content += fmt.Sprintf("accounts = %d\n", *cfg.Accounts)
	} else {
		content += "# accounts = 0\n"
	}

	if cfg.NoCache != nil && *cfg.NoCache {
		content += "no_cache = true\n"
	} else {
		content += "# no_cache = false\n"
	}

	content += "\n"

	// =============================================================================
	// GitHub Settings
	// =============================================================================
	content += "# =============================================================================\n"
	content += "# GitHub Settings\n"
	content += "# =============================================================================\n\n"

	if cfg.GitHubToken != nil {
		content += fmt.Sprintf("github_token = %q\n", *cfg.GitHubToken)
	} else {
		content += "# github_token = \"\"\n"
	}

	if cfg.CacheTTL != nil {
		content += fmt.Sprintf("cache_ttl = %q\n", *cfg.CacheTTL)
	} else {
		content += "# cache_ttl = \"1h\"\n"
	}

	return content
}
