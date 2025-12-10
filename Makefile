#!/usr/bin/make -f

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
export GOSUMDB = off

# Default target
.DEFAULT_GOAL := build

# Create build directory
$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

# Build devnet-builder
build: $(BUILDDIR)/
	@echo "Building devnet-builder..."
	@echo "  Version:    $(VERSION)"
	@echo "  Git commit: $(GIT_COMMIT)"
	@echo "  Build date: $(BUILD_DATE)"
	@GOWORK=off go build -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(BINARY_NAME) ./cmd/devnet-builder
	@echo "Build successful: $(BUILDDIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILDDIR)/
	@echo "Clean complete"

# Install to GOPATH/bin
install:
	@echo "Installing devnet-builder..."
	@echo "  Version:    $(VERSION)"
	@echo "  Git commit: $(GIT_COMMIT)"
	@echo "  Build date: $(BUILD_DATE)"
	@GOWORK=off go install -ldflags "$(LDFLAGS)" ./cmd/devnet-builder
	@echo "Install complete"

# Run tests
test:
	@echo "Running tests..."
	@GOWORK=off go test -v ./...

# Build with specific stable version
# Usage: make build-versioned VERSION=feat/usdt0-gas ARGS="build genesis-export.json"
build-versioned:
	@./scripts/build-for-version.sh -v $(VERSION) -- $(ARGS)

# Display help
help:
	@echo "Available targets:"
	@echo "  build           - Build devnet-builder binary (default)"
	@echo "  build-versioned - Build with specific stable version (VERSION=x ARGS=y)"
	@echo "  clean           - Remove build artifacts"
	@echo "  install         - Install devnet-builder to GOPATH/bin"
	@echo "  test            - Run tests"
	@echo "  help            - Display this help message"

.PHONY: build build-versioned clean install test help
