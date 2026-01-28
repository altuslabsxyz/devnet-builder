# Network Plugin Development Guide

This guide documents how to create network plugins for devnet-builder using the plugin-based network wiring architecture.

## Overview

devnet-builder uses a plugin architecture where all networks (including the built-in Cosmos support) are discovered and loaded as external plugins. Plugins are standalone binaries that communicate with devnetd via gRPC using HashiCorp's go-plugin framework.

### Key Concepts

1. **All networks are plugins** - No built-in networks; everything is discovered from `~/.devnet-builder/plugins/`
2. **Plugin-provided configuration** - Plugins implement `network.Module` interface to provide chain-specific values
3. **devnetd as plugin host** - devnetd loads plugins and exposes them via gRPC API
4. **Thin client pattern** - dvb queries devnetd for available networks

## Architecture

```
~/.devnet-builder/plugins/
  cosmos-plugin       stable-plugin       osmosis-plugin
        |                   |                   |
        +-------------------+-------------------+
                            |
                            v (go-plugin/gRPC)
+-----------------------------------------------------------------------+
|                            devnetd                                     |
|  +------------------------------------------------------------------+ |
|  |                      Plugin Loader                                | |
|  |  * Discovers plugins from ~/.devnet-builder/plugins/              | |
|  |  * Loads ALL NetworkModule implementations                        | |
|  +------------------------------------------------------------------+ |
|                            |                                          |
|                            v                                          |
|  +------------------------------------------------------------------+ |
|  |                    NetworkRegistry                                | |
|  |  * "cosmos" -> NetworkModule (from cosmos-plugin)                 | |
|  |  * "stable" -> NetworkModule (from stable-plugin)                 | |
|  +------------------------------------------------------------------+ |
|                            |                                          |
|                            v                                          |
|  +------------------------------------------------------------------+ |
|  |                   Service Layer (behavior)                        | |
|  |  BinaryBuilderService | GenesisService | NodeInitService          | |
|  +------------------------------------------------------------------+ |
+-----------------------------------------------------------------------+
                            |
                            | gRPC over Unix socket
                            v
+-----------------------------------------------------------------------+
|                            dvb                                        |
|  * ZERO network knowledge                                             |
|  * Query devnetd for everything                                       |
+-----------------------------------------------------------------------+
```

## Installation Paths

Plugins are discovered from the following directories (in priority order):

1. `./plugins/` - Project-local plugins
2. `~/.devnet-builder/plugins/` - User plugins (recommended)
3. `/usr/local/lib/devnet-builder/plugins/` - System-wide plugins

### Binary Naming Convention

Plugin binaries must be named `{network}-plugin`:

```
~/.devnet-builder/plugins/
  cosmos-plugin       # Cosmos SDK base plugin
  stable-plugin       # Stable network plugin
  osmosis-plugin      # Osmosis plugin (example)
```

## Required NetworkModule Interface

All plugins must implement the `network.Module` interface from `pkg/network/interface.go`.

### Interface Overview

```go
type Module interface {
    // Identity
    Name() string           // Unique identifier (e.g., "stable", "cosmos")
    DisplayName() string    // Human-readable name
    Version() string        // Plugin version (semver)

    // Binary Configuration
    BinaryName() string                                  // CLI binary name (e.g., "gaiad")
    BinarySource() BinarySource                          // How to acquire the binary
    DefaultBinaryVersion() string                        // Default version
    GetBuildConfig(networkType string) (*BuildConfig, error)

    // Chain Configuration
    DefaultChainID() string                              // Deprecated: use genesis
    Bech32Prefix() string                                // Address prefix (e.g., "cosmos")
    BaseDenom() string                                   // Base token (e.g., "uatom")
    GenesisConfig() GenesisConfig                        // Genesis parameters
    DefaultPorts() PortConfig                            // Port configuration

    // Docker Configuration
    DockerImage() string
    DockerImageTag(version string) string
    DockerHomeDir() string

    // Path Configuration
    DefaultNodeHome() string                             // e.g., "/root/.gaia"
    PIDFileName() string
    LogFileName() string
    ProcessPattern() string

    // Command Generation
    InitCommand(homeDir, chainID, moniker string) []string
    StartCommand(homeDir string) []string
    ExportCommand(homeDir string) []string

    // Devnet Operations
    ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error)
    GenerateDevnet(ctx context.Context, config GeneratorConfig, genesisFile string) error
    DefaultGeneratorConfig() GeneratorConfig

    // Codec and Validation
    GetCodec() ([]byte, error)
    Validate() error

    // Snapshot Configuration
    SnapshotURL(networkType string) string
    RPCEndpoint(networkType string) string
    AvailableNetworks() []string

    // Node Configuration
    GetConfigOverrides(nodeIndex int, opts NodeConfigOptions) (configToml, appToml []byte, err error)
}
```

### Optional Interfaces

