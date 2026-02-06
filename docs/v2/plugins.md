# V2 Plugin Development Guide

Comprehensive guide for developing network plugins for the devnetd daemon and dvb CLI.

## Overview

The V2 plugin system provides advanced features for managing blockchain network plugins:

- **Multi-Directory Discovery** - Plugins loaded from multiple locations
- **Hot Reload** - Update plugins without restarting the daemon
- **Version Constraints** - Semver-based compatibility checking
- **Lifecycle Management** - Proper startup, shutdown, and resource cleanup
- **Concurrent Loading** - Parallel plugin initialization

This guide focuses on V2-specific features. For general plugin development, see the [Plugin System Guide](../plugins.md).

## Adding Plugins to V2

V2 uses **automatic plugin discovery** - no registration or configuration files required. Simply place the plugin binary in the right location and it's immediately available.

### Quick Start: Install a Plugin

```bash
# 1. Create the plugins directory (if it doesn't exist)
mkdir -p ~/.devnet-builder/plugins

# 2. Copy your plugin binary
cp mynetwork-plugin ~/.devnet-builder/plugins/

# 3. Make it executable
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin

# 4. Verify it's discovered
dvb plugins list
# Output should include: mynetwork

# 5. Use it in your devnet
dvb provision --network mynetwork --validators 4
```

That's it! No daemon restart, no configuration files, no registration commands.

### Plugin Discovery Directories

V2 searches for plugins in these locations (in order of priority):

| Location | Purpose | Example |
|----------|---------|---------|
| `./plugins/` | Project-local plugins | `./plugins/mynetwork-plugin` |
| `~/.devnet-builder/plugins/` | User plugins (recommended) | `~/.devnet-builder/plugins/mynetwork-plugin` |
| `/usr/local/lib/devnet-builder/plugins/` | System-wide plugins | `/usr/local/lib/devnet-builder/plugins/mynetwork-plugin` |

**Priority:** If the same plugin exists in multiple directories, the first one found (in order above) takes precedence.

### Plugin Naming Convention

**V2 plugins MUST follow this naming pattern:**

```
{network-name}-plugin
```

| ✅ Correct | ❌ Wrong |
|-----------|----------|
| `stable-plugin` | `devnet-stable` |
| `cosmos-plugin` | `cosmos` |
| `osmosis-plugin` | `osmosis-devnet-plugin` |
| `mynetwork-plugin` | `plugin-mynetwork` |

The `{network-name}` part becomes the identifier you use in commands and YAML specs.

### Installation Methods

#### Method 1: Manual Installation (Recommended)

```bash
# Download or build your plugin
wget https://example.com/releases/mynetwork-plugin-linux-amd64
# OR
go build -o mynetwork-plugin ./cmd/plugin/

# Install to user plugins directory
mkdir -p ~/.devnet-builder/plugins
mv mynetwork-plugin ~/.devnet-builder/plugins/
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin
```

#### Method 2: From Release Archive

```bash
# Download release archive
wget https://github.com/myorg/mynetwork/releases/download/v1.0.0/mynetwork-plugin.tar.gz

# Extract to plugins directory
tar -xzf mynetwork-plugin.tar.gz -C ~/.devnet-builder/plugins/
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin
```

#### Method 3: Build from Source

```bash
# Clone and build
git clone https://github.com/myorg/mynetwork-plugin
cd mynetwork-plugin
go build -o mynetwork-plugin .

# Install
cp mynetwork-plugin ~/.devnet-builder/plugins/
```

### Verifying Plugin Installation

```bash
# List all discovered plugins
dvb plugins list

# Get detailed info about a specific plugin
dvb plugins info mynetwork

# Check plugin version
dvb plugins info mynetwork | grep version

# Test plugin by creating a devnet
dvb provision --network mynetwork --validators 2 --dry-run
```

### Using the Plugin

Once installed, reference the plugin by its network name:

**CLI Usage:**
```bash
# Provision command
dvb provision --network mynetwork --validators 4

# Apply YAML spec
dvb apply -f devnet.yaml
```

