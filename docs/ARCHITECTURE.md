# Clean Architecture Migration Guide

## Overview

This document describes the Clean Architecture implementation in devnet-builder and provides guidance for migrating legacy code to the new architecture.

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│                       cmd/ (Interfaces)                     │
│  CLI commands, HTTP handlers, gRPC services                 │
├─────────────────────────────────────────────────────────────┤
│                    internal/di/ (DI Container)              │
│  Dependency injection, Factory patterns, Wiring             │
├─────────────────────────────────────────────────────────────┤
│              internal/application/ (Application Layer)      │
│  UseCases, DTOs, Ports (interfaces)                        │
├─────────────────────────────────────────────────────────────┤
│            internal/infrastructure/ (Infrastructure)        │
│  Adapters implementing Ports                                │
├─────────────────────────────────────────────────────────────┤
│                internal/domain/ (Domain Layer)              │
│  Entities, Value Objects, Domain Services                   │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
internal/
├── application/           # Application layer
│   ├── ports/            # Interfaces (contracts)
│   │   ├── repositories.go   # DevnetRepository, NodeRepository
│   │   ├── executors.go      # ProcessExecutor, DockerExecutor
│   │   ├── clients.go        # RPCClient, EVMClient, BinaryCache
│   │   ├── services.go       # HealthChecker, NetworkModule
│   │   └── logger.go         # Logger interface
│   ├── dto/              # Data Transfer Objects
│   │   ├── devnet_dto.go     # ProvisionInput/Output, RunInput/Output, etc.
│   │   ├── upgrade_dto.go    # ProposeInput/Output, VoteInput/Output, etc.
│   │   └── build_dto.go      # BuildInput/Output, etc.
│   ├── devnet/           # Devnet UseCases
│   │   ├── provision.go      # ProvisionUseCase
│   │   ├── run.go            # RunUseCase, StopUseCase
│   │   ├── health.go         # HealthUseCase
│   │   └── reset.go          # ResetUseCase, DestroyUseCase
│   ├── upgrade/          # Upgrade UseCases
│   │   ├── propose.go        # ProposeUseCase
│   │   ├── vote.go           # VoteUseCase
│   │   ├── switch.go         # SwitchBinaryUseCase
│   │   └── execute.go        # ExecuteUpgradeUseCase
│   └── build/            # Build UseCases
│       └── build.go          # BuildUseCase, CacheListUseCase, CacheCleanUseCase
│
├── infrastructure/       # Infrastructure adapters
│   ├── persistence/      # File-based storage
│   │   ├── devnet_repository.go
│   │   └── node_repository.go
│   ├── process/          # Process execution
│   │   ├── local_executor.go
│   │   └── docker_executor.go
│   ├── rpc/              # RPC clients
│   │   └── cosmos_client.go
│   ├── cache/            # Binary caching
│   │   └── binary_cache_adapter.go
│   ├── builder/          # Binary building
│   │   └── builder_adapter.go
│   ├── node/             # Node management
│   │   └── manager.go
│   ├── genesis/          # Genesis fetching
│   │   └── fetcher.go
│   └── snapshot/         # Snapshot fetching
│       └── fetcher.go
│
├── di/                   # Dependency Injection
│   ├── container.go      # DI Container with lazy UseCase init
│   └── factory.go        # InfrastructureFactory for wiring
│
├── domain/               # Domain entities
│   └── entities.go       # Devnet, Node, Validator, etc.
│
└── [legacy packages]     # To be gradually migrated
    ├── devnet/           # Legacy devnet management
    ├── upgrade/          # Legacy upgrade management
    ├── builder/          # Legacy binary builder
    ├── node/             # Legacy node management
    ├── cache/            # Legacy binary cache
    └── snapshot/         # Legacy snapshot management
```

## Using the DI Container

### Initialization (cmd/devnet-builder/app.go)

```go
// Initialize container with configuration
container, err := InitContainer(AppConfig{
    HomeDir:           homeDir,
    BlockchainNetwork: "stable",
    ExecutionMode:     "docker",
    Verbose:           verbose,
    NoColor:           noColor,
    JSONMode:          jsonMode,
})
if err != nil {
    return err
}

