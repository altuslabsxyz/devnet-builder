// Package types provides shared types and constants for devnet-builder.
package types

// Context keys for storing values in context.Context.
//
// Deprecated: Use the ctxconfig package for context-based configuration instead.
// The ctxconfig package provides type-safe context keys and accessor functions.
//
// Migration example:
//
//	// Old pattern (deprecated):
//	ctx = context.WithValue(ctx, types.ExecutionModeCtxKey, mode)
//	mode := ctx.Value(types.ExecutionModeCtxKey).(types.ExecutionMode)
//
//	// New pattern (recommended):
//	import "github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
//	cfg := ctxconfig.New(ctxconfig.WithExecutionMode(mode))
//	ctx = ctxconfig.WithConfig(ctx, cfg)
//	mode := ctxconfig.FromContext(ctx).ExecutionMode()
const (
	// ExecutionModeCtxKey is the context key for execution mode.
	//
	// Deprecated: Use ctxconfig.WithExecutionMode and ctxconfig.FromContext instead.
	ExecutionModeCtxKey = "execution_mode"
)

// ContextKey is a type-safe key for context values.
// Using a dedicated type prevents accidental key collisions.
type ContextKey string

// Context key constants using type-safe keys.
// These are provided for backward compatibility during migration.
const (
	// CtxKeyExecutionMode is the type-safe context key for execution mode.
	//
	// Deprecated: Use ctxconfig package instead.
	CtxKeyExecutionMode ContextKey = "execution_mode"

	// CtxKeyHomeDir is the type-safe context key for home directory.
	//
	// Deprecated: Use ctxconfig package instead.
	CtxKeyHomeDir ContextKey = "home_dir"

	// CtxKeyChainID is the type-safe context key for chain ID.
	//
	// Deprecated: Use ctxconfig package instead.
	CtxKeyChainID ContextKey = "chain_id"
)
