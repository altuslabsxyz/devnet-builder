// Package ctxhelper provides utilities for use cases to access context-based configuration.
// This package bridges the gap between legacy DTO-based configuration and the new
// context-based configuration pattern in ctxconfig.
//
// Usage in use cases:
//
//	func (uc *SomeUseCase) Execute(ctx context.Context, input dto.SomeInput) (*dto.SomeOutput, error) {
//	    // Get HomeDir from context (preferred) or fall back to DTO
//	    homeDir := ctxhelper.HomeDir(ctx, input.HomeDir)
//
//	    // Get ExecutionMode from context
//	    mode := ctxhelper.ExecutionMode(ctx)
//
//	    // Continue with business logic...
//	}
package ctxhelper

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
)

// HomeDir returns the home directory from context, falling back to the provided
// fallback value if not found in context or if empty.
//
// Priority: context config > fallback value
func HomeDir(ctx context.Context, fallback string) string {
	if cfg := ctxconfig.FromContext(ctx); cfg != nil {
		if homeDir := cfg.HomeDir(); homeDir != "" {
			return homeDir
		}
	}
	return fallback
}

// HomeDirStrict returns the home directory from context only.
// Returns empty string if not in context (use for validating context setup).
func HomeDirStrict(ctx context.Context) string {
	return ctxconfig.HomeDirFromContext(ctx)
}

// ExecutionMode returns the execution mode from context.
// Returns empty ExecutionMode if not found.
func ExecutionMode(ctx context.Context) types.ExecutionMode {
	return ctxconfig.ExecutionModeFromContext(ctx)
}

// ExecutionModeWithFallback returns execution mode from context, falling back
// to the provided value if not found.
func ExecutionModeWithFallback(ctx context.Context, fallback types.ExecutionMode) types.ExecutionMode {
	if mode := ExecutionMode(ctx); mode != "" {
		return mode
	}
	return fallback
}

// Verbose returns verbose flag from context.
func Verbose(ctx context.Context) bool {
	return ctxconfig.VerboseFromContext(ctx)
}

// JSONMode returns JSON output mode flag from context.
func JSONMode(ctx context.Context) bool {
	return ctxconfig.JSONModeFromContext(ctx)
}

// NoColor returns no-color flag from context.
func NoColor(ctx context.Context) bool {
	return ctxconfig.NoColorFromContext(ctx)
}

// ChainID returns chain ID from context.
func ChainID(ctx context.Context) string {
	return ctxconfig.ChainIDFromContext(ctx)
}

// ChainIDWithFallback returns chain ID from context, falling back to provided value.
func ChainIDWithFallback(ctx context.Context, fallback string) string {
	if chainID := ChainID(ctx); chainID != "" {
		return chainID
	}
	return fallback
}

// NetworkVersion returns network version from context.
func NetworkVersion(ctx context.Context) string {
	if cfg := ctxconfig.FromContext(ctx); cfg != nil {
		return cfg.NetworkVersion()
	}
	return ""
}

// BlockchainNetwork returns the blockchain network module name from context.
func BlockchainNetwork(ctx context.Context) string {
	if cfg := ctxconfig.FromContext(ctx); cfg != nil {
		return cfg.BlockchainNetwork()
	}
	return ""
}

// NetworkName returns the network source name (mainnet/testnet) from context.
func NetworkName(ctx context.Context) string {
	if cfg := ctxconfig.FromContext(ctx); cfg != nil {
		return cfg.NetworkName()
	}
	return ""
}

// NumValidators returns the number of validators from context.
func NumValidators(ctx context.Context) int {
	if cfg := ctxconfig.FromContext(ctx); cfg != nil {
		return cfg.NumValidators()
	}
	return 0
}

// NumAccounts returns the number of accounts from context.
func NumAccounts(ctx context.Context) int {
	if cfg := ctxconfig.FromContext(ctx); cfg != nil {
		return cfg.NumAccounts()
	}
	return 0
}

// Config returns the full config from context.
// Returns nil if no config in context.
func Config(ctx context.Context) *ctxconfig.Config {
	return ctxconfig.FromContext(ctx)
}

// MustConfig returns the full config from context.
// Panics if no config in context.
func MustConfig(ctx context.Context) *ctxconfig.Config {
	return ctxconfig.MustFromContext(ctx)
}

// HasConfig returns true if context has config attached.
func HasConfig(ctx context.Context) bool {
	return ctxconfig.FromContext(ctx) != nil
}
