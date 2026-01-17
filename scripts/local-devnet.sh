#!/usr/bin/env bash
#
# local-devnet.sh - Local development devnet management script
#
# This script allows developers to run a local 4-node devnet from a clone
# of the stable-devnet repository. It supports both Docker-based execution
# (default) and local binary execution for testing code changes.
#
# Usage: ./scripts/local-devnet.sh <command> [options]
#
# Commands:
#   start       Start the local devnet
#   stop        Stop the local devnet
#   restart     Restart the local devnet
#   reset       Reset chain state (keep genesis and config)
#   clean       Remove all devnet data
#   status      Show devnet status
#   logs        View node logs
#   export-keys Export account and validator keys
#   help        Show help information
#
# See './scripts/local-devnet.sh help' for more details.

set -euo pipefail

# =============================================================================
# Script Constants
# =============================================================================

readonly SCRIPT_VERSION="1.0.0"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default directories
readonly DEFAULT_DEVNET_DIR="$PROJECT_ROOT/devnet"
readonly DEFAULT_CACHE_DIR="$HOME/.stable-devnet"

# =============================================================================
# Color Constants (T007)
# =============================================================================

# Check if terminal supports colors
if [[ -t 1 ]] && [[ "${TERM:-}" != "dumb" ]]; then
    readonly GREEN='\033[0;32m'
    readonly YELLOW='\033[0;33m'
    readonly RED='\033[0;31m'
    readonly BLUE='\033[0;34m'
    readonly BOLD='\033[1m'
    readonly NC='\033[0m' # No Color
else
    readonly GREEN=''
    readonly YELLOW=''
    readonly RED=''
    readonly BLUE=''
    readonly BOLD=''
    readonly NC=''
fi

# =============================================================================
# Logging Functions (T006)
# =============================================================================

info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

success() {
    echo -e "${GREEN}[OK]${NC} $*"
}

stage() {
    local current=$1
    local total=$2
    local message=$3
    echo ""
    echo -e "${BOLD}[$current/$total] $message${NC}"
}

progress() {
    echo -e "  ${BLUE}[>>]${NC} $*"
}

# =============================================================================
# Network Configuration Constants (T010)
# =============================================================================

# Testnet configuration
readonly TESTNET_SNAPSHOT_URL="https://stable-snapshot.s3.eu-central-1.amazonaws.com/snapshot.tar.lz4"
readonly TESTNET_RPC_ENDPOINT="https://cosmos-rpc.testnet.stable.xyz"
readonly TESTNET_CHAIN_ID="stabletestnet_2201-1"
readonly TESTNET_DECOMPRESSOR="lz4"

# Mainnet configuration
readonly MAINNET_SNAPSHOT_URL="https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst"
readonly MAINNET_RPC_ENDPOINT="https://p40zma3acd216e70s-cosmos-rpc.stable.xyz"
readonly MAINNET_CHAIN_ID="stable_988-1"
readonly MAINNET_DECOMPRESSOR="zstd"

# Default Docker image
readonly DEFAULT_STABLED_IMAGE="ghcr.io/stablelabs/stable"
readonly DEFAULT_STABLED_TAG="latest"

# =============================================================================
# Global Variables (set by flags/env)
# =============================================================================

NETWORK="${DEVNET_NETWORK:-mainnet}"
DEVNET_DIR="${DEVNET_DIR:-$DEFAULT_DEVNET_DIR}"
CACHE_DIR="${DEVNET_CACHE:-$DEFAULT_CACHE_DIR}"
CHAIN_ID="${DEVNET_CHAIN_ID:-}"
STABLED_IMAGE="${STABLED_IMAGE:-$DEFAULT_STABLED_IMAGE}"
STABLED_TAG="${STABLED_TAG:-$DEFAULT_STABLED_TAG}"

# Flags
LOCAL_BINARY=""
REBUILD=false
NO_CACHE=false
VERBOSE=false
NUM_VALIDATORS=4
NUM_ACCOUNTS=10
FORCE=false
CLEAN_CACHE=false
HARD_RESET=false
OUTPUT_FORMAT="text"
KEY_TYPE="all"
LOG_NODE="all"
LOG_FOLLOW=false
LOG_TAIL=100
SHOW_JSON=false
EXECUTION_MODE="docker"  # "docker" or "local"

# =============================================================================
# Environment Variable Loading (T012)
# =============================================================================

