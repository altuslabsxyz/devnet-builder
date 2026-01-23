# Daemon Refactor Phase 2: End-to-End Devnet Slice

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement full Devnet lifecycle through gRPC API, Controller Manager, and DevnetController, enabling `dvb` to create/manage devnets via the daemon.

**Architecture:** gRPC server receives requests → stores resources in BoltDB → Controller Manager watches store → DevnetController reconciles desired vs actual state → delegates to existing DI container for Docker orchestration.

**Tech Stack:** Go 1.24, Protocol Buffers, gRPC, BoltDB (bbolt), Cobra CLI

**Design Document:** `docs/plans/2026-01-23-daemon-refactor-design.md`

---

## Task 1: Add gRPC Dependencies

**Files:**
- Modify: `go.mod`

**Step 1: Add gRPC and protobuf dependencies**

```bash
go get google.golang.org/grpc@latest
go get google.golang.org/protobuf@latest
go mod tidy
```

**Step 2: Verify dependencies added**

Run: `grep -E "grpc|protobuf" go.mod`
Expected: Both dependencies present

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add gRPC and protobuf dependencies"
```

---

## Task 2: Define Devnet Protocol Buffers

**Files:**
- Create: `api/proto/v1/devnet.proto`
- Create: `api/proto/v1/generate.go`

**Step 1: Write devnet.proto**

```protobuf
// api/proto/v1/devnet.proto
syntax = "proto3";

package devnetd.v1;

option go_package = "github.com/altuslabsxyz/devnet-builder/api/proto/v1;v1";

import "google/protobuf/timestamp.proto";

service DevnetService {
  rpc CreateDevnet(CreateDevnetRequest) returns (Devnet);
  rpc GetDevnet(GetDevnetRequest) returns (Devnet);
  rpc ListDevnets(ListDevnetsRequest) returns (ListDevnetsResponse);
  rpc DeleteDevnet(DeleteDevnetRequest) returns (DeleteDevnetResponse);
  rpc StartDevnet(StartDevnetRequest) returns (Devnet);
  rpc StopDevnet(StopDevnetRequest) returns (Devnet);
}

message Devnet {
  DevnetMetadata metadata = 1;
  DevnetSpec spec = 2;
  DevnetStatus status = 3;
}

message DevnetMetadata {
  string name = 1;
  int64 generation = 2;
  google.protobuf.Timestamp created_at = 3;
  google.protobuf.Timestamp updated_at = 4;
  map<string, string> labels = 5;
  map<string, string> annotations = 6;
}

message DevnetSpec {
  string plugin = 1;
  string network_type = 2;
  int32 validators = 3;
  int32 full_nodes = 4;
  string mode = 5;
  string sdk_version = 6;
  string genesis_path = 7;
  string snapshot_url = 8;
}

message DevnetStatus {
  string phase = 1;
  int32 nodes = 2;
  int32 ready_nodes = 3;
  int64 current_height = 4;
  string sdk_version = 5;
  google.protobuf.Timestamp last_health_check = 6;
  string message = 7;
}

message CreateDevnetRequest {
  string name = 1;
  DevnetSpec spec = 2;
  map<string, string> labels = 3;
}

message GetDevnetRequest { string name = 1; }
message ListDevnetsRequest { map<string, string> label_selector = 1; }
message ListDevnetsResponse { repeated Devnet devnets = 1; }
message DeleteDevnetRequest { string name = 1; }
message DeleteDevnetResponse { bool deleted = 1; }
message StartDevnetRequest { string name = 1; }
message StopDevnetRequest { string name = 1; }
```

**Step 2: Generate Go code**

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
cd api/proto/v1
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative devnet.proto
```

**Step 3: Verify generated files**

Run: `ls api/proto/v1/*.pb.go`
Expected: `devnet.pb.go` and `devnet_grpc.pb.go`

**Step 4: Commit**

```bash
git add api/proto/v1/
git commit -m "feat(proto): add Devnet service protocol buffers"
```

---

## Task 3: Implement Work Queue

**Files:**
- Create: `internal/daemon/controller/queue.go`
- Test: `internal/daemon/controller/queue_test.go`

TDD approach - write test first, then implement.

**Step 1: Write queue_test.go** (tests Add, Get, Deduplication, Requeue, Shutdown)

**Step 2: Write queue.go** (WorkQueue with dirty set, processing set, condition variable)

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git add internal/daemon/controller/
git commit -m "feat(controller): add work queue with deduplication"
```

---

## Task 4: Implement Controller Manager

**Files:**
- Create: `internal/daemon/controller/manager.go`
- Test: `internal/daemon/controller/manager_test.go`

TDD approach.

**Step 1: Write manager_test.go** (tests Register, Run, Routes, Requeue on error)

**Step 2: Write manager.go** (Manager with controllers map, queues map, Start, Enqueue)

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git add internal/daemon/controller/
git commit -m "feat(controller): add controller manager"
```

