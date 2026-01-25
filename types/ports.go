package types

// Port default values - Single Source of Truth for all port constants.
// These constants are used throughout the codebase for node configuration.
const (
	// DefaultRPCPort is the Tendermint/CometBFT RPC port.
	DefaultRPCPort = 26657

	// DefaultP2PPort is the peer-to-peer networking port.
	DefaultP2PPort = 26656

	// DefaultProxyPort is the ABCI proxy application port.
	DefaultProxyPort = 26658

	// DefaultGRPCPort is the gRPC server port.
	DefaultGRPCPort = 9090

	// DefaultGRPCWebPort is the gRPC-Web server port.
	DefaultGRPCWebPort = 9091

	// DefaultAPIPort is the REST API (LCD) port.
	DefaultAPIPort = 1317

	// DefaultEVMRPCPort is the EVM JSON-RPC port.
	DefaultEVMRPCPort = 8545

	// DefaultEVMWSPort is the EVM WebSocket port.
	DefaultEVMWSPort = 8546

	// DefaultPProfPort is the pprof debugging port.
	DefaultPProfPort = 6060

	// DefaultRosettaPort is the Rosetta API port.
	DefaultRosettaPort = 8080

	// DefaultPortOffset is the port offset multiplier per node.
	// Node 0: base ports, Node 1: base + 10000, Node 2: base + 20000, etc.
	DefaultPortOffset = 10000
)

// PortConfig contains network port configuration for a node.
// This is the canonical type used throughout the codebase.
// All infrastructure packages should convert to/from this type.
type PortConfig struct {
	// RPC is the Tendermint/CometBFT RPC port (default: 26657)
	RPC int `json:"rpc"`

	// P2P is the peer-to-peer network port (default: 26656)
	P2P int `json:"p2p"`

	// Proxy is the ABCI proxy application port (default: 26658)
	Proxy int `json:"proxy,omitempty"`

	// GRPC is the gRPC server port (default: 9090)
	GRPC int `json:"grpc"`

	// GRPCWeb is the gRPC-Web server port (default: 9091)
	GRPCWeb int `json:"grpc_web,omitempty"`

	// API is the REST API (LCD) port (default: 1317)
	API int `json:"api"`

	// EVMRPC is the EVM JSON-RPC port (default: 8545)
	// Only applicable to EVM-compatible networks.
	EVMRPC int `json:"evm_rpc"`

	// EVMWS is the EVM WebSocket port (default: 8546)
	// Only applicable to EVM-compatible networks.
	EVMWS int `json:"evm_ws,omitempty"`

	// PProf is the pprof debugging port (default: 6060)
	PProf int `json:"pprof,omitempty"`

	// Rosetta is the Rosetta API port (default: 8080)
	Rosetta int `json:"rosetta,omitempty"`
}

// DefaultPortConfig returns a PortConfig with all default values.
func DefaultPortConfig() PortConfig {
	return PortConfig{
		RPC:     DefaultRPCPort,
		P2P:     DefaultP2PPort,
		Proxy:   DefaultProxyPort,
		GRPC:    DefaultGRPCPort,
		GRPCWeb: DefaultGRPCWebPort,
		API:     DefaultAPIPort,
		EVMRPC:  DefaultEVMRPCPort,
		EVMWS:   DefaultEVMWSPort,
		PProf:   DefaultPProfPort,
		Rosetta: DefaultRosettaPort,
	}
}

// PortConfigForNode returns the port configuration for a node at the given index.
// Each node has a port offset of DefaultPortOffset * index.
// Node 0 uses default ports, Node 1 adds 10000, Node 2 adds 20000, etc.
func PortConfigForNode(index int) PortConfig {
	offset := index * DefaultPortOffset
	return PortConfig{
		RPC:     DefaultRPCPort + offset,
		P2P:     DefaultP2PPort + offset,
		Proxy:   DefaultProxyPort + offset,
		GRPC:    DefaultGRPCPort + offset,
		GRPCWeb: DefaultGRPCWebPort + offset,
		API:     DefaultAPIPort + offset,
		EVMRPC:  DefaultEVMRPCPort + offset,
		EVMWS:   DefaultEVMWSPort + offset,
		PProf:   DefaultPProfPort + offset,
		Rosetta: DefaultRosettaPort + offset,
	}
}

// WithOffset returns a new PortConfig with all ports offset by the given amount.
// Useful for running multiple nodes on the same machine.
func (p PortConfig) WithOffset(offset int) PortConfig {
	return PortConfig{
		RPC:     p.RPC + offset,
		P2P:     p.P2P + offset,
		Proxy:   p.Proxy + offset,
		GRPC:    p.GRPC + offset,
		GRPCWeb: p.GRPCWeb + offset,
		API:     p.API + offset,
		EVMRPC:  p.EVMRPC + offset,
		EVMWS:   p.EVMWS + offset,
		PProf:   p.PProf + offset,
		Rosetta: p.Rosetta + offset,
	}
}

// RPCURL returns the full RPC URL for this port configuration.
func (p PortConfig) RPCURL(host string) string {
	if host == "" {
		host = "localhost"
	}
	return "http://" + host + ":" + itoa(p.RPC)
}

// EVMRPCURL returns the full EVM JSON-RPC URL for this port configuration.
func (p PortConfig) EVMRPCURL(host string) string {
	if host == "" {
		host = "localhost"
	}
	return "http://" + host + ":" + itoa(p.EVMRPC)
}

// APIURL returns the full REST API URL for this port configuration.
func (p PortConfig) APIURL(host string) string {
	if host == "" {
		host = "localhost"
	}
	return "http://" + host + ":" + itoa(p.API)
}

// AllPorts returns a slice of all configured ports.
// Useful for port conflict detection.
func (p PortConfig) AllPorts() []int {
	return []int{
		p.RPC,
		p.P2P,
		p.Proxy,
		p.GRPC,
		p.GRPCWeb,
		p.API,
		p.EVMRPC,
		p.EVMWS,
		p.PProf,
		p.Rosetta,
	}
}

// itoa is a simple int to string conversion without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	negative := i < 0
	if negative {
		i = -i
	}

	// Max int64 has 19 digits
	var buf [20]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if negative {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}
