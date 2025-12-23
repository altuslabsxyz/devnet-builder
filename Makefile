#!/usr/bin/make -f

# =============================================================================
# devnet-builder Makefile
# =============================================================================
# This Makefile supports building devnet-builder in two modes:
# 1. Default: Plugin-only mode (no built-in networks)
# 2. Private: Includes private networks for development (stable, ault)
# =============================================================================

# Build configuration
BUILDDIR ?= $(CURDIR)/build
BINARY_NAME = devnet-builder

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# ldflags for version injection
LDFLAGS = -X main.Version=$(VERSION) \
          -X main.GitCommit=$(GIT_COMMIT) \
          -X main.BuildDate=$(BUILD_DATE)

# Go environment
export GOPRIVATE = github.com/stablelabs/*

# Default target
.DEFAULT_GOAL := build

# =============================================================================
# Directory Setup
# =============================================================================

$(BUILDDIR)/:
	@mkdir -p $(BUILDDIR)/

# =============================================================================
# Main Build Targets
# =============================================================================

# Build devnet-builder (plugin-only mode)
# No built-in networks; networks are loaded as plugins at runtime
build: $(BUILDDIR)/
	@echo "Building devnet-builder ..."
	@echo "  Version:    $(VERSION)"
	@echo "  Git commit: $(GIT_COMMIT)"
	@echo "  Build date: $(BUILD_DATE)"
	@echo "  Networks:   plugin-only"
	@go build -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(BINARY_NAME) ./cmd/devnet-builder
	@echo "Build successful: $(BUILDDIR)/$(BINARY_NAME)"

# Build with private networks (stable, ault) for development
# This uses the private/ directory which has private network implementations
build-private: $(BUILDDIR)/
	@echo "Building devnet-builder (with private networks)..."
	@echo "  Version:    $(VERSION)"
	@echo "  Git commit: $(GIT_COMMIT)"
	@echo "  Build date: $(BUILD_DATE)"
	@echo "  Networks:   stable, ault (private)"
	@echo ""
	@echo "Note: Requires access to private repositories (github.com/stablelabs/*)"
	@go build -tags "network_stable,network_ault" \
		-ldflags "$(LDFLAGS) -X main.BuildNetworks=stable,ault" \
		-o $(BUILDDIR)/$(BINARY_NAME) ./cmd/devnet-builder
	@echo "Build successful: $(BUILDDIR)/$(BINARY_NAME)"

# Build with stable network only
build-stable: $(BUILDDIR)/
	@echo "Building devnet-builder (stable only)..."
	@go build -tags "network_stable" \
		-ldflags "$(LDFLAGS) -X main.BuildNetworks=stable" \
		-o $(BUILDDIR)/$(BINARY_NAME) ./cmd/devnet-builder
	@echo "Build successful: $(BUILDDIR)/$(BINARY_NAME)"

# Build with ault network only
build-ault: $(BUILDDIR)/
	@echo "Building devnet-builder (ault only)..."
	@go build -tags "network_ault" \
		-ldflags "$(LDFLAGS) -X main.BuildNetworks=ault" \
		-o $(BUILDDIR)/$(BINARY_NAME) ./cmd/devnet-builder
	@echo "Build successful: $(BUILDDIR)/$(BINARY_NAME)"

# Install to GOPATH/bin (plugin-only mode)
install:
	@echo "Installing devnet-builder ..."
	@go install -ldflags "$(LDFLAGS)" ./cmd/devnet-builder
	@echo "Install complete"

# Install with private networks
install-private:
	@echo "Installing devnet-builder (with private networks)..."
	@go install -tags "network_stable,network_ault" \
		-ldflags "$(LDFLAGS) -X main.BuildNetworks=stable,ault" \
		./cmd/devnet-builder
	@echo "Install complete"

# =============================================================================
# Plugin Build Targets
# =============================================================================

PLUGIN_BUILD_DIR = $(BUILDDIR)/plugins

$(PLUGIN_BUILD_DIR)/:
	@mkdir -p $(PLUGIN_BUILD_DIR)

# Build all public plugins (from plugins/ directory)
plugins: $(PLUGIN_BUILD_DIR)/
	@echo "Building public network plugins..."
	@if [ -d "plugins" ]; then \
		for plugin_dir in plugins/*/; do \
			if [ -d "$$plugin_dir" ]; then \
				plugin_name=$$(basename "$$plugin_dir"); \
				echo "  Building $$plugin_name plugin..."; \
				go build -ldflags "$(LDFLAGS)" \
					-o $(PLUGIN_BUILD_DIR)/devnet-$$plugin_name \
					./plugins/$$plugin_name; \
			fi \
		done; \
	else \
		echo "  No plugins/ directory found"; \
	fi
	@echo "Plugin build complete"

