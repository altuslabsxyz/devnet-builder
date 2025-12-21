// Package plugin provides HashiCorp go-plugin based network module plugin system.
//
// Architecture:
//
//	devnet-builder (Host)                    Network Plugins
//	┌─────────────────────┐                  ┌─────────────────────┐
//	│  Plugin Manager     │   gRPC/RPC       │  stable plugin      │
//	│                     │◄────────────────►│  (stable deps only) │
//	│  - Discovery        │                  └─────────────────────┘
//	│  - Loading          │                  ┌─────────────────────┐
//	│  - Communication    │◄────────────────►│  ault plugin        │
//	│                     │                  │  (ault deps only)   │
//	└─────────────────────┘                  └─────────────────────┘
//
// Benefits:
//   - Each network plugin is a separate binary with its own dependencies
//   - No cross-contamination of go.mod replace directives
//   - Dynamic loading at runtime
//   - New networks can be added without recompiling the host
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pb "github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

// Handshake is the handshake config for plugins.
// This ensures that the host and plugin are compatible.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "DEVNET_BUILDER_PLUGIN",
	MagicCookieValue: "network_module_v1",
}

// PluginMap is the map of plugins we can dispense.
var PluginMap = map[string]plugin.Plugin{
	"network": &NetworkModuleGRPCPlugin{},
}

// NetworkModuleGRPCPlugin is the plugin.GRPCPlugin implementation.
type NetworkModuleGRPCPlugin struct {
	plugin.Plugin
	Impl NetworkModule
}

// GRPCServer returns a gRPC server for the plugin.
func (p *NetworkModuleGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterNetworkModuleServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns a gRPC client for the plugin.
func (p *NetworkModuleGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: pb.NewNetworkModuleClient(c)}, nil
}

// PluginManager handles discovery and loading of network plugins.
type PluginManager struct {
	pluginDir string
	clients   map[string]*plugin.Client
	modules   map[string]NetworkModule
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager(pluginDir string) *PluginManager {
	return &PluginManager{
		pluginDir: pluginDir,
		clients:   make(map[string]*plugin.Client),
		modules:   make(map[string]NetworkModule),
	}
}

// DiscoverPlugins finds all available network plugins in the plugin directory.
func (m *PluginManager) DiscoverPlugins() ([]string, error) {
	if m.pluginDir == "" {
		return nil, fmt.Errorf("plugin directory not set")
	}

	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		return nil, nil // No plugins directory, no plugins
	}

	var plugins []string
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Plugin naming convention: devnet-<network>
		if len(name) > 7 && name[:7] == "devnet-" {
			networkName := name[7:]
			plugins = append(plugins, networkName)
		}
	}

	return plugins, nil
}

// LoadPlugin loads a specific network plugin.
func (m *PluginManager) LoadPlugin(networkName string) (NetworkModule, error) {
	if module, exists := m.modules[networkName]; exists {
		return module, nil
	}

	pluginPath := filepath.Join(m.pluginDir, fmt.Sprintf("devnet-%s", networkName))
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugin not found: %s", pluginPath)
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap,
		Cmd:             exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to connect to plugin: %w", err)
	}

	raw, err := rpcClient.Dispense("network")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense network plugin: %w", err)
	}

	module := raw.(NetworkModule)

	m.clients[networkName] = client
	m.modules[networkName] = module

	return module, nil
}

// GetModule returns a loaded module by name.
func (m *PluginManager) GetModule(networkName string) (NetworkModule, bool) {
	module, exists := m.modules[networkName]
	return module, exists
}

// ListModules returns all loaded module names.
func (m *PluginManager) ListModules() []string {
	names := make([]string, 0, len(m.modules))
	for name := range m.modules {
		names = append(names, name)
	}
	return names
}

// Close shuts down all plugin clients.
func (m *PluginManager) Close() {
	for name, client := range m.clients {
		client.Kill()
		delete(m.clients, name)
		delete(m.modules, name)
	}
}

// Serve is called by plugin binaries to serve the NetworkModule implementation.
func Serve(impl NetworkModule) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"network": &NetworkModuleGRPCPlugin{Impl: impl},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
