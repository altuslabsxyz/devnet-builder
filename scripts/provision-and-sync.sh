#!/usr/bin/env bash
#
# provision-and-sync.sh - Snapshot download, genesis fetch, and state export
#
# This script handles:
# - Snapshot caching in ~/.stable-devnet/snapshots/
# - Genesis download from RPC endpoint
# - Direct state export from snapshot using `stabled export` (NO SYNC)
# - Progress indicators for all stages
#
# This script is sourced by local-devnet.sh and provides functions for
# the provisioning phase.

# Prevent direct execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    echo "This script should be sourced, not executed directly."
    echo "Use: source $0"
    exit 1
fi

# =============================================================================
# Snapshot Cache Functions (T014)
# =============================================================================

check_snapshot_cache() {
    local network=$1
    local snapshot_file=$2
    local no_cache=${3:-false}

    if $no_cache; then
        return 1
    fi

    if [[ ! -f "$snapshot_file" ]]; then
        return 1
    fi

    # Check if snapshot is fresh (less than 24 hours old)
    local meta_file="${snapshot_file%.tar.*}.meta.json"
    if [[ -f "$meta_file" ]]; then
        local downloaded_at
        downloaded_at=$(jq -r '.downloaded_at' "$meta_file" 2>/dev/null || echo "")

        if [[ -n "$downloaded_at" ]]; then
            local downloaded_ts current_ts age_hours
            downloaded_ts=$(date -d "$downloaded_at" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%SZ" "$downloaded_at" +%s 2>/dev/null || echo 0)
            current_ts=$(date +%s)
            age_hours=$(( (current_ts - downloaded_ts) / 3600 ))

            if [[ $age_hours -lt 24 ]]; then
                success "Using cached $network snapshot (${age_hours}h old)"
                return 0
            else
                warn "Cached snapshot is ${age_hours}h old (consider re-downloading)"
                return 0
            fi
        fi
    fi

    # File exists but no metadata - use it anyway
    success "Using cached $network snapshot"
    return 0
}

# =============================================================================
# Snapshot Download (T015)
# =============================================================================

download_snapshot() {
    local url=$1
    local output_file=$2
    local max_retries=${3:-3}

    local retry_count=0
    local success_download=false

    while [[ $retry_count -lt $max_retries ]]; do
        progress "Downloading snapshot from $url (attempt $((retry_count + 1))/$max_retries)..."

        if curl -L --progress-bar -o "$output_file" "$url"; then
            success_download=true
            break
        fi

        ((retry_count++))

        if [[ $retry_count -lt $max_retries ]]; then
            local wait_time=$((retry_count * 10))
            warn "Download failed, retrying in ${wait_time}s..."
            sleep "$wait_time"
        fi
    done

    if ! $success_download; then
        error "Failed to download snapshot after $max_retries attempts"
        return 1
    fi

    # Write metadata
    local meta_file="${output_file%.tar.*}.meta.json"
    local file_size
    file_size=$(stat -f%z "$output_file" 2>/dev/null || stat -c%s "$output_file" 2>/dev/null || echo 0)

    cat > "$meta_file" << EOF
{
    "url": "$url",
    "downloaded_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "size_bytes": $file_size
}
EOF

    success "Snapshot downloaded ($(numfmt --to=iec "$file_size" 2>/dev/null || echo "${file_size} bytes"))"
    return 0
}

# =============================================================================
# Genesis Download (T016)
# =============================================================================

download_genesis() {
    local rpc_endpoint=$1
    local output_file=$2

    progress "Downloading genesis from $rpc_endpoint..."

    # Ensure the endpoint doesn't have double slashes
    rpc_endpoint="${rpc_endpoint%/}"

    if ! curl -s "${rpc_endpoint}/genesis" | jq -e '.result.genesis' > "$output_file" 2>/dev/null; then
        error "Failed to download genesis from $rpc_endpoint"
        return 1
    fi

    # Validate genesis is not empty
    if [[ ! -s "$output_file" ]] || [[ "$(jq -r '.chain_id // empty' "$output_file" 2>/dev/null)" == "" ]]; then
        error "Downloaded genesis is invalid or empty"
        return 1
    fi

    success "Genesis downloaded from $rpc_endpoint"
    return 0
}

# =============================================================================
# Snapshot Extraction (T017)
# =============================================================================

extract_snapshot() {
    local snapshot_file=$1
    local output_dir=$2
    local decompressor=$3

    progress "Extracting snapshot..."

    mkdir -p "$output_dir"

    case "$decompressor" in
        lz4)
            if ! lz4 -d -c "$snapshot_file" | tar -xf - -C "$output_dir" 2>/dev/null; then
                error "Failed to extract lz4 snapshot"
                return 1
            fi
            ;;
        zstd)
            if ! zstd -d -c "$snapshot_file" | tar -xf - -C "$output_dir" 2>/dev/null; then
                error "Failed to extract zstd snapshot"
                return 1
            fi
            ;;
        *)
            error "Unknown decompressor: $decompressor"
            return 1
            ;;
    esac

    success "Snapshot extracted to $output_dir"
    return 0
}

# =============================================================================
# State Export (T018) - NO SYNC REQUIRED
# =============================================================================

