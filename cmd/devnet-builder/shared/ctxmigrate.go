// Package shared provides shared state and utilities for devnet-builder commands.
// This file contains migration utilities for transitioning from globals to
// context-based configuration (ctxconfig).
package shared

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
)

// Migration utilities for transitioning from globals to context-based config.
// These helpers enable incremental migration without breaking existing code.

// ConfigFromGlobals creates a ctxconfig.Config from the current global values.
// Use this when you need a Config but don't have access to context yet.
//
// Deprecated: This is a migration helper. New code should pass context through
// the call chain and use ctxconfig.FromContext instead.
//
// Example:
//
//	// Old pattern (using globals):
//	homeDir := shared.GetHomeDir()
//
//	// Migration pattern (when context not available):
//	cfg := shared.ConfigFromGlobals()
//	homeDir := cfg.HomeDir()
//
//	// New pattern (recommended):
//	cfg := ctxconfig.FromContext(ctx)
//	homeDir := cfg.HomeDir()
func ConfigFromGlobals() *ctxconfig.Config {
	fileCfg := GetLoadedFileConfig()

	builder := ctxconfig.NewBuilder().
		WithHomeDir(GetHomeDir()).
		WithConfigPath(GetConfigPath()).
		WithJSONMode(GetJSONMode()).
		WithNoColor(GetNoColor()).
		WithVerbose(GetVerbose())

	// Apply FileConfig values if available
	if fileCfg != nil {
		builder = builder.FromFileConfig(fileCfg)
		// Re-apply CLI overrides (they take priority over FileConfig)
		builder = builder.
			WithHomeDir(GetHomeDir()).
			WithJSONMode(GetJSONMode()).
			WithNoColor(GetNoColor()).
			WithVerbose(GetVerbose())
	}

	return builder.Build()
}

// SyncConfigToGlobals writes the Config values back to the global variables.
// Use this during migration when updating code that still reads from globals.
//
// Deprecated: This is a migration helper. Once all code is migrated to use
// context-based config, this function should no longer be needed.
func SyncConfigToGlobals(cfg *ctxconfig.Config) {
	if cfg == nil {
		return
	}

	SetHomeDir(cfg.HomeDir())
	SetConfigPath(cfg.ConfigPath())
	SetJSONMode(cfg.JSONMode())
	SetNoColor(cfg.NoColor())
	SetVerbose(cfg.Verbose())
}

// EnsureConfigInContext checks if config exists in context, and if not, creates one
// from globals. This is a convenience helper for migrating existing code.
//
// Deprecated: This is a migration helper. New code should ensure context has
// config set at the entry point (e.g., in root.go).
func EnsureConfigInContext(ctx context.Context) (context.Context, *ctxconfig.Config) {
	cfg := ctxconfig.FromContext(ctx)
	if cfg != nil {
		return ctx, cfg
	}

	// Config not in context, create from globals
	cfg = ConfigFromGlobals()
	return ctxconfig.WithConfig(ctx, cfg), cfg
}

// WithChainIDInContext is a convenience function to add or update ChainID in context.
// It retrieves the existing config (or creates from globals), clones it with the
// new ChainID, and returns a new context.
//
// This is useful when ChainID is determined at runtime (e.g., from metadata or flags).
func WithChainIDInContext(ctx context.Context, chainID string) context.Context {
	cfg := ctxconfig.FromContext(ctx)
	if cfg == nil {
		cfg = ConfigFromGlobals()
	}

	newCfg := cfg.Clone(ctxconfig.WithChainID(chainID))
	return ctxconfig.WithConfig(ctx, newCfg)
}

// WithNetworkVersionInContext is a convenience function to add or update NetworkVersion
// in context. Similar to WithChainIDInContext.
func WithNetworkVersionInContext(ctx context.Context, version string) context.Context {
	cfg := ctxconfig.FromContext(ctx)
	if cfg == nil {
		cfg = ConfigFromGlobals()
	}

	newCfg := cfg.Clone(ctxconfig.WithNetworkVersion(version))
	return ctxconfig.WithConfig(ctx, newCfg)
}

// WithBlockchainNetworkInContext is a convenience function to add or update
// BlockchainNetwork in context. Similar to WithChainIDInContext.
func WithBlockchainNetworkInContext(ctx context.Context, network string) context.Context {
	cfg := ctxconfig.FromContext(ctx)
	if cfg == nil {
		cfg = ConfigFromGlobals()
	}

	newCfg := cfg.Clone(ctxconfig.WithBlockchainNetwork(network))
	return ctxconfig.WithConfig(ctx, newCfg)
}

// UpdateConfigInContext is a generic helper to update config in context with new options.
// It retrieves existing config (or creates from globals), applies the options,
// and returns a new context with the updated config.
func UpdateConfigInContext(ctx context.Context, opts ...ctxconfig.Option) context.Context {
	cfg := ctxconfig.FromContext(ctx)
	if cfg == nil {
		cfg = ConfigFromGlobals()
	}

	newCfg := cfg.Clone(opts...)
	return ctxconfig.WithConfig(ctx, newCfg)
}
