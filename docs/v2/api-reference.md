# gRPC API Reference

Complete reference for the devnetd gRPC API.

## Overview

The devnetd daemon exposes a gRPC API for programmatic control. All services use protobuf for serialization and support bidirectional streaming for real-time updates.

### API Endpoint

```
Unix Socket: ~/.devnet-builder/devnetd.sock
TCP (optional): localhost:50051
```

### Connection Example

```go
import (
    "google.golang.org/grpc"
    pb "github.com/altuslabsxyz/devnet-builder/api/proto/v1"
)

// Unix socket
conn, err := grpc.Dial(
    "unix:///home/user/.devnet-builder/devnetd.sock",
    grpc.WithInsecure(),
)

// Or TCP
conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())

defer conn.Close()
```

## DevnetService

Manages devnet lifecycle.

### Create

Create a new devnet:

```protobuf
rpc Create(CreateDevnetRequest) returns (CreateDevnetResponse);

message CreateDevnetRequest {
    string name = 1;
    string plugin = 2;
    int32 validators = 3;
    int32 full_nodes = 4;
    string mode = 5;  // "docker", "local"
    BinarySource binary_source = 6;
    string chain_id = 7;
    string snapshot_url = 8;
}

message CreateDevnetResponse {
    string name = 1;
    string status = 2;
}
```

**Example:**

```go
client := pb.NewDevnetServiceClient(conn)

resp, err := client.Create(ctx, &pb.CreateDevnetRequest{
    Name:       "my-devnet",
    Plugin:     "osmosisd",
    Validators: 4,
    FullNodes:  2,
    Mode:       "docker",
})
```

### Get

Get devnet details:

```protobuf
rpc Get(GetDevnetRequest) returns (GetDevnetResponse);

message GetDevnetRequest {
    string name = 1;
}

message GetDevnetResponse {
    Devnet devnet = 1;
}

message Devnet {
    ResourceMeta metadata = 1;
    DevnetSpec spec = 2;
    DevnetStatus status = 3;
}
```

### List

List all devnets:

```protobuf
rpc List(ListDevnetsRequest) returns (ListDevnetsResponse);

message ListDevnetsRequest {
    string status_filter = 1;  // "running", "stopped", etc.
    map<string, string> labels = 2;
}

message ListDevnetsResponse {
    repeated Devnet devnets = 1;
}
```

### Delete

Delete a devnet:

```protobuf
rpc Delete(DeleteDevnetRequest) returns (DeleteDevnetResponse);

message DeleteDevnetRequest {
    string name = 1;
    bool force = 2;
    bool keep_data = 3;
}
```

### Start/Stop

Control devnet state:

```protobuf
rpc Start(StartDevnetRequest) returns (StartDevnetResponse);
rpc Stop(StopDevnetRequest) returns (StopDevnetResponse);

message StartDevnetRequest {
    string name = 1;
}

message StopDevnetRequest {
    string name = 1;
    int32 graceful_timeout_seconds = 2;
}
```

### Watch

Stream devnet events:

```protobuf
rpc Watch(WatchDevnetRequest) returns (stream DevnetEvent);

message WatchDevnetRequest {
    string name = 1;  // Empty for all devnets
}

message DevnetEvent {
    string type = 1;  // "created", "updated", "deleted"
    Devnet devnet = 2;
    google.protobuf.Timestamp timestamp = 3;
}
```

**Example:**

```go
stream, err := client.Watch(ctx, &pb.WatchDevnetRequest{
    Name: "my-devnet",
})

for {
    event, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Printf("Event: %s, Phase: %s\n",
        event.Type, event.Devnet.Status.Phase)
}
```

### StreamLogs

Stream node logs:

```protobuf
rpc StreamLogs(StreamLogsRequest) returns (stream LogEntry);

message StreamLogsRequest {
    string devnet = 1;
    string node = 2;  // Empty for all nodes
    bool follow = 3;
    int32 tail = 4;
}

message LogEntry {
    string node = 1;
    string message = 2;
    google.protobuf.Timestamp timestamp = 3;
}
```

## NodeService

Manages individual nodes.

### Get

Get node details:

```protobuf
rpc Get(GetNodeRequest) returns (GetNodeResponse);

message GetNodeRequest {
    string devnet = 1;
    string node = 2;
}

message GetNodeResponse {
    Node node = 1;
}
```

### List

List nodes in devnet:

```protobuf
rpc List(ListNodesRequest) returns (ListNodesResponse);

message ListNodesRequest {
    string devnet = 1;
    string role_filter = 2;  // "validator", "fullnode"
}

message ListNodesResponse {
    repeated Node nodes = 1;
}
```

### Start/Stop/Restart

Control node state:

```protobuf
rpc Start(StartNodeRequest) returns (StartNodeResponse);
rpc Stop(StopNodeRequest) returns (StopNodeResponse);
rpc Restart(RestartNodeRequest) returns (RestartNodeResponse);

message StartNodeRequest {
    string devnet = 1;
    string node = 2;
}
```

### GetHealth

Check node health:

```protobuf
rpc GetHealth(GetHealthRequest) returns (GetHealthResponse);

message GetHealthRequest {
    string devnet = 1;
    string node = 2;  // Empty for all nodes
}

message GetHealthResponse {
    repeated NodeHealth nodes = 1;
}

message NodeHealth {
    string name = 1;
    string status = 2;
    int64 block_height = 3;
    int32 peer_count = 4;
    bool healthy = 5;
    string reason = 6;
}
```

### StreamLogs

Stream logs from node:

```protobuf
rpc StreamLogs(StreamNodeLogsRequest) returns (stream LogEntry);

message StreamNodeLogsRequest {
    string devnet = 1;
    string node = 2;
    bool follow = 3;
    int32 tail = 4;
}
```

## TransactionService

Manages transaction lifecycle.

### Submit

Submit a transaction:

```protobuf
rpc Submit(SubmitTransactionRequest) returns (SubmitTransactionResponse);

message SubmitTransactionRequest {
    string devnet = 1;
    string tx_type = 2;
    string signer = 3;
    bytes payload = 4;
    uint64 gas_limit = 5;
    string memo = 6;
}

message SubmitTransactionResponse {
    string id = 1;
    string status = 2;
}
```

**Example:**

```go
client := pb.NewTransactionServiceClient(conn)

payload := []byte(`{"to_address":"cosmos1...","amount":"1000000uatom"}`)

resp, err := client.Submit(ctx, &pb.SubmitTransactionRequest{
    Devnet:   "my-devnet",
    TxType:   "bank/send",
    Signer:   "validator:0",
    Payload:  payload,
    GasLimit: 200000,
})
```

### SubmitBatch

Submit multiple transactions:

```protobuf
rpc SubmitBatch(SubmitBatchRequest) returns (SubmitBatchResponse);

message SubmitBatchRequest {
    string devnet = 1;
    repeated TransactionRequest transactions = 2;
}

message SubmitBatchResponse {
    repeated string ids = 1;
}
```

### Get

Get transaction status:

```protobuf
rpc Get(GetTransactionRequest) returns (GetTransactionResponse);

message GetTransactionRequest {
    string id = 1;
}

message GetTransactionResponse {
    Transaction transaction = 1;
}

message Transaction {
    ResourceMeta metadata = 1;
    TransactionSpec spec = 2;
    TransactionStatus status = 3;
}
```

### List

List transactions:

```protobuf
rpc List(ListTransactionsRequest) returns (ListTransactionsResponse);

message ListTransactionsRequest {
    string devnet = 1;
    string status_filter = 2;
    string type_filter = 3;
    int32 limit = 4;
}

message ListTransactionsResponse {
    repeated Transaction transactions = 1;
}
```

### Watch

Watch transaction progress:

```protobuf
rpc Watch(WatchTransactionRequest) returns (stream TransactionEvent);

message WatchTransactionRequest {
    string id = 1;
}

message TransactionEvent {
    string type = 1;
    Transaction transaction = 2;
    google.protobuf.Timestamp timestamp = 3;
}
```

**Example:**

```go
stream, err := client.Watch(ctx, &pb.WatchTransactionRequest{
    Id: "tx-12345",
})

for {
    event, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Printf("Phase: %s\n", event.Transaction.Status.Phase)
    if event.Transaction.Status.Phase == "Confirmed" {
        break
    }
}
```

## UpgradeService

Manages chain upgrades.

### Create

Create upgrade proposal:

```protobuf
rpc Create(CreateUpgradeRequest) returns (CreateUpgradeResponse);

message CreateUpgradeRequest {
    string devnet = 1;
    string upgrade_name = 2;
    int64 target_height = 3;
    BinarySource new_binary = 4;
    bool auto_vote = 5;
    bool with_export = 6;
}

message CreateUpgradeResponse {
    string name = 1;
    uint64 proposal_id = 2;
}
```

### Get

Get upgrade status:

```protobuf
rpc Get(GetUpgradeRequest) returns (GetUpgradeResponse);

message GetUpgradeRequest {
    string name = 1;
}

message GetUpgradeResponse {
    Upgrade upgrade = 1;
}

message Upgrade {
    ResourceMeta metadata = 1;
    UpgradeSpec spec = 2;
    UpgradeStatus status = 3;
}
```

### List

List upgrades:

```protobuf
rpc List(ListUpgradesRequest) returns (ListUpgradesResponse);

message ListUpgradesRequest {
    string devnet = 1;
    string status_filter = 2;
}

message ListUpgradesResponse {
    repeated Upgrade upgrades = 1;
}
```

### Cancel

