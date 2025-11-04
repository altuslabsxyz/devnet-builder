# Stable Devnet Builder


Devnet Builder is a comprehensive tool to build and manage local development networks from exported genesis files. It supports both local usage and automated deployment via GitHub Actions.

## Table of Contents

- [Building](#building)
- [Quick Start](#quick-start)
- [Manual Usage](#manual-usage)
  - [Option 1: Export from Running Chain](#option-1-export-from-running-chain)
  - [Option 2: Provision and Sync Target Chain](#option-2-provision-and-sync-target-chain)
  - [Build Devnet](#build-devnet)
- [GitHub Actions CI/CD](#github-actions-cicd)
- [Managing Devnet](#managing-devnet)
- [Parameters Reference](#parameters-reference)
- [Output Structure](#output-structure)

## Building

Use the Makefile to build the devnet-builder binary:

```bash
make build
```

The binary will be created at `./build/devnet-builder`.

Available Makefile targets:
- `make build` - Build devnet-builder binary (default)
- `make clean` - Remove build artifacts
- `make install` - Install devnet-builder to GOPATH/bin
- `make test` - Run tests
- `make help` - Display help message

## Quick Start

The fastest way to get started is to use the automated GitHub Actions workflow. However, for local development, follow the manual usage guide below.

## Manual Usage

There are two ways to export genesis for building a devnet:

### Option 1: Export from Running Chain

If you already have a running chain, you can export its genesis directly:

```bash
stabled export --home /path/to/node > genesis-export.json
```

### Option 2: Provision with Snapshot (Recommended)

To provision a fresh chain using a snapshot download (faster and more reliable than state-sync):

```bash
./scripts/provision-with-snapshot.sh \
  --chain-id stabletestnet_2200-1 \
  --snapshot-url https://example.com/snapshots/stabletestnet-latest.tar.lz4 \
  --rpc-endpoint https://stable-rpc.testnet.chain0.dev/ \
  --stabled-binary ./stable/build/stabled \
  --base-dir /data \
  --output-file genesis-export.json
```

**Parameters:**
- `--chain-id`: Target chain-id to sync (required)
- `--snapshot-url`: URL to download snapshot from (supports .tar, .tar.gz, .tar.lz4)
- `--rpc-endpoint`: RPC endpoint to download genesis (optional)
- `--stabled-binary`: Path to stabled binary (required)
- `--base-dir`: Base directory for chain data (default: /data)
- `--output-file`: Output genesis file path
- `--persistent-peers`: Persistent peers for P2P (optional)
- `--skip-download`: Skip snapshot download (use existing data)

The script will:
1. Initialize a new chain directory at `$BASE_DIR/.$CHAIN_ID/`
2. Download genesis from RPC endpoint (if provided)
3. **Disable state-sync** in configuration
4. Download and extract snapshot to data directory
5. Start node and sync remaining blocks to latest
6. Export genesis with timestamp

**Supported snapshot formats:**
- `.tar` - Uncompressed tarball
- `.tar.gz` - Gzip compressed tarball
- `.tar.lz4` - LZ4 compressed tarball (fast decompression)
- `.tar.zst` - Zstandard compressed tarball (best compression ratio)

### Option 3: Provision and Sync with State-Sync

To provision a fresh chain and sync it via state-sync (legacy method):

```bash
./scripts/provision-and-sync.sh \
  --chain-id stabletestnet_2200-1 \
  --rpc-endpoint https://stable-rpc.testnet.chain0.dev/ \
  --stabled-binary ./stable/build/stabled \
  --base-dir /data \
  --output-file genesis-export.json
```

**Parameters:**
- `--chain-id`: Target chain-id to sync
- `--rpc-endpoint`: RPC endpoint for state-sync
- `--stabled-binary`: Path to stabled binary
- `--base-dir`: Base directory for chain data (default: /data)
- `--output-file`: Output genesis file path
- `--skip-sync`: Skip state-sync (only initialize)

The script will:
1. Initialize a new chain directory at `$BASE_DIR/.$CHAIN_ID/`
2. Download genesis from RPC endpoint
3. Configure state-sync
4. Sync the chain to latest height
5. Export genesis with timestamp

### Build Devnet

Once you have the exported genesis, build the devnet:

```bash
./build/devnet-builder build genesis-export.json \
  --validators 4 \
  --accounts 10 \
  --account-balance "1000000000000000000000astable,500000000000000000000agasusdt" \
  --validator-balance "1000000000000000000000astable,500000000000000000000agasusdt" \
  --validator-stake "100000000000000000000" \
  --chain-id stabledevnet_2200-1 \
  --output ./devnet
```

### Start Devnet Nodes

After building the devnet, start all validator nodes using screen:

```bash
./scripts/manage-devnet.sh start \
  --devnet-dir ./devnet \
  --stabled-binary ./stable/build/stabled \
  --validators 4
```

## GitHub Actions CI/CD

The repository includes a comprehensive GitHub Actions workflow for automated devnet deployment on self-hosted runners.

### Workflow Parameters

The workflow automatically provisions a fresh chain using snapshot-based sync and exports genesis.

#### Parameters

- `stable_tag`: Stable repository tag version (choice: `v1.1.0`, `v1.0.0`, `v0.8.1-testnet`, `v0.8.0-testnet`)
- `fork_target`: Fork target network (choice: `testnet`, `mainnet`)
  - **testnet**:
    - Chain ID: `stabletestnet_2201-1`
    - RPC: `https://cosmos-rpc.testnet.stable.xyz/`
    - Peers: `128accd3e8ee379bfdf54560c21345451c7048c7@peer1.testnet.stable.xyz:26656,5ed0f977a26ccf290e184e364fb04e268ef16430@peer2.testnet.stable.xyz:26656`
    - Snapshot: `https://stable-snapshot.s3.eu-central-1.amazonaws.com/snapshot.tar.lz4`
  - **mainnet**:
    - Chain ID: `stable_988-1`
    - RPC: `https://cosmos-rpc-internal.stable.xyz/`
    - Peers: `39fef24240d80e2cd5bdcbe101298c36f0d83fa1@57.129.53.87:26656`
    - Snapshot: `https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst`
- `validators`: Number of validators to create (default: 4)
- `accounts`: Number of dummy accounts to create (default: 10)
- `account_balance`: Balance for each account (optional, uses devnet-builder defaults if not provided)
- `validator_balance`: Balance for each validator (optional, uses devnet-builder defaults if not provided)

**Note:**
- If balance parameters are not provided, devnet-builder will use sensible defaults (5000 consensus power worth of astable for balances, 100 consensus power for stake)
- **All configuration is automatically determined by `fork_target` parameter**: chain-id, RPC endpoint, persistent peers, and snapshot URL
- **Uses snapshot-based sync by default for faster provisioning** (state-sync disabled)

### Workflow Steps

The workflow performs the following steps:

1. **Checkout Repository**: Fetches stable-devnet repository
2. **Get Tag Version**: Determines stable repository version
3. **Setup Stable Repository**: Clones/checkouts stable repository at correct version
4. **Build stabled Binary**: Compiles stabled from source
5. **Install Decompression Tools**: Installs lz4 and zstd for snapshot extraction
6. **Provision and Sync Target Chain**: Provisions fresh chain using either:
   - **Snapshot-based sync** (default): Downloads and extracts snapshot, syncs remaining blocks, state-sync disabled
   - **State-sync** (legacy): Syncs via state-sync from RPC endpoint
7. **Build Devnet**: Creates devnet using devnet-builder
8. **Upload Artifact**: Uploads devnet as GitHub artifact
9. **Deploy to System**: Deploys devnet to target directory
10. **Stop Existing Sessions**: Stops any existing systemd services
11. **Start Nodes**: Starts all validator nodes as systemd services
12. **Verify Deployment**: Checks node status
13. **Display Summary**: Shows deployment information

### Running the Workflow

1. Go to the Actions tab in your GitHub repository
2. Select "Deploy Devnet" workflow
3. Click "Run workflow"
4. Fill in the parameters:
   - `stable_tag`: Version to use (e.g., `v1.1.0`, `v0.8.1-testnet`)
   - `fork_target`: Select network to fork (`testnet` or `mainnet`)
     - **testnet**: Forks from stabletestnet_2201-1
     - **mainnet**: Forks from stable_988-1
   - Leave other parameters as default or customize as needed
5. Click "Run workflow"

The workflow will:
- Automatically determine chain-id, RPC endpoint, peers, and snapshot based on fork target
- Provision and sync the target chain using snapshot
- Export its genesis
- Build a local devnet
- Deploy and start all validator nodes as systemd services

## Managing Devnet

Use the `manage-devnet.sh` script to manage running devnet nodes:

### Start All Nodes

```bash
./scripts/manage-devnet.sh start \
  --devnet-dir /data/.devnet \
  --stabled-binary ./stable/build/stabled \
  --validators 4
```

### Stop All Nodes

```bash
./scripts/manage-devnet.sh stop --validators 4
```

### Check Status

```bash
./scripts/manage-devnet.sh status \
  --devnet-dir /data/.devnet \
  --validators 4
```

### View Logs

```bash
./scripts/manage-devnet.sh logs \
  --devnet-dir /data/.devnet \
  --node 0
```

### Attach to Screen Session

```bash
./scripts/manage-devnet.sh attach --node 0
```

Press `Ctrl+A` then `D` to detach from the screen session.

### List Screen Sessions

```bash
./scripts/manage-devnet.sh list
```

### Restart All Nodes

```bash
./scripts/manage-devnet.sh restart \
  --devnet-dir /data/.devnet \
  --stabled-binary ./stable/build/stabled \
  --validators 4
```

## Parameters Reference

### devnet-builder Parameters

- `--validators <number>`: Number of validators to create (default: 4)
- `--accounts <number>`: Number of dummy accounts to create (default: 10)
- `--account-balance <coins>`: Initial balance for each account (supports multiple denoms)
- `--validator-balance <coins>`: Initial balance for each validator (supports multiple denoms)
- `--validator-stake <amount>`: Staking amount for validators (base denom only)
- `--chain-id <string>`: Chain ID for devnet (optional, defaults to from genesis)
- `--output <path>`: Output directory (default: ./devnet)

**Example coin format**: `"1000000000000000000000astable,500000000000000000000agasusdt"`

### provision-with-snapshot.sh Parameters (Recommended)

- `--chain-id <string>`: Target chain-id (required)
- `--snapshot-url <url>`: URL to download snapshot from (supports .tar, .tar.gz, .tar.lz4, .tar.zst)
- `--rpc-endpoint <url>`: RPC endpoint to download genesis (optional)
- `--stabled-binary <path>`: Path to stabled binary (required)
- `--base-dir <path>`: Base directory for chain data (default: /data)
- `--output-file <path>`: Output genesis file path
- `--persistent-peers <string>`: Persistent peers for P2P (optional)
- `--skip-download`: Skip snapshot download (use existing data)

### provision-and-sync.sh Parameters (Legacy)

- `--chain-id <string>`: Target chain-id to sync (required)
- `--rpc-endpoint <url>`: RPC endpoint for state-sync
- `--stabled-binary <path>`: Path to stabled binary (required)
- `--base-dir <path>`: Base directory for chain data (default: /data)
- `--output-file <path>`: Output genesis file path
- `--skip-sync`: Skip state-sync (only initialize)

### manage-devnet.sh Parameters

- `--devnet-dir <path>`: Devnet base directory (default: /data/.devnet)
- `--stabled-binary <path>`: Path to stabled binary (required for start/restart)
- `--validators <number>`: Number of validators (default: 4)
- `--node <number>`: Node number (for logs/attach commands)

## Output Structure

After building a devnet, the output directory will have the following structure:

```
devnet/
├── node0/
│   ├── config/
│   │   ├── genesis.json              # Genesis file for this node
│   │   ├── config.toml               # Tendermint config
│   │   ├── app.toml                  # Application config
│   │   └── priv_validator_key.json   # Validator private key
│   ├── data/
│   │   └── priv_validator_state.json # Validator state
│   └── keyring-test/                 # Validator keyring
├── node1/
├── node2/
├── node3/
└── accounts/
    └── keyring-test/                 # All account keys
```

Each node directory is a complete, independent validator node with its own:
- Configuration files
- Validator keys
- Keyring with validator account

The `accounts/` directory contains keyrings for all generated dummy accounts.

## Logs and Monitoring

### Log Files

When nodes are started via the management script or GitHub Actions, logs are written to:

```
$DEVNET_BASE_DIR/node0.log
$DEVNET_BASE_DIR/node1.log
$DEVNET_BASE_DIR/node2.log
$DEVNET_BASE_DIR/node3.log
```

### View Logs in Real-time

```bash
tail -f /data/.devnet/node0.log
```

### Check Node Status via RPC

Each node exposes RPC on different ports (configured in config.toml):

```bash
# Check node0 status
curl http://localhost:26657/status

# Check sync status
curl http://localhost:26657/status | jq .result.sync_info.catching_up
```

## Troubleshooting

### Node won't start

1. Check if screen session exists: `screen -list`
2. View logs: `tail -100 /data/.devnet/node0.log`
3. Check if ports are available
4. Verify stabled binary exists and is executable

### Snapshot download fails

1. Check snapshot URL is accessible: `curl -I $SNAPSHOT_URL`
2. Verify sufficient disk space for download and extraction
3. Check network connectivity
4. Ensure required decompression tools are installed:
   - For .tar.lz4: `sudo apt-get install lz4`
   - For .tar.zst: `sudo apt-get install zstd`

### State-sync fails (legacy method)

1. Check RPC endpoint is reachable: `curl $RPC_ENDPOINT/status`
2. Verify trust height and hash are correct
3. Check network connectivity
4. Increase timeout in provision-and-sync.sh
5. Consider using snapshot-based sync instead (faster and more reliable)

### Genesis export fails

1. Ensure node is fully synced
2. Check disk space
3. Verify stabled binary version matches chain version

## Contributing

Contributions are welcome! Please ensure:

1. All scripts are executable (`chmod +x`)
2. Scripts follow bash best practices (use `set -euo pipefail`)
3. Error handling is comprehensive
4. Documentation is updated

## License

This project is part of the Stable ecosystem. Refer to the main repository for license information.
