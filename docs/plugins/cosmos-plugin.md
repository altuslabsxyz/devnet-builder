# Cosmos Plugin Extraction Guide

This document describes how to extract the cosmos-plugin from `internal/plugin/cosmos/` to create a standalone plugin binary.

## Overview

The cosmos-plugin provides base Cosmos SDK chain support. It serves as the foundation for other Cosmos-based network plugins and can be used directly for generic Cosmos SDK chains.

## Current Implementation Analysis

The cosmos plugin functionality is currently spread across three files in `internal/plugin/cosmos/`:

### 1. builder.go - Binary Building

**Purpose:** Compiles Cosmos SDK chain binaries from source.

**Key Components:**

```go
type CosmosBuilder struct {
    binaryName  string
    defaultRepo string
}
```

**Functionality:**
- `DefaultGitRepo()` - Returns default git repository URL
- `BinaryName()` - Returns expected binary name
- `DefaultBuildFlags()` - Returns default ldflags and build tags for Cosmos SDK
- `BuildBinary(ctx, opts)` - Main build function that:
  - Merges default flags with user-provided flags
  - Resolves template variables in ldflags
  - Attempts `make install` if Makefile exists
  - Falls back to direct `go build`
- `ValidateBinary(ctx, path)` - Validates binary by running `{binary} version`

**Build Flags Template:**
```go
ldflags := "-w -s " +
    "-X github.com/cosmos/cosmos-sdk/version.Name={{.BinaryName}} " +
    "-X github.com/cosmos/cosmos-sdk/version.AppName={{.BinaryName}} " +
    "-X github.com/cosmos/cosmos-sdk/version.Version={{.GitRef}} " +
    "-X github.com/cosmos/cosmos-sdk/version.Commit={{.GitCommit}}"
tags := "netgo ledger"
```

### 2. genesis.go - Genesis Handling

**Purpose:** Validates and modifies Cosmos SDK genesis files.

**Key Components:**

```go
type CosmosGenesis struct {
    binaryName   string
    rpcEndpoints map[string]string
    snapshotURLs map[string]string
}
```

**Functionality:**
- `ValidateGenesis(genesis)` - Validates required Cosmos SDK modules exist:
  - `auth`, `bank`, `staking`, `slashing`, `gov`
- `PatchGenesis(genesis, opts)` - Applies modifications:
  - Updates `chain_id`
  - Embeds binary version metadata in `devnet_builder` field
  - Patches governance voting period
  - Patches staking unbonding time
- `ExportCommandArgs(homeDir)` - Returns export command arguments

**Genesis Patch Options:**
- ChainID modification
- BinaryVersion embedding
- VotingPeriod reduction (for faster devnet testing)
- UnbondingTime reduction

### 3. initializer.go - Node Initialization

**Purpose:** Handles Cosmos SDK node initialization commands and paths.

**Key Components:**

```go
type CosmosInitializer struct {
    binaryName string
}
```

**Functionality:**
- `BinaryName()` - Returns binary name
- `DefaultChainID()` - Returns `"devnet-1"`
- `DefaultMoniker(index)` - Returns `"validator-{index}"`
- `InitCommandArgs(homeDir, moniker, chainID)` - Returns init command:
  ```go
  []string{"init", moniker, "--chain-id", chainID, "--home", homeDir, "--overwrite"}
  ```
- `ConfigDir(homeDir)` - Returns `"{homeDir}/config"`
- `DataDir(homeDir)` - Returns `"{homeDir}/data"`
- `KeyringDir(homeDir)` - Returns `homeDir` (for test backend)

## Extraction to cosmos-plugin

### Target Structure

```
cosmos-plugin/
  main.go           # Plugin entry point
  module.go         # network.Module implementation
  builder.go        # Binary building (from internal/plugin/cosmos/builder.go)
  genesis.go        # Genesis handling (from internal/plugin/cosmos/genesis.go)
  initializer.go    # Node init (from internal/plugin/cosmos/initializer.go)
  config.go         # Config overrides
  go.mod
  go.sum
```

### Implementation Steps

#### Step 1: Create module.go

Implement `network.Module` interface using the existing components:

