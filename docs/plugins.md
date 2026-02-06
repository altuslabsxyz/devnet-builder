# Plugin System Guide

Complete guide to the devnet-builder plugin system for supporting custom blockchain networks.

## Overview

Devnet-builder uses a plugin architecture to support multiple blockchain networks. Plugins are standalone binaries that communicate with devnet-builder via gRPC, providing network-specific implementations for genesis generation, node management, binary configuration, and more.

**Key Benefits:**
- **Language Independence** - Plugins can be written in any language (Go recommended)
- **Process Isolation** - Plugins run as separate processes, preventing crashes from affecting the main tool
- **Hot Reload** - Plugins can be updated without restarting the daemon
- **Easy Distribution** - Single binary distribution per network

## Plugin Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        devnet-builder / dvb                          │
├─────────────────────────────────────────────────────────────────────┤
│                         Plugin Manager                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │
│  │  gRPC Client    │  │  gRPC Client    │  │  gRPC Client    │      │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘      │
└───────────┼────────────────────┼────────────────────┼───────────────┘
            │                    │                    │
            │    gRPC/IPC        │    gRPC/IPC        │    gRPC/IPC
            │                    │                    │
┌───────────▼────────┐ ┌────────▼─────────┐ ┌────────▼─────────┐
│   stable-plugin    │ │  osmosis-plugin  │ │  cosmos-plugin   │
│  (subprocess)      │ │   (subprocess)   │ │   (subprocess)   │
└────────────────────┘ └──────────────────┘ └──────────────────┘
```

## V1 vs V2 Plugin Systems

Devnet-builder has two CLI modes, each with its own plugin loading mechanism:

| Aspect | V1 (devnet-builder) | V2 (dvb + devnetd) |
|--------|--------------------|--------------------|
| **Binary Naming** | `devnet-<network>` | `<network>-plugin` |
| **Plugin Loader** | `internal/infrastructure/plugin/` | `pkg/network/plugin/` |
| **Interface** | `network.Module` | `network.Module` (same) |
| **Discovery** | Single directory | Multiple directories |
| **Hot Reload** | No | Yes |
| **Version Constraints** | No | Yes (semver) |

Both systems use the **same `network.Module` interface**, so a single plugin implementation works for both V1 and V2. The only difference is the binary naming convention.

## Installing Plugins

Plugins use **automatic discovery** - no registration or configuration files required. Just place the binary in the correct location.

### V2 Installation (dvb + devnetd)

V2 searches multiple directories for plugins named `{network}-plugin`:

```bash
# Create plugins directory
mkdir -p ~/.devnet-builder/plugins

# Install plugin
cp mynetwork-plugin ~/.devnet-builder/plugins/
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin

# Verify installation
dvb plugins list
```

**V2 Search Paths (in priority order):**
1. `./plugins/` - Project-local
2. `~/.devnet-builder/plugins/` - User directory (recommended)
3. `/usr/local/lib/devnet-builder/plugins/` - System-wide

**V2 Naming:** `{network}-plugin` (e.g., `stable-plugin`, `cosmos-plugin`)

### V1 Installation (devnet-builder)

V1 searches the system PATH for plugins named `devnet-{network}`:

```bash
# Install to a directory in your PATH
sudo cp devnet-mynetwork /usr/local/bin/
sudo chmod +x /usr/local/bin/devnet-mynetwork

# OR add a custom directory to PATH
mkdir -p ~/bin
cp devnet-mynetwork ~/bin/
export PATH="$HOME/bin:$PATH"

# Verify installation
which devnet-mynetwork
```

**V1 Naming:** `devnet-{network}` (e.g., `devnet-stable`, `devnet-cosmos`)

### Building for Both V1 and V2

To support both CLI versions, build two binaries:

```bash
# From your plugin source code
go build -o devnet-mynetwork .     # V1 binary
go build -o mynetwork-plugin .      # V2 binary

# Install both
cp devnet-mynetwork /usr/local/bin/             # V1
cp mynetwork-plugin ~/.devnet-builder/plugins/  # V2
```

### Usage After Installation

**V1 (devnet-builder):**
```bash
devnet-builder create --network mynetwork --validators 4
devnet-builder start
```

**V2 (dvb):**
```bash
dvb provision --network mynetwork --validators 4
# OR
dvb apply -f - <<EOF
apiVersion: v1
kind: Devnet
metadata:
  name: my-devnet
spec:
  plugin: mynetwork
  validators: 4
