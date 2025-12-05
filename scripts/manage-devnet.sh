#!/usr/bin/env bash
#
# manage-devnet.sh - Lifecycle management helpers for local devnet
#
# This script provides functions for:
# - Starting devnet (Docker and local binary modes)
# - Stopping devnet
# - Status checking
# - Key export
# - Metadata management
#
# This script is sourced by local-devnet.sh.

# Prevent direct execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    echo "This script should be sourced, not executed directly."
    echo "Use: source $0"
    exit 1
fi

# =============================================================================
# Metadata Functions (T013)
# =============================================================================

read_devnet_metadata() {
    local devnet_dir=$1
    local meta_file="$devnet_dir/.devnet-meta.json"

    if [[ ! -f "$meta_file" ]]; then
        return 1
    fi

    # Export metadata as variables
    DEVNET_CHAIN_ID=$(jq -r '.chain_id // ""' "$meta_file")
    DEVNET_NETWORK=$(jq -r '.network_source // ""' "$meta_file")
    DEVNET_MODE=$(jq -r '.execution_mode // "docker"' "$meta_file")
    DEVNET_VALIDATORS=$(jq -r '.num_validators // 4' "$meta_file")
    DEVNET_ACCOUNTS=$(jq -r '.num_accounts // 10' "$meta_file")
    DEVNET_LOCAL_BINARY=$(jq -r '.local_binary // ""' "$meta_file")

    export DEVNET_CHAIN_ID DEVNET_NETWORK DEVNET_MODE DEVNET_VALIDATORS DEVNET_ACCOUNTS DEVNET_LOCAL_BINARY
    return 0
}

write_devnet_metadata() {
    local devnet_dir=$1
    local chain_id=$2
    local network=$3
    local mode=$4
    local num_validators=${5:-4}
    local num_accounts=${6:-10}
    local local_binary=${7:-}
    local stabled_image=${8:-}

    local meta_file="$devnet_dir/.devnet-meta.json"

    mkdir -p "$devnet_dir"

    cat > "$meta_file" << EOF
{
    "version": "${SCRIPT_VERSION:-1.0.0}",
    "chain_id": "$chain_id",
    "network_source": "$network",
    "execution_mode": "$mode",
    "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "last_started_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "num_validators": $num_validators,
    "num_accounts": $num_accounts,
    "stabled_image": "${stabled_image:-null}",
    "local_binary": ${local_binary:+\"$local_binary\"}${local_binary:-null}
}
EOF
}

update_devnet_started() {
    local devnet_dir=$1
    local meta_file="$devnet_dir/.devnet-meta.json"

    if [[ -f "$meta_file" ]]; then
        local tmp_file
        tmp_file=$(mktemp)
        jq --arg ts "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" '.last_started_at = $ts' "$meta_file" > "$tmp_file"
        mv "$tmp_file" "$meta_file"
    fi
}

# =============================================================================
# Node Configuration (T022)
# =============================================================================

configure_nodes() {
    local devnet_dir=$1
    local num_validators=${2:-4}

    progress "Configuring node networking..."

    # Get node IDs
    local node_ids=()
    for i in $(seq 0 $((num_validators - 1))); do
        local node_key="$devnet_dir/node$i/config/node_key.json"
        if [[ -f "$node_key" ]]; then
            # Extract node ID from node_key.json
            # The node ID is derived from the public key
            local node_id
            node_id=$(docker run --rm -v "$devnet_dir/node$i:/data" \
                "${STABLED_IMAGE:-ghcr.io/stablelabs/stable}:${STABLED_TAG:-latest}" \
                tendermint show-node-id --home /data 2>/dev/null || echo "")

            if [[ -z "$node_id" ]]; then
                # Fallback: use fixed node IDs from config
                case $i in
                    0) node_id="a18d66435236d91ba28e1bf7a82d400b9a188f5f" ;;
                    1) node_id="2edec3e0270cba790f849b44f46b9120ed3f153f" ;;
                    2) node_id="916431c30a36aff0b72a798ed86965903576d38c" ;;
                    3) node_id="48496c38733af68c8ce1cfcb6e1ff476cfac260a" ;;
                esac
            fi
            node_ids+=("$node_id")
        fi
    done

    # Configure each node
    for i in $(seq 0 $((num_validators - 1))); do
        local config_file="$devnet_dir/node$i/config/config.toml"

        if [[ ! -f "$config_file" ]]; then
            continue
        fi

        # Build persistent_peers string
        local persistent_peers=""
        for j in $(seq 0 $((num_validators - 1))); do
            if [[ $i -ne $j ]]; then
                local peer_port=$((26656 + j * 10000))
                if [[ -n "$persistent_peers" ]]; then
                    persistent_peers+=","
                fi
                persistent_peers+="${node_ids[$j]}@host.docker.internal:$peer_port"
            fi
        done

        # Calculate ports for this node
        local rpc_port=$((26657 + i * 10000))
        local p2p_port=$((26656 + i * 10000))
        local pprof_port=$((6060 + i * 10000))

        # Update config.toml
        sed -i.bak \
            -e "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:$rpc_port\"|g" \
            -e "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:$p2p_port\"|g" \
            -e "s|persistent_peers = \".*\"|persistent_peers = \"$persistent_peers\"|g" \
            -e "s|allow_duplicate_ip = false|allow_duplicate_ip = true|g" \
            -e "s|addr_book_strict = true|addr_book_strict = false|g" \
            -e "s|pprof_laddr = \".*\"|pprof_laddr = \"localhost:$pprof_port\"|g" \
            "$config_file" 2>/dev/null || true

        rm -f "$config_file.bak"

        # Update app.toml
        local app_file="$devnet_dir/node$i/config/app.toml"
        if [[ -f "$app_file" ]]; then
            local api_port=$((1317 + i * 10000))
            local grpc_port=$((9090 + i * 10000))
            local evm_rpc_port=$((8545 + i * 10000))
            local evm_ws_port=$((8546 + i * 10000))

            sed -i.bak \
                -e "s|address = \"tcp://localhost:1317\"|address = \"tcp://0.0.0.0:$api_port\"|g" \
                -e "s|address = \"localhost:9090\"|address = \"0.0.0.0:$grpc_port\"|g" \
                -e "s|address = \"127.0.0.1:8545\"|address = \"0.0.0.0:$evm_rpc_port\"|g" \
                -e "s|ws-address = \"127.0.0.1:8546\"|ws-address = \"0.0.0.0:$evm_ws_port\"|g" \
                -e "s|enable = false|enable = true|g" \
                "$app_file" 2>/dev/null || true

            rm -f "$app_file.bak"
        fi
    done

    success "Node networking configured"
}

