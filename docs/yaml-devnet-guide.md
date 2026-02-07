# YAML-Based Devnet Management

This guide covers the declarative YAML-based approach to managing devnets.

## Overview

Devnet Builder supports Kubernetes-style YAML resource definitions for declarative devnet management. This allows you to:

- Define devnet configurations as code
- Version control your devnet setups
- Apply configurations idempotently
- Preview changes before applying

## Resource Definition

### Basic Structure

```yaml
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: my-devnet
  namespace: default            # Optional: namespace for isolation (default: "default")
  labels:
    team: core
    environment: development
spec:
  network: stable
  validators: 4
  mode: docker
```

### Full Specification

```yaml
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: my-devnet
  namespace: default           # Optional: namespace for isolation (default: "default")
  labels:
    team: core
    environment: development
  annotations:
    description: "Development network for testing"
spec:
  # Network configuration (required)
  network: stable              # Required: network plugin (stable, osmosis, geth, etc.)

  # Network options (optional)
  networkType: mainnet         # Optional: mainnet, testnet (default: mainnet)
  networkVersion: v1.0.0       # Optional: specific SDK/binary version

  # Node configuration
  validators: 4                # Number of validator nodes (min: 1, default: 1)
  fullNodes: 0                 # Number of full nodes (default: 0)
  accounts: 10                 # Number of accounts to create (default: 0)

  # Execution mode
  mode: docker                 # docker or local (default: docker)

  # Resource limits (optional, Docker mode only)
  resources:
    cpu: "2"
    memory: "4Gi"
    storage: "10Gi"

  # Per-node overrides (optional)
  nodes:
    - index: 0
      role: validator
      resources:
        cpu: "4"
        memory: "8Gi"

  # Daemon configuration (optional)
  daemon:
    autoStart: true
    idleTimeout: "30m"
    logs:
      bufferSize: "10000"
      retention: "24h"
```

## Commands

### apply

Apply a devnet configuration from a YAML file.

```bash
# Apply configuration
devnet-builder apply -f devnet.yaml

# Preview changes without applying (dry-run)
devnet-builder apply -f devnet.yaml --dry-run

# Force recreation (destroy + create)
devnet-builder apply -f devnet.yaml --force

# Apply all YAML files in a directory
devnet-builder apply -f ./devnets/

# Output as JSON
devnet-builder apply -f devnet.yaml --dry-run -o json
```

**Flags:**
- `-f, --file`: Path to YAML file or directory (required)
- `--dry-run`: Preview changes without applying
- `--force`: Force recreation of existing devnet
- `-o, --output`: Output format (text, json)

## Examples

### Simple 4-Node Devnet

```yaml
# devnet.yaml
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test-devnet
spec:
  network: stable
  validators: 4
  mode: docker
```

```bash
devnet-builder apply -f devnet.yaml
```

### Multi-Environment Setup

```yaml
# devnets/dev.yaml
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: dev-network
  labels:
    environment: development
spec:
  network: stable
  validators: 2
  mode: docker
---
# devnets/staging.yaml
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: staging-network
  labels:
    environment: staging
spec:
  network: stable
  validators: 4
  mode: docker
  resources:
    cpu: "4"
    memory: "8Gi"
```

```bash
# Apply all environments
devnet-builder apply -f ./devnets/
```

### Production-Like Configuration

```yaml
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: prod-like
  namespace: staging
  labels:
    team: infra
    environment: staging
  annotations:
    description: "Production-like environment for integration testing"
spec:
  network: stable
  networkType: mainnet
  networkVersion: v1.2.0
  validators: 4                # Max 4 validators supported
  fullNodes: 3
  accounts: 100
  mode: docker
  resources:
    cpu: "4"
    memory: "8Gi"
    storage: "50Gi"
  nodes:
    - index: 0
      role: validator
      resources:
        cpu: "8"
        memory: "16Gi"
  daemon:
    autoStart: true
    idleTimeout: "1h"
    logs:
      bufferSize: "50000"
      retention: "72h"
```

