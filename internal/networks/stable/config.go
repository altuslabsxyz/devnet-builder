package stable

import (
	"github.com/stablelabs/stable-devnet/internal/network"
)

// Network identity constants
const (
	// NetworkName is the unique identifier for this network.
	NetworkName = "stable"

	// DefaultChainID is the default chain ID for Stable devnet.
	DefaultChainID = "stabledevnet_2200-1"

	// DefaultBech32Prefix is the bech32 address prefix for Stable.
	DefaultBech32Prefix = "stable"

	// DefaultBaseDenom is the base token denomination.
	DefaultBaseDenom = "ustable"

	// DefaultDisplayDenom is the human-readable denomination.
	DefaultDisplayDenom = "STABLE"

	// DefaultDockerImage is the default Docker image for Stable.
	DefaultDockerImage = "ghcr.io/stablelabs/stable"
)

// DefaultPortConfig returns the default port configuration for Stable.
func DefaultPortConfig() network.PortConfig {
	return network.PortConfig{
		RPC:     26657,
		P2P:     26656,
		GRPC:    9090,
		GRPCWeb: 9091,
		API:     1317,
		EVMRPC:  8545,
		EVMWS:   8546,
	}
}

// SnapshotURLs contains snapshot download URLs for different networks.
var SnapshotURLs = map[string]string{
	"mainnet": "https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst",
	"testnet": "https://stable-testnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst",
}

// ChainIDForNetwork returns the chain ID for a given network source.
func ChainIDForNetwork(networkSource string) string {
	switch networkSource {
	case "mainnet":
		return "stable_988-1"
	case "testnet":
		return "stabletestnet_2201-1"
	default:
		return DefaultChainID
	}
}
