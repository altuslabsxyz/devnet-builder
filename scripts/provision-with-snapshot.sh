#!/usr/bin/env bash
set -euo pipefail

# Provision chain with snapshot download and sync
# This script provisions a target chain using a snapshot download, syncs to latest block, and exports genesis

# Usage:
#   ./scripts/provision-with-snapshot.sh \
#     --chain-id stabletestnet_2200-1 \
#     --snapshot-url https://example.com/snapshots/stabletestnet-latest.tar.lz4 \
#     --stabled-binary ./build/stabled \
#     --base-dir /data \
#     --output-file genesis-export.json

# Default values
CHAIN_ID=""
SNAPSHOT_URL=""
STABLED_BINARY=""
BASE_DIR="/data"
OUTPUT_FILE="genesis-export.json"
PERSISTENT_PEERS=""
SKIP_DOWNLOAD=false
RPC_ENDPOINT=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --chain-id)
      CHAIN_ID="$2"
      shift 2
      ;;
    --snapshot-url)
      SNAPSHOT_URL="$2"
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
    --persistent-peers)
      PERSISTENT_PEERS="$2"
      shift 2
      ;;
    --skip-download)
      SKIP_DOWNLOAD=true
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
echo "Provisioning Chain with Snapshot"
echo "=========================================="
echo "Chain ID: $CHAIN_ID"
echo "Snapshot URL: $SNAPSHOT_URL"
echo "RPC Endpoint: $RPC_ENDPOINT"
echo "Stabled Binary: $STABLED_BINARY"
echo "Work Directory: $WORK_DIR"
echo "Output File: $OUTPUT_FILE"
echo "Skip Download: $SKIP_DOWNLOAD"
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
  echo "[1/7] Initializing chain..."
  "$STABLED_BINARY" init "target-node" --chain-id "$CHAIN_ID" --home "$WORK_DIR" --overwrite
  echo "    Chain initialized"
else
  echo "[1/7] Chain directory already exists: $WORK_DIR"
fi

# Step 1.5: Configure ports and settings
echo "[1.5/7] Configuring ports and app settings..."

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

  # Use awk to update ports and peers section by section
  awk -v rpc_port="$RPC_PORT" -v p2p_port="$P2P_PORT" -v proxy_port="$PROXY_APP_PORT" -v peers="$PERSISTENT_PEERS" '
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
      in_statesync = 0
    }
    else if ($0 ~ /^\[p2p\]/) {
      in_rpc = 0
      in_p2p = 1
      in_statesync = 0
    }
    else if ($0 ~ /^\[statesync\]/) {
      in_rpc = 0
      in_p2p = 0
      in_statesync = 1
    }
    else if ($0 ~ /^\[.*\]/) {
      in_rpc = 0
      in_p2p = 0
      in_statesync = 0
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

    # Update persistent_peers in P2P section
    if (in_p2p && $0 ~ /^persistent_peers = /) {
      if (peers != "") {
        print "persistent_peers = \"" peers "\""
      } else {
        print
      }
      next
    }

    # DISABLE state-sync
    if (in_statesync && $0 ~ /^enable = /) {
      print "enable = false"
      next
    }

    # Print all other lines as-is
    print
  }
  ' "$CONFIG_FILE" > "${CONFIG_FILE}.tmp"

  # Replace original with updated file
  mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"

  if [ -n "$PERSISTENT_PEERS" ]; then
    echo "    Updated config.toml ports (RPC=$RPC_PORT, P2P=$P2P_PORT, ProxyApp=$PROXY_APP_PORT), disabled state-sync, and configured peers"
  else
    echo "    Updated config.toml ports (RPC=$RPC_PORT, P2P=$P2P_PORT, ProxyApp=$PROXY_APP_PORT) and disabled state-sync"
  fi
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
echo "[2/7] Checking for existing processes..."
if pgrep -f "stabled.*--home.*$WORK_DIR" > /dev/null; then
  echo "    Stopping existing stabled process..."
  pkill -f "stabled.*--home.*$WORK_DIR" || true
  sleep 3
fi
echo "    No conflicting processes found"

# Step 3: Prepare for snapshot (data will be replaced by snapshot)
echo "[3/7] Preparing for snapshot download..."
echo "    Data directory will be replaced by snapshot"

# Step 4: Download genesis if RPC endpoint provided
if [ -n "$RPC_ENDPOINT" ]; then
  echo "[4/7] Downloading genesis from RPC endpoint..."

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
  echo "[4/7] Skipping genesis download (no RPC endpoint provided)"
fi

