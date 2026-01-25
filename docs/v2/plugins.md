# Plugin Development Guide

Learn how to create network plugins for Devnet Builder v2 to support new blockchain platforms.

## Overview

Network plugins provide chain-specific implementations for:
- Genesis generation
- Node lifecycle management
- Transaction building and signing
- Status queries and monitoring

Plugins use the HashiCorp go-plugin framework with gRPC for process isolation and language independence.

## Plugin Interface

### Core Interface

```go
// pkg/network/plugin/interface.go
package plugin

type NetworkPlugin interface {
    // Metadata
    GetInfo() (*PluginInfo, error)
    GetSupportedSDKVersions() (*SDKVersionRange, error)

    // Lifecycle
    Initialize(config *Config) error
    GenerateGenesis(req *GenesisRequest) (*GenesisResult, error)
    StartNode(req *StartNodeRequest) (*StartNodeResult, error)
    StopNode(req *StopNodeRequest) error

    // Queries
    GetNodeStatus(req *NodeStatusRequest) (*NodeStatus, error)
    GetBlockHeight(rpcURL string) (int64, error)

    // Transactions
    CreateTxBuilder(req *CreateTxBuilderRequest) (TxBuilder, error)
    GetSupportedTxTypes() (*SupportedTxTypes, error)

    // Cleanup
    Close() error
}
```

### TxBuilder Interface

```go
type TxBuilder interface {
    // BuildTx constructs an unsigned transaction
    BuildTx(ctx context.Context, req *TxBuildRequest) (*UnsignedTx, error)

    // SignTx signs an unsigned transaction
    SignTx(ctx context.Context, tx *UnsignedTx, key *SigningKey) (*SignedTx, error)

    // BroadcastTx submits a signed transaction
    BroadcastTx(ctx context.Context, tx *SignedTx) (*TxBroadcastResult, error)

    // SupportedTxTypes returns supported transaction types
    SupportedTxTypes() []TxType
}
```

## Creating a Plugin

### Project Structure

```
my-plugin/
├── main.go              # Plugin entry point
├── plugin.go            # Plugin implementation
├── txbuilder.go         # TxBuilder implementation
├── genesis.go           # Genesis generation
├── go.mod
└── go.sum
```

### Basic Plugin Implementation

```go
// main.go
package main

import (
    "github.com/hashicorp/go-plugin"
    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: plugin.Handshake,
        Plugins: map[string]plugin.Plugin{
            "network": &plugin.NetworkPluginGRPC{
                Impl: &MyPlugin{},
            },
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

```go
// plugin.go
package main