# =============================================================================
# Docker Mode Start (T023)
# =============================================================================

manage_start_docker() {
    local devnet_dir=$1

    progress "Starting nodes with docker-compose..."

    # Get project root
    local project_root
    project_root="$(cd "$(dirname "$devnet_dir")" && pwd)"
    local compose_file="$project_root/docker/docker-compose.yml"

    if [[ ! -f "$compose_file" ]]; then
        error "docker-compose.yml not found at $compose_file"
        return 1
    fi

    # Export environment for docker-compose
    export DEVNET_DIR="$devnet_dir"
    export CHAIN_ID="${CHAIN_ID:-stabledevnet_988-1}"

    cd "$project_root/docker"

    if ! docker compose up -d; then
        error "Failed to start docker-compose"
        cd "$project_root"
        return 1
    fi

    cd "$project_root"

    # Update metadata
    update_devnet_started "$devnet_dir"

    # Show node status
    for i in 0 1 2 3; do
        local container_name="devnet-node$i"
        if docker ps --format '{{.Names}}' | grep -q "$container_name"; then
            success "node$i started"
        else
            progress "node$i starting..."
        fi
    done

    return 0
}

# =============================================================================
# Local Binary Mode Start (T030)
# =============================================================================

manage_start_local() {
    local devnet_dir=$1
    local local_binary=$2
    local num_validators=${3:-4}

    progress "Starting nodes with local binary: $local_binary"

    if [[ ! -x "$local_binary" ]]; then
        error "Local binary not found or not executable: $local_binary"
        return 1
    fi

    # First, ensure all nodes have config files initialized and collect node IDs
    local node_ids=()
    for i in $(seq 0 $((num_validators - 1))); do
        local node_home="$devnet_dir/node$i"
        local config_file="$node_home/config/config.toml"
        local genesis_file="$node_home/config/genesis.json"

        if [[ ! -f "$config_file" ]]; then
            progress "Initializing config for node$i..."

            # Backup devnet-builder genesis before init
            local genesis_backup=""
            if [[ -f "$genesis_file" ]]; then
                genesis_backup=$(mktemp)
                cp "$genesis_file" "$genesis_backup"
            fi

            # Initialize config
            "$local_binary" init "node$i" --home "$node_home" --chain-id "$(jq -r '.chain_id' "$genesis_backup" 2>/dev/null || echo 'stable_988-1')" 2>/dev/null || true

            # Restore devnet-builder genesis (overwrite the one created by init)
            if [[ -n "$genesis_backup" && -f "$genesis_backup" ]]; then
                cp "$genesis_backup" "$genesis_file"
                rm -f "$genesis_backup"
            fi
        fi

        # Get node ID from node_key.json
        local node_key="$node_home/config/node_key.json"
        if [[ -f "$node_key" ]]; then
            # Extract node ID using the binary
            local node_id
            node_id=$("$local_binary" comet show-node-id --home "$node_home" 2>/dev/null || echo "")
            node_ids+=("$node_id")
        fi

        # Update config.toml to allow duplicate IPs and relaxed address book
        if [[ -f "$config_file" ]]; then
            sed -i \
                -e "s|allow_duplicate_ip = false|allow_duplicate_ip = true|g" \
                -e "s|addr_book_strict = true|addr_book_strict = false|g" \
                "$config_file"
        fi
    done

    # Now start all nodes with unique ports via command line flags
    for i in $(seq 0 $((num_validators - 1))); do
        local node_home="$devnet_dir/node$i"
        local pid_file="$node_home/stabled.pid"
        local log_file="$node_home/stabled.log"

        # Calculate ports for this node
        local rpc_port=$((26657 + i * 10000))
        local p2p_port=$((26656 + i * 10000))
        local pprof_port=$((6060 + i * 10000))
        local grpc_port=$((9090 + i * 10000))
        local evm_rpc_port=$((8545 + i * 10000))
        local evm_ws_port=$((8546 + i * 10000))

        # Build persistent_peers string (all other nodes)
        local persistent_peers=""
        for j in $(seq 0 $((num_validators - 1))); do
            if [[ $i -ne $j ]]; then
                local peer_p2p_port=$((26656 + j * 10000))
                if [[ -n "$persistent_peers" ]]; then
                    persistent_peers+=","
                fi
                persistent_peers+="${node_ids[$j]}@127.0.0.1:$peer_p2p_port"
            fi
        done

        # Skip if already running
        if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then
            success "node$i already running (PID: $(cat "$pid_file"))"
            continue
        fi

        progress "Starting node$i on ports: RPC=$rpc_port, P2P=$p2p_port, gRPC=$grpc_port, EVM=$evm_rpc_port..."

        # Start the node with port flags
        nohup "$local_binary" start \
            --home "$node_home" \
            --rpc.laddr "tcp://0.0.0.0:$rpc_port" \
            --p2p.laddr "tcp://0.0.0.0:$p2p_port" \
            --p2p.persistent_peers "$persistent_peers" \
            --grpc.address "0.0.0.0:$grpc_port" \
            --grpc.enable \
            --json-rpc.address "0.0.0.0:$evm_rpc_port" \
            --json-rpc.ws-address "0.0.0.0:$evm_ws_port" \
            --json-rpc.enable \
            --rpc.pprof_laddr "localhost:$pprof_port" \
            --api.enable \
            > "$log_file" 2>&1 &
        local pid=$!
        echo "$pid" > "$pid_file"

        # Wait a moment to check if it started
        sleep 2

        if kill -0 "$pid" 2>/dev/null; then
            success "node$i started (PID: $pid)"
        else
            error "node$i failed to start (check $log_file)"
            return 1
        fi
    done

    # Update metadata
    update_devnet_started "$devnet_dir"

    return 0
}

