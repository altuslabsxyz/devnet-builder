# Binary Passthrough Feature Design

## Overview
Enable `devnet-builder` to passthrough commands to active plugin binaries, allowing users to execute commands like:
```bash
devnet-builder stabled status
devnet-builder stabled tx bank send ...
```

This design follows Clean Architecture and SOLID principles.

## Architecture

### 1. Domain Layer (Core Business Logic)

#### Domain Entities
- `BinaryCommand`: Represents a command to be passed to a binary
  - BinaryName: string
  - Args: []string
  - WorkDir: string

#### Domain Services
- `BinaryResolver`: Interface for resolving binary paths from plugin names
  ```go
  type BinaryResolver interface {
      ResolveBinary(ctx context.Context, pluginName string) (binaryPath string, err error)
      GetActiveBinary(ctx context.Context) (binaryPath string, pluginName string, err error)
  }
  ```

- `BinaryExecutor`: Interface for executing binary commands
  ```go
  type BinaryExecutor interface {
      Execute(ctx context.Context, cmd BinaryCommand) error
  }
  ```

### 2. Application Layer (Use Cases)

#### Use Case: ExecuteBinaryPassthrough
```go
type ExecuteBinaryPassthroughUseCase struct {
    resolver BinaryResolver
    executor BinaryExecutor
    cache    BinaryCache
}

func (uc *ExecuteBinaryPassthroughUseCase) Execute(ctx context.Context, binaryName string, args []string) error
```

**Responsibilities:**
1. Resolve the binary name to an actual binary path
2. Validate the binary exists and is executable
3. Execute the binary with provided arguments
4. Stream output to user

### 3. Interface Adapters Layer

#### Plugin Binary Adapter
Adapts plugin system to BinaryResolver interface:
```go
type PluginBinaryResolver struct {
    loader        *plugin.Loader
    cache         ports.BinaryCache
    networkModule network.Module
}
```

**Responsibilities:**
- Query plugin loader for available plugins
- Extract binary name from plugin Module interface
- Check if plugin binary exists in cache
- Return full path to cached binary

#### Process Executor Adapter
Adapts existing ProcessExecutor to BinaryExecutor:
```go
type ProcessBinaryExecutor struct {
    executor ports.ProcessExecutor
}
```

### 4. Infrastructure Layer

#### CLI Command Handler
```go
func NewBinaryPassthroughCmd(container *di.Container) *cobra.Command
```

**Command Routing Logic:**
1. Check if first argument matches a known plugin binary name
2. If yes, route to binary passthrough handler
3. If no, continue with normal command handling

### 5. Integration Points

#### Root Command Enhancement
Modify `root.go` to:
1. Discover available plugins at startup
2. Create dynamic subcommands for each plugin binary
3. Route unknown commands to passthrough handler

```go
// In NewRootCmd()
func enhanceWithBinaryPassthrough(rootCmd *cobra.Command, container *di.Container) {
    // Discover plugins
    plugins := discoverPluginBinaries(container)

    // Add dynamic commands
    for _, pluginName := range plugins {
        passthroughCmd := createPassthroughCommand(pluginName, container)
        rootCmd.AddCommand(passthroughCmd)
    }
}
```

## Implementation Strategy

### Phase 1: Core Infrastructure
1. Define domain interfaces (BinaryResolver, BinaryExecutor)
2. Implement PluginBinaryResolver
3. Implement ProcessBinaryExecutor

### Phase 2: Use Case Implementation
1. Implement ExecuteBinaryPassthroughUseCase
2. Add validation and error handling
3. Add logging and telemetry

### Phase 3: CLI Integration
1. Create binary passthrough command handler
2. Integrate with root command
3. Add command completion support

### Phase 4: Testing
1. Unit tests for resolvers and executors
2. Integration tests with mock plugins
3. E2E tests with actual plugin binaries

## SOLID Principles Compliance

### Single Responsibility Principle (SRP)
- `BinaryResolver`: Only resolves binary paths
- `BinaryExecutor`: Only executes binaries
- `ExecuteBinaryPassthroughUseCase`: Orchestrates the passthrough flow

### Open/Closed Principle (OCP)
- Interfaces allow extension without modification
- New binary sources can be added by implementing BinaryResolver

### Liskov Substitution Principle (LSP)
- All BinaryResolver implementations are interchangeable
- All BinaryExecutor implementations are interchangeable

### Interface Segregation Principle (ISP)
- Small, focused interfaces (BinaryResolver, BinaryExecutor)
- Clients depend only on methods they use

### Dependency Inversion Principle (DIP)
- High-level use cases depend on abstractions (interfaces)
- Low-level implementations depend on abstractions
- DI container manages dependencies

## Error Handling

1. **Plugin Not Found**: Return clear error with available plugins
2. **Binary Not Cached**: Suggest running build/download first
3. **Binary Not Executable**: Check permissions and report
4. **Execution Failure**: Stream stderr and return exit code

## Security Considerations

1. Validate plugin binaries are in expected directories
2. Check binary signatures/checksums (future enhancement)
3. Prevent path traversal attacks
4. Limit execution to whitelisted plugins

## Example Usage

```bash
# Get active binary from config
devnet-builder stabled status

# Explicit plugin specification
devnet-builder --blockchain-network stable stabled tx bank send ...

# With passthrough to full binary
devnet-builder stabled query bank balances cosmos1...
```

## Configuration

Add to `config.toml`:
```toml
[binary_passthrough]
enabled = true
allow_all_plugins = true  # or whitelist specific plugins
whitelisted_plugins = ["stable", "ault"]
```