EOF
```

For detailed V2-specific features (hot reload, version constraints), see [V2 Plugin Development Guide](v2/plugins.md).

## Plugin Interface

All plugins must implement the `network.Module` interface from `pkg/network/interface.go`:

```go
type Module interface {
    // Identity
    Name() string                    // Unique identifier (e.g., "stable", "osmosis")
    DisplayName() string             // Human-readable name (e.g., "Stable Network")
    Version() string                 // Module version (e.g., "1.0.0")

    // Binary Configuration
    BinaryName() string              // CLI binary name (e.g., "stabled", "osmosisd")
    BinarySource() BinarySource      // How to acquire the binary
    DefaultBinaryVersion() string    // Default version to use
    GetBuildConfig(networkType string) (*BuildConfig, error)  // Build configuration

    // Chain Configuration
    DefaultChainID() string          // Default chain ID
    Bech32Prefix() string            // Address prefix (e.g., "cosmos", "osmo")
    BaseDenom() string               // Base token denomination (e.g., "uatom")
    GenesisConfig() GenesisConfig    // Genesis parameters
    DefaultPorts() PortConfig        // Default port configuration

    // Docker Configuration
    DockerImage() string             // Docker image name
    DockerImageTag(version string) string  // Version to tag mapping
    DockerHomeDir() string           // Home directory in container

    // Path Configuration
    DefaultNodeHome() string         // Default node home directory
    PIDFileName() string             // PID file name
    LogFileName() string             // Log file name
    ProcessPattern() string          // Regex to match running processes

    // Command Generation
    InitCommand(homeDir, chainID, moniker string) []string   // Node init args
    StartCommand(homeDir string) []string                     // Node start args
    ExportCommand(homeDir string) []string                    // State export args

    // Devnet Operations
    ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error)
    GenerateDevnet(ctx context.Context, config GeneratorConfig, genesisFile string) error
    DefaultGeneratorConfig() GeneratorConfig

    // Codec
    GetCodec() ([]byte, error)       // Network-specific codec configuration

    // Validation
    Validate() error                 // Validate module configuration

    // Snapshot Configuration
    SnapshotURL(networkType string) string      // Snapshot download URL
    RPCEndpoint(networkType string) string      // RPC endpoint URL
    AvailableNetworks() []string                // Supported network types

    // Node Configuration
    GetConfigOverrides(nodeIndex int, opts NodeConfigOptions) (configToml, appToml []byte, err error)
}
```

### Optional Interfaces

Plugins can implement additional interfaces for extended functionality:

#### StateExporter (for snapshot-based devnets)

```go
type StateExporter interface {
    ExportCommandWithOptions(homeDir string, opts ExportOptions) []string
    ValidateExportedGenesis(genesis []byte) error
    RequiredModules() []string
    SnapshotFormat(networkType string) SnapshotFormat
}
```

#### FileBasedGenesisModifier (for large genesis files)

```go
type FileBasedGenesisModifier interface {
    ModifyGenesisFile(inputPath, outputPath string, opts GenesisOptions) (outputSize int64, err error)
}
```

Use this when genesis files exceed 4MB (gRPC message size limit).

## Building a Plugin

### Project Structure

```
my-network-plugin/
├── main.go              # Plugin entry point
├── network.go           # network.Module implementation
├── genesis.go           # Genesis modification logic
├── rpc.go               # RPC query implementations
├── go.mod
└── go.sum
```

### Step 1: Create go.mod

```go
module github.com/myorg/mynetwork-plugin

go 1.22

require (
    github.com/altuslabsxyz/devnet-builder v1.0.0
)
```

### Step 2: Implement the Module

```go
// network.go
package main

import (
    "context"
    "time"

    "github.com/altuslabsxyz/devnet-builder/pkg/network"
)

type MyNetwork struct{}

// Ensure interface compliance at compile time
var _ network.Module = (*MyNetwork)(nil)

// ============================================
// Identity Methods
// ============================================

func (n *MyNetwork) Name() string {
    return "mynetwork"
}

func (n *MyNetwork) DisplayName() string {
    return "My Custom Network"
}

func (n *MyNetwork) Version() string {
    return "1.0.0"
}

// ============================================
// Binary Configuration
// ============================================

func (n *MyNetwork) BinaryName() string {
    return "mynetworkd"
}

func (n *MyNetwork) BinarySource() network.BinarySource {
    return network.BinarySource{
        Type:      "github",
        Owner:     "myorg",
        Repo:      "mynetwork",
        AssetName: "mynetworkd-*-linux-amd64",
    }
}

func (n *MyNetwork) DefaultBinaryVersion() string {
    return "v1.0.0"
}