**YAML Specification:**
```yaml
apiVersion: v1
kind: Devnet
metadata:
  name: my-devnet
  namespace: default
spec:
  plugin: mynetwork    # References mynetwork-plugin binary
  validators: 4
  fullNodes: 0
  mode: docker
```

### Troubleshooting Plugin Installation

**Plugin not found:**
```bash
# Check plugin exists and is named correctly
ls -la ~/.devnet-builder/plugins/
# Should show: mynetwork-plugin (not devnet-mynetwork)

# Check file is executable
file ~/.devnet-builder/plugins/mynetwork-plugin
# Should show: executable

# Check permissions
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin
```

**Plugin fails to load:**
```bash
# Test plugin binary directly (should hang waiting for handshake)
~/.devnet-builder/plugins/mynetwork-plugin
# Ctrl+C to exit

# Check for missing dependencies
ldd ~/.devnet-builder/plugins/mynetwork-plugin
```

**Version mismatch:**
```bash
# Plugin version must be valid semver
# Good: 1.0.0, 2.3.1
# Bad: v1.0.0, 1.0

# Check current constraints
dvb plugins info mynetwork
```

## Plugin Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           devnetd Daemon                             │
├─────────────────────────────────────────────────────────────────────┤
│                     Plugin Loader (pkg/network/plugin/)              │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────────────┐ │
│  │   Discover  │  │    Load     │  │  Version Constraint Checker  │ │
│  └──────┬──────┘  └──────┬──────┘  └──────────────────────────────┘ │
│         │                │                                           │
│  ┌──────▼──────────────▼─────────────────────────────────────────┐ │
│  │                    Plugin Registry                             │ │
│  │  map[string]*PluginClient                                      │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
            │                    │                    │
            ▼                    ▼                    ▼
     ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
     │stable-plugin │    │osmosis-plugin│    │cosmos-plugin │
     │ (subprocess) │    │ (subprocess) │    │ (subprocess) │
     └──────────────┘    └──────────────┘    └──────────────┘
```

## Core Interface

All V2 plugins implement the `network.Module` interface from `pkg/network/interface.go`:

```go
package network

import (
    "context"
    "time"
)

