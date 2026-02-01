// Package plugin provides helpers for developing devnet-builder network plugins.
//
// To create a new network plugin:
//
//  1. Implement the network.Module interface
//  2. Call hcplugin.Serve() with your implementation
//
// Example:
//
//	package main
//
//	import (
//	    "github.com/altuslabsxyz/devnet-builder/pkg/network"
//	    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
//	)
//
//	type MyNetwork struct{}
//
//	// Implement all network.Module methods...
//	func (m *MyNetwork) Name() string { return "mychain" }
//	// ...
//
//	func main() {
//	    hcplugin.Serve(&MyNetwork{})
//	}
package plugin

import (
	"context"

	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// Handshake is the handshake configuration for devnet-builder plugins.
// This ensures compatibility between the host and plugins.
var Handshake = hcplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "DEVNET_BUILDER_PLUGIN",
	MagicCookieValue: "network_module_v1",
}

// maxGRPCMessageSize is the maximum message size for gRPC communication.
// Genesis files can be large (100MB+), so we set this to 256MB.
const maxGRPCMessageSize = 256 * 1024 * 1024 // 256MB

// Serve starts the plugin server with the given network module implementation.
// This function blocks and should be called from main().
//
// Example:
//
//	func main() {
//	    hcplugin.Serve(&MyNetworkModule{})
//	}
func Serve(module network.Module) {
	hcplugin.Serve(&hcplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]hcplugin.Plugin{
			"network": &NetworkModulePlugin{Impl: module},
		},
		GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
			// Add message size options for large genesis files
			opts = append(opts,
				grpc.MaxRecvMsgSize(maxGRPCMessageSize),
				grpc.MaxSendMsgSize(maxGRPCMessageSize),
			)
			return grpc.NewServer(opts...)
		},
	})
}

// NetworkModulePlugin is the plugin.GRPCPlugin implementation for network modules.
type NetworkModulePlugin struct {
	hcplugin.Plugin
	Impl network.Module
}

// GRPCServer returns a gRPC server for the plugin.
func (p *NetworkModulePlugin) GRPCServer(broker *hcplugin.GRPCBroker, s *grpc.Server) error {
	RegisterNetworkModuleServer(s, NewGRPCServer(p.Impl))
	return nil
}

// GRPCClient returns a gRPC client for the plugin.
func (p *NetworkModulePlugin) GRPCClient(ctx context.Context, broker *hcplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return NewGRPCClient(c), nil
}
