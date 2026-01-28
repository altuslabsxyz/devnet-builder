# Stable Plugin Requirements

This document describes the chain-specific values that the Stable network plugin must provide.

**Note:** The stable-plugin will be maintained in a separate private repository.

## Overview

The stable-plugin provides support for the Stable network, a Cosmos SDK-based chain with EVM compatibility. It extends the base Cosmos SDK functionality with Stable-specific configurations.

## Repository Information

| Property | Value |
|----------|-------|
| Repository | Private (separate from devnet-builder) |
| Binary Name | `stable-plugin` |
| Installation | `~/.devnet-builder/plugins/stable-plugin` |

## Chain-Specific Values to Provide

### Identity

| Property | Value | Description |
|----------|-------|-------------|
| Name | `stable` | Unique plugin identifier |
| DisplayName | `Stable Network` | Human-readable name |
| Version | `1.x.x` | Plugin version (semver) |

### Binary Configuration

| Property | Value | Description |
|----------|-------|-------------|
| BinaryName | `stabled` | CLI binary name |
| BinarySource.Type | `github` | Source type |
| BinarySource.Owner | `(private)` | GitHub organization |
| BinarySource.Repo | `(private)` | GitHub repository |
| DefaultBinaryVersion | `v1.x.x` | Default version to build |

### Chain Configuration

| Property | Value | Description |
|----------|-------|-------------|
| Bech32Prefix | `stable` | Address prefix |
| BaseDenom | `ustable` | Base token denomination |
| EVMChainID | `(network-specific)` | EVM chain ID |
| DefaultChainID | `stabledevnet_{evmid}-1` | Devnet chain ID pattern |

### Genesis Parameters

| Property | Devnet Value | Description |
|----------|--------------|-------------|
| VotingPeriod | `30s` - `60s` | Fast voting for testing |
| UnbondingTime | `120s` | Fast unbonding for testing |
| MinDeposit | `10000000ustable` | Governance deposit |
| MaxValidators | `100` | Maximum validator count |
| CommunityTax | `0.02` | Distribution tax |
| DenomExponent | `18` | Decimal places |

### Port Configuration

| Port | Default | Description |
|------|---------|-------------|
| RPC | `26657` | Tendermint RPC |
| P2P | `26656` | P2P networking |
| GRPC | `9090` | gRPC server |
| GRPCWeb | `9091` | gRPC-Web |
| API | `1317` | REST API |
| EVMRPC | `8545` | EVM JSON-RPC |
| EVMSocket | `8546` | EVM WebSocket |

### Docker Configuration

| Property | Value | Description |
|----------|-------|-------------|
| DockerImage | `(private registry)` | Docker image name |
| DockerHomeDir | `/home/stabled` | Container home directory |

### Network Endpoints

| Network Type | RPC Endpoint | Snapshot URL |
|--------------|--------------|--------------|
| mainnet | `https://cosmos-rpc.stable.xyz` | `(snapshot URL)` |
| testnet | `(testnet RPC)` | `(testnet snapshot)` |

## Required Functionality

### 1. Binary Building

The stable-plugin must support building the `stabled` binary with:
- Network-specific EVM chain ID injection via ldflags
- Build tags: `netgo`, `ledger`
- Support for private repository authentication

**Build Configuration Example:**
```go
func (m *StableModule) GetBuildConfig(networkType string) (*network.BuildConfig, error) {
    var evmChainID string
    switch networkType {
    case "mainnet":
        evmChainID = "2201"
    case "testnet":
        evmChainID = "2202"
    default:
        evmChainID = "2200"  // devnet
    }

    return &network.BuildConfig{
        Tags:    []string{"netgo", "ledger"},
        LDFlags: []string{
            fmt.Sprintf("-X github.com/stablenetwork/stable/app.EVMChainID=%s", evmChainID),
        },
        Env: map[string]string{
            "CGO_ENABLED": "1",
        },
    }, nil
}
```

### 2. Genesis Modification

Must support:
- Chain ID modification
- EVM-specific module configuration
- Validator injection
- Account balance setup
- Governance parameter adjustment

### 3. Node Configuration

Must provide config overrides for:
- EVM JSON-RPC enabling
- EVM WebSocket enabling
- API enabling
- P2P configuration

**Config Override Example:**
```go
func (m *StableModule) GetConfigOverrides(nodeIndex int, opts network.NodeConfigOptions) ([]byte, []byte, error) {
    appToml := fmt.Sprintf(`
[json-rpc]
enable = true
address = "0.0.0.0:%d"
ws-address = "0.0.0.0:%d"
api = "eth,net,web3,txpool,debug"

[api]
enable = true
address = "tcp://0.0.0.0:%d"
`, opts.Ports.EVMRPC, opts.Ports.EVMSocket, opts.Ports.API)

    return nil, []byte(appToml), nil
}
```

### 4. RPC Operations

Must implement optional `RPCProvider` interface for:
- `GetBlockHeight` - Current block height
- `GetBlockTime` - Average block time
- `IsChainRunning` - Health check
- `WaitForBlock` - Block synchronization
- `GetProposal` - Governance proposal query
- `GetUpgradePlan` - Upgrade plan query
- `GetAppVersion` - Application version

### 5. TxBuilder Support

Must implement `TxBuilderFactory` for transaction building:
- Cosmos SDK transactions
- EVM transactions
- Governance transactions

## Implementation Checklist

- [ ] Create private repository for stable-plugin
- [ ] Implement `network.Module` interface
- [ ] Implement `network.FileBasedGenesisModifier` (large genesis support)
- [ ] Implement `network.StateExporter` (snapshot support)
- [ ] Implement `RPCProvider` interface
- [ ] Implement `TxBuilderFactory` interface
- [ ] Add mainnet RPC endpoints
- [ ] Add testnet RPC endpoints
- [ ] Add snapshot URLs
- [ ] Configure build tags and ldflags
- [ ] Set up CI/CD for plugin binary releases
- [ ] Document installation instructions

## Installation

```bash
# Download from private releases (requires authentication)
# OR build from source

# Install
mkdir -p ~/.devnet-builder/plugins
cp stable-plugin ~/.devnet-builder/plugins/
chmod +x ~/.devnet-builder/plugins/stable-plugin

# Verify
dvb plugins list
```

## Usage

```bash
# Create a Stable devnet
dvb provision --network stable --validators 4

# Create with YAML
dvb apply -f - <<EOF
apiVersion: v1
kind: Devnet
metadata:
  name: stable-devnet
spec:
  plugin: stable
  validators: 4
  accounts: 10
EOF

# Fork from mainnet
dvb provision --network stable --fork mainnet --validators 4
```

## Relationship to cosmos-plugin

The stable-plugin extends cosmos-plugin functionality:

```
cosmos-plugin (base Cosmos SDK)
    |
    +-- stable-plugin
        |-- EVM support (json-rpc, ws)
        |-- Network-specific chain IDs
        |-- Custom genesis modules
        |-- Private repository access
```

## Security Considerations

1. **Private Repository Access**
   - Plugin may require GitHub token for building from private repos
   - Token should be configured via environment variable

2. **Network Endpoints**
   - RPC endpoints should use HTTPS
   - Consider rate limiting for public endpoints

3. **Genesis Handling**
   - Validate genesis before modification
   - Handle sensitive data appropriately

## See Also

- [README.md](./README.md) - Plugin development overview
- [cosmos-plugin.md](./cosmos-plugin.md) - Base Cosmos plugin
- `pkg/network/interface.go` - Full Module interface
