#!/bin/bash
set -euo pipefail

# Build devnet using Docker containers
# All operations are performed inside Docker containers

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(dirname "$DOCKER_DIR")"

# Default values
GENESIS_FILE=""
STABLED_IMAGE="${STABLED_IMAGE:-ghcr.io/stablelabs/stable}"
STABLED_TAG="${STABLED_TAG:-latest-testnet}"
OUTPUT_DIR="${PROJECT_ROOT}/docker/devnet"
CHAIN_ID="stabledevnet_2201-1"
NUM_VALIDATORS=4
NUM_ACCOUNTS=4
NODE_KEYS_DIR="${PROJECT_ROOT}/config/devnet-keys"

# Node IDs (derived from fixed node_key.json files)
NODE0_ID="a18d66435236d91ba28e1bf7a82d400b9a188f5f"
NODE1_ID="2edec3e0270cba790f849b44f46b9120ed3f153f"
NODE2_ID="916431c30a36aff0b72a798ed86965903576d38c"
NODE3_ID="48496c38733af68c8ce1cfcb6e1ff476cfac260a"

usage() {
    cat <<EOF
Usage: $0 [OPTIONS] <genesis-export.json>

Build a devnet from an exported genesis file using Docker.

Options:
    --image         Docker image (default: ghcr.io/stablelabs/stable)
    --tag           Docker image tag (default: latest-testnet)
    --output        Output directory (default: ./docker/devnet)
    --chain-id      Chain ID for devnet (default: stabledevnet_2201-1)
    -h, --help      Show this help message

Example:
    $0 genesis-export.json --chain-id stabledevnet_2201-1
EOF
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --image) STABLED_IMAGE="$2"; shift 2 ;;
        --tag) STABLED_TAG="$2"; shift 2 ;;
        --output) OUTPUT_DIR="$2"; shift 2 ;;
        --chain-id) CHAIN_ID="$2"; shift 2 ;;
        -h|--help) usage ;;
        -*) echo "Unknown option: $1"; exit 1 ;;
        *) GENESIS_FILE="$1"; shift ;;
    esac
done

# Validate
if [ -z "$GENESIS_FILE" ]; then
    echo "Error: Genesis file is required"
    usage
fi

if [ ! -f "$GENESIS_FILE" ]; then
    echo "Error: Genesis file not found: $GENESIS_FILE"
    exit 1
fi

IMAGE="${STABLED_IMAGE}:${STABLED_TAG}"
GENESIS_FILE_ABS="$(cd "$(dirname "$GENESIS_FILE")" && pwd)/$(basename "$GENESIS_FILE")"

echo "=========================================="
echo "Building Devnet with Docker"
echo "=========================================="
echo "Image: $IMAGE"
echo "Genesis: $GENESIS_FILE_ABS"
echo "Output: $OUTPUT_DIR"
echo "Chain ID: $CHAIN_ID"
echo "Validators: $NUM_VALIDATORS"
echo ""

# Clean and create output directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Create accounts directory
mkdir -p "$OUTPUT_DIR/accounts/keyring-test"

# Function to run stabled in Docker (run as root with explicit home)
run_stabled() {
    local node_home="$1"
    shift
    docker run --rm -u 0 \
        -v "${node_home}:/data" \
        "$IMAGE" \
        "$@" --home /data
}

# Step 1: Initialize nodes
echo "[1/5] Initializing validator nodes..."
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    NODE_DIR="$OUTPUT_DIR/node${i}"
    mkdir -p "$NODE_DIR"

    echo "  Initializing node${i}..."
    docker run --rm -u 0 \
        -v "${NODE_DIR}:/data" \
        "$IMAGE" \
        init "validator${i}" --chain-id "$CHAIN_ID" --home /data 2>/dev/null

    # Copy fixed node_key.json
    if [ -f "${NODE_KEYS_DIR}/node${i}/node_key.json" ]; then
        cp "${NODE_KEYS_DIR}/node${i}/node_key.json" "${NODE_DIR}/config/node_key.json"
    fi
done

# Step 2: Generate validator keys
echo "[2/5] Generating validator keys..."
for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    NODE_DIR="$OUTPUT_DIR/node${i}"

    echo "  Creating validator${i} key..."
    docker run --rm -u 0 \
        -v "${NODE_DIR}:/data" \
        "$IMAGE" \
        keys add "validator${i}" --keyring-backend test --algo eth_secp256k1 --home /data 2>/dev/null || true

    # Copy validator key to accounts directory
    cp "${NODE_DIR}/keyring-test/"* "$OUTPUT_DIR/accounts/keyring-test/" 2>/dev/null || true