func (n *MyNetwork) GetBuildConfig(networkType string) (*network.BuildConfig, error) {
    // Return custom build configuration for different network types
    switch networkType {
    case "mainnet":
        return &network.BuildConfig{
            Tags:    []string{"netgo", "ledger"},
            LDFlags: []string{"-X main.EVMChainID=1234"},
            Env:     map[string]string{"CGO_ENABLED": "0"},
        }, nil
    case "testnet":
        return &network.BuildConfig{
            Tags: []string{"netgo"},
        }, nil
    default:
        return &network.BuildConfig{}, nil
    }
}

// ============================================
// Chain Configuration
// ============================================

func (n *MyNetwork) DefaultChainID() string {
    return "mynetwork-devnet-1"
}

func (n *MyNetwork) Bech32Prefix() string {
    return "mynet"
}

func (n *MyNetwork) BaseDenom() string {
    return "umytoken"
}

func (n *MyNetwork) GenesisConfig() network.GenesisConfig {
    return network.GenesisConfig{
        ChainIDPattern:    "mynetwork-{type}-{num}",
        EVMChainID:        1234,
        BaseDenom:         "umytoken",
        DenomExponent:     18,
        DisplayDenom:      "MYTOKEN",
        BondDenom:         "umytoken",
        MinSelfDelegation: "1",
        UnbondingTime:     120 * time.Second,
        MaxValidators:     100,
        MinDeposit:        "10000000umytoken",
        VotingPeriod:      60 * time.Second,
        MaxDepositPeriod:  120 * time.Second,
        CommunityTax:      "0.02",
    }
}

func (n *MyNetwork) DefaultPorts() network.PortConfig {
    return network.PortConfig{
        RPC:       26657,
        P2P:       26656,
        GRPC:      9090,
        GRPCWeb:   9091,
        API:       1317,
        EVMRPC:    8545,  // Set to 0 if no EVM
        EVMSocket: 8546,  // Set to 0 if no EVM
    }
}

// ============================================
// Docker Configuration
// ============================================

func (n *MyNetwork) DockerImage() string {
    return "ghcr.io/myorg/mynetwork"
}

func (n *MyNetwork) DockerImageTag(version string) string {
    return version
}

func (n *MyNetwork) DockerHomeDir() string {
    return "/home/mynetwork"
}

// ============================================
// Path Configuration
// ============================================

func (n *MyNetwork) DefaultNodeHome() string {
    return "/root/.mynetwork"
}

func (n *MyNetwork) PIDFileName() string {
    return "mynetworkd.pid"
}

func (n *MyNetwork) LogFileName() string {
    return "mynetworkd.log"
}

func (n *MyNetwork) ProcessPattern() string {
    return "mynetworkd.*start"
}

// ============================================
// Command Generation
// ============================================

func (n *MyNetwork) InitCommand(homeDir, chainID, moniker string) []string {
    return []string{
        "init", moniker,
        "--chain-id", chainID,
        "--home", homeDir,
    }
}

func (n *MyNetwork) StartCommand(homeDir string, networkMode string) []string {
    args := []string{
        "start",
        "--home", homeDir,
    }
    // Add chain-id based on network mode if specified
    if networkMode == "mainnet" {
        args = append(args, "--chain-id", "mychain-1")
    } else if networkMode == "testnet" {
        args = append(args, "--chain-id", "mychain-testnet-1")
    }
    return args
}

func (n *MyNetwork) ExportCommand(homeDir string) []string {
    return []string{
        "export",
        "--home", homeDir,
    }
}

// ============================================
// Devnet Operations
// ============================================

func (n *MyNetwork) ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
    // Parse genesis JSON, apply modifications, return modified JSON
    // Common modifications:
    // - Update chain_id
    // - Reduce governance voting period for faster testing
    // - Adjust staking parameters
    // - Add/modify validator set
    return genesis, nil
}

func (n *MyNetwork) GenerateDevnet(ctx context.Context, config network.GeneratorConfig, genesisFile string) error {
    // Generate validator keys, accounts, and genesis transactions
    // This is called during devnet provisioning
    return nil
}

func (n *MyNetwork) DefaultGeneratorConfig() network.GeneratorConfig {
    return network.GeneratorConfig{
        NumValidators:    4,
        NumAccounts:      10,
        AccountBalance:   "100000000000umytoken",
        ValidatorBalance: "1000000000000umytoken",
        ValidatorStake:   "100000000umytoken",
        OutputDir:        "./devnet",
        ChainID:          "mynetwork-devnet-1",
    }
}

// ============================================
// Codec
// ============================================

func (n *MyNetwork) GetCodec() ([]byte, error) {
    return nil, nil  // Return nil for standard Cosmos SDK codec
}

