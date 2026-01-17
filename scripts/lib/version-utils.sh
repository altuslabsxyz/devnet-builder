#!/usr/bin/env bash
#
# version-utils.sh - Shared utility functions for version management
#
# This file contains utility functions used by build-for-version.sh
# and other scripts that need version management capabilities.
#

# Prevent multiple sourcing
if [[ -n "${_VERSION_UTILS_LOADED:-}" ]]; then
    return 0
fi
readonly _VERSION_UTILS_LOADED=1

# =============================================================================
# Logging Functions (T004)
# =============================================================================

error() {
    echo -e "\033[0;31m[ERROR]\033[0m $*" >&2
}

warn() {
    echo -e "\033[0;33m[WARN]\033[0m $*" >&2
}

progress() {
    echo -e "  \033[0;34m[>>]\033[0m $*" >&2
}

success() {
    echo -e "\033[0;32m[OK]\033[0m $*" >&2
}

debug() {
    if [[ "${VERBOSE:-false}" == "true" ]]; then
        echo -e "\033[0;90m[DEBUG]\033[0m $*" >&2
    fi
}

# =============================================================================
# Prerequisite Check Functions (T005)
# =============================================================================

check_go_installed() {
    if ! command -v go &>/dev/null; then
        error "Go is not installed or not in PATH"
        echo ""
        echo "To install Go:"
        echo "  macOS:   brew install go"
        echo "  Linux:   sudo apt install golang-go"
        echo "  Manual:  https://go.dev/dl/"
        echo ""
        echo "Required version: Go 1.21 or later"
        return 5
    fi

    local go_version
    go_version=$(go version | awk '{print $3}' | sed 's/go//')
    debug "Go version: $go_version"

    success "Go $(go version | awk '{print $3}') available"
    return 0
}

check_git_installed() {
    if ! command -v git &>/dev/null; then
        error "Git is not installed or not in PATH"
        return 1
    fi
    debug "Git available: $(git --version)"
    return 0
}

check_prerequisites() {
    progress "Checking prerequisites..."

    check_go_installed || exit 5
    check_git_installed || exit 1

    success "All prerequisites met"
}

# =============================================================================
# Platform Detection Functions (T006)
# =============================================================================

get_platform() {
    local os arch

    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)

    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        i386|i686) arch="386" ;;
    esac

    echo "${os}-${arch}"
}

get_go_version() {
    go version | awk '{print $3}' | sed 's/go//' | cut -d. -f1,2
}

# =============================================================================
# Version Hash Generation Functions (T007)
# =============================================================================

generate_cache_key() {
    local module=$1
    local version=$2
    local platform=${3:-$(get_platform)}
    local go_ver=${4:-$(get_go_version)}

    # Create unique hash from all components
    local hash_input="${module}@${version}@${platform}@go${go_ver}"
    local hash
    hash=$(echo -n "$hash_input" | shasum -a 256 | cut -c1-16)

    # Create readable cache key
    local version_short
    version_short=$(echo "$version" | sed 's/[^a-zA-Z0-9.-]/-/g' | cut -c1-20)

    echo "stable-${version_short}-${hash}-${platform}"
}

# =============================================================================
# Cache Directory Management Functions (T008)
# =============================================================================

ensure_cache_dir() {
    local cache_dir=$1

    if [[ ! -d "$cache_dir" ]]; then
        debug "Creating cache directory: $cache_dir"
        mkdir -p "$cache_dir/binaries"
    fi
}

get_cached_binary_path() {
    local cache_dir=$1
    local cache_key=$2

    echo "$cache_dir/binaries/$cache_key/devnet-builder"
}

get_cached_metadata_path() {
    local cache_dir=$1
    local cache_key=$2

    echo "$cache_dir/binaries/$cache_key/metadata.json"
}

check_cache_exists() {
    local cache_dir=$1
    local cache_key=$2

    local binary_path
    binary_path=$(get_cached_binary_path "$cache_dir" "$cache_key")

    if [[ -f "$binary_path" ]] && [[ -x "$binary_path" ]]; then
        debug "Cache hit: $binary_path"
        return 0
    fi

    debug "Cache miss for key: $cache_key"
    return 1
}

