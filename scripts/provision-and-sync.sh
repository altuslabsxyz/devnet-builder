#!/usr/bin/env bash
set -euo pipefail

# Provision and sync target chain script
# This script provisions a target chain, syncs it via state-sync, and exports the genesis

# Usage:
#   ./scripts/provision-and-sync.sh \
#     --chain-id stabletestnet_2200-1 \
#     --rpc-endpoint https://stable-rpc.testnet.chain0.dev/ \
#     --stabled-binary ./build/stabled \
#     --base-dir /data \
#     --output-file genesis-export.json

# Default values
CHAIN_ID=""
RPC_ENDPOINT=""
STABLED_BINARY=""
BASE_DIR="/data"
OUTPUT_FILE="genesis-export.json"
SKIP_SYNC=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --chain-id)
      CHAIN_ID="$2"
      shift 2
      ;;
    --rpc-endpoint)
      RPC_ENDPOINT="$2"
      shift 2
      ;;
    --stabled-binary)
      STABLED_BINARY="$2"
      shift 2
      ;;
    --base-dir)
      BASE_DIR="$2"
      shift 2
      ;;
    --output-file)
      OUTPUT_FILE="$2"
      shift 2
      ;;
    --skip-sync)
      SKIP_SYNC=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Validate required parameters
if [ -z "$CHAIN_ID" ]; then
  echo "Error: --chain-id is required"
  exit 1
fi

if [ -z "$STABLED_BINARY" ]; then
  echo "Error: --stabled-binary is required"
  exit 1
fi

if [ ! -f "$STABLED_BINARY" ]; then
  echo "Error: stabled binary not found at $STABLED_BINARY"
  exit 1
fi

# Make stabled executable
chmod +x "$STABLED_BINARY"

# Set up directories
WORK_DIR="$BASE_DIR/.$CHAIN_ID"
DATA_DIR="$WORK_DIR/data"
CONFIG_DIR="$WORK_DIR/config"
CONFIG_FILE="$CONFIG_DIR/config.toml"
APP_CONFIG_FILE="$CONFIG_DIR/app.toml"

echo "=========================================="
echo "Provisioning Target Chain"
echo "=========================================="
echo "Chain ID: $CHAIN_ID"
echo "RPC Endpoint: $RPC_ENDPOINT"
echo "Stabled Binary: $STABLED_BINARY"
echo "Work Directory: $WORK_DIR"
echo "Output File: $OUTPUT_FILE"
echo "Skip Sync: $SKIP_SYNC"
echo "=========================================="
echo ""

# Function to find available port
# Usage: find_available_port START_PORT [EXCLUDED_PORTS...]
find_available_port() {
  local start_port=$1
  shift
  local excluded_ports=("$@")
  local port=$start_port

  while [ $port -lt $((start_port + 1000)) ]; do
    # Check if port is in excluded list
    local is_excluded=false
    for excluded in "${excluded_ports[@]}"; do
      if [ "$port" = "$excluded" ]; then
        is_excluded=true
        break
      fi
    done

    # Check if port is available (not in use and not excluded)
    if [ "$is_excluded" = false ] && ! lsof -i:$port > /dev/null 2>&1; then
      echo $port
      return 0
    fi
    port=$((port + 1))
  done

  echo "Error: Could not find available port starting from $start_port" >&2
  exit 1
}

# Step 1: Initialize chain if not exists
if [ ! -d "$WORK_DIR" ]; then
  echo "[1/6] Initializing chain..."
  "$STABLED_BINARY" init "target-node" --chain-id "$CHAIN_ID" --home "$WORK_DIR" --overwrite
  echo "    Chain initialized"
else
  echo "[1/6] Chain directory already exists: $WORK_DIR"
fi

# Step 1.5: Configure ports and settings
echo "[1.5/6] Configuring ports and app settings..."

# Find available ports (ensure they don't overlap)
RPC_PORT=$(find_available_port 26657)
P2P_PORT=$(find_available_port 26656 $RPC_PORT)
PROXY_APP_PORT=$(find_available_port 26658 $RPC_PORT $P2P_PORT)

echo "    Using RPC port: $RPC_PORT"
echo "    Using P2P port: $P2P_PORT"
echo "    Using Proxy App port: $PROXY_APP_PORT"