# =============================================================================
# PID File Management (T031)
# =============================================================================

get_node_pid() {
    local devnet_dir=$1
    local node_index=$2

    local pid_file="$devnet_dir/node$node_index/stabled.pid"

    if [[ -f "$pid_file" ]]; then
        cat "$pid_file"
    fi
}

is_node_running() {
    local devnet_dir=$1
    local node_index=$2

    local pid
    pid=$(get_node_pid "$devnet_dir" "$node_index")

    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
        return 0
    fi
    return 1
}

# =============================================================================
# Health Check (T024)
# =============================================================================

wait_for_health() {
    local num_validators=${1:-4}
    local timeout=${2:-120}
    local interval=${3:-5}

    progress "Waiting for nodes to be healthy (timeout: ${timeout}s)..."

    local start_time
    start_time=$(date +%s)

    while true; do
        local healthy_count=0

        for i in $(seq 0 $((num_validators - 1))); do
            local port=$((26657 + i * 10000))

            if curl -s "http://localhost:$port/status" &> /dev/null; then
                ((healthy_count++))
            fi
        done

        if [[ $healthy_count -eq $num_validators ]]; then
            success "All $num_validators nodes are healthy"
            return 0
        fi

        local elapsed=$(( $(date +%s) - start_time ))
        if [[ $elapsed -ge $timeout ]]; then
            warn "Timeout waiting for nodes ($healthy_count/$num_validators healthy)"
            return 1
        fi

        progress "$healthy_count/$num_validators nodes healthy, waiting..."
        sleep "$interval"
    done
}