// ============================================
// Validation
// ============================================

func (n *MyNetwork) Validate() error {
    if n.Name() == "" {
        return fmt.Errorf("network name is required")
    }
    if n.BinaryName() == "" {
        return fmt.Errorf("binary name is required")
    }
    return nil
}

// ============================================
// Snapshot Configuration
// ============================================

func (n *MyNetwork) SnapshotURL(networkType string) string {
    switch networkType {
    case "mainnet":
        return "https://snapshots.mynetwork.io/mainnet/latest.tar.zst"
    case "testnet":
        return "https://snapshots.mynetwork.io/testnet/latest.tar.zst"
    default:
        return ""
    }
}

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

func (n *MyNetwork) AvailableNetworks() []string {
    return []string{"mainnet", "testnet"}
}

// ============================================
// Node Configuration
// ============================================

func (n *MyNetwork) GetConfigOverrides(nodeIndex int, opts network.NodeConfigOptions) ([]byte, []byte, error) {
    // Return TOML overrides for config.toml and app.toml
    // Return nil, nil, nil to use defaults

    // Example: Enable EVM JSON-RPC for EVM-compatible chains
    appToml := []byte(fmt.Sprintf(`
[json-rpc]
enable = true
address = "0.0.0.0:%d"

[json-rpc.ws]
address = "0.0.0.0:%d"
`, opts.Ports.EVMRPC, opts.Ports.EVMSocket))

    return nil, appToml, nil
}
```

### Step 3: Create Main Entry Point

```go
// main.go
package main

