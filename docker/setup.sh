#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEVNET_DIR="${SCRIPT_DIR}/../devnet"
DOCKER_DEVNET_DIR="${SCRIPT_DIR}/devnet"

# Node IDs for persistent peers (derived from fixed node_key.json)
NODE0_ID="a18d66435236d91ba28e1bf7a82d400b9a188f5f"
NODE1_ID="2edec3e0270cba790f849b44f46b9120ed3f153f"
NODE2_ID="916431c30a36aff0b72a798ed86965903576d38c"
NODE3_ID="48496c38733af68c8ce1cfcb6e1ff476cfac260a"

echo "Setting up Docker devnet configuration..."

# Check if source devnet exists
if [ ! -d "${DEVNET_DIR}" ]; then
    echo "Error: Source devnet directory not found at ${DEVNET_DIR}"
    echo "Please run devnet-builder first to generate the devnet configuration."
    exit 1
fi

# Create Docker devnet directory
rm -rf "${DOCKER_DEVNET_DIR}"
mkdir -p "${DOCKER_DEVNET_DIR}"

# Copy devnet configuration
echo "Copying devnet configuration..."
cp -r "${DEVNET_DIR}/"* "${DOCKER_DEVNET_DIR}/"

# Update persistent_peers for Docker networking
echo "Updating persistent_peers for Docker networking..."

for i in 0 1 2 3; do
    CONFIG_FILE="${DOCKER_DEVNET_DIR}/node${i}/config/config.toml"

    if [ -f "${CONFIG_FILE}" ]; then
        echo "  Updating node${i}/config/config.toml..."

        # Replace 127.0.0.1 with Docker container hostnames
        sed -i.bak \
            -e "s/${NODE0_ID}@127.0.0.1:6656/${NODE0_ID}@node0:6656/g" \
            -e "s/${NODE1_ID}@127.0.0.1:16656/${NODE1_ID}@node1:6656/g" \
            -e "s/${NODE2_ID}@127.0.0.1:26656/${NODE2_ID}@node2:6656/g" \
            -e "s/${NODE3_ID}@127.0.0.1:36656/${NODE3_ID}@node3:6656/g" \
            "${CONFIG_FILE}"

        # Also update any localhost references
        sed -i.bak \
            -e 's/laddr = "tcp:\/\/127.0.0.1:/laddr = "tcp:\/\/0.0.0.0:/g' \
            -e 's/laddr = "tcp:\/\/localhost:/laddr = "tcp:\/\/0.0.0.0:/g' \
            "${CONFIG_FILE}"

        rm -f "${CONFIG_FILE}.bak"
    fi
done

# Update app.toml to bind to all interfaces
echo "Updating app.toml for Docker networking..."

for i in 0 1 2 3; do
    APP_FILE="${DOCKER_DEVNET_DIR}/node${i}/config/app.toml"

    if [ -f "${APP_FILE}" ]; then
        echo "  Updating node${i}/config/app.toml..."

        # Update addresses to bind to all interfaces
        sed -i.bak \
            -e 's/address = "tcp:\/\/localhost:/address = "tcp:\/\/0.0.0.0:/g' \
            -e 's/address = "127.0.0.1:/address = "0.0.0.0:/g' \
            -e 's/ws-address = "127.0.0.1:/ws-address = "0.0.0.0:/g' \
            -e 's/metrics-address = "127.0.0.1:/metrics-address = "0.0.0.0:/g' \
            "${APP_FILE}"

        rm -f "${APP_FILE}.bak"
    fi
done

# Initialize data directories with priv_validator_state.json
echo "Initializing data directories..."

for i in 0 1 2 3; do
    DATA_DIR="${DOCKER_DEVNET_DIR}/node${i}/data"
    mkdir -p "${DATA_DIR}"

    if [ ! -f "${DATA_DIR}/priv_validator_state.json" ]; then
        echo '{"height":"0","round":0,"step":0}' > "${DATA_DIR}/priv_validator_state.json"
    fi
done

echo ""
echo "Docker devnet setup complete!"
echo ""
echo "To start the devnet:"
echo "  cd ${SCRIPT_DIR}"
echo "  cp .env.example .env  # (optional: customize settings)"
echo "  docker compose up -d"
echo ""
echo "To check status:"
echo "  docker compose ps"
echo "  curl http://localhost:26657/status"
echo ""
echo "To view logs:"
echo "  docker compose logs -f"
echo ""
echo "To stop the devnet:"
echo "  docker compose down"
