#!/usr/bin/make -f

# Build configuration
BUILDDIR ?= $(CURDIR)/build
BINARY_NAME = devnet-builder

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
	@GOWORK=off go build -o $(BUILDDIR)/$(BINARY_NAME) ./cmd/devnet-builder
	@echo "Build successful: $(BUILDDIR)/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILDDIR)/
	@echo "Clean complete"

# Install to GOPATH/bin
install:
	@echo "Installing devnet-builder..."
	@GOWORK=off go install ./cmd/devnet-builder
	@echo "Install complete"

# Run tests
test:
	@echo "Running tests..."
	@GOWORK=off go test -v ./...

# Display help
help:
	@echo "Available targets:"
	@echo "  build    - Build devnet-builder binary (default)"
	@echo "  clean    - Remove build artifacts"
	@echo "  install  - Install devnet-builder to GOPATH/bin"
	@echo "  test     - Run tests"
	@echo "  help     - Display this help message"

.PHONY: build clean install test help