// Module is the interface that all network plugins must implement.
type Module interface {
    // ============================================
    // Identity Methods
    // ============================================

    // Name returns the unique identifier for this network.
    // Must be lowercase, alphanumeric with hyphens only.
    // Examples: "stable", "osmosis", "cosmos-hub"
    Name() string

    // DisplayName returns a human-readable name.
    // Examples: "Stable Network", "Osmosis DEX"
    DisplayName() string

    // Version returns the module version (semantic versioning).
    // Examples: "1.0.0", "2.3.1"
    Version() string

    // ============================================
    // Binary Configuration
    // ============================================

    // BinaryName returns the network CLI binary name.
    // Examples: "stabled", "osmosisd", "gaiad"
    BinaryName() string

    // BinarySource returns configuration for acquiring the binary.
    BinarySource() BinarySource

    // DefaultBinaryVersion returns the default version to use.
    // Examples: "v1.1.3", "latest"
    DefaultBinaryVersion() string

    // GetBuildConfig returns network-specific build configuration.
    // Parameters:
    //   - networkType: "mainnet", "testnet", "devnet"
    // Returns custom build tags, ldflags, and environment variables.
    GetBuildConfig(networkType string) (*BuildConfig, error)

    // ============================================
    // Chain Configuration
    // ============================================

    // DefaultChainID returns the default chain ID for devnets.
    // Deprecated: Chain ID extracted from genesis. Return empty string.
    DefaultChainID() string

    // Bech32Prefix returns the address prefix.
    // Examples: "stable", "osmo", "cosmos"
    Bech32Prefix() string

    // BaseDenom returns the base token denomination.
    // Examples: "ustable", "uosmo", "uatom"
    BaseDenom() string

    // GenesisConfig returns default genesis parameters.
    GenesisConfig() GenesisConfig

    // DefaultPorts returns the default port configuration.
    DefaultPorts() PortConfig

    // ============================================
    // Docker Configuration
    // ============================================

    // DockerImage returns the Docker image name.
    // Example: "ghcr.io/stablelabs/stable"
    DockerImage() string

    // DockerImageTag returns the Docker tag for a version.
    DockerImageTag(version string) string

    // DockerHomeDir returns the home directory in containers.
    DockerHomeDir() string

    // ============================================
    // Path Configuration
    // ============================================

    DefaultNodeHome() string    // Default node home (e.g., "/root/.stabled")
    PIDFileName() string        // PID file name (e.g., "stabled.pid")
    LogFileName() string        // Log file name (e.g., "stabled.log")
    ProcessPattern() string     // Regex for process matching

    // ============================================
    // Command Generation
    // ============================================

    // InitCommand returns node initialization arguments.
    InitCommand(homeDir, chainID, moniker string) []string

    // StartCommand returns node start arguments.
    StartCommand(homeDir string) []string

    // ExportCommand returns state export arguments.
    ExportCommand(homeDir string) []string

    // ============================================
    // Devnet Operations
    // ============================================

    // ModifyGenesis applies network-specific genesis modifications.
    ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error)

    // GenerateDevnet generates validators, accounts, and genesis.
    GenerateDevnet(ctx context.Context, config GeneratorConfig, genesisFile string) error

    // DefaultGeneratorConfig returns default devnet generation config.
    DefaultGeneratorConfig() GeneratorConfig

    // ============================================
    // Codec
    // ============================================

    // GetCodec returns network-specific codec configuration.
    GetCodec() ([]byte, error)

    // ============================================
    // Validation
    // ============================================

    // Validate checks if module configuration is valid.
    Validate() error

    // ============================================
    // Snapshot Configuration
    // ============================================

    // SnapshotURL returns snapshot download URL for network type.
    SnapshotURL(networkType string) string

    // RPCEndpoint returns RPC endpoint for network type.
    RPCEndpoint(networkType string) string

    // AvailableNetworks returns supported network types.
    AvailableNetworks() []string

    // ============================================
    // Node Configuration
    // ============================================

    // GetConfigOverrides returns TOML config overrides for a node.
    // Returns config.toml and app.toml partial overrides.
    GetConfigOverrides(nodeIndex int, opts NodeConfigOptions) (configToml, appToml []byte, err error)
}
```

## Supporting Types

### BinarySource

```go
type BinarySource struct {
    Type      string   `json:"type"`       // "github", "local", "docker"
    Owner     string   `json:"owner"`      // GitHub owner/org
    Repo      string   `json:"repo"`       // GitHub repository
    AssetName string   `json:"asset_name"` // Release asset pattern
    LocalPath string   `json:"local_path"` // Path to local binary
    BuildTags []string `json:"build_tags"` // Go build tags
}
```

### BuildConfig

```go
type BuildConfig struct {
    Tags      []string          `json:"tags"`       // Go build tags
    LDFlags   []string          `json:"ldflags"`    // Linker flags
    Env       map[string]string `json:"env"`        // Environment variables
    ExtraArgs []string          `json:"extra_args"` // Additional build args
}
```

### PortConfig

```go
type PortConfig struct {
    RPC       int `json:"rpc"`        // Tendermint RPC (26657)
    P2P       int `json:"p2p"`        // P2P networking (26656)
    GRPC      int `json:"grpc"`       // gRPC server (9090)
    GRPCWeb   int `json:"grpc_web"`   // gRPC-Web (9091)
    API       int `json:"api"`        // REST API (1317)
    EVMRPC    int `json:"evm_rpc"`    // EVM JSON-RPC (8545)
    EVMSocket int `json:"evm_socket"` // EVM WebSocket (8546)
}
```

### GenesisConfig

```go
type GenesisConfig struct {
    ChainIDPattern    string        `json:"chain_id_pattern"`
    EVMChainID        int64         `json:"evm_chain_id"`
    BaseDenom         string        `json:"base_denom"`
    DenomExponent     int           `json:"denom_exponent"`
    DisplayDenom      string        `json:"display_denom"`
    BondDenom         string        `json:"bond_denom"`
    MinSelfDelegation string        `json:"min_self_delegation"`
    UnbondingTime     time.Duration `json:"unbonding_time"`
    MaxValidators     uint32        `json:"max_validators"`
    MinDeposit        string        `json:"min_deposit"`
    VotingPeriod      time.Duration `json:"voting_period"`
    MaxDepositPeriod  time.Duration `json:"max_deposit_period"`
    CommunityTax      string        `json:"community_tax"`
}
```

## Optional Interfaces

### FileBasedGenesisModifier

For genesis files exceeding gRPC message size limits (4MB default):

```go
type FileBasedGenesisModifier interface {
    // ModifyGenesisFile processes large genesis files via filesystem.
    // Used for fork-based devnets with 50-100+ MB genesis files.
    ModifyGenesisFile(inputPath, outputPath string, opts GenesisOptions) (outputSize int64, err error)
}
```

**When to implement:** Fork devnets from mainnet/testnet with large state exports.

### StateExporter

For snapshot-based devnet creation:

```go
type StateExporter interface {
    // ExportCommandWithOptions returns export arguments with options.
    ExportCommandWithOptions(homeDir string, opts ExportOptions) []string

    // ValidateExportedGenesis validates the exported genesis.
    ValidateExportedGenesis(genesis []byte) error

    // RequiredModules returns modules that must be present.
    RequiredModules() []string

    // SnapshotFormat returns the archive format.
    SnapshotFormat(networkType string) SnapshotFormat
}
```

## Creating a V2 Plugin

### Step 1: Project Structure

```
mynetwork-plugin/
├── main.go              # Entry point with plugin.Serve()
├── module.go            # network.Module implementation
├── genesis.go           # Genesis modification logic
├── rpc.go               # RPC query implementations
├── config.go            # Configuration generation
├── go.mod
└── go.sum
```

### Step 2: Main Entry Point

```go
// main.go
package main