import (
    "context"
    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

type MyPlugin struct {
    config *plugin.Config
}

func (p *MyPlugin) GetInfo() (*plugin.PluginInfo, error) {
    return &plugin.PluginInfo{
        Name:        "my-chain",
        Version:     "v1.0.0",
        NetworkType: "cosmos",
        Description: "My custom blockchain plugin",
    }, nil
}

func (p *MyPlugin) GetSupportedSDKVersions() (*plugin.SDKVersionRange, error) {
    return &plugin.SDKVersionRange{
        Min: "v0.50.0",
        Max: "v0.50.99",
    }, nil
}

func (p *MyPlugin) Initialize(config *plugin.Config) error {
    p.config = config
    return nil
}

func (p *MyPlugin) GenerateGenesis(req *plugin.GenesisRequest) (*plugin.GenesisResult, error) {
    // Implement genesis generation
    // Return genesis.json and config files
    return &plugin.GenesisResult{
        GenesisJSON: genesisBytes,
        ConfigFiles: map[string][]byte{
            "config.toml": configToml,
            "app.toml":    appToml,
        },
    }, nil
}

func (p *MyPlugin) StartNode(req *plugin.StartNodeRequest) (*plugin.StartNodeResult, error) {
    // Start the node process
    // Return process info
    return &plugin.StartNodeResult{
        PID:         pid,
        ContainerID: containerID,
        RPCAddress:  "http://localhost:26657",
    }, nil
}

func (p *MyPlugin) StopNode(req *plugin.StopNodeRequest) error {
    // Stop the node gracefully
    return nil
}

func (p *MyPlugin) GetNodeStatus(req *plugin.NodeStatusRequest) (*plugin.NodeStatus, error) {
    // Query node status via RPC
    return &plugin.NodeStatus{
        BlockHeight: height,
        PeerCount:   peers,
        CatchingUp:  false,
        Synced:      true,
    }, nil
}

func (p *MyPlugin) GetBlockHeight(rpcURL string) (int64, error) {
    // Query current block height
    return height, nil
}

func (p *MyPlugin) CreateTxBuilder(req *plugin.CreateTxBuilderRequest) (plugin.TxBuilder, error) {
    // Create SDK-version-specific builder
    return &MyTxBuilder{
        chainID:    req.ChainID,
        sdkVersion: req.SDKVersion,
        rpcClient:  client,
    }, nil
}

func (p *MyPlugin) GetSupportedTxTypes() (*plugin.SupportedTxTypes, error) {
    return &plugin.SupportedTxTypes{
        Types: []plugin.TxType{
            plugin.TxTypeGovProposal,
            plugin.TxTypeGovVote,
            plugin.TxTypeBankSend,
        },
    }, nil
}

func (p *MyPlugin) Close() error {
    // Cleanup resources
    return nil
}
```

### TxBuilder Implementation

```go
// txbuilder.go
package main

import (
    "context"
    "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

type MyTxBuilder struct {
    chainID    string
    sdkVersion string
    rpcClient  RPCClient
}

func (b *MyTxBuilder) BuildTx(ctx context.Context, req *plugin.TxBuildRequest) (*plugin.UnsignedTx, error) {
    switch req.TxType {
    case plugin.TxTypeBankSend:
        return b.buildBankSend(ctx, req)
    case plugin.TxTypeGovProposal:
        return b.buildGovProposal(ctx, req)
    case plugin.TxTypeGovVote:
        return b.buildGovVote(ctx, req)
    default:
        return nil, fmt.Errorf("unsupported tx type: %s", req.TxType)
    }
}

func (b *MyTxBuilder) buildBankSend(ctx context.Context, req *plugin.TxBuildRequest) (*plugin.UnsignedTx, error) {
    // Parse payload
    var payload struct {
        ToAddress string `json:"to_address"`
        Amount    string `json:"amount"`
    }
    if err := json.Unmarshal(req.Payload, &payload); err != nil {
        return nil, err
    }

    // Query account info
    account, err := b.rpcClient.GetAccount(ctx, req.Sender)
    if err != nil {
        return nil, err
    }

    // Build message using SDK
    msg := &banktypes.MsgSend{
        FromAddress: req.Sender,
        ToAddress:   payload.ToAddress,
        Amount:      sdk.NewCoins(sdk.MustParse Coin(payload.Amount)),
    }

    // Create transaction
    txBuilder := b.getTxBuilder()
    if err := txBuilder.SetMsgs(msg); err != nil {
        return nil, err
    }

    // Set gas and fees
    txBuilder.SetGasLimit(req.GasLimit)
    if req.Memo != "" {
        txBuilder.SetMemo(req.Memo)
    }

    // Generate sign doc
    signDoc, err := b.getSignDoc(txBuilder, account)
    if err != nil {
        return nil, err
    }

    return &plugin.UnsignedTx{
        TxBytes:       txBuilder.GetTx().GetTxBytes(),
        SignDoc:       signDoc,
        AccountNumber: account.GetAccountNumber(),
        Sequence:      account.GetSequence(),
    }, nil
}

func (b *MyTxBuilder) SignTx(ctx context.Context, tx *plugin.UnsignedTx, key *plugin.SigningKey) (*plugin.SignedTx, error) {
    // Sign the transaction
    privKey := secp256k1.PrivKey{Key: key.PrivKey}
    signature, err := privKey.Sign(tx.SignDoc)
    if err != nil {
        return nil, err
    }

    // Build signed tx
    signedTxBytes, err := b.attachSignature(tx.TxBytes, signature)
    if err != nil {
        return nil, err
    }

    return &plugin.SignedTx{
        TxBytes:   signedTxBytes,
        Signature: signature,
        PubKey:    privKey.PubKey().Bytes(),
    }, nil
}

func (b *MyTxBuilder) BroadcastTx(ctx context.Context, tx *plugin.SignedTx) (*plugin.TxBroadcastResult, error) {
    // Broadcast to network
    result, err := b.rpcClient.BroadcastTxSync(ctx, tx.TxBytes)
    if err != nil {
        return nil, err
    }

    return &plugin.TxBroadcastResult{
        TxHash: result.Hash.String(),
        Code:   result.Code,
        Log:    result.Log,
        Height: result.Height,
    }, nil
}

func (b *MyTxBuilder) SupportedTxTypes() []plugin.TxType {
    return []plugin.TxType{
        plugin.TxTypeBankSend,
        plugin.TxTypeGovProposal,
        plugin.TxTypeGovVote,
    }
}
```

## Building and Installing

### Build Plugin

```bash
# Build binary
go build -o my-chain-plugin main.go

# Install to plugin directory
cp my-chain-plugin ~/.devnet-builder/plugins/

# Verify
dvb plugins list
```

### Plugin Discovery

Plugins are discovered automatically from:

```
~/.devnet-builder/plugins/
├── cosmos-v047-plugin
├── cosmos-v050-plugin
├── my-chain-plugin  # Your plugin
└── evm-geth-plugin
```

## Testing Plugins

### Unit Tests

```go
// plugin_test.go
func TestPluginInfo(t *testing.T) {
    p := &MyPlugin{}
    info, err := p.GetInfo()
    require.NoError(t, err)
    assert.Equal(t, "my-chain", info.Name)
}

func TestBuildBankSend(t *testing.T) {
    builder := &MyTxBuilder{
        chainID: "test-1",
        rpcClient: mockClient,
    }

    req := &plugin.TxBuildRequest{
        TxType:  plugin.TxTypeBankSend,
        Sender:  "cosmos1abc...",
        Payload: []byte(`{"to_address":"cosmos1xyz...","amount":"1000000uatom"}`),
    }

    tx, err := builder.BuildTx(context.Background(), req)
    require.NoError(t, err)
    assert.NotEmpty(t, tx.TxBytes)
}
```

### Integration Tests

```bash
# Deploy test devnet with your plugin
dvb deploy my-chain --name test-devnet

# Test transaction submission
dvb tx submit test-devnet \
  --type bank/send \
  --signer validator:0 \
  --payload '{"to_address":"...","amount":"1000000"}'

# Verify
dvb status test-devnet
```

## Example Plugins

### Cosmos SDK v0.50 Plugin

See full implementation: `pkg/network/cosmos/v050/`

Key features:
- Genesis generation with SDK modules
- Transaction building with protobuf
- Query support for status/height
- Support for multiple tx types

### EVM Plugin

See full implementation: `pkg/network/evm/`

Key features:
- Geth integration
- Native and contract transactions
- EIP-155 chain ID support
- RLP encoding/decoding

## Best Practices

1. **Version SDK dependencies** - Pin exact SDK versions
2. **Handle all tx types** - Return clear errors for unsupported types
3. **Validate payloads** - Check required fields early
4. **Use structured logging** - Help with debugging
5. **Test thoroughly** - Unit and integration tests
6. **Document tx formats** - Clear payload examples
7. **Support SDK upgrades** - Version-aware builders
8. **Clean up resources** - Implement Close() properly

## Troubleshooting

### Plugin Not Loading

```bash
# Check plugin exists
ls ~/.devnet-builder/plugins/my-chain-plugin

# Check permissions
chmod +x ~/.devnet-builder/plugins/my-chain-plugin

# Test plugin directly
./my-chain-plugin

# View daemon logs
dvb daemon logs --follow | grep plugin
```

### Transaction Building Fails

```bash
# Enable debug logging
devnetd start --log-level debug

# View detailed errors
dvb tx status tx-12345 --output json | jq .status.error

# Test payload locally
go test -v ./txbuilder_test.go -run TestBuildBankSend
```

## Next Steps

- **[Architecture](architecture.md)** - Understanding the plugin system
- **[Transactions](transactions.md)** - Transaction payload formats
- **[API Reference](api-reference.md)** - Plugin gRPC interface
- **Example Plugins** - Study existing implementations in `pkg/network/`
