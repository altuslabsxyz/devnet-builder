#!/usr/bin/env bash
set -euo pipefail

# Devnet management script
# This script helps manage devnet nodes running in screen sessions

# Default values
DEVNET_BASE_DIR="/data/.devnet"
STABLED_BINARY=""
VALIDATORS=4

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print colored message
print_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# Show usage
show_usage() {
  cat <<EOF
Usage: $0 <command> [options]

Commands:
  start       Start all devnet nodes
  stop        Stop all devnet nodes
  restart     Restart all devnet nodes
  status      Show status of all devnet nodes
  logs        Show logs for a specific node
  attach      Attach to a node's screen session
  list        List all screen sessions

Options:
  --devnet-dir <path>      Devnet base directory (default: /data/.devnet)
  --stabled-binary <path>  Path to stabled binary (required for start/restart)
  --validators <number>    Number of validators (default: 4)
  --node <number>          Node number (for logs/attach commands)

Examples:
  $0 start --devnet-dir /data/.devnet --stabled-binary ./build/stabled --validators 4
  $0 stop --validators 4
  $0 status --validators 4
  $0 logs --node 0
  $0 attach --node 0

EOF
}

# Parse command line arguments
COMMAND=""
NODE_NUM=""

if [ $# -eq 0 ]; then
  show_usage
  exit 1
fi

COMMAND=$1
shift

while [[ $# -gt 0 ]]; do
  case $1 in
    --devnet-dir)
      DEVNET_BASE_DIR="$2"
      shift 2
      ;;
    --stabled-binary)
      STABLED_BINARY="$2"
      shift 2
      ;;
    --validators)
      VALIDATORS="$2"
      shift 2
      ;;
    --node)
      NODE_NUM="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      show_usage
      exit 1
      ;;
  esac
done

# Start all nodes
start_nodes() {
  if [ -z "$STABLED_BINARY" ]; then
    print_error "stabled binary path is required for start command"
    echo "Use: $0 start --stabled-binary <path>"
    exit 1
  fi

  if [ ! -f "$STABLED_BINARY" ]; then
    print_error "stabled binary not found at: $STABLED_BINARY"
    exit 1
  fi

  if [ ! -d "$DEVNET_BASE_DIR" ]; then
    print_error "Devnet directory not found: $DEVNET_BASE_DIR"
    exit 1
  fi

  # Get chain-id from genesis
  CHAIN_ID=$(jq -r '.chain_id' "$DEVNET_BASE_DIR/node0/config/genesis.json" 2>/dev/null || echo "")
  if [ -z "$CHAIN_ID" ]; then
    print_error "Could not determine chain-id from genesis"
    exit 1
  fi

  print_info "Starting devnet nodes..."
  print_info "Devnet Directory: $DEVNET_BASE_DIR"
  print_info "Chain ID: $CHAIN_ID"
  print_info "Validators: $VALIDATORS"

  # Start each validator node
  for i in $(seq 0 $((VALIDATORS - 1))); do
    NODE_NAME="node$i"
    NODE_HOME="$DEVNET_BASE_DIR/$NODE_NAME"
    LOG_FILE="$DEVNET_BASE_DIR/$NODE_NAME.log"

    if [ ! -d "$NODE_HOME" ]; then
      print_warn "Node directory not found: $NODE_HOME (skipping)"
      continue
    fi

    # Check if already running
    if screen -list | grep -q "\.$NODE_NAME\s"; then
      print_warn "$NODE_NAME is already running"
      continue
    fi

    print_info "Starting $NODE_NAME..."

    # Create new screen session with logging
    screen -dmS "$NODE_NAME" -L -Logfile "$LOG_FILE" \
      bash -c "$STABLED_BINARY start --home $NODE_HOME --chain-id $CHAIN_ID; exec bash"

    sleep 1

    # Verify screen session is running
    if screen -list | grep -q "\.$NODE_NAME\s"; then
      print_info "$NODE_NAME started successfully"
    else
      print_error "Failed to start screen session for $NODE_NAME"
    fi
  done

  print_info ""
  print_info "All nodes started. Use '$0 status' to check status"
}