import (
    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

func main() {
    plugin.Serve(&MyNetwork{})
}
```

### Step 4: Build the Plugin

```bash
# For V1 (devnet-builder)
go build -o devnet-mynetwork .

# For V2 (dvb/devnetd)
go build -o mynetwork-plugin .

# For both (recommended: create both binaries)
go build -o devnet-mynetwork .
cp devnet-mynetwork mynetwork-plugin
```

### Step 5: Install the Plugin

```bash
# Create plugin directory if it doesn't exist
mkdir -p ~/.devnet-builder/plugins

# Install for V1
cp devnet-mynetwork ~/.devnet-builder/plugins/

# Install for V2
cp mynetwork-plugin ~/.devnet-builder/plugins/

# Make executable
chmod +x ~/.devnet-builder/plugins/*
```

### Step 6: Verify Installation

```bash
# V1 - List available networks
devnet-builder networks

# V2 - List loaded plugins
dvb plugins list
```

## Plugin Discovery

### V1 Discovery (devnet-builder)

V1 discovers plugins from a single directory:

```
~/.devnet-builder/plugins/
├── devnet-stable       # Stable network plugin
├── devnet-osmosis      # Osmosis network plugin
└── devnet-cosmos       # Cosmos Hub plugin
```

**Naming Convention:** `devnet-<network>`

### V2 Discovery (dvb/devnetd)

V2 discovers plugins from multiple directories (in order of priority):

1. `./plugins/` - Current working directory
2. `~/.devnet-builder/plugins/` - User plugin directory
3. `/usr/local/lib/devnet-builder/plugins/` - System plugin directory

**Naming Convention:** `<network>-plugin`

```
~/.devnet-builder/plugins/
├── stable-plugin       # Stable network plugin
├── osmosis-plugin      # Osmosis network plugin
└── cosmos-plugin       # Cosmos Hub plugin
```

## Makefile Targets

If you're developing plugins within the devnet-builder repository:

```bash
# Build all public plugins
make plugins

# Build all private plugins
make plugins-private

# Build all plugins (public + private)
make plugins-all

# Build a specific plugin
make plugin-mynetwork

# List available plugins
make list-plugins

# Generate protobuf for plugin SDK
make proto-gen
```

## gRPC Protocol Details

### Handshake Configuration

Both V1 and V2 use the same handshake:

```go
var Handshake = plugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "DEVNET_BUILDER_PLUGIN",
    MagicCookieValue: "network_module_v1",
}
```

### Protocol Buffer Definition

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
    rpc DefaultBinaryVersion(Empty) returns (StringResponse);
    rpc GetBuildConfig(BuildConfigRequest) returns (BuildConfigResponse);

    // Chain Configuration
    rpc Bech32Prefix(Empty) returns (StringResponse);
    rpc BaseDenom(Empty) returns (StringResponse);
    rpc GenesisConfig(Empty) returns (GenesisConfigResponse);
    rpc DefaultPorts(Empty) returns (PortConfigResponse);

    // Docker
    rpc DockerImage(Empty) returns (StringResponse);
    rpc DockerImageTag(StringRequest) returns (StringResponse);
    rpc DockerHomeDir(Empty) returns (StringResponse);

    // Commands
    rpc InitCommand(InitCommandRequest) returns (StringListResponse);
    rpc StartCommand(StringRequest) returns (StringListResponse);
    rpc ExportCommand(StringRequest) returns (StringListResponse);

    // Operations
    rpc ModifyGenesis(ModifyGenesisRequest) returns (BytesResponse);
    rpc ModifyGenesisFile(ModifyGenesisFileRequest) returns (ModifyGenesisFileResponse);
    rpc GenerateDevnet(GenerateDevnetRequest) returns (ErrorResponse);

    // RPC Operations (plugin-delegated)
    rpc GetGovernanceParams(GovernanceParamsRequest) returns (GovernanceParamsResponse);
    rpc GetBlockHeight(BlockHeightRequest) returns (BlockHeightResponse);
    rpc WaitForBlock(WaitForBlockRequest) returns (WaitForBlockResponse);
    rpc GetProposal(ProposalRequest) returns (ProposalResponse);
    rpc GetUpgradePlan(UpgradePlanRequest) returns (UpgradePlanResponse);
    // ... and more
}
```

## Best Practices

### 1. Version Your Plugin

Use semantic versioning and implement `Version()` correctly:

```go
func (n *MyNetwork) Version() string {
    return "1.2.3"  // Major.Minor.Patch
}
```

### 2. Validate Thoroughly

Implement comprehensive validation:

```go
func (n *MyNetwork) Validate() error {
    if n.Name() == "" {
        return fmt.Errorf("name is required")
    }
    if n.BinaryName() == "" {
        return fmt.Errorf("binary name is required")
    }
    if n.Bech32Prefix() == "" {
        return fmt.Errorf("bech32 prefix is required")
    }
    if n.BaseDenom() == "" {
        return fmt.Errorf("base denom is required")
    }
    return nil
}
```

### 3. Support Multiple Network Types

Handle mainnet, testnet, and devnet configurations:

```go
func (n *MyNetwork) SnapshotURL(networkType string) string {
    urls := map[string]string{
        "mainnet": "https://snapshots.example.com/mainnet/latest.tar.zst",
        "testnet": "https://snapshots.example.com/testnet/latest.tar.zst",
    }
    return urls[networkType]
}
```

### 4. Use File-Based Genesis for Large Files

For networks with large genesis files (50MB+):

```go
// Implement FileBasedGenesisModifier interface
func (n *MyNetwork) ModifyGenesisFile(inputPath, outputPath string, opts network.GenesisOptions) (int64, error) {
    // Stream-process the file to avoid memory issues
    // ...
    return outputSize, nil
}
```

### 5. Handle RPC Timeouts Gracefully

For RPC query methods, handle timeouts:

```go
func (n *MyNetwork) GetBlockHeight(ctx context.Context, rpcEndpoint string) (*plugin.BlockHeightResponse, error) {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Query with timeout...

    select {
    case <-ctx.Done():
        return &plugin.BlockHeightResponse{
            Error: "request timed out",
        }, nil
    default:
        // Continue processing
    }
}
```

## Troubleshooting

### Plugin Not Loading

```bash
# Check plugin exists and is executable
ls -la ~/.devnet-builder/plugins/

# Test plugin directly (should hang waiting for handshake)
./~/.devnet-builder/plugins/mynetwork-plugin

# Check daemon logs
devnetd start --log-level debug
```

### Version Mismatch

```bash
# Check plugin version
dvb plugins info mynetwork

# Check devnet-builder version
devnet-builder version
dvb version
```

### gRPC Errors

Enable debug logging:

```bash
# V1
devnet-builder --log-level debug deploy --network mynetwork

# V2
devnetd start --log-level debug
```

### Plugin Crashes

Check the plugin process:

```bash
# Find plugin processes
ps aux | grep mynetwork-plugin

# Check for core dumps
ls -la /var/crash/ 2>/dev/null || ls -la ~/Library/Logs/DiagnosticReports/ 2>/dev/null
```

## Example Plugins

See working examples in the repository:

- `examples/cosmos-plugin/` - Complete Cosmos Hub plugin example
- `pkg/network/stable/` - Production Stable network plugin
- `pkg/network/osmosis/` - Production Osmosis plugin (if available)

## See Also

- [V2 Plugin Development](v2/plugins.md) - V2-specific plugin features
- [Configuration Guide](configuration.md) - Plugin configuration options
- [Workflows](workflows.md) - Using plugins in workflows
- [API Reference](v2/api-reference.md) - gRPC API documentation
