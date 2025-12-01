#!/bin/sh
set -e

HOME_DIR="${STABLED_HOME:-/root/.stabled}"
CONFIG_DIR="${HOME_DIR}/config"
DATA_DIR="${HOME_DIR}/data"

# Node IDs for persistent peers (derived from fixed node_key.json)
NODE0_ID="a18d66435236d91ba28e1bf7a82d400b9a188f5f"
NODE1_ID="2edec3e0270cba790f849b44f46b9120ed3f153f"
NODE2_ID="916431c30a36aff0b72a798ed86965903576d38c"
NODE3_ID="48496c38733af68c8ce1cfcb6e1ff476cfac260a"

# Ensure data directory exists and has correct structure
mkdir -p "${DATA_DIR}"

# Initialize priv_validator_state.json if it doesn't exist
if [ ! -f "${DATA_DIR}/priv_validator_state.json" ]; then
    echo '{"height":"0","round":0,"step":0}' > "${DATA_DIR}/priv_validator_state.json"
fi

# Update persistent_peers in config.toml to use Docker hostnames
if [ -f "${CONFIG_DIR}/config.toml" ]; then
    # Create a temporary file for the updated config
    TEMP_CONFIG=$(mktemp)

    # Replace 127.0.0.1 with Docker container hostnames in persistent_peers
    sed -e "s/${NODE0_ID}@127.0.0.1:6656/${NODE0_ID}@node0:6656/g" \
        -e "s/${NODE1_ID}@127.0.0.1:16656/${NODE1_ID}@node1:6656/g" \
        -e "s/${NODE2_ID}@127.0.0.1:26656/${NODE2_ID}@node2:6656/g" \
        -e "s/${NODE3_ID}@127.0.0.1:36656/${NODE3_ID}@node3:6656/g" \
        "${CONFIG_DIR}/config.toml" > "${TEMP_CONFIG}"

    # Copy back to config directory (requires writable config)
    cp "${TEMP_CONFIG}" "${CONFIG_DIR}/config.toml" 2>/dev/null || {
        echo "Warning: Could not update config.toml (read-only mount). Using environment-based configuration."
    }
    rm -f "${TEMP_CONFIG}"
fi

echo "Starting stabled node..."
exec stabled "$@"