# =============================================================================
# Stop Command (T035)
# =============================================================================

manage_stop() {
    local devnet_dir=$1
    local mode=${2:-docker}

    if [[ "$mode" == "docker" ]]; then
        progress "Stopping docker-compose..."

        local project_root
        project_root="$(cd "$(dirname "$devnet_dir")" && pwd)"

        cd "$project_root/docker"
        docker compose down
        cd "$project_root"

        success "Docker containers stopped"
    else
        progress "Stopping local processes..."

        for i in 0 1 2 3; do
            local pid_file="$devnet_dir/node$i/stabled.pid"

            if [[ -f "$pid_file" ]]; then
                local pid
                pid=$(cat "$pid_file")

                if kill -0 "$pid" 2>/dev/null; then
                    kill "$pid" 2>/dev/null || true
                    success "node$i stopped (PID: $pid)"
                fi
                rm -f "$pid_file"
            fi
        done

        success "Local processes stopped"
    fi
}

# =============================================================================
# Reset Command (T038, T039)
# =============================================================================

manage_reset() {
    local devnet_dir=$1
    local hard=${2:-false}

    # First stop
    local mode="docker"
    if [[ -f "$devnet_dir/.devnet-meta.json" ]]; then
        mode=$(jq -r '.execution_mode // "docker"' "$devnet_dir/.devnet-meta.json")
    fi

    manage_stop "$devnet_dir" "$mode"

    if $hard; then
        progress "Hard reset - removing all devnet data..."
        rm -rf "$devnet_dir"
        success "Hard reset complete - run 'start' to rebuild"
    else
        progress "Soft reset - clearing chain data..."

        for i in 0 1 2 3; do
            local data_dir="$devnet_dir/node$i/data"

            if [[ -d "$data_dir" ]]; then
                # Remove everything except priv_validator_state.json
                find "$data_dir" -mindepth 1 -maxdepth 1 ! -name 'priv_validator_state.json' -exec rm -rf {} +

                # Reset priv_validator_state.json
                echo '{"height":"0","round":0,"step":0}' > "$data_dir/priv_validator_state.json"
            fi
        done

        success "Chain data cleared - genesis and config preserved"
    fi
}

# =============================================================================
# Clean Command (T041, T042, T043)
# =============================================================================

