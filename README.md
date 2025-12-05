# Stable Devnet

Docker-based local development network for Stable blockchain.

## Prerequisites

- Docker
- Docker Compose v2+
- Go 1.21+ (for devnet-builder)
- curl, jq (for provisioning)
- lz4 (for testnet snapshots) or zstd (for mainnet snapshots)

## Quick Start (Recommended)

Use the local devnet script to spin up a 4-node devnet with a single command:

```bash
# Start devnet from mainnet snapshot (default)
./scripts/local-devnet.sh start

# Start from testnet snapshot
./scripts/local-devnet.sh start --network testnet

# Start with a local stabled binary (for testing code changes)
./scripts/local-devnet.sh start --local-binary /path/to/stabled
```

The script will automatically:
1. Download and cache the network snapshot
2. Export state from snapshot (no syncing required)
3. Build devnet with 4 validators
4. Start nodes via Docker

### Verify Devnet is Running

```bash
# Check status
./scripts/local-devnet.sh status

# Check node health via RPC
curl http://localhost:26657/status

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

# Reset chain state (keep genesis and config)
./scripts/local-devnet.sh reset

# Full reset (regenerate genesis)
./scripts/local-devnet.sh reset --hard

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
# Custom chain ID
./scripts/local-devnet.sh start --chain-id mydevnet_1234-1

# Custom number of validators (1-4)
./scripts/local-devnet.sh start --validators 2

# Custom number of test accounts
./scripts/local-devnet.sh start --accounts 20

# Force rebuild even if devnet exists
./scripts/local-devnet.sh start --rebuild

# Skip snapshot cache
./scripts/local-devnet.sh start --no-cache
```

For full help:
```bash
./scripts/local-devnet.sh help
./scripts/local-devnet.sh help start
```

## Manual Setup (Alternative)

### 1. Build Devnet from Exported Genesis

```bash
# Build devnet using Docker
./docker/scripts/build-devnet.sh genesis-export.json --chain-id stabledevnet_2201-1
```

### 2. Start Devnet

```bash
cd docker
cp .env.example .env  # (optional) customize settings
docker compose up -d
```

### 3. Check Status

```bash
# Check containers
docker compose ps

# Check node status
curl http://localhost:26657/status

# Check EVM JSON-RPC
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

## Provisioning from Network

To create a genesis export from an existing network:

```bash
./docker/scripts/provision.sh \
  --chain-id stabletestnet_2201-1 \
  --snapshot-url https://example.com/snapshot.tar.lz4 \
  --rpc-endpoint https://rpc.testnet.stable.xyz/
```

## Configuration

### Environment Variables

Copy `.env.example` to `.env` and customize:

| Variable | Default | Description |
|----------|---------|-------------|
| `STABLED_IMAGE` | `ghcr.io/stablelabs/stable` | Docker image |
| `STABLED_TAG` | `latest-testnet` | Image tag |
| `NODE0_RPC_PORT` | `26657` | Node 0 RPC port |
| `NODE0_EVM_PORT` | `8545` | Node 0 EVM JSON-RPC port |

### Fixed Node Keys

Node keys are pre-generated in `config/devnet-keys/` for consistent node IDs:

| Node | ID |
|------|------|
| node0 | `a18d66435236d91ba28e1bf7a82d400b9a188f5f` |
| node1 | `2edec3e0270cba790f849b44f46b9120ed3f153f` |
| node2 | `916431c30a36aff0b72a798ed86965903576d38c` |
| node3 | `48496c38733af68c8ce1cfcb6e1ff476cfac260a` |

## Endpoints

| Service | Node 0 | Node 1 | Node 2 | Node 3 |
|---------|--------|--------|--------|--------|
| RPC | 26657 | 36657 | 46657 | 56657 |
| P2P | 26656 | 36656 | 46656 | 56656 |
| EVM JSON-RPC | 8545 | 18545 | 28545 | 38545 |
| gRPC | 9090 | 19090 | 29090 | 39090 |

## Commands

```bash
# Start devnet
docker compose up -d

# Stop devnet
docker compose down

# View logs
docker compose logs -f
docker compose logs -f node0

# Reset devnet (clear data)
docker compose down -v
rm -rf docker/devnet/*/data/*

# Rebuild devnet
./docker/scripts/build-devnet.sh genesis-export.json
```

## Directory Structure

```
stable-devnet/
├── cmd/devnet-builder/     # Go devnet builder
├── scripts/                # Local devnet management scripts
│   ├── local-devnet.sh     # Main CLI script
│   ├── provision-and-sync.sh  # Snapshot/genesis helpers
│   └── manage-devnet.sh    # Lifecycle management helpers
├── config/
│   ├── node_key.json       # Node key for provisioning
│   └── devnet-keys/        # Fixed validator node keys
│       ├── node0/
│       ├── node1/
│       ├── node2/
│       └── node3/
├── docker/
│   ├── docker-compose.yml  # Docker Compose configuration
│   ├── docker-compose.local.yml  # Local binary mode override
│   ├── .env.example        # Environment variables template
│   ├── Dockerfile.devnet-builder  # Builder image (optional)
│   ├── devnet/             # Generated devnet data (gitignored)
│   └── scripts/
│       ├── provision.sh    # Provision from network
│       └── build-devnet.sh # Build devnet from genesis
├── devnet/                 # Generated devnet data (gitignored)
└── ~/.stable-devnet/       # Cached snapshots and genesis (user home)
```

## License

Refer to the main Stable repository for license information.