export_state() {
    local temp_dir=$1
    local genesis_file=$2
    local source_chain_id=$3
    local output_file=$4

    progress "Exporting state from snapshot (no sync required)..."

    # Create temp node home
    local temp_home="$temp_dir/export-node"
    mkdir -p "$temp_home/config" "$temp_home/data"

    # Copy genesis
    cp "$genesis_file" "$temp_home/config/genesis.json"

    # Find and copy snapshot data
    # Snapshots may have data directly or in a subdirectory
    if [[ -d "$temp_dir/data" ]]; then
        cp -r "$temp_dir/data"/* "$temp_home/data/" 2>/dev/null || true
    fi

    # Look for common snapshot structures
    for subdir in "" "node" "stabled"; do
        if [[ -d "$temp_dir/$subdir/data" ]]; then
            cp -r "$temp_dir/$subdir/data"/* "$temp_home/data/" 2>/dev/null || true
            break
        fi
    done

    # Check if we have any data
    if [[ ! -d "$temp_home/data/application.db" ]] && [[ ! -d "$temp_home/data/state.db" ]]; then
        warn "No database found in snapshot, using genesis directly"
        cp "$genesis_file" "$output_file"
        success "Using genesis as exported state"
        return 0
    fi

    # Initialize priv_validator_state.json if not present
    if [[ ! -f "$temp_home/data/priv_validator_state.json" ]]; then
        echo '{"height":"0","round":0,"step":0}' > "$temp_home/data/priv_validator_state.json"
    fi

    # Initialize config.toml if not present (required for export)
    if [[ ! -f "$temp_home/config/config.toml" ]]; then
        # Use local binary or docker to init config
        if [[ -n "$LOCAL_BINARY" && -x "$LOCAL_BINARY" ]]; then
            "$LOCAL_BINARY" init export-node --home "$temp_home" --chain-id "$source_chain_id" 2>/dev/null || true
            # Restore the original genesis after init
            cp "$genesis_file" "$temp_home/config/genesis.json"
        fi
    fi

    local export_success=false

    # Try local binary first if available
    if [[ -n "$LOCAL_BINARY" && -x "$LOCAL_BINARY" ]]; then
        progress "Running stabled export with local binary..."
        if "$LOCAL_BINARY" export --home "$temp_home" > "$output_file" 2>/dev/null; then
            if [[ -s "$output_file" ]] && jq -e '.chain_id' "$output_file" &>/dev/null; then
                export_success=true
            fi
        fi
    fi

    # Fallback to Docker if local binary failed or not available
    if [[ "$export_success" != "true" ]]; then
        local stabled_image="${STABLED_IMAGE:-ghcr.io/stablelabs/stable}"
        local stabled_tag="${STABLED_TAG:-latest}"

        progress "Running stabled export with Docker..."
        if docker run --rm \
            -v "$temp_home:/data" \
            "${stabled_image}:${stabled_tag}" \
            export --home /data > "$output_file" 2>/dev/null; then

            if [[ -s "$output_file" ]] && jq -e '.chain_id' "$output_file" &>/dev/null; then
                export_success=true
            fi
        fi
    fi

    if [[ "$export_success" == "true" ]]; then
        local export_height
        export_height=$(jq -r '.initial_height // "0"' "$output_file")
        success "State exported successfully (height: $export_height)"
        return 0
    fi

    # Fallback: use genesis directly
    warn "Export failed, using genesis directly"
    cp "$genesis_file" "$output_file"
    success "Using genesis as exported state"
    return 0
}

# =============================================================================
# Genesis Caching (T019)
# =============================================================================

cache_exported_genesis() {
    local genesis_file=$1
    local network=$2
    local cache_dir=$3
    local export_height=${4:-0}

    local cached_file="$cache_dir/genesis/${network}-exported-genesis.json"
    local meta_file="$cache_dir/genesis/${network}-genesis.meta.json"

    mkdir -p "$cache_dir/genesis"

    cp "$genesis_file" "$cached_file"

    # Get chain ID from genesis
    local chain_id
    chain_id=$(jq -r '.chain_id // "unknown"' "$genesis_file" 2>/dev/null || echo "unknown")

    cat > "$meta_file" << EOF
{
    "network": "$network",
    "exported_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "export_height": $export_height,
    "original_chain_id": "$chain_id"
}
EOF

    success "Genesis cached to $cached_file"
}

check_genesis_cache() {
    local network=$1
    local cache_dir=$2
    local max_age_hours=${3:-24}

    local cached_file="$cache_dir/genesis/${network}-exported-genesis.json"
    local meta_file="$cache_dir/genesis/${network}-genesis.meta.json"

    if [[ ! -f "$cached_file" ]] || [[ ! -f "$meta_file" ]]; then
        return 1
    fi

    # Check age
    local exported_at
    exported_at=$(jq -r '.exported_at' "$meta_file" 2>/dev/null || echo "")

    if [[ -z "$exported_at" ]]; then
        return 1
    fi

    local exported_ts current_ts age_hours
    exported_ts=$(date -d "$exported_at" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%SZ" "$exported_at" +%s 2>/dev/null || echo 0)
    current_ts=$(date +%s)
    age_hours=$(( (current_ts - exported_ts) / 3600 ))

    if [[ $age_hours -lt $max_age_hours ]]; then
        success "Using cached exported genesis (${age_hours}h old)"
        echo "$cached_file"
        return 0
    fi

    return 1
}
