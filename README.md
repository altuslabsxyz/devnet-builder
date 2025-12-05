# Stable Devnet

Local development network for Stable blockchain using mainnet state export.

## Prerequisites

- Go 1.21+ (for devnet-builder)
- `stabled` binary (local build or from release)
- curl, jq (for provisioning)
- zstd (for mainnet snapshot decompression)

## Quick Start

Use the local devnet script to spin up a 4-node devnet with a single command:

```bash
# Start devnet with local stabled binary
./scripts/local-devnet.sh start --local-binary /path/to/stabled
```

The script will automatically:
1. Download and cache the mainnet snapshot
2. Download genesis from mainnet RPC
3. Export state from snapshot (no syncing required)
4. Build devnet with 4 validators and 10 test accounts
5. Start 4 validator nodes locally

### Verify Devnet is Running

```bash
# Check status
./scripts/local-devnet.sh status

# Check node health via RPC
curl http://localhost:26657/status | jq '.result.sync_info.latest_block_height'

# Check all nodes
curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height'
curl -s http://localhost:36657/status | jq -r '.result.sync_info.latest_block_height'
curl -s http://localhost:46657/status | jq -r '.result.sync_info.latest_block_height'
curl -s http://localhost:56657/status | jq -r '.result.sync_info.latest_block_height'

# Check EVM JSON-RPC
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### Lifecycle Commands

```bash
# Stop devnet
./scripts/local-devnet.sh stop

# Restart devnet
./scripts/local-devnet.sh restart

# Remove all devnet data
./scripts/local-devnet.sh clean

# Remove devnet + cached snapshots
./scripts/local-devnet.sh clean --cache
```

### Export Keys

```bash
# Export all keys (text format)
./scripts/local-devnet.sh export-keys

# Export in JSON format
./scripts/local-devnet.sh export-keys --format json

# Export in environment variable format
./scripts/local-devnet.sh export-keys --format env
```

### View Logs

```bash
# View all logs
./scripts/local-devnet.sh logs

# Follow logs for a specific node
./scripts/local-devnet.sh logs node0 -f

# View last 50 lines
./scripts/local-devnet.sh logs --tail 50
```

### Customization

```bash
# Custom number of validators (1-4)
./scripts/local-devnet.sh start --local-binary /path/to/stabled --validators 2

# Custom number of test accounts
./scripts/local-devnet.sh start --local-binary /path/to/stabled --accounts 20

# Force rebuild even if devnet exists
./scripts/local-devnet.sh start --local-binary /path/to/stabled --rebuild

# Skip snapshot cache (re-download)
./scripts/local-devnet.sh start --local-binary /path/to/stabled --no-cache
```

For full help:
```bash
./scripts/local-devnet.sh help
./scripts/local-devnet.sh help start
```

## Network Configuration

### RPC Endpoints

- Mainnet Cosmos RPC: `https://p40zma3acd216e70s-cosmos-rpc.stable.xyz`
- Testnet Cosmos RPC: `https://cosmos-rpc.testnet.stable.xyz`

### Snapshot Source

- Mainnet: `https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst`

## Endpoints

| Service | Node 0 | Node 1 | Node 2 | Node 3 |
|---------|--------|--------|--------|--------|
| RPC | 26657 | 36657 | 46657 | 56657 |
| P2P | 26656 | 36656 | 46656 | 56656 |
| EVM JSON-RPC | 8545 | 18545 | 28545 | 38545 |
| EVM WebSocket | 8546 | 18546 | 28546 | 38546 |
| gRPC | 9090 | 19090 | 29090 | 39090 |

## How It Works

1. **Snapshot Download**: Downloads the latest mainnet pruned snapshot (~13GB)
2. **Genesis Download**: Fetches genesis from mainnet RPC endpoint
3. **State Export**: Uses `stabled export` to export current state from snapshot
4. **Devnet Build**: `devnet-builder` creates:
   - 4 new validators with fresh keys
   - 10 test accounts with tokens
   - Redistributes tokens from largest holder
   - Clears old staking state and creates new validators
   - Preserves total supply (no inflation/deflation)
5. **Node Start**: Starts 4 validator nodes with unique ports

## Directory Structure

```
stable-devnet/
├── cmd/devnet-builder/        # Go devnet builder tool
├── scripts/                   # Local devnet management scripts
│   ├── local-devnet.sh        # Main CLI script
│   ├── provision-and-sync.sh  # Snapshot/genesis helpers
│   └── manage-devnet.sh       # Lifecycle management helpers
├── devnet/                    # Generated devnet data (gitignored)
│   ├── accounts/              # Test account keyrings
│   ├── node0/                 # Validator 0 home directory
│   ├── node1/                 # Validator 1 home directory
│   ├── node2/                 # Validator 2 home directory
│   └── node3/                 # Validator 3 home directory
└── ~/.stable-devnet/          # Cached data (user home)
    ├── snapshots/             # Downloaded snapshots
    │   └── mainnet/
    └── genesis/               # Downloaded genesis files
        └── mainnet-genesis.json
```

## Building devnet-builder

```bash
# Build the devnet-builder tool
make build

# The binary will be at ./build/devnet-builder
```

## Troubleshooting

### Nodes not starting
Check the node logs:
```bash
cat devnet/node0/node.log
```

### Port conflicts
Make sure ports 26656-26657, 36656-36657, 46656-46657, 56656-56657, 8545, 18545, 28545, 38545, 9090, 19090, 29090, 39090 are available.

### Kill all stabled processes
```bash
pkill -f stabled
```

### Clean and restart
```bash
./scripts/local-devnet.sh clean
./scripts/local-devnet.sh start --local-binary /path/to/stabled
```

## License

Refer to the main Stable repository for license information.
