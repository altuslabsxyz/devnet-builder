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
# Try to get version from git tags, fallback to commit hash if no tags exist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# ldflags for version injection
# Inject to both internal and internal/version for backwards compatibility
LDFLAGS = -X github.com/altuslabsxyz/devnet-builder/internal.Version=$(VERSION) \
          -X github.com/altuslabsxyz/devnet-builder/internal.GitCommit=$(GIT_COMMIT) \
          -X github.com/altuslabsxyz/devnet-builder/internal.BuildDate=$(BUILD_DATE) \
          -X github.com/altuslabsxyz/devnet-builder/internal/version.Version=$(VERSION) \
          -X github.com/altuslabsxyz/devnet-builder/internal/version.GitCommit=$(GIT_COMMIT) \
          -X github.com/altuslabsxyz/devnet-builder/internal/version.BuildDate=$(BUILD_DATE)

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
	@go install -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(BINARY_NAME) ./cmd/devnetd
	@go install -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(BINARY_NAME) ./cmd/dvb
	@echo "Build successful: $(BUILDDIR)/$(BINARY_NAME)"


# Install to GOPATH/bin (plugin-only mode)
install:
	@echo "Installing devnet-builder ..."
	@go install -ldflags "$(LDFLAGS)" ./cmd/devnet-builder
	@go install -ldflags "$(LDFLAGS)" ./cmd/devnetd
	@go install -ldflags "$(LDFLAGS)" ./cmd/dvb
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

# E2E test environment setup
e2e-setup:
	@echo "Setting up E2E test environment..."
	@chmod +x tests/e2e/scripts/setup-test-env.sh
	@bash tests/e2e/scripts/setup-test-env.sh

# Run E2E tests
e2e-test: build
	@echo "==================================================================="
	@echo "Running E2E Test Suite"
	@echo "==================================================================="
	@echo ""
	@chmod +x tests/e2e/scripts/setup-test-env.sh
	@chmod +x tests/e2e/scripts/generate-test-report.sh
	@bash tests/e2e/scripts/setup-test-env.sh
	@echo ""
	@echo "Executing E2E tests (real-time output)..."
	@echo "Note: Tests run sequentially with verbose output"
	@echo ""
	@mkdir -p tests/e2e/results
	@bash -c ' \
		set -o pipefail; \
		if [ -f .e2e-test.env ]; then \
			echo "[INFO] Loading E2E test environment from .e2e-test.env"; \
			export $$(grep -v "^\#" .e2e-test.env | xargs); \
		fi; \
		echo "[INFO] Starting test execution with 30m timeout..."; \
		echo ""; \
		go test -v -timeout 30m -p 1 ./tests/e2e/... 2>&1 | tee tests/e2e/results/test-output.log; \
		exit_code=$$?; \
		echo ""; \
		echo "Generating test report..."; \
		bash tests/e2e/scripts/generate-test-report.sh tests/e2e/results/test-output.log tests/e2e/TEST_RESULTS.md; \
		exit $$exit_code; \
	'
	@echo ""
	@echo "==================================================================="
	@echo "E2E Test Suite Complete"
	@echo "==================================================================="
	@echo "Report: tests/e2e/TEST_RESULTS.md"
	@echo "Full log: tests/e2e/results/test-output.log"
	@rm -f .e2e-test.env .git-askpass.sh

# Run E2E tests with specific binary
e2e-test-with-binary: build
	@if [ -z "$(BINARY)" ]; then \
		echo "Error: BINARY path not specified"; \
		echo "Usage: make e2e-test-with-binary BINARY=/path/to/stabled"; \
		exit 1; \
	fi
	@echo "Running E2E tests with binary: $(BINARY)"
	@E2E_BINARY_SOURCE=$(BINARY) $(MAKE) e2e-test

# Run E2E tests (quick - skip deploy tests)
e2e-test-quick: build
	@echo "Running quick E2E tests (config, cache, error handling only)..."
	@go test -v -timeout 10m ./tests/e2e/config_test.go ./tests/e2e/errors_test.go -run "TestConfig|TestCache|TestVersions|TestNetworks|TestDeploy_DockerNotRunning|TestEdgeCase"

# Clean E2E test artifacts
e2e-clean:
	@echo "Cleaning E2E test artifacts..."
	@rm -rf tests/e2e/results/
	@rm -f tests/e2e/TEST_RESULTS.md
	@rm -f .e2e-test.env .git-askpass.sh
	@echo "E2E test artifacts cleaned"

# =============================================================================
# Daemon API Protobuf Targets (buf)
# =============================================================================

# Generate Go code from daemon API protos using buf
proto:
	@echo "Generating daemon API protobuf code..."
	@if ! command -v buf &> /dev/null; then \
		echo "Error: buf is not installed. Install with:"; \
		echo "  brew install bufbuild/buf/buf"; \
		exit 1; \
	fi
	@buf generate
	@echo "Daemon API protobuf generation complete"

# Lint daemon API protos for style violations
proto-lint:
	@echo "Linting daemon API protos..."
	@buf lint
	@echo "Proto lint complete"

# Check for breaking changes against main branch
proto-breaking:
	@echo "Checking for breaking changes..."
	@buf breaking --against '.git#branch=main'
	@echo "No breaking changes detected"

# Verify generated code is up-to-date (for CI)
proto-check: proto
	@if [ -n "$$(git status --porcelain api/proto/gen)" ]; then \
		echo "Error: Generated proto code is out of date. Run 'make proto' and commit."; \
		git diff api/proto/gen; \
		exit 1; \
	fi
	@echo "Generated proto code is up-to-date"

# Clean generated daemon API protobuf files
proto-api-clean:
	@echo "Cleaning generated daemon API protobuf files..."
	@rm -rf api/proto/gen
	@mkdir -p api/proto/gen
	@touch api/proto/gen/.gitkeep
	@echo "Daemon API protobuf cleanup complete"

# =============================================================================
# Plugin SDK Protobuf Targets (protoc)
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
	@echo "  test                 - Run public tests"
	@echo "  test-private         - Run all tests including private"
	@echo "  test-coverage        - Run tests with coverage report"
	@echo "  e2e-setup            - Setup E2E test environment"
	@echo "  e2e-test             - Run full E2E test suite"
	@echo "  e2e-test-quick       - Run quick E2E tests (skip deploy)"
	@echo "  e2e-test-with-binary - Run E2E with specific binary (BINARY=/path)"
	@echo "  e2e-clean            - Clean E2E test artifacts"
	@echo ""
	@echo "Daemon API Protobuf (buf):"
	@echo "  proto           - Generate Go code from daemon API protos"
	@echo "  proto-lint      - Lint daemon API protos for style"
	@echo "  proto-breaking  - Check for breaking changes vs main"
	@echo "  proto-check     - Verify generated code is up-to-date (CI)"
	@echo "  proto-api-clean - Clean generated daemon API protos"
	@echo ""
	@echo "Plugin SDK Protobuf (protoc):"
	@echo "  proto-gen       - Generate Go code from plugin SDK proto"
	@echo "  proto-clean     - Clean generated plugin SDK protos"
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
        e2e-setup e2e-test e2e-test-quick e2e-test-with-binary e2e-clean \
        proto proto-lint proto-breaking proto-check proto-api-clean \
        proto-gen proto-clean \
        clean fmt lint verify \
        build-versioned list-plugins help
