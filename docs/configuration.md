# Configuration Guide

This guide covers all configuration options for devnet-builder.

## Table of Contents

- [Configuration Priority](#configuration-priority)
- [config.toml Reference](#configtoml-reference)
- [Environment Variables](#environment-variables)
- [Example Configurations](#example-configurations)
- [Multiple Devnets](#multiple-devnets)

---

## Configuration Priority

Configuration values are resolved in the following order (highest to lowest priority):

1. **CLI flags** - Direct command-line arguments
2. **Environment variables** - `DEVNET_*` prefixed variables
3. **Local config file** - `./config.toml` in current directory
4. **User config file** - `~/.devnet-builder/config.toml`
5. **Default values** - Built-in defaults

### Example

```bash
# Default: validators = 4
# In ~/.devnet-builder/config.toml: validators = 2
# In ./config.toml: validators = 3
# CLI flag wins:
devnet-builder deploy --validators 1  # Uses 1 validator
```

---

## config.toml Reference

Create a config file with `devnet-builder config init` or manually create `~/.devnet-builder/config.toml`.

### Complete Reference

```toml
# Base directory for all devnet data
# Default: ~/.devnet-builder
home = "~/.devnet-builder"

# Number of validator nodes
# Default: 4
# Range: 1-10
validators = 4

# Number of additional funded accounts
# Default: 0
accounts = 0

# Network source for snapshot data
# Options: "mainnet", "testnet"
# Default: "mainnet"
network = "mainnet"

# Execution mode
# Options: "docker", "local"
# Default: "docker"
mode = "docker"

# Network version to use
# Options: "latest", specific version tag
# Default: "latest"
network_version = "latest"

# Skip snapshot cache, always download fresh
# Default: false
no_cache = false

# Enable verbose logging
# Default: false
verbose = false

# Output in JSON format
# Default: false
json = false

# Disable colored output
# Default: false
no_color = false
```

### Option Details

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `home` | string | `~/.devnet-builder` | Base directory for devnet data, cache, and configs |
| `validators` | int | 4 | Number of validator nodes (1-10) |
| `accounts` | int | 0 | Additional funded test accounts |
| `network` | string | mainnet | Network source: mainnet or testnet |
| `mode` | string | docker | Execution mode: docker or local |
| `network_version` | string | latest | Version tag for network binary |
| `no_cache` | bool | false | Skip cached snapshots |
| `verbose` | bool | false | Enable debug logging |
| `json` | bool | false | JSON output for scripts |
| `no_color` | bool | false | Disable terminal colors |

---

## Environment Variables

All configuration options can be set via environment variables with the `DEVNET_` prefix.

### Variable Mapping

| Environment Variable | config.toml Key | Type |
|---------------------|-----------------|------|
| `DEVNET_HOME` | `home` | string |
| `DEVNET_VALIDATORS` | `validators` | int |
| `DEVNET_ACCOUNTS` | `accounts` | int |
| `DEVNET_NETWORK` | `network` | string |
| `DEVNET_MODE` | `mode` | string |
| `DEVNET_NETWORK_VERSION` | `network_version` | string |
| `DEVNET_NO_CACHE` | `no_cache` | bool |
| `DEVNET_VERBOSE` | `verbose` | bool |
| `DEVNET_JSON` | `json` | bool |
| `DEVNET_NO_COLOR` | `no_color` | bool |

### Examples

```bash
# Set default home directory
export DEVNET_HOME=/data/devnet

# Set default validator count
export DEVNET_VALIDATORS=2

# Enable verbose logging globally
export DEVNET_VERBOSE=true

# Use local mode by default
export DEVNET_MODE=local

# Disable colors in CI
export DEVNET_NO_COLOR=true

# Now deploy uses these defaults
devnet-builder deploy
```

### CI/CD Example

```bash
# .github/workflows/test.yml
env:
  DEVNET_HOME: /tmp/devnet
  DEVNET_VALIDATORS: 1
  DEVNET_NO_COLOR: true
  DEVNET_JSON: true
```

---

## Example Configurations

### Minimal Development Config

```toml
# ~/.devnet-builder/config.toml
# Fast startup for local development
validators = 1
mode = "docker"
```

### Full Testing Config

```toml
# ~/.devnet-builder/config.toml
# Full consensus testing setup
validators = 4
accounts = 5
network = "mainnet"
mode = "docker"
verbose = true
```

### CI Pipeline Config

```toml
# config.toml (in repository root)
# Configuration for CI testing
validators = 2
accounts = 3
mode = "docker"
no_color = true
json = true
```

### Local Binary Development

```toml
# ~/.devnet-builder/config.toml
# For developers building network locally
validators = 2
mode = "local"
verbose = true
```

### Multiple Network Testing

```toml
# mainnet-config.toml
validators = 4
network = "mainnet"
home = "~/.devnet-builder-mainnet"
```

```toml
# testnet-config.toml
validators = 4
network = "testnet"
home = "~/.devnet-builder-testnet"
```

```bash
# Use specific config
devnet-builder --config mainnet-config.toml deploy
devnet-builder --config testnet-config.toml deploy
```

---

## Multiple Devnets

Run multiple independent devnets by using different home directories.

### Using Different Home Directories

```bash
# Mainnet devnet
devnet-builder --home ~/.devnet-mainnet deploy --network mainnet

# Testnet devnet (in parallel)
devnet-builder --home ~/.devnet-testnet deploy --network testnet
```

### Port Considerations

When running multiple devnets, be aware of port conflicts. Each devnet uses these ports:

| Node | P2P | RPC | gRPC | EVM RPC | EVM WS |
|------|-----|-----|------|---------|--------|
| node0 | 26656 | 26657 | 9090 | 8545 | 8546 |
| node1 | 26666 | 26667 | 9091 | - | - |
| node2 | 26676 | 26677 | 9092 | - | - |
| node3 | 26686 | 26687 | 9093 | - | - |

To avoid conflicts, stop one devnet before starting another:

```bash
# Stop mainnet devnet
devnet-builder --home ~/.devnet-mainnet down

# Start testnet devnet
devnet-builder --home ~/.devnet-testnet up
```

### Using Shell Aliases

```bash
# ~/.bashrc
alias devnet-mainnet='devnet-builder --home ~/.devnet-mainnet'
alias devnet-testnet='devnet-builder --home ~/.devnet-testnet'

# Usage
devnet-mainnet deploy
devnet-mainnet status

devnet-testnet deploy
devnet-testnet status
```

---

## Data Directory Structure

Understanding the home directory structure helps with troubleshooting:

```
~/.devnet-builder/
├── bin/                    # Local binaries (symlinks)
├── build/                  # Build artifacts
├── cache/                  # Binary cache for upgrades
│   └── binaries/          # Cached network binaries by commit
├── config.toml            # User configuration
├── devnet/                # Active devnet data
│   ├── metadata.json      # Devnet state and config
│   ├── node0/             # First validator
│   │   ├── config/        # genesis.json, config.toml, app.toml
│   │   ├── data/          # Chain data
│   │   └── keyring-test/  # Validator key
│   ├── node1/             # Additional validators...
│   ├── node2/
│   ├── node3/
│   └── accounts/          # Funded test accounts
│       └── keyring-test/
├── genesis/               # Genesis exports
└── snapshots/             # Snapshot cache
    └── mainnet/           # Network-specific snapshots
```

### Key Files

| File | Description |
|------|-------------|
| `config.toml` | User configuration |
| `devnet/metadata.json` | Current devnet state |
| `devnet/node*/config/genesis.json` | Chain genesis |
| `devnet/node*/config/config.toml` | Tendermint config |
| `devnet/node*/config/app.toml` | Application config |
| `devnet/node*/keyring-test/` | Validator keys |

---

## See Also

- [Command Reference](commands.md) - All CLI commands and flags
- [Workflows](workflows.md) - Common debugging workflows
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