import (
    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

func main() {
    // Serve registers the plugin with the gRPC server
    // and handles the HashiCorp go-plugin handshake
    plugin.Serve(&MyNetwork{})
}
```

### Step 3: Implement network.Module

```go
// module.go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/altuslabsxyz/devnet-builder/pkg/network"
)

type MyNetwork struct{}

var _ network.Module = (*MyNetwork)(nil)

// Identity
func (n *MyNetwork) Name() string        { return "mynetwork" }
func (n *MyNetwork) DisplayName() string { return "My Network" }
func (n *MyNetwork) Version() string     { return "1.0.0" }

// Binary Configuration
func (n *MyNetwork) BinaryName() string { return "mynetworkd" }

func (n *MyNetwork) BinarySource() network.BinarySource {
    return network.BinarySource{
        Type:      "github",
        Owner:     "myorg",
        Repo:      "mynetwork",
        AssetName: "mynetworkd-*-linux-amd64",
    }
}

func (n *MyNetwork) DefaultBinaryVersion() string { return "v1.0.0" }

func (n *MyNetwork) GetBuildConfig(networkType string) (*network.BuildConfig, error) {
    switch networkType {
    case "mainnet":
        return &network.BuildConfig{
            Tags:    []string{"netgo", "ledger"},
            LDFlags: []string{"-X main.Version=1.0.0"},
        }, nil
    default:
        return &network.BuildConfig{}, nil
    }
}

// Chain Configuration
func (n *MyNetwork) DefaultChainID() string  { return "" } // Deprecated
func (n *MyNetwork) Bech32Prefix() string    { return "mynet" }
func (n *MyNetwork) BaseDenom() string       { return "umytoken" }

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
        EVMRPC:    8545,
        EVMSocket: 8546,
    }
}

// Docker Configuration
func (n *MyNetwork) DockerImage() string                      { return "ghcr.io/myorg/mynetwork" }
func (n *MyNetwork) DockerImageTag(version string) string     { return version }
func (n *MyNetwork) DockerHomeDir() string                    { return "/home/mynetwork" }

// Path Configuration
func (n *MyNetwork) DefaultNodeHome() string  { return "/root/.mynetwork" }
func (n *MyNetwork) PIDFileName() string      { return "mynetworkd.pid" }
func (n *MyNetwork) LogFileName() string      { return "mynetworkd.log" }
func (n *MyNetwork) ProcessPattern() string   { return "mynetworkd.*start" }

