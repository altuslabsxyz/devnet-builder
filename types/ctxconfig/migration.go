package ctxconfig

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
)

// Migration utilities for transitioning from globals to context-based config.
// These helpers enable incremental migration without breaking existing code.

// FromGlobals creates a Config from the current shared package global values.
// Use this when you need a Config but don't have access to context yet.
//
// Deprecated: This is a migration helper. New code should pass context through
// the call chain and use FromContext instead.
//
// Example:
//
//	// Old pattern (using globals):
//	homeDir := shared.GetHomeDir()
//
//	// Migration pattern (when context not available):
//	cfg := ctxconfig.FromGlobals()
//	homeDir := cfg.HomeDir()
//
//	// New pattern (recommended):
//	cfg := ctxconfig.FromContext(ctx)
//	homeDir := cfg.HomeDir()
func FromGlobals() *Config {
	fileCfg := shared.GetLoadedFileConfig()

	builder := NewBuilder().
		WithHomeDir(shared.GetHomeDir()).
		WithConfigPath(shared.GetConfigPath()).
		WithJSONMode(shared.GetJSONMode()).
		WithNoColor(shared.GetNoColor()).
		WithVerbose(shared.GetVerbose())

	// Apply FileConfig values if available
	if fileCfg != nil {
		builder = builder.FromFileConfig(fileCfg)
		// Re-apply CLI overrides (they take priority over FileConfig)
		builder = builder.
			WithHomeDir(shared.GetHomeDir()).
			WithJSONMode(shared.GetJSONMode()).
			WithNoColor(shared.GetNoColor()).
			WithVerbose(shared.GetVerbose())
	}

	return builder.Build()
}

// SyncToGlobals writes the Config values back to the shared package globals.
// Use this during migration when updating code that still reads from globals.
//
// Deprecated: This is a migration helper. Once all code is migrated to use
// context-based config, this function should no longer be needed.
func SyncToGlobals(cfg *Config) {
	if cfg == nil {
		return
	}

	shared.SetHomeDir(cfg.HomeDir())
	shared.SetConfigPath(cfg.ConfigPath())
	shared.SetJSONMode(cfg.JSONMode())
	shared.SetNoColor(cfg.NoColor())
	shared.SetVerbose(cfg.Verbose())
}

// EnsureInContext checks if config exists in context, and if not, creates one
// from globals. This is a convenience helper for migrating existing code.
//
// Deprecated: This is a migration helper. New code should ensure context has
// config set at the entry point (e.g., in root.go).
func EnsureInContext(ctx context.Context) (context.Context, *Config) {
	cfg := FromContext(ctx)
	if cfg != nil {
		return ctx, cfg
	}

	// Config not in context, create from globals
	cfg = FromGlobals()
	return WithConfig(ctx, cfg), cfg
}

// WithChainIDInContext is a convenience function to add or update ChainID in context.
// It retrieves the existing config (or creates from globals), clones it with the
// new ChainID, and returns a new context.
//
// This is useful when ChainID is determined at runtime (e.g., from metadata or flags).
func WithChainIDInContext(ctx context.Context, chainID string) context.Context {
	cfg := FromContext(ctx)
	if cfg == nil {
		cfg = FromGlobals()
	}

	newCfg := cfg.Clone(WithChainID(chainID))
	return WithConfig(ctx, newCfg)
}

// WithNetworkVersionInContext is a convenience function to add or update NetworkVersion
// in context. Similar to WithChainIDInContext.
func WithNetworkVersionInContext(ctx context.Context, version string) context.Context {
	cfg := FromContext(ctx)
	if cfg == nil {
		cfg = FromGlobals()
	}

	newCfg := cfg.Clone(WithNetworkVersion(version))
	return WithConfig(ctx, newCfg)
}

// WithBlockchainNetworkInContext is a convenience function to add or update
// BlockchainNetwork in context. Similar to WithChainIDInContext.
func WithBlockchainNetworkInContext(ctx context.Context, network string) context.Context {
	cfg := FromContext(ctx)
	if cfg == nil {
		cfg = FromGlobals()
	}

	newCfg := cfg.Clone(WithBlockchainNetwork(network))
	return WithConfig(ctx, newCfg)
}

// UpdateInContext is a generic helper to update config in context with new options.
// It retrieves existing config (or creates from globals), applies the options,
// and returns a new context with the updated config.
func UpdateInContext(ctx context.Context, opts ...Option) context.Context {
	cfg := FromContext(ctx)
	if cfg == nil {
		cfg = FromGlobals()
	}

	newCfg := cfg.Clone(opts...)
	return WithConfig(ctx, newCfg)
}