Cancel pending upgrade:

```protobuf
rpc Cancel(CancelUpgradeRequest) returns (CancelUpgradeResponse);

message CancelUpgradeRequest {
    string name = 1;
}
```

### Watch

Watch upgrade progress:

```protobuf
rpc Watch(WatchUpgradeRequest) returns (stream UpgradeEvent);

message WatchUpgradeRequest {
    string name = 1;
}

message UpgradeEvent {
    string type = 1;
    Upgrade upgrade = 2;
    google.protobuf.Timestamp timestamp = 3;
}
```

## ExportService

Manages state exports.

### Create

Create state export:

```protobuf
rpc Create(CreateExportRequest) returns (CreateExportResponse);

message CreateExportRequest {
    string devnet = 1;
    int64 height = 2;  // 0 for latest
    string output_path = 3;
}

message CreateExportResponse {
    string id = 1;
    string path = 2;
}
```

### Get

Get export details:

```protobuf
rpc Get(GetExportRequest) returns (GetExportResponse);

message GetExportRequest {
    string id = 1;
}

message GetExportResponse {
    Export export = 1;
}
```

### List

List exports:

```protobuf
rpc List(ListExportsRequest) returns (ListExportsResponse);

message ListExportsRequest {
    string devnet = 1;
}

message ListExportsResponse {
    repeated Export exports = 1;
}
```

### Delete

Delete export:

```protobuf
rpc Delete(DeleteExportRequest) returns (DeleteExportResponse);

message DeleteExportRequest {
    string id = 1;
}
```

## PluginService

Manages network plugins.

### List

List installed plugins:

```protobuf
rpc List(ListPluginsRequest) returns (ListPluginsResponse);

message ListPluginsRequest {}

message ListPluginsResponse {
    repeated PluginInfo plugins = 1;
}

message PluginInfo {
    string name = 1;
    string version = 2;
    string network_type = 3;
    repeated string supported_sdk_versions = 4;
    repeated string supported_tx_types = 5;
}
```

### Get

Get plugin details:

```protobuf
rpc Get(GetPluginRequest) returns (GetPluginResponse);

message GetPluginRequest {
    string name = 1;
}

message GetPluginResponse {
    PluginInfo plugin = 1;
}
```

### Install

Install new plugin:

```protobuf
rpc Install(InstallPluginRequest) returns (InstallPluginResponse);

message InstallPluginRequest {
    string path = 1;
    string name = 2;
}
```

### Uninstall

Uninstall plugin:

```protobuf
rpc Uninstall(UninstallPluginRequest) returns (UninstallPluginResponse);

message UninstallPluginRequest {
    string name = 1;
}
```

## DaemonService

Manages daemon itself.

### GetStatus

Get daemon status:

```protobuf
rpc GetStatus(GetStatusRequest) returns (GetStatusResponse);

message GetStatusRequest {}

message GetStatusResponse {
    string status = 1;
    int32 pid = 2;
    string version = 3;
    int64 uptime_seconds = 4;
    int32 active_devnets = 5;
    int32 total_nodes = 6;
    int64 memory_usage_bytes = 7;
}
```

### GetConfig

Get daemon configuration:

```protobuf
rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);

message GetConfigRequest {}

message GetConfigResponse {
    map<string, string> config = 1;
}
```

### Shutdown

Shutdown daemon:

```protobuf
rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);

message ShutdownRequest {
    bool force = 1;
}
```

## Error Handling

All RPCs return standard gRPC status codes:

```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

_, err := client.Create(ctx, req)
if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.AlreadyExists:
            // Handle duplicate
        case codes.InvalidArgument:
            // Handle validation error
        case codes.NotFound:
            // Handle not found
        default:
            // Handle other errors
        }
    }
}
```

## Authentication

When TLS and auth are enabled:

```go
import (
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/metadata"
)

// TLS credentials
creds, err := credentials.NewClientTLSFromFile("ca.crt", "")
conn, err := grpc.Dial("localhost:50051",
    grpc.WithTransportCredentials(creds),
)

// Add auth token to context
md := metadata.Pairs("authorization", "Bearer "+token)
ctx := metadata.NewOutgoingContext(context.Background(), md)

// Use authenticated context
resp, err := client.Create(ctx, req)
```

## Client Libraries

### Go

```bash
go get github.com/altuslabsxyz/devnet-builder/api/proto/v1
```

### Python (Coming Soon)

```bash
pip install devnet-builder-api
```

### JavaScript (Coming Soon)

```bash
npm install @altuslabs/devnet-builder-api
```

## Next Steps

- **[Client Reference](client.md)** - CLI commands that use this API
- **[Architecture](architecture.md)** - Understanding the gRPC layer
- **[Plugin Development](plugins.md)** - Plugin gRPC interface
- **Protobuf Definitions** - See `api/proto/v1/` for full schemas
