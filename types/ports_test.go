package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultPortConfig(t *testing.T) {
	cfg := DefaultPortConfig()

	require.Equal(t, DefaultRPCPort, cfg.RPC)
	require.Equal(t, DefaultP2PPort, cfg.P2P)
	require.Equal(t, DefaultProxyPort, cfg.Proxy)
	require.Equal(t, DefaultGRPCPort, cfg.GRPC)
	require.Equal(t, DefaultGRPCWebPort, cfg.GRPCWeb)
	require.Equal(t, DefaultAPIPort, cfg.API)
	require.Equal(t, DefaultEVMRPCPort, cfg.EVMRPC)
	require.Equal(t, DefaultEVMWSPort, cfg.EVMWS)
	require.Equal(t, DefaultPProfPort, cfg.PProf)
	require.Equal(t, DefaultRosettaPort, cfg.Rosetta)
}

func TestPortConfigForNode(t *testing.T) {
	tests := []struct {
		name       string
		nodeIndex  int
		wantRPC    int
		wantP2P    int
		wantGRPC   int
		wantAPI    int
		wantEVMRPC int
	}{
		{
			name:       "node 0 uses default ports",
			nodeIndex:  0,
			wantRPC:    26657,
			wantP2P:    26656,
			wantGRPC:   9090,
			wantAPI:    1317,
			wantEVMRPC: 8545,
		},
		{
			name:       "node 1 adds 10000 offset",
			nodeIndex:  1,
			wantRPC:    36657,
			wantP2P:    36656,
			wantGRPC:   19090,
			wantAPI:    11317,
			wantEVMRPC: 18545,
		},
		{
			name:       "node 2 adds 20000 offset",
			nodeIndex:  2,
			wantRPC:    46657,
			wantP2P:    46656,
			wantGRPC:   29090,
			wantAPI:    21317,
			wantEVMRPC: 28545,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := PortConfigForNode(tt.nodeIndex)

			require.Equal(t, tt.wantRPC, cfg.RPC, "RPC port mismatch")
			require.Equal(t, tt.wantP2P, cfg.P2P, "P2P port mismatch")
			require.Equal(t, tt.wantGRPC, cfg.GRPC, "GRPC port mismatch")
			require.Equal(t, tt.wantAPI, cfg.API, "API port mismatch")
			require.Equal(t, tt.wantEVMRPC, cfg.EVMRPC, "EVMRPC port mismatch")
		})
	}
}

func TestPortConfig_WithOffset(t *testing.T) {
	base := DefaultPortConfig()

	// Apply +100 offset
	offset := base.WithOffset(100)

	require.Equal(t, base.RPC+100, offset.RPC)
	require.Equal(t, base.P2P+100, offset.P2P)
	require.Equal(t, base.Proxy+100, offset.Proxy)
	require.Equal(t, base.GRPC+100, offset.GRPC)
	require.Equal(t, base.GRPCWeb+100, offset.GRPCWeb)
	require.Equal(t, base.API+100, offset.API)
	require.Equal(t, base.EVMRPC+100, offset.EVMRPC)
	require.Equal(t, base.EVMWS+100, offset.EVMWS)
	require.Equal(t, base.PProf+100, offset.PProf)
	require.Equal(t, base.Rosetta+100, offset.Rosetta)
}

func TestPortConfig_WithOffset_Negative(t *testing.T) {
	base := PortConfigForNode(1) // Node 1 has +10000 offset

	// Apply -5000 offset
	adjusted := base.WithOffset(-5000)

	require.Equal(t, 36657-5000, adjusted.RPC)
	require.Equal(t, 36656-5000, adjusted.P2P)
}

func TestPortConfig_RPCURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantURL string
	}{
		{
			name:    "empty host defaults to localhost",
			host:    "",
			wantURL: "http://localhost:26657",
		},
		{
			name:    "custom host",
			host:    "192.168.1.100",
			wantURL: "http://192.168.1.100:26657",
		},
		{
			name:    "hostname",
			host:    "node.example.com",
			wantURL: "http://node.example.com:26657",
		},
	}

	cfg := DefaultPortConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := cfg.RPCURL(tt.host)
			require.Equal(t, tt.wantURL, url)
		})
	}
}

func TestPortConfig_EVMRPCURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantURL string
	}{
		{
			name:    "empty host defaults to localhost",
			host:    "",
			wantURL: "http://localhost:8545",
		},
		{
			name:    "custom host",
			host:    "192.168.1.100",
			wantURL: "http://192.168.1.100:8545",
		},
	}

	cfg := DefaultPortConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := cfg.EVMRPCURL(tt.host)
			require.Equal(t, tt.wantURL, url)
		})
	}
}

func TestPortConfig_APIURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantURL string
	}{
		{
			name:    "empty host defaults to localhost",
			host:    "",
			wantURL: "http://localhost:1317",
		},
		{
			name:    "custom host",
			host:    "192.168.1.100",
			wantURL: "http://192.168.1.100:1317",
		},
	}

	cfg := DefaultPortConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := cfg.APIURL(tt.host)
			require.Equal(t, tt.wantURL, url)
		})
	}
}

func TestPortConfig_AllPorts(t *testing.T) {
	cfg := DefaultPortConfig()
	ports := cfg.AllPorts()

	// Should return all 10 ports
	require.Len(t, ports, 10)

	// All ports should be present
	require.Contains(t, ports, DefaultRPCPort)
	require.Contains(t, ports, DefaultP2PPort)
	require.Contains(t, ports, DefaultProxyPort)
	require.Contains(t, ports, DefaultGRPCPort)
	require.Contains(t, ports, DefaultGRPCWebPort)
	require.Contains(t, ports, DefaultAPIPort)
	require.Contains(t, ports, DefaultEVMRPCPort)
	require.Contains(t, ports, DefaultEVMWSPort)
	require.Contains(t, ports, DefaultPProfPort)
	require.Contains(t, ports, DefaultRosettaPort)
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{input: 0, want: "0"},
		{input: 1, want: "1"},
		{input: 26657, want: "26657"},
		{input: 100000, want: "100000"},
		{input: -1, want: "-1"},
		{input: -26657, want: "-26657"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := itoa(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultPortOffset(t *testing.T) {
	// Verify the offset constant is 10000
	require.Equal(t, 10000, DefaultPortOffset)
}

func TestPortConstants(t *testing.T) {
	// Verify all port constants are correct
	require.Equal(t, 26657, DefaultRPCPort)
	require.Equal(t, 26656, DefaultP2PPort)
	require.Equal(t, 26658, DefaultProxyPort)
	require.Equal(t, 9090, DefaultGRPCPort)
	require.Equal(t, 9091, DefaultGRPCWebPort)
	require.Equal(t, 1317, DefaultAPIPort)
	require.Equal(t, 8545, DefaultEVMRPCPort)
	require.Equal(t, 8546, DefaultEVMWSPort)
	require.Equal(t, 6060, DefaultPProfPort)
	require.Equal(t, 8080, DefaultRosettaPort)
}