// Command Generation
func (n *MyNetwork) InitCommand(homeDir, chainID, moniker string) []string {
    return []string{"init", moniker, "--chain-id", chainID, "--home", homeDir}
}

func (n *MyNetwork) StartCommand(homeDir string, networkMode string) []string {
    args := []string{"start", "--home", homeDir}
    // Add chain-id based on network mode
    if networkMode == "mainnet" {
        args = append(args, "--chain-id", "mychain-1")
    } else if networkMode == "testnet" {
        args = append(args, "--chain-id", "mychain-testnet-1")
    }
    return args
}

func (n *MyNetwork) ExportCommand(homeDir string) []string {
    return []string{"export", "--home", homeDir}
}

// Devnet Operations
func (n *MyNetwork) ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
    // Implement genesis modification
    return genesis, nil
}

func (n *MyNetwork) GenerateDevnet(ctx context.Context, config network.GeneratorConfig, genesisFile string) error {
    // Implement devnet generation
    return nil
}

func (n *MyNetwork) DefaultGeneratorConfig() network.GeneratorConfig {
    return network.GeneratorConfig{
        NumValidators:    4,
        NumAccounts:      10,
        AccountBalance:   "100000000000umytoken",
        ValidatorBalance: "1000000000000umytoken",
        ValidatorStake:   "100000000umytoken",
        ChainID:          "mynetwork-devnet-1",
    }
}

// Codec
func (n *MyNetwork) GetCodec() ([]byte, error) { return nil, nil }

// Validation
func (n *MyNetwork) Validate() error {
    if n.Name() == "" || n.BinaryName() == "" {
        return fmt.Errorf("name and binary name are required")
    }
    return nil
}

// Snapshot Configuration
func (n *MyNetwork) SnapshotURL(networkType string) string {
    urls := map[string]string{
        "mainnet": "https://snapshots.mynetwork.io/mainnet/latest.tar.zst",
        "testnet": "https://snapshots.mynetwork.io/testnet/latest.tar.zst",
    }
    return urls[networkType]
}

func (n *MyNetwork) RPCEndpoint(networkType string) string {
    endpoints := map[string]string{
        "mainnet": "https://rpc.mynetwork.io",
        "testnet": "https://rpc-testnet.mynetwork.io",
    }
    return endpoints[networkType]
}

func (n *MyNetwork) AvailableNetworks() []string {
    return []string{"mainnet", "testnet"}
}

// Node Configuration
func (n *MyNetwork) GetConfigOverrides(nodeIndex int, opts network.NodeConfigOptions) ([]byte, []byte, error) {
    // Return TOML overrides for EVM-enabled chains
    appToml := []byte(fmt.Sprintf(`
[json-rpc]
enable = true
address = "0.0.0.0:%d"
`, opts.Ports.EVMRPC))
    return nil, appToml, nil
}
```

### Step 4: Build and Install

```bash
# Build for V2
go build -o mynetwork-plugin .

# Install to plugin directory
mkdir -p ~/.devnet-builder/plugins
cp mynetwork-plugin ~/.devnet-builder/plugins/
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin

# Verify
dvb plugins list
```

## V2-Specific Features

### Plugin Discovery

V2 discovers plugins from multiple directories:

```go
// Default search paths (in order of priority)
[]string{
    "./plugins",                              // Current directory
    "~/.devnet-builder/plugins",              // User directory
    "/usr/local/lib/devnet-builder/plugins",  // System directory
}
```

**Naming Convention:** `<network>-plugin` (e.g., `stable-plugin`, `osmosis-plugin`)

### Version Constraints

V2 supports semantic versioning constraints:

```go
// Loader with version constraint
loader := plugin.NewLoader(
    plugin.WithVersionConstraint(plugin.VersionConstraint{
        MinVersion: "1.0.0",
        MaxVersion: "2.0.0",
    }),
)

// Check specific plugin version
version, err := loader.GetPluginVersion("mynetwork")

