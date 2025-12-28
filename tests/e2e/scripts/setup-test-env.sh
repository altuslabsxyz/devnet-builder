#!/usr/bin/env bash
# =============================================================================
# E2E Test Environment Setup Script
# =============================================================================
# This script sets up the environment for E2E tests, handling binary setup
# and GitHub authentication in a clean, predictable way.
# =============================================================================

export GIT_TERMINAL_PROMPT=0
export GIT_ASKPASS=""
export SSH_ASKPASS=""

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DEVNET_HOME="${DEVNET_HOME:-$HOME/.devnet-builder}"
BINARY_DIR="${DEVNET_HOME}/bin"
BINARY_PATH="${BINARY_DIR}/stabled"

# =============================================================================
# Helper Functions
# =============================================================================

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# =============================================================================
# Binary Setup
# =============================================================================

setup_binary() {
    info "Setting up blockchain binary for E2E tests..."

    # Check if binary already exists
    if [ -f "$BINARY_PATH" ]; then
        info "Binary already exists at: $BINARY_PATH"
        if ! $BINARY_PATH version &>/dev/null; then
            warning "Existing binary is not executable or invalid"
            rm -f "$BINARY_PATH"
        else
            success "Binary is valid and ready"
            return 0
        fi
    fi

    # Create binary directory
    mkdir -p "$BINARY_DIR"

    # Option 1: Use pre-built binary from environment variable
    if [ -n "$E2E_BINARY_SOURCE" ]; then
        info "Using binary from E2E_BINARY_SOURCE: $E2E_BINARY_SOURCE"
        if [ -f "$E2E_BINARY_SOURCE" ]; then
            cp "$E2E_BINARY_SOURCE" "$BINARY_PATH"
            chmod +x "$BINARY_PATH"
            success "Binary copied from $E2E_BINARY_SOURCE"
            return 0
        else
            error "Binary not found at: $E2E_BINARY_SOURCE"
            return 1
        fi
    fi

    # Option 2: Check if binary exists in $PATH
    if command -v stabled &>/dev/null; then
        info "Found stabled in PATH"
        cp "$(command -v stabled)" "$BINARY_PATH"
        chmod +x "$BINARY_PATH"
        success "Binary copied from PATH"
        return 0
    fi

    # Option 3: Build from source if stable-plugin exists
    PLUGIN_DIR="${DEVNET_HOME}/plugins/stable-plugin"
    if [ -d "$PLUGIN_DIR" ]; then
        info "Building binary from stable-plugin..."

        # Check for GitHub token if needed
        if [ -n "$GITHUB_TOKEN" ]; then
            info "GitHub token provided via GITHUB_TOKEN"
            export GIT_ASKPASS="$(mktemp)"
            cat > "$GIT_ASKPASS" <<EOF
#!/bin/sh
echo "\$GITHUB_TOKEN"
EOF
            chmod +x "$GIT_ASKPASS"
        fi

        (
            cd "$PLUGIN_DIR"
            if [ -f "Makefile" ]; then
                make install
            else
                go build -o "$BINARY_PATH" ./cmd/stabled 2>/dev/null || \
                go build -o "$BINARY_PATH" .
            fi
        )

        if [ -f "$BINARY_PATH" ]; then
            chmod +x "$BINARY_PATH"
            success "Binary built from source"
            return 0
        fi
    fi

    # Option 4: Skip binary-dependent tests
    warning "No blockchain binary available"
    warning "Binary-dependent tests will be skipped"
    warning ""
    warning "To enable all tests, provide binary via:"
    warning "  1. Set E2E_BINARY_SOURCE=/path/to/stabled"
    warning "  2. Install stabled to PATH"
    warning "  3. Build from stable-plugin with GITHUB_TOKEN"
    warning ""
    return 0
}

# =============================================================================
# GitHub Authentication Setup
# =============================================================================

setup_github_auth() {
    info "Checking GitHub authentication..."

    # Disable interactive prompts
    export GIT_TERMINAL_PROMPT=0

    # If GITHUB_TOKEN is set, configure git
    if [ -n "$GITHUB_TOKEN" ]; then
        info "Configuring GitHub authentication via GITHUB_TOKEN"

        # Use --replace-all to avoid "multiple values" error
        git config --global --replace-all url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/" 2>/dev/null || \
        git config --global --add url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

        # Also configure for Go private modules
        export GOPRIVATE="github.com/*"

        success "GitHub authentication configured"
        return 0
    fi

    # Check if git can already access github without prompting
    if git ls-remote https://github.com/stablelabs/stable-plugin.git HEAD &>/dev/null; then
        success "GitHub authentication already working"
        return 0
    fi

    warning "No GitHub authentication available"
    return 0
}

# =============================================================================
# Test Environment Validation
# =============================================================================

validate_environment() {
    info "Validating test environment..."

    # Check Go installation
    if ! command -v go &>/dev/null; then
        error "Go is not installed"
        return 1
    fi

    local go_version=$(go version | awk '{print $3}' | sed 's/go//')
    info "Go version: $go_version"

    # Check if devnet-builder binary is built
    if [ ! -f "./devnet-builder" ]; then
        warning "devnet-builder binary not found in current directory"
        info "Building devnet-builder..."
        go build -o devnet-builder ./cmd/devnet-builder
        success "devnet-builder built"
    else
        success "devnet-builder binary found"
    fi

    # Check if stable-plugin is available
    if [ -d "$HOME/.devnet-builder/plugins/stable-plugin" ]; then
        success "stable-plugin found"
    else
        warning "stable-plugin not found at: $HOME/.devnet-builder/plugins/stable-plugin"
    fi

    success "Environment validation complete"
}

# =============================================================================
# Main Execution
# =============================================================================

main() {
    info "==================================================================="
    info "E2E Test Environment Setup"
    info "==================================================================="
    echo ""

    # Validate basic environment
    validate_environment
    echo ""

    # Check GitHub authentication
    setup_github_auth
    echo ""

    # Setup blockchain binary
    setup_binary
    echo ""

    # Print environment summary
    info "==================================================================="
    info "Environment Summary"
    info "==================================================================="
    info "DEVNET_HOME:        $DEVNET_HOME"
    info "BINARY_PATH:        $BINARY_PATH"
    info "Git Credentials:    $([ -f "$HOME/.git-credentials" ] && echo "FOUND" || echo "NOT FOUND")"
    info "E2E_BINARY_SOURCE:  ${E2E_BINARY_SOURCE:-[NOT SET]}"

    if [ -f "$BINARY_PATH" ]; then
        info "Binary version:     $($BINARY_PATH version 2>&1 | head -1 || echo 'unknown')"
        success "Binary available: Full test suite will run"
    else
        warning "Binary unavailable: Deploy tests will be skipped"
    fi
    echo ""

    success "Environment setup complete!"
    echo ""
}

# Run main function
main "$@"