# Update config.toml with available ports
if [ -f "$CONFIG_FILE" ]; then
  # Backup original config
  cp "$CONFIG_FILE" "${CONFIG_FILE}.bak"

  # Use awk to update ports section by section
  awk -v rpc_port="$RPC_PORT" -v p2p_port="$P2P_PORT" -v proxy_port="$PROXY_APP_PORT" '
  {
    # Update proxy_app at the beginning of file
    if ($0 ~ /^proxy_app = "tcp:\/\/127\.0\.0\.1:[0-9]+"/) {
      print "proxy_app = \"tcp://0.0.0.0:" proxy_port "\""
      next
    }

    # Track which section we are in
    if ($0 ~ /^\[rpc\]/) {
      in_rpc = 1
      in_p2p = 0
    }
    else if ($0 ~ /^\[p2p\]/) {
      in_rpc = 0
      in_p2p = 1
    }
    else if ($0 ~ /^\[.*\]/) {
      in_rpc = 0
      in_p2p = 0
    }

    # Update RPC laddr
    if (in_rpc && $0 ~ /^laddr = "tcp:\/\//) {
      print "laddr = \"tcp://0.0.0.0:" rpc_port "\""
      next
    }

    # Update P2P laddr
    if (in_p2p && $0 ~ /^laddr = "tcp:\/\//) {
      print "laddr = \"tcp://0.0.0.0:" p2p_port "\""
      next
    }

    # Print all other lines as-is
    print
  }
  ' "$CONFIG_FILE" > "${CONFIG_FILE}.tmp"

  # Replace original with updated file
  mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"

  echo "    Updated config.toml ports (RPC=$RPC_PORT, P2P=$P2P_PORT, ProxyApp=$PROXY_APP_PORT)"
fi

# Update app.toml - disable all "enable = true" settings
if [ -f "$APP_CONFIG_FILE" ]; then
  # Backup original app config
  cp "$APP_CONFIG_FILE" "${APP_CONFIG_FILE}.bak"

  # Disable all enable = true settings
  sed -i.tmp 's/enable = true/enable = false/g' "$APP_CONFIG_FILE"

  # Remove sed backup files
  rm -f "${APP_CONFIG_FILE}.tmp"

  echo "    Updated app.toml settings"
fi

echo "    Configuration complete"

# Step 2: Stop any existing process
echo "[2/6] Checking for existing processes..."
if pgrep -f "stabled.*--home.*$WORK_DIR" > /dev/null; then
  echo "    Stopping existing stabled process..."
  pkill -f "stabled.*--home.*$WORK_DIR" || true
  sleep 3
fi
echo "    No conflicting processes found"

# Step 3: Reset data directory
echo "[3/6] Resetting data directory..."
rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"
cat > "$DATA_DIR/priv_validator_state.json" <<EOF
{
  "height": "0",
  "round": 0,
  "step": 0
}
EOF
echo "    Data directory reset complete"

# Step 4: Download genesis if RPC endpoint provided
if [ -n "$RPC_ENDPOINT" ]; then
  echo "[4/6] Downloading genesis from RPC endpoint..."

  # Remove trailing slash
  RPC_ENDPOINT="${RPC_ENDPOINT%/}"

  # Download genesis
  GENESIS_URL="$RPC_ENDPOINT/genesis"
  echo "    Fetching genesis from: $GENESIS_URL"

  if curl -sf "$GENESIS_URL" | jq -r .result.genesis > "$CONFIG_DIR/genesis.json"; then
    echo "    Genesis downloaded successfully"
  else
    echo "    Warning: Failed to download genesis from RPC endpoint"
    echo "    Continuing with existing genesis..."
  fi
else
  echo "[4/6] Skipping genesis download (no RPC endpoint provided)"
fi