load_env() {
    local env_file="$PROJECT_ROOT/docker/.env"
    if [[ -f "$env_file" ]]; then
        # Source env file but don't fail on errors
        set +e
        # shellcheck source=/dev/null
        source "$env_file" 2>/dev/null
        set -e

        # Override globals from env if set
        STABLED_IMAGE="${STABLED_IMAGE:-$DEFAULT_STABLED_IMAGE}"
        STABLED_TAG="${STABLED_TAG:-$DEFAULT_STABLED_TAG}"
    fi

    # Also check for .env in project root
    local root_env_file="$PROJECT_ROOT/.env"
    if [[ -f "$root_env_file" ]]; then
        set +e
        # shellcheck source=/dev/null
        source "$root_env_file" 2>/dev/null
        set -e
    fi
}

# =============================================================================
# Prerequisite Check Functions (T008)
# =============================================================================

check_command() {
    local cmd=$1
    local name=${2:-$1}
    if command -v "$cmd" &> /dev/null; then
        local version
        version=$("$cmd" --version 2>/dev/null | head -1 || echo "unknown")
        success "$name: $version"
        return 0
    else
        error "$name is not installed"
        return 1
    fi
}

check_docker_running() {
    if ! docker info &> /dev/null; then
        error "Docker daemon is not running"
        error "Please start Docker and try again"
        return 1
    fi
    success "Docker daemon is running"
    return 0
}

check_prerequisites() {
    local failed=0

    info "Checking prerequisites..."

    # Docker is only required for docker mode
    if [[ "$EXECUTION_MODE" == "docker" ]]; then
        # Required tools
        check_command docker "Docker" || ((failed++))

        # Check docker compose (v2 style)
        if docker compose version &> /dev/null; then
            local compose_version
            compose_version=$(docker compose version --short 2>/dev/null || echo "unknown")
            success "docker-compose: $compose_version"
        else
            error "docker-compose is not available"
            error "Please install Docker Compose v2+"
            ((failed++))
        fi

        # Check Docker daemon is running
        check_docker_running || ((failed++))
    else
        info "Local binary mode - skipping Docker checks"
    fi

    check_command go "Go" || ((failed++))
    check_command curl "curl" || ((failed++))
    check_command jq "jq" || ((failed++))

    # Network-specific decompressor
    if [[ "$NETWORK" == "testnet" ]]; then
        check_command lz4 "lz4" || ((failed++))
    else
        check_command zstd "zstd" || ((failed++))
    fi

    if [[ $failed -gt 0 ]]; then
        error "Prerequisites check failed ($failed errors)"
        return 1
    fi

    return 0
}

# =============================================================================
# Disk Space Check (T009)
# =============================================================================

check_disk_space() {
    local required_gb=${1:-50}
    local path=${2:-$PROJECT_ROOT}
    local min_gb=${3:-5}  # Minimum required space

    # Get available space in GB
    local available_gb
    if [[ "$(uname)" == "Darwin" ]]; then
        # macOS
        available_gb=$(df -g "$path" | tail -1 | awk '{print $4}')
    else
        # Linux
        available_gb=$(df -BG "$path" | tail -1 | awk '{print $4}' | tr -d 'G')
    fi

    if [[ "$available_gb" -lt "$min_gb" ]]; then
        error "Insufficient disk space: ${available_gb}GB available, need at least ${min_gb}GB"
        return 1
    elif [[ "$available_gb" -lt 20 ]]; then
        warn "Low disk space: ${available_gb}GB available (recommend 20GB+)"
        warn "Proceeding anyway - may fail if snapshot download needed"
    elif [[ "$available_gb" -lt "$required_gb" ]]; then
        warn "Low disk space: ${available_gb}GB available (recommend ${required_gb}GB for mainnet)"
    else
        success "Disk space: ${available_gb}GB available"
    fi

    return 0
}

# =============================================================================
# Devnet Builder Functions (T020, T021)
# =============================================================================

