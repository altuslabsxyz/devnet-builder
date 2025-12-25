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

## Optional Methods

Plugins can implement additional optional methods to enable advanced features:

### GetGovernanceParams (Optional)

Query governance parameters from a running blockchain. This method is used during upgrade workflows to determine voting periods and deposit requirements dynamically.

```go
func (n *MyNetwork) GetGovernanceParams(rpcEndpoint, networkType string) (*plugin.GovernanceParamsResponse, error) {
    // Query the blockchain's governance module via REST API
    // Return voting periods and deposit amounts
    return &plugin.GovernanceParamsResponse{
        VotingPeriodNs:           int64(48 * time.Hour),  // Regular voting period (nanoseconds)
        ExpeditedVotingPeriodNs:  int64(24 * time.Hour),  // Expedited voting period (nanoseconds)
        MinDeposit:               "10000000uatom",        // Minimum deposit for regular proposals
        ExpeditedMinDeposit:      "50000000uatom",        // Minimum deposit for expedited proposals
        Error:                    "",                      // Empty = success, populated = error message
    }, nil
}
```

**Parameters:**
- `rpcEndpoint`: HTTP/HTTPS URL of the blockchain RPC or REST endpoint (e.g., "http://localhost:1317")
- `networkType`: Network context ("mainnet", "testnet", "devnet", or custom)

**Returns:**
- `VotingPeriodNs`: Regular voting period in nanoseconds (must be positive)
- `ExpeditedVotingPeriodNs`: Expedited voting period in nanoseconds (must be < VotingPeriodNs if set)
- `MinDeposit`: Minimum deposit as numeric string (e.g., "10000000" or "10000000uatom")
- `ExpeditedMinDeposit`: Minimum deposit for expedited proposals (optional)
- `Error`: Error message if query fails, empty string on success

**Implementation approach:**
1. Make HTTP requests to `rpcEndpoint + "/cosmos/gov/v1/params/voting"` and `/cosmos/gov/v1/params/deposit`
2. Parse JSON responses from the governance module
3. Extract and validate parameter values
4. Convert time.Duration to nanoseconds (int64)
5. Return error in Error field if network/parsing fails

**Error handling:**
- Network errors: Populate Error field with descriptive message
- Parsing errors: Populate Error field with parse failure details
- Success: Leave Error field empty, populate all parameter fields

**Backward compatibility:**
- If not implemented, devnet-builder falls back to REST API queries
- Old plugins continue to work without implementing this method

**Example:** See `examples/cosmos-plugin/main.go` for a complete reference implementation.

### RPC Operations (Optional)

Plugins can implement chain-specific RPC operations. These methods allow full delegation of blockchain queries to the plugin, enabling support for non-standard Cosmos SDK chains or custom RPC implementations.

All RPC methods follow the same pattern: try plugin first, fall back to REST if Unimplemented.

#### GetBlockHeight

Returns the current block height from the chain.

```go
func (n *MyNetwork) GetBlockHeight(ctx context.Context, rpcEndpoint string) (*plugin.BlockHeightResponse, error) {
    // Query rpcEndpoint + "/status"
    return &plugin.BlockHeightResponse{Height: 12345, Error: ""}, nil
}
```

#### GetBlockTime

Estimates average block time by sampling recent blocks.

```go
func (n *MyNetwork) GetBlockTime(ctx context.Context, rpcEndpoint string, sampleSize int) (*plugin.BlockTimeResponse, error) {
    return &plugin.BlockTimeResponse{BlockTimeNs: int64(6 * time.Second), Error: ""}, nil
}
```

#### IsChainRunning

Checks if the chain is responding to RPC requests.

```go
func (n *MyNetwork) IsChainRunning(ctx context.Context, rpcEndpoint string) (*plugin.ChainStatusResponse, error) {
    return &plugin.ChainStatusResponse{IsRunning: true, Error: ""}, nil
}
```

#### WaitForBlock

Waits until the chain reaches the specified height.

```go
func (n *MyNetwork) WaitForBlock(ctx context.Context, rpcEndpoint string, targetHeight int64, timeoutMs int64) (*plugin.WaitForBlockResponse, error) {
    return &plugin.WaitForBlockResponse{CurrentHeight: targetHeight, Reached: true, Error: ""}, nil
}
```

#### GetProposal

Retrieves a governance proposal by ID.

```go
func (n *MyNetwork) GetProposal(ctx context.Context, rpcEndpoint string, proposalID uint64) (*plugin.ProposalResponse, error) {
    return &plugin.ProposalResponse{
        Id:                proposalID,
        Status:            "PROPOSAL_STATUS_VOTING_PERIOD",
        VotingEndTimeUnix: time.Now().Add(60 * time.Second).Unix(),
        Error:             "",
    }, nil
}
```

#### GetUpgradePlan

Retrieves the current upgrade plan if any.

```go
func (n *MyNetwork) GetUpgradePlan(ctx context.Context, rpcEndpoint string) (*plugin.UpgradePlanResponse, error) {
    return &plugin.UpgradePlanResponse{HasPlan: false, Error: ""}, nil
}
```

#### GetAppVersion

Returns the application version from ABCI info.

```go
func (n *MyNetwork) GetAppVersion(ctx context.Context, rpcEndpoint string) (*plugin.AppVersionResponse, error) {
    return &plugin.AppVersionResponse{Version: "v1.0.0", Error: ""}, nil
}
```

**Backward compatibility:** All RPC methods are optional. If not implemented, devnet-builder falls back to direct REST API queries.

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