# Stop all nodes
stop_nodes() {
  print_info "Stopping devnet nodes..."

  STOPPED_COUNT=0
  for i in $(seq 0 $((VALIDATORS - 1))); do
    NODE_NAME="node$i"
    if screen -list | grep -q "\.$NODE_NAME\s"; then
      print_info "Stopping $NODE_NAME..."
      screen -S "$NODE_NAME" -X quit || true
      STOPPED_COUNT=$((STOPPED_COUNT + 1))
    fi
  done

  if [ $STOPPED_COUNT -eq 0 ]; then
    print_warn "No running nodes found"
  else
    print_info "Stopped $STOPPED_COUNT node(s)"
  fi
}

# Check status of all nodes
check_status() {
  print_info "Checking devnet node status..."
  echo ""

  RUNNING_COUNT=0
  for i in $(seq 0 $((VALIDATORS - 1))); do
    NODE_NAME="node$i"
    NODE_HOME="$DEVNET_BASE_DIR/$NODE_NAME"
    LOG_FILE="$DEVNET_BASE_DIR/$NODE_NAME.log"

    if screen -list | grep -q "\.$NODE_NAME\s"; then
      echo -e "  ${GREEN}●${NC} $NODE_NAME: ${GREEN}RUNNING${NC}"
      RUNNING_COUNT=$((RUNNING_COUNT + 1))

      # Check log file
      if [ -f "$LOG_FILE" ]; then
        LOG_SIZE=$(stat -f%z "$LOG_FILE" 2>/dev/null || stat -c%s "$LOG_FILE" 2>/dev/null)
        echo "    Log: $LOG_FILE ($LOG_SIZE bytes)"
      fi
    else
      echo -e "  ${RED}○${NC} $NODE_NAME: ${RED}STOPPED${NC}"
    fi
  done

  echo ""
  print_info "Status: $RUNNING_COUNT/$VALIDATORS nodes running"
}

# Show logs for a specific node
show_logs() {
  if [ -z "$NODE_NUM" ]; then
    print_error "Node number is required for logs command"
    echo "Use: $0 logs --node <number>"
    exit 1
  fi

  NODE_NAME="node$NODE_NUM"
  LOG_FILE="$DEVNET_BASE_DIR/$NODE_NAME.log"

  if [ ! -f "$LOG_FILE" ]; then
    print_error "Log file not found: $LOG_FILE"
    exit 1
  fi

  print_info "Showing logs for $NODE_NAME..."
  print_info "Log file: $LOG_FILE"
  echo ""
  tail -f "$LOG_FILE"
}

# Attach to a node's screen session
attach_node() {
  if [ -z "$NODE_NUM" ]; then
    print_error "Node number is required for attach command"
    echo "Use: $0 attach --node <number>"
    exit 1
  fi

  NODE_NAME="node$NODE_NUM"

  if ! screen -list | grep -q "\.$NODE_NAME\s"; then
    print_error "$NODE_NAME is not running"
    exit 1
  fi

  print_info "Attaching to $NODE_NAME screen session..."
  print_info "Press Ctrl+A then D to detach"
  sleep 2
  screen -r "$NODE_NAME"
}

# List all screen sessions
list_sessions() {
  print_info "Screen sessions:"
  echo ""
  screen -list || echo "  No screen sessions found"
}

# Execute command
case $COMMAND in
  start)
    start_nodes
    ;;
  stop)
    stop_nodes
    ;;
  restart)
    stop_nodes
    sleep 2
    start_nodes
    ;;
  status)
    check_status
    ;;
  logs)
    show_logs
    ;;
  attach)
    attach_node
    ;;
  list)
    list_sessions
    ;;
  help|--help|-h)
    show_usage
    ;;
  *)
    print_error "Unknown command: $COMMAND"
    show_usage
    exit 1
    ;;
esac
