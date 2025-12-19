package ault

import (
	"github.com/stablelabs/stable-devnet/internal/network"
)

// Network identity constants
const (
	// NetworkName is the unique identifier for this network.
	NetworkName = "ault"

	// DefaultChainID is the default chain ID for Ault devnet.
	DefaultChainID = "aultdevnet_900-1"

	// DefaultBech32Prefix is the bech32 address prefix for Ault.
	DefaultBech32Prefix = "ault"

	// DefaultBaseDenom is the base token denomination (atto).
	DefaultBaseDenom = "aault"

	// DefaultDisplayDenom is the human-readable denomination.
	DefaultDisplayDenom = "AULT"

	// DefaultDockerImage is the default Docker image for Ault.
	DefaultDockerImage = "ghcr.io/bharvest/ault"
)

// DefaultPortConfig returns the default port configuration for Ault.
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
// Note: These are placeholder URLs - update when actual Ault snapshots are available.
var SnapshotURLs = map[string]string{
	"mainnet": "https://ault-mainnet-data.s3.amazonaws.com/snapshots/ault_pruned.tar.zst",
	"testnet": "https://ault-testnet-data.s3.amazonaws.com/snapshots/ault_pruned.tar.zst",
}

// ChainIDForNetwork returns the chain ID for a given network source.
func ChainIDForNetwork(networkSource string) string {
	switch networkSource {
	case "mainnet":
		return "ault_904-1"
	case "testnet":
		return "ault_10904-1"
	default:
		return DefaultChainID
	}
}
