package network

import "github.com/altuslabsxyz/devnet-builder/types"

// PortConfig is an alias to the canonical types.PortConfig.
//
// Deprecated: Use types.PortConfig directly.
type PortConfig = types.PortConfig

// DefaultPortConfig returns a standard Cosmos SDK port configuration.
//
// Deprecated: Use types.DefaultPortConfig() directly.
func DefaultPortConfig() PortConfig {
	return types.DefaultPortConfig()
}