# Step 5: Download and extract snapshot
if [ "$SKIP_DOWNLOAD" = false ] && [ -n "$SNAPSHOT_URL" ]; then
  echo "[5/7] Downloading and extracting snapshot..."

  # Remove existing data directory before extracting snapshot
  echo "    Removing existing data directory..."
  rm -rf "$DATA_DIR"

  SNAPSHOT_FILE="$BASE_DIR/snapshot.tar.lz4"

  # Detect compression format from URL and check for required tools
  if [[ "$SNAPSHOT_URL" == *.tar.lz4 ]]; then
    SNAPSHOT_FILE="$BASE_DIR/snapshot.tar.lz4"

    # Check if lz4 is installed
    if ! command -v lz4 &> /dev/null; then
      echo "    Error: lz4 is not installed"
      echo "    Please install it: sudo apt-get install lz4"
      exit 1
    fi

    EXTRACT_CMD="lz4 -dc \"$SNAPSHOT_FILE\" | tar xf - -C \"$WORK_DIR\""
  elif [[ "$SNAPSHOT_URL" == *.tar.zst ]]; then
    SNAPSHOT_FILE="$BASE_DIR/snapshot.tar.zst"

    # Check if zstd is installed
    if ! command -v zstd &> /dev/null; then
      echo "    Error: zstd is not installed"
      echo "    Please install it: sudo apt-get install zstd"
      exit 1
    fi

    EXTRACT_CMD="zstd -dc \"$SNAPSHOT_FILE\" | tar xf - -C \"$WORK_DIR\""
  elif [[ "$SNAPSHOT_URL" == *.tar.gz ]]; then
    SNAPSHOT_FILE="$BASE_DIR/snapshot.tar.gz"
    EXTRACT_CMD="tar xzf \"$SNAPSHOT_FILE\" -C \"$WORK_DIR\""
  elif [[ "$SNAPSHOT_URL" == *.tar ]]; then
    SNAPSHOT_FILE="$BASE_DIR/snapshot.tar"
    EXTRACT_CMD="tar xf \"$SNAPSHOT_FILE\" -C \"$WORK_DIR\""
  else
    echo "    Error: Unsupported snapshot format. Supported: .tar, .tar.gz, .tar.lz4, .tar.zst"
    exit 1
  fi

  # Download snapshot with progress
  echo "    Downloading snapshot from: $SNAPSHOT_URL"
  if curl -L --progress-bar -o "$SNAPSHOT_FILE" "$SNAPSHOT_URL"; then
    echo "    Snapshot downloaded successfully"

    # Get snapshot size
    SNAPSHOT_SIZE=$(stat -f%z "$SNAPSHOT_FILE" 2>/dev/null || stat -c%s "$SNAPSHOT_FILE" 2>/dev/null)
    echo "    Snapshot size: $SNAPSHOT_SIZE bytes"

    # Extract snapshot
    echo "    Extracting snapshot..."
    eval "$EXTRACT_CMD"
    echo "    Snapshot extracted successfully"

    # Cleanup snapshot file
    rm -f "$SNAPSHOT_FILE"
    echo "    Cleaned up snapshot file"
  else
    echo "    Error: Failed to download snapshot"
    exit 1
  fi

  # Create priv_validator_state.json if it doesn't exist
  if [ ! -f "$DATA_DIR/priv_validator_state.json" ]; then
    cat > "$DATA_DIR/priv_validator_state.json" <<EOF
{
  "height": "0",
  "round": 0,
  "step": 0
}
EOF
    echo "    Created priv_validator_state.json"
  fi
else
  echo "[5/7] Skipping snapshot download (--skip-download or no snapshot URL)"
fi

# Step 6: Start node and sync to latest
echo "[6/8] Starting node to sync to latest block..."

# Start stabled to sync remaining blocks
echo "    Starting stabled for sync..."
"$STABLED_BINARY" start --home "$WORK_DIR" --chain-id $CHAIN_ID > "$WORK_DIR/sync.log" 2>&1 &
STABLED_PID=$!

echo "    Waiting for sync to complete (PID: $STABLED_PID)..."
echo "    Log file: $WORK_DIR/sync.log"

# Wait for sync to complete
MAX_WAIT=1800  # 30 minutes timeout
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

  # Check sync status using dynamically assigned RPC port
  if SYNC_STATUS=$(curl -s http://127.0.0.1:$RPC_PORT/status 2>/dev/null | jq -r .result.sync_info.catching_up 2>/dev/null); then
    if [ "$SYNC_STATUS" = "false" ]; then
      echo "    Sync completed successfully"
      break
    else
      # Get current height
      CURRENT_HEIGHT=$(curl -s http://127.0.0.1:$RPC_PORT/status 2>/dev/null | jq -r .result.sync_info.latest_block_height 2>/dev/null)
      echo "    Still syncing... (height: ${CURRENT_HEIGHT:-unknown}, ${WAIT_COUNT}s elapsed)"
    fi
  else
    echo "    Waiting for RPC to be available on port $RPC_PORT... (${WAIT_COUNT}s elapsed)"
  fi
done

if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
  echo "    Error: Sync timed out after ${MAX_WAIT}s"
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

echo "    Sync complete"

# Step 7: Verify data directory
echo "[7/8] Verifying data directory..."
if [ -d "$DATA_DIR/application.db" ] || [ -d "$DATA_DIR/blockstore.db" ] || [ -f "$DATA_DIR/priv_validator_state.json" ]; then
  echo "    Data directory contains valid data"
else
  echo "    Warning: Data directory may not contain complete snapshot data"
fi

# Step 8: Export genesis
echo "[8/8] Exporting genesis..."

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
echo "State-sync: DISABLED"
echo "Sync Method: SNAPSHOT"
echo "=========================================="