ensure_devnet_builder() {
    local builder_path="$PROJECT_ROOT/build/devnet-builder"

    # If NETWORK_VERSION is set, use build-for-version.sh for dynamic version
    if [[ -n "${NETWORK_VERSION:-}" ]]; then
        info "NETWORK_VERSION is set to '$NETWORK_VERSION'"
        info "Using build-for-version.sh for dynamic version support"

        local version_script="$SCRIPT_DIR/build-for-version.sh"
        if [[ ! -x "$version_script" ]]; then
            error "build-for-version.sh not found at $version_script"
            return 1
        fi

        # Set up alias for the versioned binary
        # The script will cache the binary and we can get its path
        local versioned_binary
        versioned_binary=$("$version_script" -v "$NETWORK_VERSION" 2>&1 | grep -o '/[^[:space:]]*devnet-builder' | tail -1)

        if [[ -n "$versioned_binary" ]] && [[ -x "$versioned_binary" ]]; then
            # Create symlink to versioned binary in build directory
            mkdir -p "$PROJECT_ROOT/build"
            ln -sf "$versioned_binary" "$builder_path"
            success "Using devnet-builder for network version: $NETWORK_VERSION"
            return 0
        else
            error "Failed to get versioned devnet-builder binary"
            error "Try running: $version_script -v $NETWORK_VERSION --verbose"
            return 1
        fi
    fi

    # Standard path: use existing or build default
    if [[ -x "$builder_path" ]]; then
        success "devnet-builder is available"
        return 0
    fi

    progress "Building devnet-builder..."

    if ! make -C "$PROJECT_ROOT" build &> /dev/null; then
        error "Failed to build devnet-builder"
        error "Run 'make build' manually to see errors"
        return 1
    fi

    success "devnet-builder built successfully"
    return 0
}

run_devnet_builder() {
    local genesis_file=$1
    local output_dir=$2
    local chain_id=$3

    local builder_path="$PROJECT_ROOT/build/devnet-builder"

    progress "Running devnet-builder..."

    local builder_args=(
        "build"
        "$genesis_file"
        "--output" "$output_dir"
        "--validators" "$NUM_VALIDATORS"
        "--accounts" "$NUM_ACCOUNTS"
    )

    if [[ -n "$chain_id" ]]; then
        builder_args+=("--chain-id" "$chain_id")
    fi

    if $VERBOSE; then
        "$builder_path" "${builder_args[@]}"
    else
        "$builder_path" "${builder_args[@]}" &> /dev/null
    fi

    success "Devnet built with $NUM_VALIDATORS validators and $NUM_ACCOUNTS accounts"
}

# =============================================================================
# Command Parser and Help (T011)
# =============================================================================

show_version() {
    echo "Local Devnet Script v$SCRIPT_VERSION"
}

show_help() {
    local command=${1:-}

    if [[ -z "$command" ]]; then
        cat << 'EOF'
Local Devnet Script - Run a local 4-node devnet

USAGE:
    ./scripts/local-devnet.sh <command> [options]

COMMANDS:
    start         Start the local devnet
    stop          Stop the local devnet
    restart       Restart the local devnet
    reset         Reset chain state (keep genesis and config)
    clean         Remove all devnet data
    status        Show devnet status
    logs          View node logs
    export-keys   Export account and validator keys
    help          Show this help message

GLOBAL OPTIONS:
    --verbose     Enable verbose output
    --version     Show version information

EXAMPLES:
    # Start devnet with mainnet snapshot (default)
    ./scripts/local-devnet.sh start

    # Start with testnet snapshot
    ./scripts/local-devnet.sh start --network testnet

    # Start with local binary
    ./scripts/local-devnet.sh start --local-binary /path/to/stabled

    # Start with specific network version/branch
    NETWORK_VERSION=feat/usdt0-gas ./scripts/local-devnet.sh start

    # Stop the devnet
    ./scripts/local-devnet.sh stop

    # View logs for node0
    ./scripts/local-devnet.sh logs node0

    # Export keys in JSON format
    ./scripts/local-devnet.sh export-keys --format json

ENVIRONMENT VARIABLES:
    NETWORK_VERSION         Build devnet-builder with a specific network version/branch
                            (e.g., v1.1.3, feat/usdt0-gas, commit-hash)
    DEVNET_NETWORK          Source network (mainnet or testnet)
    DEVNET_DIR              Custom devnet data directory
    DEVNET_CACHE            Custom cache directory

Run './scripts/local-devnet.sh help <command>' for more info on a command.
EOF
    else
        case "$command" in
            start)
                cat << 'EOF'
Start the local devnet

USAGE:
    ./scripts/local-devnet.sh start [options]

