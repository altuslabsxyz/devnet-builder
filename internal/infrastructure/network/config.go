package network

// PortConfig contains default network port configuration.
// Used for node startup, health checks, and client connections.
type PortConfig struct {
	// RPC is the Tendermint/CometBFT RPC port (default: 26657)
	RPC int

	// P2P is the peer-to-peer network port (default: 26656)
	P2P int

	// GRPC is the gRPC server port (default: 9090)
	GRPC int

	// GRPCWeb is the gRPC-Web server port (default: 9091)
	GRPCWeb int

	// API is the REST API port (default: 1317)
	API int

	// EVMRPC is the EVM JSON-RPC port (default: 8545)
	// Only applicable to EVM-compatible networks
	EVMRPC int

	// EVMWS is the EVM WebSocket port (default: 8546)
	// Only applicable to EVM-compatible networks
	EVMWS int
}

// DefaultPortConfig returns a standard Cosmos SDK port configuration.
func DefaultPortConfig() PortConfig {
	return PortConfig{
		RPC:     26657,
		P2P:     26656,
		GRPC:    9090,
		GRPCWeb: 9091,
		API:     1317,
		EVMRPC:  8545,
		EVMWS:   8546,
	}
}

// WithOffset returns a new PortConfig with all ports offset by the given amount.
// Useful for running multiple nodes on the same machine.
func (p PortConfig) WithOffset(offset int) PortConfig {
	return PortConfig{
		RPC:     p.RPC + offset,
		P2P:     p.P2P + offset,
		GRPC:    p.GRPC + offset,
		GRPCWeb: p.GRPCWeb + offset,
		API:     p.API + offset,
		EVMRPC:  p.EVMRPC + offset,
		EVMWS:   p.EVMWS + offset,
	}
}