#### FileBasedGenesisModifier

For genesis files larger than 4MB (gRPC message size limit):

```go
type FileBasedGenesisModifier interface {
    ModifyGenesisFile(inputPath, outputPath string, opts GenesisOptions) (outputSize int64, err error)
}
```

#### StateExporter

For snapshot-based devnets:

```go
type StateExporter interface {
    ExportCommandWithOptions(homeDir string, opts ExportOptions) []string
    ValidateExportedGenesis(genesis []byte) error
    RequiredModules() []string
    SnapshotFormat(networkType string) SnapshotFormat
}
```

## Plugin Binary Structure

### Project Layout

```
my-network-plugin/
  main.go              # Plugin entry point
  network.go           # network.Module implementation
  genesis.go           # Genesis modification logic
  config.go            # Config override logic
  rpc.go               # Optional: RPC query implementations
  go.mod
  go.sum
```

### Entry Point (main.go)

```go
package main

import (
    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

func main() {
    plugin.Serve(&MyNetwork{})
}
```

The `plugin.Serve()` function handles:
- HashiCorp go-plugin handshake
- gRPC server setup
- Network module registration

### Handshake Configuration

The plugin system uses this handshake:

```go
var Handshake = hcplugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "DEVNET_BUILDER_PLUGIN",
    MagicCookieValue: "network_module_v1",
}
```

## Building a Plugin

### Step 1: Create go.mod

```go
module github.com/myorg/mynetwork-plugin

go 1.22

require (
    github.com/altuslabsxyz/devnet-builder v1.0.0
)
```

### Step 2: Implement network.Module

See the example in `docs/plugins/cosmos-plugin.md` for a complete implementation.

### Step 3: Build

```bash
go build -o mynetwork-plugin .
```

### Step 4: Install

```bash
mkdir -p ~/.devnet-builder/plugins
cp mynetwork-plugin ~/.devnet-builder/plugins/
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin
```

### Step 5: Verify

```bash
dvb plugins list
```

## Plugin Communication

Plugins communicate with devnetd via gRPC over Unix domain sockets.

### Protocol Buffer Service

The gRPC service is defined in `pkg/network/plugin/network.proto`:

```protobuf
service NetworkModule {
    // Identity
    rpc Name(Empty) returns (StringResponse);
    rpc DisplayName(Empty) returns (StringResponse);
    rpc Version(Empty) returns (StringResponse);

    // Binary
    rpc BinaryName(Empty) returns (StringResponse);
    rpc BinarySource(Empty) returns (BinarySourceResponse);
    // ... many more methods
}
```

### gRPC Client/Server

The plugin package provides:
- `GRPCClient` - Client-side implementation for host
- `GRPCServer` - Server-side implementation for plugins

## Version Compatibility

Plugins must return a version string that satisfies devnetd's version constraint:

```go
func (n *MyNetwork) Version() string {
    return "1.0.0"  // Must be >= 1.0.0
}
```

The loader validates versions using semantic versioning comparison.

## Best Practices

### 1. Validate Thoroughly

```go
func (n *MyNetwork) Validate() error {
    if n.Name() == "" {
        return fmt.Errorf("name is required")
    }
    if n.BinaryName() == "" {
        return fmt.Errorf("binary name is required")
    }
    // ... more validations
    return nil
}
```

### 2. Handle Multiple Network Types

```go
func (n *MyNetwork) RPCEndpoint(networkType string) string {
    switch networkType {
    case "mainnet":
        return "https://rpc.mynetwork.io"
    case "testnet":
        return "https://rpc-testnet.mynetwork.io"
    default:
        return ""
    }
}
```

### 3. Use File-Based Genesis for Large Files

```go
// Implement FileBasedGenesisModifier for 50MB+ genesis files
func (n *MyNetwork) ModifyGenesisFile(inputPath, outputPath string, opts GenesisOptions) (int64, error) {
    // Stream-process the file to avoid memory issues
}
```

### 4. Handle RPC Timeouts

```go
func (n *MyNetwork) GetBlockHeight(ctx context.Context, rpcEndpoint string) (*BlockHeightResponse, error) {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    // ...
}
```

## Troubleshooting

### Plugin Not Found

```bash
# Check plugin exists and is executable
ls -la ~/.devnet-builder/plugins/

# Test plugin directly (should hang waiting for handshake)
./~/.devnet-builder/plugins/mynetwork-plugin
```

### Version Mismatch

```bash
# Check plugin version
dvb plugins info mynetwork

# Check devnet-builder version
dvb version
```

### gRPC Errors

Enable debug logging:

```bash
devnetd start --log-level debug
```

## See Also

- [cosmos-plugin.md](./cosmos-plugin.md) - Cosmos plugin extraction guide
- [stable-plugin.md](./stable-plugin.md) - Stable plugin requirements
- [../plugins.md](../plugins.md) - Full plugin system documentation
