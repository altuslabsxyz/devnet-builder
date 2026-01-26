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
  labels:
    team: core
    environment: development
  annotations:
    description: "Development network for testing"
spec:
  # Network configuration
  network: stable              # Required: network plugin (stable, cosmos, etc.)
  networkType: mainnet         # Optional: mainnet, testnet (default: mainnet)
  networkVersion: v1.0.0       # Optional: specific SDK version

  # Node configuration
  validators: 4                # Number of validator nodes (default: 1)
  fullNodes: 0                 # Number of full nodes (default: 0)
  accounts: 10                 # Number of accounts to create (default: 10)

  # Execution mode
  mode: docker                 # docker or local (default: docker)

  # Resource limits (optional)
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

### diff

Compare a YAML definition against the current state.

```bash
# Show differences
devnet-builder diff -f devnet.yaml

# Output as JSON
devnet-builder diff -f devnet.yaml -o json
```

**Flags:**
- `-f, --file`: Path to YAML file (required)
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
  labels:
    team: infra
    environment: staging
  annotations:
    description: "Production-like environment for integration testing"
spec:
  network: stable
  networkType: mainnet
  networkVersion: v1.2.0
  validators: 7
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

### 4. Make Changes

Edit the YAML file and check differences:

```bash
# Edit devnet.yaml (e.g., change validators to 6)
devnet-builder diff -f devnet.yaml
```

### 5. Apply Updates

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
  - spec.validators must be between 1 and 100
```

## Best Practices

1. **Version Control**: Store YAML files in git alongside your application code
2. **Use Labels**: Add labels for filtering and organization
3. **Dry Run First**: Always preview changes with `--dry-run` before applying
4. **Environment Separation**: Use separate files or directories for different environments
5. **Resource Limits**: Specify resource limits for production-like environments
