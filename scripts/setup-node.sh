#!/usr/bin/env bash
set -euo pipefail

# Ensure pv is installed for progress bars
if ! command -v pv >/dev/null; then
  echo "pv not found. Installing..."
  if command -v apt-get >/dev/null; then
    sudo apt-get update
    sudo apt-get install -y pv
  else
    echo "Warning: apt-get not found, please install pv manually."
  fi
fi

### Configuration ###
SNAP_RPC="https://stable-rpc.testnet.chain0.dev/"
SERVICE="stabletestnet_2200-1.service"
WORK_DIR="/data/.stabletestnet_2200-1/"
DATA_DIR="$WORK_DIR/data"
CONFIG="$WORK_DIR/config/config.toml"

### 0. Pre-sync setup: stop service & reset data ###
echo "[0/6] Stopping $SERVICE and resetting data directory..."
sudo systemctl stop "$SERVICE"
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"
cat > "$DATA_DIR/priv_validator_state.json" <<EOF
{
  "height": "0",
  "round": 0,
  "step": 0
}
EOF

echo "    Data directory reset complete."

### 1. Calculate trust height & hash ###
echo "[1/6] Fetching latest height & trust hash from $SNAP_RPC..."
LATEST_HEIGHT=$(curl -s "$SNAP_RPC/block" | jq -r .result.block.header.height)
BLOCK_HEIGHT=$((LATEST_HEIGHT - 100))
TRUST_HASH=$(curl -s "$SNAP_RPC/block?height=$BLOCK_HEIGHT" | jq -r .result.block_id.hash)

echo "    Trust height: $BLOCK_HEIGHT, hash: $TRUST_HASH"

### 2. Patch config.toml ###
echo "[2/6] Patching config.toml for state sync..."
cp "$CONFIG" "${CONFIG}.bak"
sed -E -i \
  -e "s|^(enable[[:space:]]*=).*|\1 true|" \
  -e "s|^(rpc_servers[[:space:]]*=).*|\1 \"$SNAP_RPC,$SNAP_RPC\"|" \
  -e "s|^(trust_height[[:space:]]*=).*|\1 $BLOCK_HEIGHT|" \
  -e "s|^(trust_hash[[:space:]]*=).*|\1 \"$TRUST_HASH\"|" \
  -e "s|^(seeds[[:space:]]*=).*|\1 \"\"|" \
  "$CONFIG"

echo "    Config patched."

### 3. Start state-sync service ###
echo "[3/6] Starting $SERVICE for state sync..."
sudo systemctl start "$SERVICE"

echo "    Waiting for state-sync to complete..."
until [ "$(curl -s http://127.0.0.1:36657/status | jq -r .result.sync_info.catching_up)" = "false" ]; do
  sleep 1
  echo "    still catching up..."
done

echo "    State sync completed."

### 4. Stop service & prepare snapshot ###
echo "[4/6] Stopping $SERVICE and creating snapshot files..."
sudo systemctl stop "$SERVICE"
cd "$WORK_DIR"

timestamp=$(date +%Y-%m-%dT%H-%M-%S)
filename="$(chain_id)-${timestamp}.json"

stabled export > $filename

