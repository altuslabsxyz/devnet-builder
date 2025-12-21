package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"

	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

func main() {
	// Enable color output
	color.NoColor = false

	// Load plugins from ~/.devnet-builder/plugins/
	loader := plugin.NewLoader()
	defer loader.Close()

	// Discover and load all plugins
	plugins, _ := loader.LoadAll()

	// Register loaded plugins with the network registry
	for _, p := range plugins {
		// Create an adapter to convert pkg/network.Module to internal/network.NetworkModule
		adapter := newPluginAdapter(p.Module())
		_ = network.MustRegister(adapter, false)
	}

	// Initialize root command
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