// Validate plugin version without permanent load
err := loader.ValidatePlugin("mynetwork")
```

### Hot Reload

Plugins can be reloaded without restarting the daemon:

```bash
# Reload a specific plugin
dvb plugins reload mynetwork

# Reload all plugins
dvb plugins reload --all
```

Programmatic reload:

```go
loader := plugin.NewLoader()

// Reload a specific plugin
err := loader.Reload("mynetwork")

// Unload and reload
loader.UnloadPlugin("mynetwork")
err := loader.Load("mynetwork")
```

### Concurrent Loading

V2 loads plugins in parallel for faster startup:

```go
loader := plugin.NewLoader()

// Load all plugins concurrently
result := loader.LoadAllWithErrors()

for name, err := range result.Errors {
    log.Printf("Failed to load %s: %v", name, err)
}

for _, name := range result.Loaded {
    log.Printf("Loaded: %s", name)
}
```

### Plugin Lifecycle

```go
type PluginClient struct {
    client *hcplugin.Client
    module network.Module
    name   string
}

// Lifecycle methods
loader.Load("mynetwork")           // Start plugin process
loader.IsLoaded("mynetwork")       // Check if loaded
loader.LoadedPlugins()             // List loaded plugins
loader.UnloadPlugin("mynetwork")   // Stop plugin process
loader.Reload("mynetwork")         // Unload and reload
```

## RPC Query Methods

V2 plugins can implement blockchain RPC queries for upgrade workflows:

```go
// Governance Parameters
func (n *MyNetwork) GetGovernanceParams(rpcEndpoint, networkType string) (*plugin.GovernanceParamsResponse, error) {
    // Query /cosmos/gov/v1/params/voting and /cosmos/gov/v1/params/deposit
    return &plugin.GovernanceParamsResponse{
        VotingPeriodNs:          int64(60 * time.Second),
        ExpeditedVotingPeriodNs: int64(30 * time.Second),
        MinDeposit:              "10000000umytoken",
        ExpeditedMinDeposit:     "50000000umytoken",
    }, nil
}

// Block Height
func (n *MyNetwork) GetBlockHeight(ctx context.Context, rpcEndpoint string) (*plugin.BlockHeightResponse, error) {
    // Query /status endpoint
    return &plugin.BlockHeightResponse{Height: 12345}, nil
}

// Wait for Block
func (n *MyNetwork) WaitForBlock(ctx context.Context, rpcEndpoint string, targetHeight int64, timeoutMs int64) (*plugin.WaitForBlockResponse, error) {
    // Poll until target height reached
    return &plugin.WaitForBlockResponse{
        CurrentHeight: targetHeight,
        Reached:       true,
    }, nil
}

// Governance Proposal
func (n *MyNetwork) GetProposal(ctx context.Context, rpcEndpoint string, proposalID uint64) (*plugin.ProposalResponse, error) {
    // Query /cosmos/gov/v1/proposals/{id}
    return &plugin.ProposalResponse{
        Id:     proposalID,
        Status: "PROPOSAL_STATUS_VOTING_PERIOD",
    }, nil
}

// Upgrade Plan
func (n *MyNetwork) GetUpgradePlan(ctx context.Context, rpcEndpoint string) (*plugin.UpgradePlanResponse, error) {
    // Query /cosmos/upgrade/v1beta1/current_plan
    return &plugin.UpgradePlanResponse{HasPlan: false}, nil
}
```

## Testing Plugins

### Unit Tests

```go
func TestModuleInterface(t *testing.T) {
    m := &MyNetwork{}

    // Test identity
    assert.Equal(t, "mynetwork", m.Name())
    assert.Equal(t, "1.0.0", m.Version())

    // Test validation
    assert.NoError(t, m.Validate())

    // Test build config
    cfg, err := m.GetBuildConfig("mainnet")
    assert.NoError(t, err)
    assert.Contains(t, cfg.Tags, "netgo")
}