done

# Step 3: Generate account keys
echo "[3/5] Generating account keys..."
ACCOUNTS_DIR="$OUTPUT_DIR/accounts"
for i in $(seq 0 $((NUM_ACCOUNTS - 1))); do
    echo "  Creating account${i} key..."
    docker run --rm -u 0 \
        -v "${ACCOUNTS_DIR}:/data" \
        "$IMAGE" \
        keys add "account${i}" --keyring-backend test --algo eth_secp256k1 --home /data 2>/dev/null || true
done

# Step 4: Update genesis and copy to all nodes
echo "[4/5] Updating genesis..."

# Copy base genesis to first node
cp "$GENESIS_FILE_ABS" "$OUTPUT_DIR/node0/config/genesis.json"

# Update chain ID in genesis
jq --arg chainId "$CHAIN_ID" '.chain_id = $chainId' \
    "$OUTPUT_DIR/node0/config/genesis.json" > "$OUTPUT_DIR/node0/config/genesis.json.tmp"
mv "$OUTPUT_DIR/node0/config/genesis.json.tmp" "$OUTPUT_DIR/node0/config/genesis.json"

# Copy genesis to all nodes
for i in $(seq 1 $((NUM_VALIDATORS - 1))); do
    cp "$OUTPUT_DIR/node0/config/genesis.json" "$OUTPUT_DIR/node${i}/config/genesis.json"
done

# Step 5: Configure networking
echo "[5/5] Configuring network..."

# Build persistent_peers string for Docker networking
PERSISTENT_PEERS="${NODE0_ID}@node0:6656,${NODE1_ID}@node1:6656,${NODE2_ID}@node2:6656,${NODE3_ID}@node3:6656"

# Port configurations for each node
declare -A NODE_PORTS
NODE_PORTS[0]="6656 6657 8545 9090 6660"
NODE_PORTS[1]="6656 6657 8545 9090 6660"
NODE_PORTS[2]="6656 6657 8545 9090 6660"
NODE_PORTS[3]="6656 6657 8545 9090 6660"

for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
    CONFIG_FILE="$OUTPUT_DIR/node${i}/config/config.toml"
    APP_FILE="$OUTPUT_DIR/node${i}/config/app.toml"

    echo "  Configuring node${i}..."

    # Update config.toml
    if [ -f "$CONFIG_FILE" ]; then
        # Set persistent peers (use ^ to avoid matching experimental_max_gossip_connections_to_persistent_peers)
        sed -i.bak "s|^persistent_peers = \".*\"|persistent_peers = \"${PERSISTENT_PEERS}\"|g" "$CONFIG_FILE"

        # Bind to all interfaces
        sed -i.bak 's|laddr = "tcp://127.0.0.1:|laddr = "tcp://0.0.0.0:|g' "$CONFIG_FILE"

        # Allow duplicate IPs (for Docker)
        sed -i.bak 's|allow_duplicate_ip = false|allow_duplicate_ip = true|g' "$CONFIG_FILE"

        # Disable strict address book
        sed -i.bak 's|addr_book_strict = true|addr_book_strict = false|g' "$CONFIG_FILE"

        rm -f "${CONFIG_FILE}.bak"
    fi

    # Update app.toml
    if [ -f "$APP_FILE" ]; then
        # Bind to all interfaces
        sed -i.bak 's|address = "tcp://localhost:|address = "tcp://0.0.0.0:|g' "$APP_FILE"
        sed -i.bak 's|address = "127.0.0.1:|address = "0.0.0.0:|g' "$APP_FILE"
        sed -i.bak 's|ws-address = "127.0.0.1:|ws-address = "0.0.0.0:|g' "$APP_FILE"
        sed -i.bak 's|metrics-address = "127.0.0.1:|metrics-address = "0.0.0.0:|g' "$APP_FILE"

        # Enable JSON-RPC
        sed -i.bak 's|enable = false|enable = true|g' "$APP_FILE"

        rm -f "${APP_FILE}.bak"
    fi

    # Initialize priv_validator_state.json
    mkdir -p "$OUTPUT_DIR/node${i}/data"
    echo '{"height":"0","round":0,"step":0}' > "$OUTPUT_DIR/node${i}/data/priv_validator_state.json"
done

echo ""
echo "=========================================="
echo "Devnet Build Complete!"
echo "=========================================="
echo ""
echo "Output directory: $OUTPUT_DIR"
echo ""
echo "To start the devnet:"
echo "  cd docker && docker compose up -d"
echo ""
echo "To check status:"
echo "  docker compose ps"
echo "  curl http://localhost:26657/status"