store_in_cache() {
    local cache_dir=$1
    local cache_key=$2
    local binary_path=$3
    local version=$4
    local resolved_commit=${5:-"unknown"}

    local target_dir="$cache_dir/binaries/$cache_key"
    local target_binary="$target_dir/devnet-builder"
    local metadata_file="$target_dir/metadata.json"

    mkdir -p "$target_dir"

    # Copy binary
    cp "$binary_path" "$target_binary"
    chmod +x "$target_binary"

    # Generate metadata
    local checksum
    checksum=$(shasum -a 256 "$target_binary" | cut -d' ' -f1)
    local size_bytes
    size_bytes=$(stat -f%z "$target_binary" 2>/dev/null || stat -c%s "$target_binary" 2>/dev/null)

    cat > "$metadata_file" << EOF
{
    "version": "1.0",
    "cache_key": "$cache_key",
    "module": "github.com/stablelabs/stable",
    "version_requested": "$version",
    "version_resolved": "$resolved_commit",
    "built_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "built_with_go": "$(go version | awk '{print $3}')",
    "platform": "$(get_platform)",
    "size_bytes": $size_bytes,
    "checksum_sha256": "$checksum",
    "verified": true
}
EOF

    success "Cached at: $target_dir"
}

# =============================================================================
# Lock Acquisition/Release Functions (T009)
# =============================================================================

acquire_lock() {
    local lock_dir=$1
    local timeout=${2:-300}
    local wait_time=0
    local wait_interval=1

    while [[ $wait_time -lt $timeout ]]; do
        # Try to create lock directory atomically
        if mkdir "$lock_dir" 2>/dev/null; then
            # Write PID for debugging/cleanup
            echo "$$" > "$lock_dir/pid"
            echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" > "$lock_dir/acquired_at"
            debug "Lock acquired: $lock_dir"
            return 0
        fi

        # Check if lock holder is still running
        if [[ -f "$lock_dir/pid" ]]; then
            local lock_pid
            lock_pid=$(cat "$lock_dir/pid" 2>/dev/null || echo "")
            if [[ -n "$lock_pid" ]] && ! kill -0 "$lock_pid" 2>/dev/null; then
                warn "Cleaning up stale lock (PID $lock_pid)"
                rm -rf "$lock_dir" 2>/dev/null || true
                continue
            fi
        fi

        if [[ $((wait_time % 10)) -eq 0 ]] && [[ $wait_time -gt 0 ]]; then
            progress "Waiting for lock... (${wait_time}/${timeout}s)"
        fi

        sleep "$wait_interval"
        ((wait_time += wait_interval))
    done

    error "Lock acquisition timeout after ${timeout}s (another build in progress?)"
    return 7
}

release_lock() {
    local lock_dir=$1

    if [[ -d "$lock_dir" ]]; then
        rm -rf "$lock_dir"
        debug "Lock released: $lock_dir"
    fi
}

# =============================================================================
# Version Validation Functions (T011-T014)
# =============================================================================

detect_version_type() {
    local version=$1

    # Check if it's a semver tag (v1.2.3)
    if [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
        echo "tag"
        return 0
    fi

    # Check if it's a full commit hash (40 hex chars)
    if [[ "$version" =~ ^[a-f0-9]{40}$ ]]; then
        echo "commit"
        return 0
    fi

    # Check if it's a short commit hash (7-39 hex chars)
    if [[ "$version" =~ ^[a-f0-9]{7,39}$ ]]; then
        echo "commit_short"
        return 0
    fi

    # Otherwise assume it's a branch name
    echo "branch"
    return 0
}

validate_branch_exists() {
    local repo_url=$1
    local branch=$2

    debug "Validating branch '$branch' in $repo_url..."

    local result
    if result=$(git ls-remote --heads "$repo_url" "$branch" 2>/dev/null); then
        if echo "$result" | grep -q "refs/heads/$branch"; then
            local commit_hash
            commit_hash=$(echo "$result" | awk '{print $1}')
            debug "Branch '$branch' exists (commit: ${commit_hash:0:7})"
            echo "$commit_hash"
            return 0
        fi
    fi

    return 1
}

validate_tag_exists() {
    local repo_url=$1
    local tag=$2

    debug "Validating tag '$tag' in $repo_url..."

    local result
    if result=$(git ls-remote --tags "$repo_url" "$tag" 2>/dev/null); then
        if echo "$result" | grep -q "refs/tags/$tag"; then
            local commit_hash
            commit_hash=$(echo "$result" | head -1 | awk '{print $1}')
            debug "Tag '$tag' exists (commit: ${commit_hash:0:7})"
            echo "$commit_hash"
            return 0
        fi
    fi

    return 1
}

resolve_version_to_commit() {
    local repo_url=$1
    local version=$2

    local version_type
    version_type=$(detect_version_type "$version")

    case "$version_type" in
        tag)
            validate_tag_exists "$repo_url" "$version"
            return $?
            ;;
        commit|commit_short)
            # For commits, just return as-is (validation happens at build time)
            echo "$version"
            return 0
            ;;
        branch)
            validate_branch_exists "$repo_url" "$version"
            return $?
            ;;
    esac
}