### Multi-Document YAML

You can define multiple devnets in a single file using YAML document separators:

```yaml
# multi-devnet.yaml
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: network-a
spec:
  network: stable
  validators: 2
---
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: network-b
spec:
  network: stable
  validators: 4
```

## Workflow

### 1. Create Configuration

Create a YAML file defining your devnet:

```bash
cat > devnet.yaml << 'EOF'
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: my-devnet
spec:
  network: stable
  validators: 4
  mode: docker
EOF
```

### 2. Preview Changes

Use dry-run to see what will be created:

```bash
devnet-builder apply -f devnet.yaml --dry-run
```

Output:
```
Devnet: my-devnet (dry-run)

Plan: 1 to create, 0 to update, 0 to destroy

+ devnet/my-devnet
    network:        stable
    networkVersion:
    mode:           docker
    validators:     4

  + node/my-devnet-0 (validator)
  + node/my-devnet-1 (validator)
  + node/my-devnet-2 (validator)
  + node/my-devnet-3 (validator)

Run without --dry-run to apply.
```

### 3. Apply Configuration

Apply the configuration:

```bash
devnet-builder apply -f devnet.yaml
```

### 4. Apply Updates

Apply the updated configuration:

```bash
devnet-builder apply -f devnet.yaml
```

## Validation

The YAML loader validates configurations and reports errors:

```bash
$ devnet-builder apply -f invalid.yaml
Error: validation failed:
  - metadata.name is required
  - spec.network is required
  - spec.validators must be at least 1
  - spec.mode must be 'docker' or 'local'
  - spec.networkType must be 'mainnet' or 'testnet'
```

## Field Reference

### Metadata Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Unique name for the devnet |
| `namespace` | string | No | `default` | Namespace for resource isolation |
| `labels` | map[string]string | No | - | Key-value pairs for organization |
| `annotations` | map[string]string | No | - | Arbitrary metadata |

### Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `network` | string | Yes | - | Network plugin name (stable, osmosis, geth, etc.) |
| `networkType` | string | No | `mainnet` | Network source: mainnet or testnet |
| `networkVersion` | string | No | (plugin default) | Specific SDK/binary version |
| `mode` | string | No | `docker` | Execution mode: docker or local |
| `validators` | int | No | `1` | Number of validator nodes (min: 1) |
| `fullNodes` | int | No | `0` | Number of full nodes |
| `accounts` | int | No | `0` | Number of funded accounts to create |

### Resources Fields (Optional)

| Field | Type | Description |
|-------|------|-------------|
| `cpu` | string | CPU limit (e.g., "2", "4") |
| `memory` | string | Memory limit (e.g., "4Gi", "8Gi") |
| `storage` | string | Storage limit (e.g., "10Gi") |

### Node Override Fields (Optional)

| Field | Type | Description |
|-------|------|-------------|
| `index` | int | Node index (0-based) |
| `role` | string | Node role: validator or fullnode |
| `resources` | Resources | Per-node resource overrides |

### Daemon Config Fields (Optional)

| Field | Type | Description |
|-------|------|-------------|
| `autoStart` | bool | Auto-start nodes on daemon launch |
| `idleTimeout` | string | Idle timeout duration (e.g., "30m") |
| `logs.bufferSize` | string | Log buffer size |
| `logs.retention` | string | Log retention period (e.g., "24h") |

## Best Practices

1. **Version Control**: Store YAML files in git alongside your application code
2. **Use Labels**: Add labels for filtering and organization
3. **Dry Run First**: Always preview changes with `--dry-run` before applying
4. **Environment Separation**: Use separate files or directories for different environments
5. **Resource Limits**: Specify resource limits for production-like environments
6. **Namespaces**: Use namespaces to isolate devnets for different projects or teams