---

## Task 5: Implement DevnetController

**Files:**
- Create: `internal/daemon/controller/devnet.go`
- Create: `internal/daemon/store/memory.go`
- Test: `internal/daemon/controller/devnet_test.go`

TDD approach.

**Step 1: Write devnet_test.go** (tests ReconcileNew, ReconcileDeleted, ReconcileRunning)

**Step 2: Write memory.go** (MemoryStore for testing)

**Step 3: Write devnet.go** (DevnetController with phase-based reconciliation)

**Step 4: Run tests, verify pass**

**Step 5: Commit**

```bash
git add internal/daemon/controller/ internal/daemon/store/memory.go
git commit -m "feat(controller): add DevnetController with reconciliation"
```

---

## Task 6: Implement Proto Converters

**Files:**
- Create: `internal/daemon/server/convert.go`
- Test: `internal/daemon/server/convert_test.go`

TDD approach.

**Step 1: Write convert_test.go** (tests DevnetToProto, DevnetFromProto)

**Step 2: Write convert.go** (conversion functions)

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git add internal/daemon/server/
git commit -m "feat(server): add proto converters for Devnet"
```

---

## Task 7: Implement gRPC DevnetService

**Files:**
- Create: `internal/daemon/server/devnet_service.go`
- Test: `internal/daemon/server/devnet_service_test.go`

TDD approach.

**Step 1: Write devnet_service_test.go** (tests Create, CreateAlreadyExists, Get, GetNotFound, List, Delete)

**Step 2: Write devnet_service.go** (DevnetService implementing gRPC interface)

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git add internal/daemon/server/
git commit -m "feat(server): add gRPC DevnetService"
```

---

## Task 8: Wire gRPC Server into devnetd

**Files:**
- Modify: `cmd/devnetd/main.go`
- Modify: `internal/daemon/server/server.go`

**Step 1: Update server.go** to create gRPC server, register DevnetService, start controller manager

**Step 2: Update main.go** with flags and logger setup

**Step 3: Verify builds**: `go build ./cmd/devnetd`

**Step 4: Commit**

```bash
git add cmd/devnetd/ internal/daemon/server/
git commit -m "feat(devnetd): wire gRPC server with DevnetService"
```

---

## Task 9: Add gRPC Client to dvb

**Files:**
- Create: `internal/client/grpc.go`
- Modify: `internal/client/detect.go`
- Modify: `cmd/dvb/main.go`

**Step 1: Write grpc.go** (GRPCClient wrapper with CreateDevnet, GetDevnet, etc.)

**Step 2: Update detect.go** with DefaultSocketPath, IsDaemonRunning

**Step 3: Update main.go** with deploy, list, status, start, stop, destroy commands

**Step 4: Verify builds**: `go build ./cmd/dvb`

**Step 5: Commit**

```bash
git add cmd/dvb/ internal/client/
git commit -m "feat(dvb): add gRPC client and daemon commands"
```

---

## Task 10: Integration Test

**Files:**
- Create: `internal/daemon/integration_test.go`

**Step 1: Write integration_test.go** (start server, connect client, Create/List/Delete)

**Step 2: Run**: `go test ./internal/daemon/... -tags=integration -v`

**Step 3: Commit**

```bash
git add internal/daemon/
git commit -m "test: add daemon integration test"
```

---

## Summary

| Task | Component | Key Files |
|------|-----------|-----------|
| 1 | Dependencies | `go.mod` |
| 2 | Protocol Buffers | `api/proto/v1/devnet.proto` |
| 3 | Work Queue | `internal/daemon/controller/queue.go` |
| 4 | Controller Manager | `internal/daemon/controller/manager.go` |
| 5 | DevnetController | `internal/daemon/controller/devnet.go` |
| 6 | Proto Converters | `internal/daemon/server/convert.go` |
| 7 | gRPC DevnetService | `internal/daemon/server/devnet_service.go` |
| 8 | Wire devnetd | `cmd/devnetd/main.go`, `internal/daemon/server/server.go` |
| 9 | Wire dvb | `cmd/dvb/main.go`, `internal/client/grpc.go` |
| 10 | Integration Test | `internal/daemon/integration_test.go` |

**Next Phase:** Phase 3 will add DevnetProvisioner integration with existing DI container for actual Docker orchestration.
