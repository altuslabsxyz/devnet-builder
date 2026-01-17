#!/usr/bin/env bash
#
# build-for-version.sh - Build devnet-builder with a specific network version
#
# This wrapper script enables dynamic network repository version/branch selection
# for devnet-builder CLI to support testing against different network branches.
#
# Usage:
#   ./scripts/build-for-version.sh [OPTIONS] -- [DEVNET_BUILDER_ARGS]
#
# Options:
#   -v, --network-version VERSION  Network repository version (tag, branch, or commit)
#   -s, --skip-cache              Force rebuild even if cached binary exists
#   -c, --cache-dir DIR           Cache directory location (default: ~/.cache/devnet-builder)
#       --verbose                 Enable verbose output
#       --dry-run                 Show what would be done without executing
#       --list-cached             List all cached versions
#       --clean-cache             Remove all cached binaries
#   -h, --help                    Show this help message
#
# Environment Variables:
#   NETWORK_VERSION          Target network version (overridden by CLI flag)
#   DEVNET_BUILDER_CACHE     Cache directory (default: ~/.cache/devnet-builder)
#   DEVNET_BUILDER_SKIP_CACHE  Skip cache lookup (default: false)
#   DEVNET_BUILDER_VERBOSE   Enable verbose logging (default: false)
#   DEVNET_BUILDER_TIMEOUT   Build timeout in seconds (default: 300)
#
# Exit Codes:
#   0 - Success
#   1 - General error
#   2 - Invalid arguments
#   3 - Version not found
#   4 - Build failed
#   5 - Go not installed
#   6 - Network error
#   7 - Lock acquisition timeout
#

set -euo pipefail

# Script directory and project root
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Source utility functions
# shellcheck source=lib/version-utils.sh
source "$SCRIPT_DIR/lib/version-utils.sh"

# Constants
readonly STABLE_MODULE="github.com/stablelabs/stable"
readonly STABLE_REPO_URL="https://github.com/stablelabs/stable.git"
readonly DEFAULT_CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/devnet-builder"
readonly DEFAULT_TIMEOUT=300

# Global variables (set by parse_args or environment)
NETWORK_VERSION="${NETWORK_VERSION:-}"
CACHE_DIR="${DEVNET_BUILDER_CACHE:-$DEFAULT_CACHE_DIR}"
SKIP_CACHE="${DEVNET_BUILDER_SKIP_CACHE:-false}"
VERBOSE="${DEVNET_BUILDER_VERBOSE:-false}"
DRY_RUN=false
LIST_CACHED=false
CLEAN_CACHE=false
SHOW_HELP=false
DEVNET_BUILDER_ARGS=()

# Main entry point
main() {
    parse_args "$@"

    if [[ "$SHOW_HELP" == "true" ]]; then
        show_help
        exit 0
    fi

    if [[ "$LIST_CACHED" == "true" ]]; then
        list_cached_versions
        exit 0
    fi

    if [[ "$CLEAN_CACHE" == "true" ]]; then
        clean_cache
        exit 0
    fi

    # Check prerequisites
    check_prerequisites

    # Resolve version
    resolve_version

    # Get or build binary
    local binary_path
    binary_path=$(get_or_build_binary)

    # In dry-run mode, binary_path is empty
    if [[ -z "$binary_path" ]]; then
        exit 0
    fi

    # Execute devnet-builder with passthrough args
    if [[ ${#DEVNET_BUILDER_ARGS[@]} -gt 0 ]]; then
        execute_devnet_builder "$binary_path" "${DEVNET_BUILDER_ARGS[@]}"
    else
        success "Binary ready at: $binary_path"
        progress "No devnet-builder arguments provided. Use -- to pass arguments."
    fi
}

# Run main
main "$@"