```go
package main

import (
    "context"
    "time"

    "github.com/altuslabsxyz/devnet-builder/pkg/network"
)

type CosmosModule struct {
    builder     *CosmosBuilder
    genesis     *CosmosGenesis
    initializer *CosmosInitializer
}

func NewCosmosModule() *CosmosModule {
    binaryName := "gaiad"
    return &CosmosModule{
        builder:     NewCosmosBuilder(binaryName, "https://github.com/cosmos/gaia"),
        genesis:     NewCosmosGenesis(binaryName),
        initializer: NewCosmosInitializer(binaryName),
    }
}

// Ensure interface compliance
var _ network.Module = (*CosmosModule)(nil)

// Identity methods
func (m *CosmosModule) Name() string        { return "cosmos" }
func (m *CosmosModule) DisplayName() string { return "Cosmos Hub" }
func (m *CosmosModule) Version() string     { return "1.0.0" }

// Binary configuration
func (m *CosmosModule) BinaryName() string { return m.initializer.BinaryName() }
func (m *CosmosModule) BinarySource() network.BinarySource {
    return network.BinarySource{
        Type:  "github",
        Owner: "cosmos",
        Repo:  "gaia",
    }
}
func (m *CosmosModule) DefaultBinaryVersion() string { return "v18.1.0" }

// Chain configuration
func (m *CosmosModule) DefaultChainID() string { return m.initializer.DefaultChainID() }
func (m *CosmosModule) Bech32Prefix() string   { return "cosmos" }
func (m *CosmosModule) BaseDenom() string      { return "uatom" }

func (m *CosmosModule) GenesisConfig() network.GenesisConfig {
    return network.GenesisConfig{
        ChainIDPattern:    "cosmos-devnet-{num}",
        BaseDenom:         "uatom",
        DenomExponent:     6,
        DisplayDenom:      "ATOM",
        BondDenom:         "uatom",
        MinSelfDelegation: "1",
        UnbondingTime:     120 * time.Second,  // Fast for devnet
        MaxValidators:     100,
        MinDeposit:        "10000000uatom",
        VotingPeriod:      60 * time.Second,   // Fast for devnet
        MaxDepositPeriod:  120 * time.Second,
        CommunityTax:      "0.02",
    }
}

func (m *CosmosModule) DefaultPorts() network.PortConfig {
    return network.PortConfig{
        RPC:     26657,
        P2P:     26656,
        GRPC:    9090,
        GRPCWeb: 9091,
        API:     1317,
    }
}

// Command generation
func (m *CosmosModule) InitCommand(homeDir, chainID, moniker string) []string {
    return m.initializer.InitCommandArgs(homeDir, moniker, chainID)
}

func (m *CosmosModule) StartCommand(homeDir string) []string {
    return []string{"start", "--home", homeDir}
}

func (m *CosmosModule) ExportCommand(homeDir string) []string {
    return m.genesis.ExportCommandArgs(homeDir)
}

// Genesis operations
func (m *CosmosModule) ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
    patchOpts := GenesisPatchOptions{
        ChainID:       opts.ChainID,
        VotingPeriod:  m.GenesisConfig().VotingPeriod,
        UnbondingTime: m.GenesisConfig().UnbondingTime,
    }
    return m.genesis.PatchGenesis(genesis, patchOpts)
}

// ... implement remaining methods
```

#### Step 2: Create main.go

```go
package main

import (
    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

func main() {
    plugin.Serve(NewCosmosModule())
}
```

#### Step 3: Copy and adapt internal files

Copy the following files, updating package name to `main`:
- `internal/plugin/cosmos/builder.go`
- `internal/plugin/cosmos/genesis.go`
- `internal/plugin/cosmos/initializer.go`

Update imports to remove dependency on `internal/plugin/types`.

### Build Instructions

```bash
# Navigate to plugin directory
cd cosmos-plugin

# Initialize module
go mod init github.com/altuslabsxyz/cosmos-plugin
go mod tidy

# Build plugin binary
go build -o cosmos-plugin .

# Install
mkdir -p ~/.devnet-builder/plugins
cp cosmos-plugin ~/.devnet-builder/plugins/
chmod +x ~/.devnet-builder/plugins/cosmos-plugin

# Verify
dvb plugins list
```

## What cosmos-plugin Will Contain

### Configuration Values

| Property | Value |
|----------|-------|
| Name | `cosmos` |
| Binary Name | `gaiad` |
| Bech32 Prefix | `cosmos` |
| Base Denom | `uatom` |
| Default Chain ID | `devnet-1` |
| Default Binary Version | `v18.1.0` |

### Supported Features

1. **Binary Building**
   - Clone from GitHub
   - Build with `make install` or `go build`
   - Version injection via ldflags
   - Build tag support (`netgo`, `ledger`)

2. **Genesis Modification**
   - Chain ID modification
   - Voting period adjustment
   - Unbonding time adjustment
   - Binary version metadata embedding
   - Module validation

3. **Node Initialization**
   - Standard Cosmos SDK init command
   - Config directory paths
   - Moniker generation

4. **Network Endpoints**
   - Mainnet RPC: `https://rpc.cosmos.network`
   - Testnet RPC: `https://rpc.sentry-01.theta-testnet.polypore.xyz`

## Files to Delete After Extraction

Once cosmos-plugin is functional as a standalone binary:

| File | Status |
|------|--------|
| `internal/plugin/cosmos/builder.go` | Delete |
| `internal/plugin/cosmos/genesis.go` | Delete |
| `internal/plugin/cosmos/initializer.go` | Delete |
| `internal/plugin/types/` | Delete (old interfaces) |

## Relationship to Other Plugins

The cosmos-plugin serves as a base that other plugins can reference or extend:

```
cosmos-plugin (base Cosmos SDK)
    |
    +-- stable-plugin (extends with EVM support)
    +-- osmosis-plugin (extends with AMM modules)
    +-- evmos-plugin (extends with full EVM)
```

## Testing

```bash
# After installation, verify plugin loads
dvb plugins list

# Check plugin info
dvb plugins info cosmos

# Create a test devnet
dvb provision --network cosmos --validators 2
```

## See Also

- [README.md](./README.md) - Plugin development overview
- [stable-plugin.md](./stable-plugin.md) - Stable network plugin
- `pkg/network/interface.go` - Full Module interface definition
- `pkg/network/plugin/` - Plugin communication infrastructure