# Build all private plugins (from private/plugins/ directory)
plugins-private: $(PLUGIN_BUILD_DIR)/
	@echo "Building private network plugins..."
	@if [ -d "private/plugins" ]; then \
		for plugin_dir in private/plugins/*/; do \
			if [ -d "$$plugin_dir" ]; then \
				plugin_name=$$(basename "$$plugin_dir"); \
				echo "  Building $$plugin_name plugin..."; \
				go build -ldflags "$(LDFLAGS)" \
					-o $(PLUGIN_BUILD_DIR)/devnet-$$plugin_name \
					./private/plugins/$$plugin_name; \
			fi \
		done; \
	else \
		echo "  No private/plugins/ directory found"; \
	fi
	@echo "Private plugin build complete"

# Build all plugins (public + private)
plugins-all: plugins plugins-private

# Build specific public plugin
# Usage: make plugin-<name> (e.g., make plugin-osmosis)
plugin-%: $(PLUGIN_BUILD_DIR)/
	@echo "Building $* plugin..."
	@if [ -d "plugins/$*" ]; then \
		go build -ldflags "$(LDFLAGS)" \
			-o $(PLUGIN_BUILD_DIR)/devnet-$* \
			./plugins/$*; \
	elif [ -d "private/plugins/$*" ]; then \
		go build -ldflags "$(LDFLAGS)" \
			-o $(PLUGIN_BUILD_DIR)/devnet-$* \
			./private/plugins/$*; \
	else \
		echo "Error: Plugin '$*' not found in plugins/ or private/plugins/"; \
		exit 1; \
	fi
	@echo "Plugin $* build complete: $(PLUGIN_BUILD_DIR)/devnet-$*"

# =============================================================================
# Test Targets
# =============================================================================

# Run all tests (public code only)
test:
	@echo "Running tests..."
	@go test -v ./cmd/... ./internal/... ./pkg/...

# Run all tests including private code
test-private:
	@echo "Running tests (including private)..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./cmd/... ./internal/... ./pkg/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# =============================================================================
# Protobuf Targets
# =============================================================================

# Public SDK proto (used by plugin developers)
PKG_PROTO_DIR = pkg/network/plugin
PKG_PROTO_OUT = pkg/network/plugin

# Generate Go code from protobuf definitions
proto-gen:
	@echo "Generating protobuf Go code..."
	@if ! command -v protoc &> /dev/null; then \
		echo "Error: protoc is not installed. Install with:"; \
		echo "  brew install protobuf"; \
		exit 1; \
	fi
	@if ! command -v protoc-gen-go &> /dev/null; then \
		echo "Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
	fi
	@if ! command -v protoc-gen-go-grpc &> /dev/null; then \
		echo "Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	fi
	@echo "  Generating public SDK proto..."
	@protoc \
		--go_out=$(PKG_PROTO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PKG_PROTO_OUT) \
		--go-grpc_opt=paths=source_relative \
		-I$(PKG_PROTO_DIR) \
		$(PKG_PROTO_DIR)/network.proto
	@echo "Protobuf generation complete"

# Clean generated protobuf files
proto-clean:
	@echo "Cleaning generated protobuf files..."
	@rm -f $(PKG_PROTO_OUT)/*.pb.go
	@echo "Protobuf cleanup complete"

# =============================================================================
# Utility Targets
# =============================================================================

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILDDIR)/
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  brew install golangci-lint"; \
	fi

# Verify dependencies
verify:
	@echo "Verifying dependencies..."
	@go mod verify
	@go mod tidy
	@echo "Dependencies verified"

# =============================================================================
# Development Helpers
# =============================================================================

# Build with specific version (for testing upgrades)
# Usage: make build-versioned VERSION=feat/usdt0-gas ARGS="build genesis-export.json"
build-versioned:
	@./scripts/build-for-version.sh -v $(VERSION) -- $(ARGS)

# List available plugins
list-plugins:
	@echo "Available plugins:"
	@echo ""
	@echo "Public plugins (plugins/):"
	@if [ -d "plugins" ]; then \
		for d in plugins/*/; do \
			if [ -d "$$d" ]; then \
				echo "  - $$(basename $$d)"; \
			fi \
		done; \
	else \
		echo "  (none)"; \
	fi
	@echo ""
	@echo "Private plugins (private/plugins/):"
	@if [ -d "private/plugins" ]; then \
		for d in private/plugins/*/; do \
			if [ -d "$$d" ]; then \
				echo "  - $$(basename $$d)"; \
			fi \
		done; \
	else \
		echo "  (none)"; \
	fi

# =============================================================================
# Help
# =============================================================================

help:
	@echo "devnet-builder Makefile"
	@echo ""
	@echo "Build Targets:"
	@echo "  build           - Build devnet-builder (plugin-only mode)"
	@echo "  build-private   - Build with private networks (stable, ault)"
	@echo "  build-stable    - Build with stable network only"
	@echo "  build-ault      - Build with ault network only"
	@echo "  install         - Install to GOPATH/bin "
	@echo "  install-private - Install with private networks"
	@echo ""
	@echo "Plugin Targets:"
	@echo "  plugins         - Build all public plugins (plugins/)"
	@echo "  plugins-private - Build all private plugins (private/plugins/)"
	@echo "  plugins-all     - Build all plugins (public + private)"
	@echo "  plugin-<name>   - Build specific plugin (e.g., plugin-osmosis)"
	@echo "  list-plugins    - List available plugins"
	@echo ""
	@echo "Test Targets:"
	@echo "  test            - Run public tests"
	@echo "  test-private    - Run all tests including private"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo ""
	@echo "Protobuf Targets:"
	@echo "  proto-gen       - Generate Go code from protobuf"
	@echo "  proto-clean     - Clean generated protobuf files"
	@echo ""
	@echo "Utility Targets:"
	@echo "  clean           - Remove build artifacts"
	@echo "  fmt             - Format code"
	@echo "  lint            - Run linter"
	@echo "  verify          - Verify and tidy dependencies"
	@echo ""
	@echo "Development:"
	@echo "  build-versioned - Build with specific version for testing"
	@echo ""
	@echo "Examples:"
	@echo "  make build                 # Build plugin-only mode"
	@echo "  make build-private         # Build with stable & ault"
	@echo "  make plugins               # Build all public plugins"
	@echo "  make plugin-osmosis        # Build specific plugin"
	@echo "  make test                  # Run tests"

.PHONY: build build-private build-stable build-ault install install-private \
        plugins plugins-private plugins-all \
        test test-private test-coverage \
        proto-gen proto-clean \
        clean fmt lint verify \
        build-versioned list-plugins help