manage_clean() {
    local devnet_dir=$1
    local clean_cache=${2:-false}
    local cache_dir=${3:-$HOME/.stable-devnet}

    # First stop if running
    local mode="docker"
    if [[ -f "$devnet_dir/.devnet-meta.json" ]]; then
        mode=$(jq -r '.execution_mode // "docker"' "$devnet_dir/.devnet-meta.json")
    fi

    manage_stop "$devnet_dir" "$mode" 2>/dev/null || true

    # Remove devnet directory
    if [[ -d "$devnet_dir" ]]; then
        rm -rf "$devnet_dir"
        success "Removed $devnet_dir"
    fi

    # Remove cache if requested
    if $clean_cache && [[ -d "$cache_dir" ]]; then
        rm -rf "$cache_dir"
        success "Removed $cache_dir"
    fi
}

# =============================================================================
# Status Command (T051, T052, T053)
# =============================================================================

manage_status() {
    local devnet_dir=$1
    local json_output=${2:-false}

    if [[ ! -f "$devnet_dir/.devnet-meta.json" ]]; then
        if $json_output; then
            echo '{"error": "Devnet not found", "status": "not_found"}'
        else
            error "Devnet not found at $devnet_dir"
        fi
        return 2
    fi

    # Read metadata
    local chain_id network mode num_validators
    chain_id=$(jq -r '.chain_id // ""' "$devnet_dir/.devnet-meta.json")
    network=$(jq -r '.network_source // ""' "$devnet_dir/.devnet-meta.json")
    mode=$(jq -r '.execution_mode // "docker"' "$devnet_dir/.devnet-meta.json")
    num_validators=$(jq -r '.num_validators // 4' "$devnet_dir/.devnet-meta.json")

    # Collect node status
    local nodes_json="["
    local running_count=0

    for i in $(seq 0 $((num_validators - 1))); do
        local port=$((26657 + i * 10000))
        local status="stopped"
        local height=0
        local catching_up=false
        local peers=0

        if response=$(curl -s --connect-timeout 2 "http://localhost:$port/status" 2>/dev/null); then
            if [[ -n "$response" ]] && echo "$response" | jq -e '.result' &>/dev/null; then
                status="running"
                height=$(echo "$response" | jq -r '.result.sync_info.latest_block_height // 0')
                catching_up=$(echo "$response" | jq -r '.result.sync_info.catching_up // false')
                ((running_count++))

                # Get peer count
                if net_response=$(curl -s --connect-timeout 2 "http://localhost:$port/net_info" 2>/dev/null); then
                    peers=$(echo "$net_response" | jq -r '.result.n_peers // 0')
                fi
            fi
        fi

        if [[ $i -gt 0 ]]; then
            nodes_json+=","
        fi
        nodes_json+="{\"name\":\"node$i\",\"status\":\"$status\",\"height\":$height,\"catching_up\":$catching_up,\"peers\":$peers}"
    done
    nodes_json+="]"

    # Determine overall status
    local overall_status="stopped"
    if [[ $running_count -eq $num_validators ]]; then
        overall_status="running"
    elif [[ $running_count -gt 0 ]]; then
        overall_status="partial"
    fi

    if $json_output; then
        cat << EOF
{
    "chain_id": "$chain_id",
    "network": "$network",
    "mode": "$mode",
    "status": "$overall_status",
    "nodes": $nodes_json
}
EOF
    else
        echo ""
        echo "========================================"
        echo "Devnet Status"
        echo "========================================"
        echo ""
        echo "Chain ID: $chain_id"
        echo "Source:   $network"
        echo "Mode:     $mode"
        echo "Status:   $overall_status"
        echo ""
        echo "Nodes:"

        for i in $(seq 0 $((num_validators - 1))); do
            local node_info
            node_info=$(echo "$nodes_json" | jq -r ".[$i]")
            local status height catching_up peers
            status=$(echo "$node_info" | jq -r '.status')
            height=$(echo "$node_info" | jq -r '.height')
            catching_up=$(echo "$node_info" | jq -r '.catching_up')
            peers=$(echo "$node_info" | jq -r '.peers')

            echo "  node$i: $status  height=$height  peers=$peers  catching_up=$catching_up"
        done
    fi

    if [[ "$overall_status" == "running" ]]; then
        return 0
    else
        return 1
    fi
}

# =============================================================================
# Logs Command (T055, T056, T057)
# =============================================================================