OPTIONS:
    --network <net>       Source network (testnet or mainnet, default: mainnet)
    --local-binary <path> Use local stabled binary instead of Docker
    --rebuild             Force rebuild even if devnet exists
    --no-cache            Don't use cached snapshots
    --chain-id <id>       Custom chain ID
    --validators <n>      Number of validators (1-4, default: 4)
    --accounts <n>        Number of test accounts (default: 10)
    --devnet-dir <path>   Custom devnet data directory
    --verbose             Enable verbose output

EXAMPLES:
    ./scripts/local-devnet.sh start
    ./scripts/local-devnet.sh start --network testnet
    ./scripts/local-devnet.sh start --local-binary ./build/stabled
    ./scripts/local-devnet.sh start --rebuild --chain-id mydevnet_2200-1
EOF
                ;;
            stop)
                cat << 'EOF'
Stop the local devnet

USAGE:
    ./scripts/local-devnet.sh stop

This command stops all running devnet nodes gracefully.
EOF
                ;;
            restart)
                cat << 'EOF'
Restart the local devnet

USAGE:
    ./scripts/local-devnet.sh restart

This command stops and then starts the devnet.
EOF
                ;;
            reset)
                cat << 'EOF'
Reset chain state

USAGE:
    ./scripts/local-devnet.sh reset [options]

OPTIONS:
    --hard    Also regenerate genesis (full reset)

This command clears the chain data but keeps the genesis and configuration.
Use --hard to also regenerate the genesis from scratch.
EOF
                ;;
            clean)
                cat << 'EOF'
Remove all devnet data

USAGE:
    ./scripts/local-devnet.sh clean [options]

OPTIONS:
    --cache   Also remove cached snapshots (~/.stable-devnet/)
    --force   Skip confirmation prompt

This command removes all devnet data from the project.
EOF
                ;;
            status)
                cat << 'EOF'
Show devnet status

USAGE:
    ./scripts/local-devnet.sh status [options]

OPTIONS:
    --json    Output in JSON format

Shows the current status of all devnet nodes including block height and health.
EOF
                ;;
            logs)
                cat << 'EOF'
View node logs

USAGE:
    ./scripts/local-devnet.sh logs [node] [options]

ARGUMENTS:
    node              Node to view logs for (node0, node1, node2, node3, or all)

OPTIONS:
    -f, --follow      Follow log output
    -n, --tail <n>    Number of lines to show (default: 100)

EXAMPLES:
    ./scripts/local-devnet.sh logs
    ./scripts/local-devnet.sh logs node0 -f
    ./scripts/local-devnet.sh logs node1 --tail 50
EOF
                ;;
            export-keys)
                cat << 'EOF'
Export account and validator keys

USAGE:
    ./scripts/local-devnet.sh export-keys [options]

OPTIONS:
    --format <fmt>    Output format (text, json, or env, default: text)
    --type <type>     Type of keys (all, validators, or accounts, default: all)

EXAMPLES:
    ./scripts/local-devnet.sh export-keys
    ./scripts/local-devnet.sh export-keys --format json
    ./scripts/local-devnet.sh export-keys --type accounts --format env
EOF
                ;;
            *)
                error "Unknown command: $command"
                show_help
                return 1
                ;;
        esac
    fi
}

parse_start_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --network)
                NETWORK="$2"
                if [[ "$NETWORK" != "testnet" && "$NETWORK" != "mainnet" ]]; then
                    error "Invalid network: $NETWORK (must be 'testnet' or 'mainnet')"
                    exit 1
                fi
                shift 2
                ;;
            --local-binary)
                LOCAL_BINARY="$2"
                if [[ ! -x "$LOCAL_BINARY" ]]; then
                    error "Local binary not found or not executable: $LOCAL_BINARY"
                    exit 1
                fi
                EXECUTION_MODE="local"
                shift 2
                ;;
            --rebuild)
                REBUILD=true
                shift
                ;;
            --no-cache)
                NO_CACHE=true
                shift
                ;;
            --chain-id)
                CHAIN_ID="$2"
                shift 2
                ;;
            --validators)
                NUM_VALIDATORS="$2"
                if [[ "$NUM_VALIDATORS" -lt 1 || "$NUM_VALIDATORS" -gt 4 ]]; then
                    error "Invalid validator count: $NUM_VALIDATORS (must be 1-4)"
                    exit 1
                fi
                shift 2
                ;;
            --accounts)
                NUM_ACCOUNTS="$2"
                shift 2
                ;;
            --devnet-dir)
                DEVNET_DIR="$2"
                shift 2
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            *)
                error "Unknown option for start: $1"
                show_help start
                exit 1
                ;;
        esac
    done
}