# Step 5: Configure state-sync if not skipped
if [ "$SKIP_SYNC" = false ] && [ -n "$RPC_ENDPOINT" ]; then
  echo "[5/6] Configuring state-sync..."

  # Remove trailing slash
  RPC_ENDPOINT="${RPC_ENDPOINT%/}"

  # Calculate trust height & hash
  echo "    Fetching latest height & trust hash from $RPC_ENDPOINT..."
  LATEST_HEIGHT=$(curl -s "$RPC_ENDPOINT/block" | jq -r .result.block.header.height)
  BLOCK_HEIGHT=$((LATEST_HEIGHT - 100))
  TRUST_HASH=$(curl -s "$RPC_ENDPOINT/block?height=$BLOCK_HEIGHT" | jq -r .result.block_id.hash)

  echo "    Trust height: $BLOCK_HEIGHT, hash: $TRUST_HASH"

  # Backup config.toml
  cp "$CONFIG_FILE" "${CONFIG_FILE}.bak"

  # Patch config.toml for state-sync
  sed -E -i.tmp \
    -e "s|^(enable[[:space:]]*=).*|\1 true|" \
    -e "s|^(rpc_servers[[:space:]]*=).*|\1 \"$RPC_ENDPOINT,$RPC_ENDPOINT\"|" \
    -e "s|^(trust_height[[:space:]]*=).*|\1 $BLOCK_HEIGHT|" \
    -e "s|^(trust_hash[[:space:]]*=).*|\1 \"$TRUST_HASH\"|" \
    -e "s|^(seeds[[:space:]]*=).*|\1 \"\"|" \
    "$CONFIG_FILE"

  rm -f "${CONFIG_FILE}.tmp"

  echo "    Config patched for state-sync"

  # Start stabled for state-sync
  echo "    Starting stabled for state-sync..."
  "$STABLED_BINARY" start --home "$WORK_DIR" > "$WORK_DIR/sync.log" 2>&1 &
  STABLED_PID=$!

  echo "    Waiting for state-sync to complete (PID: $STABLED_PID)..."
  echo "    Log file: $WORK_DIR/sync.log"

  # Wait for sync to complete
  MAX_WAIT=600  # 10 minutes timeout
  WAIT_COUNT=0
  while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
    sleep 5
    WAIT_COUNT=$((WAIT_COUNT + 5))

    # Check if process is still running
    if ! kill -0 $STABLED_PID 2>/dev/null; then
      echo "    Error: stabled process died unexpectedly"
      tail -20 "$WORK_DIR/sync.log"
      exit 1
    fi

    # Check sync status
    if SYNC_STATUS=$(curl -s http://127.0.0.1:26657/status 2>/dev/null | jq -r .result.sync_info.catching_up 2>/dev/null); then
      if [ "$SYNC_STATUS" = "false" ]; then
        echo "    State-sync completed successfully"
        break
      else
        echo "    Still syncing... (${WAIT_COUNT}s elapsed)"
      fi
    else
      echo "    Waiting for RPC to be available... (${WAIT_COUNT}s elapsed)"
    fi
  done

  if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
    echo "    Error: State-sync timed out after ${MAX_WAIT}s"
    kill $STABLED_PID 2>/dev/null || true
    exit 1
  fi

  # Stop stabled
  echo "    Stopping stabled..."
  kill $STABLED_PID 2>/dev/null || true
  sleep 3

  # Make sure it's stopped
  pkill -f "stabled.*--home.*$WORK_DIR" || true
  sleep 2

  echo "    State-sync complete"
else
  echo "[5/6] Skipping state-sync (--skip-sync or no RPC endpoint)"
fi

# Step 6: Export genesis
echo "[6/6] Exporting genesis..."

# Get absolute path for output file
if [[ "$OUTPUT_FILE" != /* ]]; then
  OUTPUT_FILE="$(pwd)/$OUTPUT_FILE"
fi

echo "    Exporting to: $OUTPUT_FILE"

# Add timestamp to filename
timestamp=$(date +%Y-%m-%dT%H-%M-%S)
TIMESTAMPED_OUTPUT="${OUTPUT_FILE%.json}-${timestamp}.json"

# Export genesis
"$STABLED_BINARY" export --home "$WORK_DIR" > "$TIMESTAMPED_OUTPUT"

# Verify export
if [ ! -f "$TIMESTAMPED_OUTPUT" ]; then
  echo "    Error: Genesis export failed"
  exit 1
fi

GENESIS_SIZE=$(stat -f%z "$TIMESTAMPED_OUTPUT" 2>/dev/null || stat -c%s "$TIMESTAMPED_OUTPUT" 2>/dev/null)
echo "    Genesis exported successfully (size: $GENESIS_SIZE bytes)"

# Validate JSON
if ! jq empty "$TIMESTAMPED_OUTPUT" 2>/dev/null; then
  echo "    Error: Exported genesis is not valid JSON"
  exit 1
fi

# Create symlink to latest
ln -sf "$TIMESTAMPED_OUTPUT" "$OUTPUT_FILE"

echo ""
echo "=========================================="
echo "Provisioning Complete"
echo "=========================================="
echo "Chain ID: $CHAIN_ID"
echo "Work Directory: $WORK_DIR"
echo "Exported Genesis: $TIMESTAMPED_OUTPUT"
echo "Latest Symlink: $OUTPUT_FILE"
echo "=========================================="