func TestGenesisModification(t *testing.T) {
    m := &MyNetwork{}

    genesis := []byte(`{"chain_id":"test-1"}`)
    opts := network.GenesisOptions{ChainID: "modified-1"}

    modified, err := m.ModifyGenesis(genesis, opts)
    assert.NoError(t, err)
    assert.Contains(t, string(modified), "modified-1")
}
```

### Integration Tests

```bash
# Deploy with your plugin
dvb apply -f - <<EOF
apiVersion: v1
kind: Devnet
metadata:
  name: test-devnet
  namespace: default
spec:
  plugin: mynetwork
  validators: 2
EOF

# Verify deployment
dvb get devnets
dvb describe devnet test-devnet

# Check node health
dvb node health -n default test-devnet

# Cleanup
dvb delete devnet test-devnet
```

## Debugging

### Enable Debug Logging

```bash
# Start daemon with debug logging
devnetd start --log-level debug

# View plugin-specific logs
dvb daemon logs | grep mynetwork
```

### Test Plugin Directly

```bash
# Plugin should hang waiting for handshake when run directly
./mynetwork-plugin
# Expected: No output (waiting for gRPC handshake)
# Ctrl+C to exit

# Check if plugin responds to handshake
DEVNET_BUILDER_PLUGIN=network_module_v1 ./mynetwork-plugin
```

### Common Issues

**Plugin not discovered:**
```bash
# Check naming convention
ls ~/.devnet-builder/plugins/
# Should be: mynetwork-plugin (not devnet-mynetwork for V2)

# Check permissions
chmod +x ~/.devnet-builder/plugins/mynetwork-plugin
```

**Version mismatch:**
```bash
# Check plugin version
dvb plugins info mynetwork

# Ensure Version() returns valid semver
# Good: "1.0.0", "2.3.1-beta"
# Bad: "v1.0.0", "1.0" (missing patch)
```

**gRPC errors:**
```bash
# Check if port is in use
lsof -i :50051

# Restart daemon
dvb daemon stop
devnetd start
```

## Best Practices

### 1. Semantic Versioning

Always return proper semver from `Version()`:

```go
func (n *MyNetwork) Version() string {
    return "1.2.3"  // Major.Minor.Patch
}
```

### 2. Error Handling in RPC Methods

Return errors in response, not as Go errors:

```go
func (n *MyNetwork) GetBlockHeight(ctx context.Context, rpcEndpoint string) (*plugin.BlockHeightResponse, error) {
    height, err := queryHeight(rpcEndpoint)
    if err != nil {
        return &plugin.BlockHeightResponse{
            Height: 0,
            Error:  err.Error(),  // Return error in response
        }, nil  // Return nil Go error
    }
    return &plugin.BlockHeightResponse{Height: height}, nil
}
```

### 3. Timeout Handling

Always respect context deadlines:

```go
func (n *MyNetwork) WaitForBlock(ctx context.Context, rpcEndpoint string, target int64, timeoutMs int64) (*plugin.WaitForBlockResponse, error) {
    deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

    for time.Now().Before(deadline) {
        select {
        case <-ctx.Done():
            return &plugin.WaitForBlockResponse{
                Error: "context cancelled",
            }, nil
        default:
            height, _ := queryHeight(rpcEndpoint)
            if height >= target {
                return &plugin.WaitForBlockResponse{
                    CurrentHeight: height,
                    Reached:       true,
                }, nil
            }
            time.Sleep(time.Second)
        }
    }

    return &plugin.WaitForBlockResponse{
        Error: "timeout waiting for block",
    }, nil
}
```

### 4. Resource Cleanup

Plugins should handle cleanup gracefully:

```go
// The plugin framework handles process cleanup automatically
// But if you have custom resources, implement cleanup in module methods
func (n *MyNetwork) GenerateDevnet(ctx context.Context, config network.GeneratorConfig, genesisFile string) error {
    // Create temp files
    tmpDir, err := os.MkdirTemp("", "devnet-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmpDir)  // Cleanup on exit

    // ... implementation
}
```

## See Also

- [Plugin System Guide](../plugins.md) - General plugin development
- [Architecture](architecture.md) - V2 system architecture
- [API Reference](api-reference.md) - gRPC API documentation
- [Example Plugin](../../examples/cosmos-plugin/) - Complete working example