parse_clean_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --cache)
                CLEAN_CACHE=true
                shift
                ;;
            --force)
                FORCE=true
                shift
                ;;
            *)
                error "Unknown option for clean: $1"
                show_help clean
                exit 1
                ;;
        esac
    done
}

parse_reset_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --hard)
                HARD_RESET=true
                shift
                ;;
            *)
                error "Unknown option for reset: $1"
                show_help reset
                exit 1
                ;;
        esac
    done
}

parse_status_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json)
                SHOW_JSON=true
                shift
                ;;
            *)
                error "Unknown option for status: $1"
                show_help status
                exit 1
                ;;
        esac
    done
}

parse_logs_args() {
    # First non-option argument is the node
    if [[ $# -gt 0 && ! "$1" =~ ^- ]]; then
        LOG_NODE="$1"
        shift
    fi

    while [[ $# -gt 0 ]]; do
        case "$1" in
            -f|--follow)
                LOG_FOLLOW=true
                shift
                ;;
            -n|--tail)
                LOG_TAIL="$2"
                shift 2
                ;;
            *)
                error "Unknown option for logs: $1"
                show_help logs
                exit 1
                ;;
        esac
    done
}

parse_export_keys_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --format)
                OUTPUT_FORMAT="$2"
                if [[ "$OUTPUT_FORMAT" != "text" && "$OUTPUT_FORMAT" != "json" && "$OUTPUT_FORMAT" != "env" ]]; then
                    error "Invalid format: $OUTPUT_FORMAT (must be 'text', 'json', or 'env')"
                    exit 1
                fi
                shift 2
                ;;
            --type)
                KEY_TYPE="$2"
                if [[ "$KEY_TYPE" != "all" && "$KEY_TYPE" != "validators" && "$KEY_TYPE" != "accounts" ]]; then
                    error "Invalid type: $KEY_TYPE (must be 'all', 'validators', or 'accounts')"
                    exit 1
                fi
                shift 2
                ;;
            *)
                error "Unknown option for export-keys: $1"
                show_help export-keys
                exit 1
                ;;
        esac
    done
}

# =============================================================================
# Source Helper Scripts
# =============================================================================

source_helpers() {
    local provision_script="$SCRIPT_DIR/provision-and-sync.sh"
    local manage_script="$SCRIPT_DIR/manage-devnet.sh"

    if [[ -f "$provision_script" ]]; then
        # shellcheck source=provision-and-sync.sh
        source "$provision_script"
    fi

    if [[ -f "$manage_script" ]]; then
        # shellcheck source=manage-devnet.sh
        source "$manage_script"
    fi
}

# =============================================================================
# Command Implementations (T026, T036, T037, T040, T044, T050, T054, T058)
# =============================================================================