# =============================================================================
# Fuzzy Matching Functions (T027)
# =============================================================================

get_available_refs() {
    local repo_url=$1
    local ref_type=${2:-all}

    case "$ref_type" in
        branches)
            git ls-remote --heads "$repo_url" 2>/dev/null | awk '{print $2}' | sed 's|refs/heads/||'
            ;;
        tags)
            git ls-remote --tags "$repo_url" 2>/dev/null | awk '{print $2}' | sed 's|refs/tags/||' | grep -v '\^{}'
            ;;
        *)
            {
                git ls-remote --heads "$repo_url" 2>/dev/null | awk '{print $2}' | sed 's|refs/heads/||'
                git ls-remote --tags "$repo_url" 2>/dev/null | awk '{print $2}' | sed 's|refs/tags/||' | grep -v '\^{}'
            } | sort -u
            ;;
    esac
}

fuzzy_match_refs() {
    local repo_url=$1
    local search_term=$2
    local max_results=${3:-5}

    get_available_refs "$repo_url" "all" | grep -i "$search_term" 2>/dev/null | head -n "$max_results" || true
}

# =============================================================================
# Go Module Functions (T016-T018)
# =============================================================================

backup_go_mod() {
    local project_root=$1

    cp "$project_root/go.mod" "$project_root/go.mod.bak"
    if [[ -f "$project_root/go.sum" ]]; then
        cp "$project_root/go.sum" "$project_root/go.sum.bak"
    fi
    debug "Backed up go.mod and go.sum"
}

restore_go_mod() {
    local project_root=$1

    if [[ -f "$project_root/go.mod.bak" ]]; then
        mv "$project_root/go.mod.bak" "$project_root/go.mod"
        debug "Restored go.mod"
    fi
    if [[ -f "$project_root/go.sum.bak" ]]; then
        mv "$project_root/go.sum.bak" "$project_root/go.sum"
        debug "Restored go.sum"
    fi
}

cleanup_go_mod_backup() {
    local project_root=$1

    rm -f "$project_root/go.mod.bak" "$project_root/go.sum.bak"
    debug "Cleaned up go.mod backups"
}

update_go_mod_version() {
    local project_root=$1
    local module=$2
    local version=$3

    progress "Updating go.mod to ${module}@${version}..."

    cd "$project_root"

    if ! go mod edit -require="${module}@${version}"; then
        error "Failed to update go.mod"
        return 4
    fi

    progress "Running go mod tidy..."

    local tidy_output
    if ! tidy_output=$(GOWORK=off go mod tidy 2>&1); then
        error "go mod tidy failed"
        if echo "$tidy_output" | grep -q "package .* provided by .* but not at required version"; then
            error "Version mismatch: devnet-builder imports packages that don't exist in $version"
            error "This typically means the stable branch has different modules than expected."
            error "You may need to update devnet-builder code to match the target branch's API."
        fi
        if [[ "${VERBOSE:-false}" == "true" ]]; then
            echo "$tidy_output" >&2
        fi
        return 4
    fi

    success "go.mod updated successfully"
    return 0
}

get_current_stable_version() {
    local project_root=$1

    # Match the require line specifically (not module line or other stablelabs dependencies)
    grep -E "^\s+github\.com/stablelabs/stable\s+" "$project_root/go.mod" | awk '{print $2}' | head -1
}

# =============================================================================
# Build Functions (T019)
# =============================================================================

build_devnet_builder() {
    local project_root=$1
    local output_binary=$2

    progress "Building devnet-builder..."

    cd "$project_root"

    # Build to temporary location first
    local temp_binary
    temp_binary=$(mktemp)

    if ! GOWORK=off go build -o "$temp_binary" ./cmd/devnet-builder 2>&1; then
        error "Build failed"
        rm -f "$temp_binary"
        return 4
    fi

    # Verify binary works
    if ! "$temp_binary" --help &>/dev/null; then
        error "Binary verification failed"
        rm -f "$temp_binary"
        return 4
    fi

    # Move to final location
    mv "$temp_binary" "$output_binary"
    chmod +x "$output_binary"

    success "Build successful"
    return 0
}

