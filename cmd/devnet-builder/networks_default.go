//go:build !private

package main

// Default build: plugin-only mode (no built-in networks)
// Networks are loaded dynamically via the plugin system.
//
// To build with private networks (stable, ault), use:
//   make build-private
//
// Or build with the 'private' tag:
//   go build -tags private ./cmd/devnet-builder