cmd_start() {
    echo ""
    echo "========================================"
    echo "Local Devnet Script v$SCRIPT_VERSION"
    echo "========================================"

    # Stage 1: Prerequisites
    stage 1 5 "Checking prerequisites..."
    check_prerequisites || exit 1
    check_disk_space || exit 1

    # Determine chain ID - use original chain ID to match snapshot data
    if [[ -z "$CHAIN_ID" ]]; then
        if [[ "$NETWORK" == "mainnet" ]]; then
            CHAIN_ID="$MAINNET_CHAIN_ID"
        else
            CHAIN_ID="$TESTNET_CHAIN_ID"
        fi
    fi

    # Check if devnet already exists
    if [[ -d "$DEVNET_DIR" && -f "$DEVNET_DIR/.devnet-meta.json" ]] && ! $REBUILD; then
        info "Existing devnet found at $DEVNET_DIR"

        # Read existing metadata
        if command -v read_devnet_metadata &> /dev/null; then
            read_devnet_metadata "$DEVNET_DIR"
        fi

        # Start existing devnet
        stage 5 5 "Starting nodes..."
        if [[ "$EXECUTION_MODE" == "docker" ]]; then
            if command -v manage_start_docker &> /dev/null; then
                manage_start_docker "$DEVNET_DIR"
            else
                error "manage_start_docker function not available"
                exit 1
            fi
        else
            if command -v manage_start_local &> /dev/null; then
                manage_start_local "$DEVNET_DIR" "$LOCAL_BINARY"
            else
                error "manage_start_local function not available"
                exit 1
            fi
        fi

        show_start_output
        return 0
    fi

    # Stage 2: Download snapshot and genesis
    stage 2 5 "Downloading snapshot & genesis..."

    local snapshot_url rpc_endpoint source_chain_id decompressor
    if [[ "$NETWORK" == "mainnet" ]]; then
        snapshot_url="$MAINNET_SNAPSHOT_URL"
        rpc_endpoint="$MAINNET_RPC_ENDPOINT"
        source_chain_id="$MAINNET_CHAIN_ID"
        decompressor="$MAINNET_DECOMPRESSOR"
    else
        snapshot_url="$TESTNET_SNAPSHOT_URL"
        rpc_endpoint="$TESTNET_RPC_ENDPOINT"
        source_chain_id="$TESTNET_CHAIN_ID"
        decompressor="$TESTNET_DECOMPRESSOR"
    fi

    # Create cache directory
    mkdir -p "$CACHE_DIR/snapshots/$NETWORK"
    mkdir -p "$CACHE_DIR/genesis"

    # Download or use cached snapshot
    local snapshot_file="$CACHE_DIR/snapshots/$NETWORK/snapshot"
    if [[ "$decompressor" == "lz4" ]]; then
        snapshot_file="${snapshot_file}.tar.lz4"
    else
        snapshot_file="${snapshot_file}.tar.zst"
    fi

    if command -v check_snapshot_cache &> /dev/null; then
        check_snapshot_cache "$NETWORK" "$snapshot_file" "$NO_CACHE" || true
    fi

    if [[ ! -f "$snapshot_file" ]] || $NO_CACHE; then
        if command -v download_snapshot &> /dev/null; then
            download_snapshot "$snapshot_url" "$snapshot_file"
        else
            progress "Downloading snapshot from $snapshot_url..."
            curl -L --progress-bar -o "$snapshot_file" "$snapshot_url"
            success "Snapshot downloaded"
        fi
    else
        success "Using cached $NETWORK snapshot"
    fi

    # Download genesis
    local genesis_file="$CACHE_DIR/genesis/${NETWORK}-genesis.json"
    if command -v download_genesis &> /dev/null; then
        download_genesis "$rpc_endpoint" "$genesis_file"
    else
        progress "Downloading genesis from $rpc_endpoint..."
        curl -s "${rpc_endpoint}genesis" | jq '.result.genesis' > "$genesis_file"
        success "Genesis downloaded"
    fi

    # Stage 3: Export state from snapshot
    stage 3 5 "Exporting state from snapshot..."

    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT

    if command -v extract_snapshot &> /dev/null; then
        extract_snapshot "$snapshot_file" "$temp_dir" "$decompressor"
    else
        progress "Extracting snapshot..."
        if [[ "$decompressor" == "lz4" ]]; then
            lz4 -d -c "$snapshot_file" | tar -xf - -C "$temp_dir"
        else
            zstd -d -c "$snapshot_file" | tar -xf - -C "$temp_dir"
        fi
        success "Snapshot extracted"
    fi

    local exported_genesis="$temp_dir/exported-genesis.json"
    if command -v export_state &> /dev/null; then
        export_state "$temp_dir" "$genesis_file" "$source_chain_id" "$exported_genesis"
    else
        # Use Docker to run stabled export
        progress "Running stabled export..."
        local temp_home="$temp_dir/node"
        mkdir -p "$temp_home/config" "$temp_home/data"

        # Copy genesis
        cp "$genesis_file" "$temp_home/config/genesis.json"

        # Copy snapshot data
        if [[ -d "$temp_dir/data" ]]; then
            cp -r "$temp_dir/data"/* "$temp_home/data/"
        fi

        # Run export using Docker
        docker run --rm \
            -v "$temp_home:/data" \
            "${STABLED_IMAGE}:${STABLED_TAG}" \
            export --home /data > "$exported_genesis" 2>/dev/null || {
                error "Failed to export state"
                exit 1
            }

        success "State exported successfully"
    fi

    # Stage 4: Build devnet
    stage 4 5 "Building devnet..."

    ensure_devnet_builder || exit 1

    # Clean existing devnet if rebuilding
    if [[ -d "$DEVNET_DIR" ]] && $REBUILD; then
        progress "Removing existing devnet..."
        rm -rf "$DEVNET_DIR"
    fi

    mkdir -p "$DEVNET_DIR"

    run_devnet_builder "$exported_genesis" "$DEVNET_DIR" "$CHAIN_ID"

    # Configure nodes
    if command -v configure_nodes &> /dev/null; then
        configure_nodes "$DEVNET_DIR" "$NUM_VALIDATORS"
    fi

    # Write metadata
    if command -v write_devnet_metadata &> /dev/null; then
        write_devnet_metadata "$DEVNET_DIR" "$CHAIN_ID" "$NETWORK" "$EXECUTION_MODE"
    else
        # Write basic metadata
        cat > "$DEVNET_DIR/.devnet-meta.json" << EOF
{
    "version": "$SCRIPT_VERSION",
    "chain_id": "$CHAIN_ID",
    "network_source": "$NETWORK",
    "execution_mode": "$EXECUTION_MODE",
    "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "num_validators": $NUM_VALIDATORS,
    "num_accounts": $NUM_ACCOUNTS,
    "stabled_image": "${STABLED_IMAGE}:${STABLED_TAG}",
    "local_binary": ${LOCAL_BINARY:+\"$LOCAL_BINARY\"}${LOCAL_BINARY:-null}
}
EOF
    fi

    # Stage 5: Start nodes
    stage 5 5 "Starting nodes..."

    if [[ "$EXECUTION_MODE" == "docker" ]]; then
        if command -v manage_start_docker &> /dev/null; then
            manage_start_docker "$DEVNET_DIR"
        else
            progress "Starting with docker-compose..."
            cd "$PROJECT_ROOT/docker"
            DEVNET_DIR="$DEVNET_DIR" docker compose up -d
            cd "$PROJECT_ROOT"
            success "Nodes started"
        fi
    else
        if command -v manage_start_local &> /dev/null; then
            manage_start_local "$DEVNET_DIR" "$LOCAL_BINARY"
        else
            error "Local binary mode requires manage-devnet.sh"
            exit 1
        fi
    fi

    # Wait for health
    if command -v wait_for_health &> /dev/null; then
        wait_for_health "$NUM_VALIDATORS"
    else
        progress "Waiting for nodes to be healthy..."
        sleep 10
        success "Nodes should be ready"
    fi

    show_start_output
}

show_start_output() {
    echo ""
    echo "========================================"
    echo "Devnet is ready!"
    echo "========================================"
    echo ""
    echo "Endpoints:"
    echo "  Node 0 RPC:     http://localhost:26657"
    echo "  Node 0 EVM:     http://localhost:8545"
    echo "  Node 0 API:     http://localhost:1317"
    echo "  Node 0 gRPC:    localhost:9090"
    echo ""
    echo "Chain ID: $CHAIN_ID"
    echo "Source:   $NETWORK"
    echo ""
    echo "Run './scripts/local-devnet.sh export-keys' for full key export"
    echo "Run './scripts/local-devnet.sh stop' to stop the devnet"
}

cmd_stop() {
    info "Stopping devnet..."

    if command -v manage_stop &> /dev/null; then
        manage_stop "$DEVNET_DIR" "$EXECUTION_MODE"
    else
        cd "$PROJECT_ROOT/docker"
        docker compose down
        cd "$PROJECT_ROOT"
    fi

    success "Devnet stopped"
}

cmd_restart() {
    info "Restarting devnet..."
    cmd_stop
    sleep 2
    cmd_start
}

cmd_reset() {
    info "Resetting devnet..."

    if command -v manage_reset &> /dev/null; then
        manage_reset "$DEVNET_DIR" "$HARD_RESET"
    else
        cmd_stop

        # Clear data directories
        for i in $(seq 0 $((NUM_VALIDATORS - 1))); do
            local node_dir="$DEVNET_DIR/node$i/data"
            if [[ -d "$node_dir" ]]; then
                rm -rf "$node_dir"/*
                # Recreate priv_validator_state.json
                echo '{"height":"0","round":0,"step":0}' > "$node_dir/priv_validator_state.json"
            fi
        done
    fi

    success "Devnet reset complete"
}

cmd_clean() {
    if ! $FORCE; then
        read -rp "Are you sure you want to remove all devnet data? [y/N] " confirm
        if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
            info "Cancelled"
            return 0
        fi
    fi

    info "Cleaning devnet data..."

    # Stop first if running
    if docker compose -f "$PROJECT_ROOT/docker/docker-compose.yml" ps -q 2>/dev/null | grep -q .; then
        cmd_stop
    fi

    # Remove devnet directory
    if [[ -d "$DEVNET_DIR" ]]; then
        rm -rf "$DEVNET_DIR"
        success "Removed $DEVNET_DIR"
    fi

    # Remove cache if requested
    if $CLEAN_CACHE && [[ -d "$CACHE_DIR" ]]; then
        rm -rf "$CACHE_DIR"
        success "Removed $CACHE_DIR"
    fi

    success "Clean complete"
}

cmd_status() {
    if command -v manage_status &> /dev/null; then
        manage_status "$DEVNET_DIR" "$SHOW_JSON"
    else
        echo ""
        echo "========================================"
        echo "Devnet Status"
        echo "========================================"

        if [[ ! -f "$DEVNET_DIR/.devnet-meta.json" ]]; then
            error "Devnet not found at $DEVNET_DIR"
            exit 2
        fi

        # Read metadata
        local chain_id network mode
        chain_id=$(jq -r '.chain_id' "$DEVNET_DIR/.devnet-meta.json")
        network=$(jq -r '.network_source' "$DEVNET_DIR/.devnet-meta.json")
        mode=$(jq -r '.execution_mode' "$DEVNET_DIR/.devnet-meta.json")

        echo ""
        echo "Chain ID: $chain_id"
        echo "Source:   $network"
        echo "Mode:     $mode"
        echo ""
        echo "Nodes:"

        # Check each node
        for i in 0 1 2 3; do
            local port=$((26657 + i * 10000))
            local status height catching_up

            if curl -s "http://localhost:$port/status" &> /dev/null; then
                local response
                response=$(curl -s "http://localhost:$port/status")
                height=$(echo "$response" | jq -r '.result.sync_info.latest_block_height')
                catching_up=$(echo "$response" | jq -r '.result.sync_info.catching_up')
                status="running"
            else
                status="stopped"
                height="-"
                catching_up="-"
            fi

            echo "  node$i: $status  height=$height  catching_up=$catching_up"
        done
    fi
}

cmd_logs() {
    if command -v manage_logs &> /dev/null; then
        manage_logs "$LOG_NODE" "$LOG_FOLLOW" "$LOG_TAIL"
    else
        local compose_args=("logs")

        if $LOG_FOLLOW; then
            compose_args+=("-f")
        fi

        compose_args+=("--tail" "$LOG_TAIL")

        if [[ "$LOG_NODE" != "all" ]]; then
            compose_args+=("$LOG_NODE")
        fi

        cd "$PROJECT_ROOT/docker"
        docker compose "${compose_args[@]}"
        cd "$PROJECT_ROOT"
    fi
}

cmd_export_keys() {
    if command -v manage_export_keys &> /dev/null; then
        manage_export_keys "$DEVNET_DIR" "$OUTPUT_FORMAT" "$KEY_TYPE"
    else
        error "export-keys requires manage-devnet.sh"
        exit 1
    fi
}

# =============================================================================
# Main Entry Point
# =============================================================================

main() {
    # Load environment
    load_env

    # Source helper scripts
    source_helpers

    # Parse global options
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --version)
                show_version
                exit 0
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            -*)
                # Unknown global option, might be command-specific
                break
                ;;
            *)
                # Command or positional argument
                break
                ;;
        esac
    done

    # Get command
    local command=${1:-help}
    shift || true

    # Execute command
    case "$command" in
        start)
            parse_start_args "$@"
            cmd_start
            ;;
        stop)
            cmd_stop
            ;;
        restart)
            parse_start_args "$@"
            cmd_restart
            ;;
        reset)
            parse_reset_args "$@"
            cmd_reset
            ;;
        clean)
            parse_clean_args "$@"
            cmd_clean
            ;;
        status)
            parse_status_args "$@"
            cmd_status
            ;;
        logs)
            parse_logs_args "$@"
            cmd_logs
            ;;
        export-keys)
            parse_export_keys_args "$@"
            cmd_export_keys
            ;;
        help)
            show_help "$@"
            ;;
        *)
            error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac
}

# Run main if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