# =============================================================================
# CLI Argument Parsing (T010)
# =============================================================================

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--network-version)
                NETWORK_VERSION="$2"
                shift 2
                ;;
            -s|--skip-cache)
                SKIP_CACHE=true
                shift
                ;;
            -c|--cache-dir)
                CACHE_DIR="$2"
                shift 2
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --list-cached)
                LIST_CACHED=true
                shift
                ;;
            --clean-cache)
                CLEAN_CACHE=true
                shift
                ;;
            -h|--help)
                SHOW_HELP=true
                shift
                ;;
            --)
                shift
                DEVNET_BUILDER_ARGS=("$@")
                break
                ;;
            *)
                error "Unknown option: $1"
                error "Use --help for usage information"
                exit 2
                ;;
        esac
    done
}

show_help() {
    cat << 'EOF'
build-for-version.sh - Build devnet-builder with a specific network version

Usage:
  ./scripts/build-for-version.sh [OPTIONS] -- [DEVNET_BUILDER_ARGS]

Options:
  -v, --network-version VERSION  Network repository version (tag, branch, or commit)
  -s, --skip-cache              Force rebuild even if cached binary exists
  -c, --cache-dir DIR           Cache directory location
      --verbose                 Enable verbose output
      --dry-run                 Show what would be done without executing
      --list-cached             List all cached versions
      --clean-cache             Remove all cached binaries
  -h, --help                    Show this help message

Environment Variables:
  NETWORK_VERSION            Target network version (overridden by CLI flag)
  DEVNET_BUILDER_CACHE       Cache directory (default: ~/.cache/devnet-builder)
  DEVNET_BUILDER_SKIP_CACHE  Skip cache lookup (default: false)
  DEVNET_BUILDER_VERBOSE     Enable verbose logging (default: false)

Examples:
  # Build with specific branch
  ./scripts/build-for-version.sh -v feat/usdt0-gas -- build genesis-export.json

  # Build with specific tag
  ./scripts/build-for-version.sh -v v1.2.3 -- build genesis-export.json

  # Build with commit hash
  ./scripts/build-for-version.sh -v abc123def -- build genesis-export.json

  # List cached versions
  ./scripts/build-for-version.sh --list-cached

  # Force rebuild
  ./scripts/build-for-version.sh -v feat/usdt0-gas --skip-cache -- build genesis-export.json

Exit Codes:
  0 - Success
  1 - General error
  2 - Invalid arguments
  3 - Version not found
  4 - Build failed
  5 - Go not installed
  6 - Network error
  7 - Lock acquisition timeout
EOF
}

# =============================================================================
# Cache Management Functions (T032-T033)
# =============================================================================

list_cached_versions() {
    local cache_dir="${CACHE_DIR:-$DEFAULT_CACHE_DIR}"
    local binaries_dir="$cache_dir/binaries"

    if [[ ! -d "$binaries_dir" ]]; then
        echo "No cached versions found."
        return 0
    fi

    echo "Cached versions:"
    echo ""

    local found=false
    for dir in "$binaries_dir"/*/; do
        if [[ -d "$dir" ]] && [[ -f "$dir/metadata.json" ]]; then
            found=true
            local cache_key
            cache_key=$(basename "$dir")
            local metadata="$dir/metadata.json"

            local version_req
            version_req=$(grep '"version_requested"' "$metadata" | sed 's/.*: *"\([^"]*\)".*/\1/')
            local built_at
            built_at=$(grep '"built_at"' "$metadata" | sed 's/.*: *"\([^"]*\)".*/\1/')
            local size_bytes
            size_bytes=$(grep '"size_bytes"' "$metadata" | sed 's/.*: *\([0-9]*\).*/\1/')
            local size_mb=$((size_bytes / 1048576))

            printf "  %-50s (%d MB, built %s)\n" "$cache_key" "$size_mb" "${built_at:0:10}"
        fi
    done

    if [[ "$found" == "false" ]]; then
        echo "No cached versions found."
    fi
}

clean_cache() {
    local cache_dir="${CACHE_DIR:-$DEFAULT_CACHE_DIR}"
    local binaries_dir="$cache_dir/binaries"

    if [[ ! -d "$binaries_dir" ]]; then
        echo "Cache is already empty."
        return 0
    fi

    progress "Removing all cached binaries..."
    rm -rf "$binaries_dir"
    success "Cache cleaned."
}

# =============================================================================
# Main Workflow Functions
# =============================================================================