// Use UseCases from container
result, err := container.DestroyUseCase().Execute(ctx, dto.DestroyInput{
    HomeDir: homeDir,
    Force:   true,
})
```

### Available UseCases

| UseCase | Description |
|---------|-------------|
| `ProvisionUseCase` | Create devnet configuration and genesis |
| `RunUseCase` | Start devnet nodes |
| `StopUseCase` | Stop running nodes |
| `HealthUseCase` | Check node health status |
| `ResetUseCase` | Reset devnet data |
| `DestroyUseCase` | Destroy devnet completely |
| `ProposeUseCase` | Submit upgrade proposal |
| `VoteUseCase` | Vote on proposal |
| `SwitchBinaryUseCase` | Switch node binary |
| `ExecuteUpgradeUseCase` | Full upgrade orchestration |
| `BuildUseCase` | Build binary from source |
| `CacheListUseCase` | List cached binaries |
| `CacheCleanUseCase` | Clean binary cache |

## Migration Guide

### Step 1: Initialize Container in Command

```go
func runMyCommand(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    // Initialize container
    container, err := InitContainerForCommand(blockchainNetwork, executionMode)
    if err != nil {
        return err
    }

    // Use UseCase instead of legacy package
    result, err := container.DestroyUseCase().Execute(ctx, dto.DestroyInput{
        HomeDir: homeDir,
    })
    if err != nil {
        return err
    }

    // Handle result
    ...
}
```

### Step 2: Replace Legacy Imports

**Before (Legacy):**
```go
import (
    "github.com/b-harvest/devnet-builder/internal/devnet"
)

// Using legacy package directly
d, err := devnet.LoadDevnetWithNodes(homeDir, logger)
d.Stop(ctx, timeout)
```

**After (Clean Architecture):**
```go
import (
    "github.com/b-harvest/devnet-builder/internal/application/dto"
)

// Using DI Container
container := GetContainer()
result, err := container.StopUseCase().Execute(ctx, dto.StopInput{
    HomeDir: homeDir,
    Timeout: timeout,
})
```

### Step 3: Use DTOs for Input/Output

```go
// Input DTO
input := dto.ProvisionInput{
    HomeDir:       homeDir,
    NetworkName:   "mainnet",
    NumValidators: 4,
    ExecutionMode: ports.ExecutionModeDocker,
}

// Execute UseCase
output, err := container.ProvisionUseCase().Execute(ctx, input)

// Output DTO
fmt.Printf("Created %d nodes at %s\n", len(output.Nodes), output.HomeDir)
```

## Legacy Package Status

| Package | Status | Replacement |
|---------|--------|-------------|
| `internal/devnet/` | **Legacy** | `application/devnet/` + `infrastructure/persistence/` |
| `internal/upgrade/` | **Legacy** | `application/upgrade/` |
| `internal/builder/` | **Legacy** | `infrastructure/builder/` |
| `internal/cache/` | **Legacy** | `infrastructure/cache/` |
| `internal/node/` | **Legacy** | `infrastructure/node/` |
| `internal/snapshot/` | **Legacy** | `infrastructure/snapshot/` |
| `internal/network/` | **Keep** | Core plugin interface |
| `internal/output/` | **Keep** | Logger utility |
| `internal/config/` | **Keep** | Configuration management |
| `internal/interactive/` | **Keep** | TUI components |
| `internal/github/` | **Keep** | GitHub API client |

## Testing with DI

The DI Container makes testing easy by allowing mock injection:

```go
func TestProvision(t *testing.T) {
    // Create mock implementations
    mockDevnetRepo := &MockDevnetRepository{}
    mockNodeRepo := &MockNodeRepository{}

    // Create container with mocks
    container := di.New(
        di.WithDevnetRepository(mockDevnetRepo),
        di.WithNodeRepository(mockNodeRepo),
    )

    // Test UseCase
    uc := container.ProvisionUseCase()
    result, err := uc.Execute(ctx, input)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 4, len(result.Nodes))
}
```

## Best Practices

1. **Always use DTOs** for UseCase input/output
2. **Never import infrastructure** directly from commands
3. **Use ports interfaces** in application layer
4. **Keep UseCases focused** on single responsibility
5. **Test through DI** with mock implementations
6. **Initialize container once** per application lifecycle
