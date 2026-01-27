# gRPC API Reference

Complete reference for the devnetd gRPC API.

## Overview

The devnetd daemon exposes a gRPC API for programmatic control. All services use protobuf for serialization. The daemon supports four core services: DevnetService, NodeService, UpgradeService, and TransactionService.

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

Manages devnet lifecycle. Supports namespace isolation for resource organization.

### CreateDevnet

Create a new devnet:

```protobuf
rpc CreateDevnet(CreateDevnetRequest) returns (CreateDevnetResponse);

message CreateDevnetRequest {
    string name = 1;
    string namespace = 2;  // Optional, defaults to "default"
    DevnetSpec spec = 3;
    map<string, string> labels = 4;
    map<string, string> annotations = 5;
}

message CreateDevnetResponse {
    Devnet devnet = 1;
}
```

**Example:**

```go
client := pb.NewDevnetServiceClient(conn)

resp, err := client.CreateDevnet(ctx, &pb.CreateDevnetRequest{
    Name:      "my-devnet",
    Namespace: "default",
    Spec: &pb.DevnetSpec{
        Plugin:     "osmosisd",
        Validators: 4,
        FullNodes:  2,
        ChainId:    "osmosis-devnet-1",
    },
})
```

### GetDevnet

Get devnet details:

```protobuf
rpc GetDevnet(GetDevnetRequest) returns (GetDevnetResponse);

message GetDevnetRequest {
    string name = 1;
    string namespace = 2;  // Optional, empty searches all namespaces
}

message GetDevnetResponse {
    Devnet devnet = 1;
}
```

### ListDevnets

List all devnets:

```protobuf
rpc ListDevnets(ListDevnetsRequest) returns (ListDevnetsResponse);

message ListDevnetsRequest {
    string namespace = 1;       // Optional, empty returns all namespaces
    string label_selector = 2;  // Optional, format: "key1=value1,key2=value2"
}

message ListDevnetsResponse {
    repeated Devnet devnets = 1;
}
```

### DeleteDevnet

Delete a devnet (cascades to nodes and upgrades):

```protobuf
rpc DeleteDevnet(DeleteDevnetRequest) returns (DeleteDevnetResponse);

message DeleteDevnetRequest {
    string name = 1;
    string namespace = 2;  // Optional, defaults to "default"
}

message DeleteDevnetResponse {
    bool deleted = 1;
}
```

### StartDevnet / StopDevnet

Control devnet state:

```protobuf
rpc StartDevnet(StartDevnetRequest) returns (StartDevnetResponse);
rpc StopDevnet(StopDevnetRequest) returns (StopDevnetResponse);

message StartDevnetRequest {
    string name = 1;
    string namespace = 2;
}

message StopDevnetRequest {
    string name = 1;
    string namespace = 2;
}
```

### ApplyDevnet

Create or update a devnet (idempotent):

```protobuf
rpc ApplyDevnet(ApplyDevnetRequest) returns (ApplyDevnetResponse);

message ApplyDevnetRequest {
    string name = 1;
    string namespace = 2;
    DevnetSpec spec = 3;
    map<string, string> labels = 4;
    map<string, string> annotations = 5;
}

message ApplyDevnetResponse {
    Devnet devnet = 1;
    string action = 2;  // "created", "configured", "unchanged"
}
```

### UpdateDevnet

Update an existing devnet:

```protobuf
rpc UpdateDevnet(UpdateDevnetRequest) returns (UpdateDevnetResponse);

message UpdateDevnetRequest {
    string name = 1;
    string namespace = 2;
    DevnetSpec spec = 3;
    map<string, string> labels = 4;
    map<string, string> annotations = 5;
}

message UpdateDevnetResponse {
    Devnet devnet = 1;
}
```

## NodeService

Manages individual blockchain nodes within devnets.

### GetNode

Get node details:

```protobuf
rpc GetNode(GetNodeRequest) returns (GetNodeResponse);

message GetNodeRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
}

message GetNodeResponse {
    Node node = 1;
}
```

### ListNodes

List nodes in a devnet:

```protobuf
rpc ListNodes(ListNodesRequest) returns (ListNodesResponse);

message ListNodesRequest {
    string namespace = 1;
    string devnet_name = 2;
}

message ListNodesResponse {
    repeated Node nodes = 1;
}
```

### StartNode / StopNode / RestartNode

Control node state:

```protobuf
rpc StartNode(StartNodeRequest) returns (StartNodeResponse);
rpc StopNode(StopNodeRequest) returns (StopNodeResponse);
rpc RestartNode(RestartNodeRequest) returns (RestartNodeResponse);

message StartNodeRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
}

message StopNodeRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
}

message RestartNodeRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
}
```

### GetNodeHealth

Check node health:

```protobuf
rpc GetNodeHealth(GetNodeHealthRequest) returns (GetNodeHealthResponse);

message GetNodeHealthRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
}

message GetNodeHealthResponse {
    string phase = 1;
    int64 block_height = 2;
    bool catching_up = 3;
    int32 peer_count = 4;
    string rpc_status = 5;
}
```

### ExecInNode

Execute a command in a node container:

```protobuf
rpc ExecInNode(ExecInNodeRequest) returns (ExecInNodeResponse);

message ExecInNodeRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
    repeated string command = 4;
}

message ExecInNodeResponse {
    int32 exit_code = 1;
    string stdout = 2;
    string stderr = 3;
}
```

### GetNodePorts

Get exposed ports for a node:

```protobuf
rpc GetNodePorts(GetNodePortsRequest) returns (GetNodePortsResponse);

message GetNodePortsRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
}

message GetNodePortsResponse {
    map<string, int32> ports = 1;  // e.g., {"rpc": 26657, "grpc": 9090}
}
```

### StreamNodeLogs

Stream logs from a node:

```protobuf
rpc StreamNodeLogs(StreamNodeLogsRequest) returns (stream LogEntry);

message StreamNodeLogsRequest {
    string namespace = 1;
    string devnet_name = 2;
    string node_name = 3;
    bool follow = 4;
    int32 tail = 5;
}

message LogEntry {
    string content = 1;
    google.protobuf.Timestamp timestamp = 2;
}
```

## TransactionService

Manages transaction lifecycle for blockchain operations.

### SubmitTransaction

Submit a transaction:

```protobuf
rpc SubmitTransaction(SubmitTransactionRequest) returns (SubmitTransactionResponse);

message SubmitTransactionRequest {
    string devnet = 1;
    string tx_type = 2;
    string signer = 3;
    bytes payload = 4;
    uint64 gas_limit = 5;
    string memo = 6;
}

message SubmitTransactionResponse {
    Transaction transaction = 1;
}
```

**Example:**

```go
client := pb.NewTransactionServiceClient(conn)

payload := []byte(`{"to_address":"cosmos1...","amount":"1000000uatom"}`)

resp, err := client.SubmitTransaction(ctx, &pb.SubmitTransactionRequest{
    Devnet:   "my-devnet",
    TxType:   "bank/send",
    Signer:   "validator:0",
    Payload:  payload,
    GasLimit: 200000,
})
```

### GetTransaction

Get transaction status:

```protobuf
rpc GetTransaction(GetTransactionRequest) returns (GetTransactionResponse);

message GetTransactionRequest {
    string name = 1;
}

message GetTransactionResponse {
    Transaction transaction = 1;
}

message Transaction {
    string name = 1;
    string devnet_ref = 2;
    string tx_type = 3;
    string signer = 4;
    bytes payload = 5;
    string phase = 6;      // Pending, Building, Signing, Submitted, Confirmed, Failed
    string tx_hash = 7;
    int64 height = 8;
    int64 gas_used = 9;
    string error = 10;
    string message = 11;
    google.protobuf.Timestamp created_at = 12;
    google.protobuf.Timestamp updated_at = 13;
}
```

### ListTransactions

List transactions:

```protobuf
rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse);

message ListTransactionsRequest {
    string devnet = 1;   // Required
    string tx_type = 2;  // Optional filter
    string phase = 3;    // Optional filter
    int32 limit = 4;     // Optional limit
}

message ListTransactionsResponse {
    repeated Transaction transactions = 1;
}
```

