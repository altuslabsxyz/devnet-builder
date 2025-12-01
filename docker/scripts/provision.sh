#!/bin/bash
set -euo pipefail

# Provision chain with snapshot download and sync using Docker
# All operations are performed inside Docker containers

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$DOCKER_DIR")"

# Default values
CHAIN_ID=""
SNAPSHOT_URL=""
RPC_ENDPOINT=""
STABLED_IMAGE="${STABLED_IMAGE:-ghcr.io/stablelabs/stable}"
STABLED_TAG="${STABLED_TAG:-latest-testnet}"
DATA_DIR="${DATA_DIR:-${PROJECT_ROOT}/data}"
OUTPUT_FILE="genesis-export.json"
PERSISTENT_PEERS=""
SKIP_DOWNLOAD=false
NODE_KEY_FILE="${PROJECT_ROOT}/config/node_key.json"

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Provision a chain using Docker containers with snapshot sync.

Options:
    --chain-id          Chain ID to sync (required)
    --snapshot-url      URL to download snapshot from
    --rpc-endpoint      RPC endpoint to download genesis
    --image             Docker image (default: ghcr.io/stablelabs/stable)
    --tag               Docker image tag (default: latest-testnet)
    --data-dir          Data directory (default: ./data)
    --output-file       Output genesis file (default: genesis-export.json)
    --persistent-peers  Persistent peers for P2P
    --skip-download     Skip snapshot download
    -h, --help          Show this help message

Example:
    $0 --chain-id stabletestnet_2201-1 \\
       --snapshot-url https://example.com/snapshot.tar.lz4 \\
       --rpc-endpoint https://rpc.testnet.stable.xyz/
EOF
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --chain-id) CHAIN_ID="$2"; shift 2 ;;
        --snapshot-url) SNAPSHOT_URL="$2"; shift 2 ;;
        --rpc-endpoint) RPC_ENDPOINT="$2"; shift 2 ;;
        --image) STABLED_IMAGE="$2"; shift 2 ;;
        --tag) STABLED_TAG="$2"; shift 2 ;;
        --data-dir) DATA_DIR="$2"; shift 2 ;;
        --output-file) OUTPUT_FILE="$2"; shift 2 ;;
        --persistent-peers) PERSISTENT_PEERS="$2"; shift 2 ;;
        --skip-download) SKIP_DOWNLOAD=true; shift ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# Validate required parameters
if [ -z "$CHAIN_ID" ]; then
    echo "Error: --chain-id is required"
    exit 1
fi

# Setup directories
WORK_DIR="${DATA_DIR}/.${CHAIN_ID}"
mkdir -p "${WORK_DIR}"

IMAGE="${STABLED_IMAGE}:${STABLED_TAG}"

echo "=========================================="
echo "Provisioning Chain with Docker"
echo "=========================================="
echo "Chain ID: $CHAIN_ID"
echo "Image: $IMAGE"
echo "Data Directory: $DATA_DIR"
echo "Work Directory: $WORK_DIR"
echo ""

# Function to run stabled command in Docker
run_stabled() {
    docker run --rm \
        -v "${WORK_DIR}:/root/.stabled" \
        -v "${NODE_KEY_FILE}:/root/.stabled/config/node_key.json:ro" \
        --network host \
        "$IMAGE" \
        "$@"
}

# Step 1: Initialize chain
echo "[1/6] Initializing chain..."
if [ ! -f "${WORK_DIR}/config/genesis.json" ]; then
    docker run --rm \
        -v "${WORK_DIR}:/root/.stabled" \
        "$IMAGE" \
        init "devnet-provisioner" --chain-id "$CHAIN_ID"

    # Copy fixed node_key.json
    if [ -f "$NODE_KEY_FILE" ]; then
        cp "$NODE_KEY_FILE" "${WORK_DIR}/config/node_key.json"
        echo "  Copied fixed node_key.json"
    fi
else
    echo "  Chain already initialized, skipping..."
fi

# Step 2: Download genesis from RPC
if [ -n "$RPC_ENDPOINT" ]; then
    echo "[2/6] Downloading genesis from RPC..."
    curl -s "${RPC_ENDPOINT}/genesis" | jq '.result.genesis' > "${WORK_DIR}/config/genesis.json"
    echo "  Genesis downloaded"
else
    echo "[2/6] Skipping genesis download (no RPC endpoint)"
fi

# Step 3: Download and extract snapshot
if [ -n "$SNAPSHOT_URL" ] && [ "$SKIP_DOWNLOAD" = false ]; then
    echo "[3/6] Downloading snapshot..."
    SNAPSHOT_FILE="${DATA_DIR}/snapshot.tar.lz4"

    # Download snapshot
    curl -L -o "$SNAPSHOT_FILE" "$SNAPSHOT_URL"

    echo "  Extracting snapshot..."
    # Clear existing data
    rm -rf "${WORK_DIR}/data"
    mkdir -p "${WORK_DIR}/data"

    # Extract based on file extension
    case "$SNAPSHOT_URL" in
        *.tar.lz4)
            lz4 -dc "$SNAPSHOT_FILE" | tar -xf - -C "${WORK_DIR}/data"
            ;;
        *.tar.zst)
            zstd -dc "$SNAPSHOT_FILE" | tar -xf - -C "${WORK_DIR}/data"
            ;;
        *.tar.gz)
            tar -xzf "$SNAPSHOT_FILE" -C "${WORK_DIR}/data"
            ;;
        *.tar)
            tar -xf "$SNAPSHOT_FILE" -C "${WORK_DIR}/data"
            ;;
        *)
            echo "Unknown snapshot format"
            exit 1
            ;;
    esac

    rm -f "$SNAPSHOT_FILE"
    echo "  Snapshot extracted"
else
    echo "[3/6] Skipping snapshot download"
fi

# Step 4: Configure node
echo "[4/6] Configuring node..."

# Update config.toml
CONFIG_FILE="${WORK_DIR}/config/config.toml"
if [ -f "$CONFIG_FILE" ]; then
    # Disable state-sync (using snapshot instead)
    sed -i.bak 's/enable = true/enable = false/g' "$CONFIG_FILE"

    # Set persistent peers if provided
    if [ -n "$PERSISTENT_PEERS" ]; then
        sed -i.bak "s/persistent_peers = \"\"/persistent_peers = \"${PERSISTENT_PEERS}\"/g" "$CONFIG_FILE"
    fi

    rm -f "${CONFIG_FILE}.bak"
fi

# Step 5: Sync to latest block (optional, run in background)
echo "[5/6] Starting node to sync..."
echo "  Note: This will sync the chain to the latest block"
echo "  Press Ctrl+C to stop once synced"

# Run the node
docker run --rm -it \
    -v "${WORK_DIR}:/root/.stabled" \
    --network host \
    --name "stabled-provision-${CHAIN_ID}" \
    "$IMAGE" \
    start --home /root/.stabled || true

# Step 6: Export genesis
echo "[6/6] Exporting genesis..."
docker run --rm \
    -v "${WORK_DIR}:/root/.stabled" \
    -v "${PROJECT_ROOT}:/output" \
    "$IMAGE" \
    export --home /root/.stabled > "${PROJECT_ROOT}/${OUTPUT_FILE}"

echo ""
echo "=========================================="
echo "Provisioning Complete!"
echo "=========================================="
echo "Exported genesis: ${PROJECT_ROOT}/${OUTPUT_FILE}"