manage_logs() {
    local node=${1:-all}
    local follow=${2:-false}
    local tail=${3:-100}

    local project_root
    project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

    local compose_args=("logs")

    if $follow; then
        compose_args+=("-f")
    fi

    compose_args+=("--tail" "$tail")

    if [[ "$node" != "all" ]]; then
        compose_args+=("$node")
    fi

    cd "$project_root/docker"
    docker compose "${compose_args[@]}"
    cd "$project_root"
}

# =============================================================================
# Key Export (T045, T046, T047, T048, T049)
# =============================================================================

manage_export_keys() {
    local devnet_dir=$1
    local format=${2:-text}
    local key_type=${3:-all}

    if [[ ! -d "$devnet_dir" ]]; then
        error "Devnet not found at $devnet_dir"
        return 1
    fi

    local validators_json="["
    local accounts_json="["

    # Export validator keys
    if [[ "$key_type" == "all" || "$key_type" == "validators" ]]; then
        for i in 0 1 2 3; do
            local keyring_dir="$devnet_dir/node$i/keyring-test"

            if [[ ! -d "$keyring_dir" ]]; then
                continue
            fi

            # Find key info files
            for info_file in "$keyring_dir"/*.info; do
                if [[ ! -f "$info_file" ]]; then
                    continue
                fi

                local name
                name=$(basename "$info_file" .info)

                # Read key info (it's a JSON file)
                local key_data
                key_data=$(cat "$info_file" 2>/dev/null || echo "{}")

                # Extract address if available
                local address=""
                local evm_address=""

                # For validators, get the address from the info file
                if echo "$key_data" | jq -e '.address' &>/dev/null; then
                    address=$(echo "$key_data" | jq -r '.address // ""')
                fi

                if [[ $i -gt 0 || "$name" != "validator0" ]]; then
                    validators_json+=","
                fi
                validators_json+="{\"name\":\"$name\",\"node\":\"node$i\",\"cosmos_address\":\"$address\"}"
            done
        done
    fi
    validators_json+="]"

    # Export account keys
    if [[ "$key_type" == "all" || "$key_type" == "accounts" ]]; then
        local accounts_dir="$devnet_dir/accounts/keyring-test"

        if [[ -d "$accounts_dir" ]]; then
            local first_account=true
            for info_file in "$accounts_dir"/*.info; do
                if [[ ! -f "$info_file" ]]; then
                    continue
                fi

                local name
                name=$(basename "$info_file" .info)

                local key_data
                key_data=$(cat "$info_file" 2>/dev/null || echo "{}")

                local address=""
                if echo "$key_data" | jq -e '.address' &>/dev/null; then
                    address=$(echo "$key_data" | jq -r '.address // ""')
                fi

                if ! $first_account; then
                    accounts_json+=","
                fi
                first_account=false
                accounts_json+="{\"name\":\"$name\",\"cosmos_address\":\"$address\"}"
            done
        fi
    fi
    accounts_json+="]"

    # Output based on format
    case "$format" in
        json)
            cat << EOF
{
    "validators": $validators_json,
    "accounts": $accounts_json
}
EOF
            ;;
        env)
            # Parse and output as environment variables
            echo "# Validator Keys"
            echo "$validators_json" | jq -r '.[] | "VALIDATOR_\(.name | ascii_upcase)_ADDRESS=\(.cosmos_address)"'
            echo ""
            echo "# Account Keys"
            echo "$accounts_json" | jq -r '.[] | "ACCOUNT_\(.name | ascii_upcase)_ADDRESS=\(.cosmos_address)"'
            ;;
        text|*)
            echo ""
            echo "========================================"
            echo "Devnet Keys"
            echo "========================================"
            echo ""

            if [[ "$key_type" == "all" || "$key_type" == "validators" ]]; then
                echo "Validators:"
                echo "$validators_json" | jq -r '.[] | "  \(.name) (\(.node)):\n    Cosmos: \(.cosmos_address)"'
                echo ""
            fi

            if [[ "$key_type" == "all" || "$key_type" == "accounts" ]]; then
                echo "Accounts:"
                echo "$accounts_json" | jq -r '.[] | "  \(.name):\n    Cosmos: \(.cosmos_address)"'
            fi

            echo ""
            echo "Note: Use 'stabled keys export' for full private key export"
            ;;
    esac
}
