// Package plugin provides helpers for developing devnet-builder network plugins.
//
// To create a new network plugin:
//
//  1. Implement the network.Module interface
//  2. Call plugin.Serve() with your implementation
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
//	    plugin.Serve(&MyNetwork{})
//	}
package plugin

import (
	"context"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// Handshake is the handshake configuration for devnet-builder plugins.
// This ensures compatibility between the host and plugins.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "DEVNET_BUILDER_PLUGIN",
	MagicCookieValue: "network_module_v1",
}

// Serve starts the plugin server with the given network module implementation.
// This function blocks and should be called from main().
//
// Example:
//
//	func main() {
//	    plugin.Serve(&MyNetworkModule{})
//	}
func Serve(module network.Module) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"network": &NetworkModulePlugin{Impl: module},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

// NetworkModulePlugin is the plugin.GRPCPlugin implementation for network modules.
type NetworkModulePlugin struct {
	plugin.Plugin
	Impl network.Module
}

// GRPCServer returns a gRPC server for the plugin.
func (p *NetworkModulePlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	RegisterNetworkModuleServer(s, NewGRPCServer(p.Impl))
	return nil
}

// GRPCClient returns a gRPC client for the plugin.
func (p *NetworkModulePlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return NewGRPCClient(c), nil
}