### CancelTransaction

Cancel a pending transaction:

```protobuf
rpc CancelTransaction(CancelTransactionRequest) returns (CancelTransactionResponse);

message CancelTransactionRequest {
    string name = 1;
}

message CancelTransactionResponse {
    Transaction transaction = 1;
}
```

### SubmitGovVote

Submit a governance vote transaction:

```protobuf
rpc SubmitGovVote(SubmitGovVoteRequest) returns (SubmitGovVoteResponse);

message SubmitGovVoteRequest {
    string devnet = 1;
    uint64 proposal_id = 2;
    string vote_option = 3;  // "yes", "no", "abstain", "no_with_veto"
    string voter = 4;
}

message SubmitGovVoteResponse {
    Transaction transaction = 1;
}
```

### SubmitGovProposal

Submit a governance proposal transaction:

```protobuf
rpc SubmitGovProposal(SubmitGovProposalRequest) returns (SubmitGovProposalResponse);

message SubmitGovProposalRequest {
    string devnet = 1;
    string proposal_type = 2;
    string title = 3;
    string description = 4;
    bytes content = 5;
    string proposer = 6;
}

message SubmitGovProposalResponse {
    Transaction transaction = 1;
}
```

## UpgradeService

Manages chain upgrades with governance proposal flow.

### CreateUpgrade

Create an upgrade:

```protobuf
rpc CreateUpgrade(CreateUpgradeRequest) returns (CreateUpgradeResponse);

message CreateUpgradeRequest {
    string name = 1;
    string namespace = 2;
    UpgradeSpec spec = 3;
}

message UpgradeSpec {
    string devnet_ref = 1;
    string upgrade_name = 2;
    int64 target_height = 3;
    BinarySource new_binary = 4;
    bool auto_vote = 5;
}

message CreateUpgradeResponse {
    Upgrade upgrade = 1;
}
```

### GetUpgrade

Get upgrade status:

```protobuf
rpc GetUpgrade(GetUpgradeRequest) returns (GetUpgradeResponse);

message GetUpgradeRequest {
    string name = 1;
    string namespace = 2;
}

message GetUpgradeResponse {
    Upgrade upgrade = 1;
}

message Upgrade {
    string name = 1;
    string namespace = 2;
    UpgradeSpec spec = 3;
    UpgradeStatus status = 4;
    google.protobuf.Timestamp created_at = 5;
    google.protobuf.Timestamp updated_at = 6;
}

message UpgradeStatus {
    string phase = 1;      // Pending, Proposing, Voting, Waiting, Switching, Verifying, Completed, Failed
    uint64 proposal_id = 2;
    int64 current_height = 3;
    string message = 4;
    string error = 5;
}
```

### ListUpgrades

List upgrades:

```protobuf
rpc ListUpgrades(ListUpgradesRequest) returns (ListUpgradesResponse);

message ListUpgradesRequest {
    string namespace = 1;
    string devnet_name = 2;  // Optional filter
}

message ListUpgradesResponse {
    repeated Upgrade upgrades = 1;
}
```

### DeleteUpgrade

Delete an upgrade (only pending/completed/failed):

```protobuf
rpc DeleteUpgrade(DeleteUpgradeRequest) returns (DeleteUpgradeResponse);

message DeleteUpgradeRequest {
    string name = 1;
    string namespace = 2;
}

message DeleteUpgradeResponse {
    bool deleted = 1;
}
```

### CancelUpgrade

Cancel an in-progress upgrade:

```protobuf
rpc CancelUpgrade(CancelUpgradeRequest) returns (CancelUpgradeResponse);

message CancelUpgradeRequest {
    string name = 1;
    string namespace = 2;
}

message CancelUpgradeResponse {
    Upgrade upgrade = 1;
}
```

### RetryUpgrade

Retry a failed upgrade:

```protobuf
rpc RetryUpgrade(RetryUpgradeRequest) returns (RetryUpgradeResponse);

message RetryUpgradeRequest {
    string name = 1;
    string namespace = 2;
}

message RetryUpgradeResponse {
    Upgrade upgrade = 1;
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