resolve_version() {
    local repo_url="${STABLE_REPO_URL:-https://github.com/stablelabs/stable.git}"

    # Use default version if none specified
    if [[ -z "$NETWORK_VERSION" ]]; then
        NETWORK_VERSION=$(get_current_stable_version "$PROJECT_ROOT")
        progress "Using default version from go.mod: $NETWORK_VERSION"
    fi

    progress "Validating version '$NETWORK_VERSION'..."

    local commit_hash
    if ! commit_hash=$(resolve_version_to_commit "$repo_url" "$NETWORK_VERSION"); then
        error "Version '$NETWORK_VERSION' not found"
        echo ""

        # Try fuzzy matching
        local suggestions
        suggestions=$(fuzzy_match_refs "$repo_url" "$NETWORK_VERSION" 5)

        if [[ -n "$suggestions" ]]; then
            echo "Did you mean one of these?"
            echo "$suggestions" | while read -r ref; do
                echo "  - $ref"
            done
        else
            echo "Available tags:"
            get_available_refs "$repo_url" "tags" | head -5 | while read -r ref; do
                echo "  - $ref"
            done
        fi
        echo ""
        echo "Use --list-cached to see locally cached versions."
        exit 3
    fi

    RESOLVED_COMMIT="$commit_hash"
    success "Version '$NETWORK_VERSION' validated (commit: ${commit_hash:0:7})"
}

get_or_build_binary() {
    local cache_key
    cache_key=$(generate_cache_key "$STABLE_MODULE" "$NETWORK_VERSION")

    ensure_cache_dir "$CACHE_DIR"

    # Check cache (unless skip-cache is set)
    if [[ "$SKIP_CACHE" != "true" ]]; then
        progress "Checking cache..."
        if check_cache_exists "$CACHE_DIR" "$cache_key"; then
            local cached_binary
            cached_binary=$(get_cached_binary_path "$CACHE_DIR" "$cache_key")
            success "Cache hit: $cached_binary"
            echo "$cached_binary"
            return 0
        fi
        progress "Cache miss - building devnet-builder..."
    else
        progress "Skip cache enabled - building devnet-builder..."
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        progress "[DRY RUN] Would build devnet-builder with $STABLE_MODULE@$NETWORK_VERSION"
        progress "[DRY RUN] Would cache to: $CACHE_DIR/binaries/$cache_key/"
        echo ""
        return 0
    fi

    # Acquire build lock
    BUILD_LOCK_DIR="$CACHE_DIR/.build-lock"
    if ! acquire_lock "$BUILD_LOCK_DIR" "${DEVNET_BUILDER_TIMEOUT:-$DEFAULT_TIMEOUT}"; then
        exit 7
    fi

    # Setup cleanup trap using global variable
    trap 'restore_go_mod "$PROJECT_ROOT"; release_lock "$BUILD_LOCK_DIR"' EXIT

    # Backup go.mod
    backup_go_mod "$PROJECT_ROOT"

    # Determine version string for go mod
    local go_mod_version="$NETWORK_VERSION"
    local version_type
    version_type=$(detect_version_type "$NETWORK_VERSION")

    if [[ "$version_type" == "branch" ]]; then
        # For branches, resolve to pseudo-version using go list
        progress "Resolving branch to pseudo-version..."
        local resolved_version
        if ! resolved_version=$(GOWORK=off go list -m "${STABLE_MODULE}@${NETWORK_VERSION}" 2>&1 | awk '{print $2}'); then
            error "Failed to resolve branch version"
            restore_go_mod "$PROJECT_ROOT"
            release_lock "$BUILD_LOCK_DIR"
            exit 4
        fi
        go_mod_version="$resolved_version"
        debug "Resolved to: $go_mod_version"
    fi

    # Update go.mod
    if ! update_go_mod_version "$PROJECT_ROOT" "$STABLE_MODULE" "$go_mod_version"; then
        error "Failed to update go.mod"
        restore_go_mod "$PROJECT_ROOT"
        release_lock "$BUILD_LOCK_DIR"
        exit 4
    fi

    # Build binary
    local temp_binary
    temp_binary=$(mktemp)

    if ! build_devnet_builder "$PROJECT_ROOT" "$temp_binary"; then
        error "Build failed"
        restore_go_mod "$PROJECT_ROOT"
        release_lock "$BUILD_LOCK_DIR"
        exit 4
    fi

    # Store in cache
    store_in_cache "$CACHE_DIR" "$cache_key" "$temp_binary" "$NETWORK_VERSION" "${RESOLVED_COMMIT:-unknown}"
    rm -f "$temp_binary"

    # Restore go.mod (keep original state)
    restore_go_mod "$PROJECT_ROOT"

    # Release lock
    release_lock "$BUILD_LOCK_DIR"
    trap - EXIT

    # Return cached binary path
    get_cached_binary_path "$CACHE_DIR" "$cache_key"
}

execute_devnet_builder() {
    local binary_path=$1
    shift

    progress "Executing devnet-builder..."
    debug "Binary: $binary_path"
    debug "Args: $*"

    exec "$binary_path" "$@"
}
