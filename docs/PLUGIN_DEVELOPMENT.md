# Plugin Development Guide

This guide explains how to create custom network plugins for devnet-builder.

## Overview

devnet-builder uses HashiCorp's go-plugin framework to support dynamic loading of network modules. This allows you to create plugins for any Cosmos SDK-based blockchain without modifying the devnet-builder core.

## Quick Start

### 1. Create a new Go module

```bash
mkdir my-network-plugin
cd my-network-plugin
go mod init github.com/yourorg/my-network-plugin
```

### 2. Add the devnet-builder SDK dependency

```bash
go get github.com/b-harvest/devnet-builder/pkg/network
```

### 3. Implement the network.Module interface

Create `main.go`:

```go
package main

import (
    "context"
    "time"

    "github.com/b-harvest/devnet-builder/pkg/network"
    "github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

func main() {
    plugin.Serve(&MyNetwork{})
}

type MyNetwork struct{}

// Implement all network.Module methods...
func (n *MyNetwork) Name() string { return "mychain" }
func (n *MyNetwork) DisplayName() string { return "My Chain" }
// ... see examples/cosmos-plugin for complete implementation
```

### 4. Build the plugin

```bash
go build -o devnet-mychain .
```

### 5. Install the plugin

Copy the binary to one of the plugin directories:
- `~/.devnet-builder/plugins/`
- `/usr/local/lib/devnet-builder/plugins/`
- `./plugins/` (relative to devnet-builder working directory)

## network.Module Interface

Your plugin must implement the `network.Module` interface:

```go
type Module interface {
    // Identity
    Name() string           // Unique identifier (e.g., "mychain")
    DisplayName() string    // Human-readable name (e.g., "My Chain")
    Version() string        // Module version (e.g., "1.0.0")

    // Binary Configuration
    BinaryName() string             // CLI binary name (e.g., "mychaind")
    BinarySource() BinarySource     // How to acquire the binary
    DefaultBinaryVersion() string   // Default version to use

    // Chain Configuration
    DefaultChainID() string         // Default chain ID
    Bech32Prefix() string           // Address prefix (e.g., "mychain")
    BaseDenom() string              // Base token denom (e.g., "umytoken")
    GenesisConfig() GenesisConfig   // Genesis parameters
    DefaultPorts() PortConfig       // Port configuration

    // Docker Configuration
    DockerImage() string                    // Docker image name
    DockerImageTag(version string) string   // Version to tag mapping
    DockerHomeDir() string                  // Home directory in container

    // Path Configuration
    DefaultNodeHome() string    // Default node home (e.g., "/root/.mychaind")
    PIDFileName() string        // PID file name (e.g., "mychaind.pid")
    LogFileName() string        // Log file name (e.g., "mychaind.log")
    ProcessPattern() string     // Regex to match running processes

    // Command Generation
    InitCommand(homeDir, chainID, moniker string) []string
    StartCommand(homeDir string) []string
    ExportCommand(homeDir string) []string

    // Devnet Operations
    ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error)
    GenerateDevnet(ctx context.Context, config GeneratorConfig, genesisFile string) error
    DefaultGeneratorConfig() GeneratorConfig
    GetCodec() ([]byte, error)

    // Validation
    Validate() error
}
```

## Configuration Types

### BinarySource

Defines how to acquire the network binary:

```go
type BinarySource struct {
    Type      string // "github", "local", or "docker"
    Owner     string // GitHub owner (for "github" type)
    Repo      string // GitHub repository (for "github" type)
    LocalPath string // Path to local binary (for "local" type)
    AssetName string // Release asset name pattern (for "github" type)
}
```

### GenesisConfig

Default genesis parameters:

```go
type GenesisConfig struct {
    ChainIDPattern    string        // e.g., "mychain_{evmid}-1"
    EVMChainID        int64         // EVM chain ID (0 if no EVM)
    BaseDenom         string        // e.g., "umytoken"
    DenomExponent     int           // Decimal places
    DisplayDenom      string        // Human-readable (e.g., "MYTOKEN")
    BondDenom         string        // Staking token denom
    MinSelfDelegation string        // Minimum self-delegation
    UnbondingTime     time.Duration // Unbonding period
    MaxValidators     uint32        // Maximum validator count
    MinDeposit        string        // Minimum governance deposit
    VotingPeriod      time.Duration // Governance voting period
    MaxDepositPeriod  time.Duration // Max deposit period
    CommunityTax      string        // Community tax rate
}
```

### PortConfig

Network port configuration:

```go
type PortConfig struct {
    RPC       int // Tendermint RPC (default: 26657)
    P2P       int // P2P networking (default: 26656)
    GRPC      int // gRPC server (default: 9090)
    GRPCWeb   int // gRPC-Web (default: 9091)
    API       int // REST API (default: 1317)
    EVMRPC    int // EVM JSON-RPC (default: 8545, 0 if no EVM)
    EVMSocket int // EVM WebSocket (default: 8546, 0 if no EVM)
}
```

## Example Implementation

See `examples/cosmos-plugin/` for a complete example implementing the Cosmos Hub network.

## Testing Your Plugin

1. Build the plugin:
   ```bash
   go build -o devnet-mychain .
   ```

2. Copy to plugins directory:
   ```bash
   mkdir -p ~/.devnet-builder/plugins
   cp devnet-mychain ~/.devnet-builder/plugins/
   ```

3. Verify it's discovered:
   ```bash
   devnet-builder networks
   ```

4. Generate a devnet:
   ```bash
   devnet-builder --network mychain generate
   ```

## Plugin Naming Convention

Plugin binaries must follow the naming convention: `devnet-<network-name>`

Examples:
- `devnet-cosmos` for Cosmos Hub
- `devnet-osmosis` for Osmosis
- `devnet-mychain` for your custom chain

## Troubleshooting

### Plugin not discovered

1. Check the binary is executable: `chmod +x devnet-mychain`
2. Verify the naming convention: binary must start with `devnet-`
3. Check the plugin directory permissions

### Plugin fails to load

1. Check the handshake configuration matches
2. Verify the plugin implements all required interface methods
3. Run with debug logging: `devnet-builder --debug networks`

### Genesis modification errors

1. Ensure your `ModifyGenesis` function handles the genesis JSON correctly
2. Use proper JSON unmarshaling/marshaling
3. Test with a sample genesis file first

## Best Practices

1. **Use semantic versioning** for your module version
2. **Implement Validate()** to check configuration correctness
3. **Handle errors gracefully** in all interface methods
4. **Document chain-specific requirements** in your plugin's README
5. **Test with real genesis files** before deployment
