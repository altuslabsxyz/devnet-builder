package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"

	"github.com/b-harvest/devnet-builder/internal"
	"github.com/b-harvest/devnet-builder/internal/di"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/version/migrations"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

func main() {
	// Enable color output
	color.NoColor = false

	// Load plugins from ~/.devnet-builder/plugins/
	loader := plugin.NewLoader()

	// Discover and load all plugins
	plugins, _ := loader.LoadAll()

	// Register loaded plugins with the network registry
	for _, p := range plugins {
		// Create an adapter to convert pkg/network.Module to internal/network.NetworkModule
		adapter := newPluginAdapter(p.Module())
		_ = network.MustRegister(adapter, false)
	}

	// Check and migrate version before executing commands
	homeDir := DefaultHomeDir()
	if err := checkAndMigrateVersion(homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "Version migration failed: %v\n", err)
		loader.Close()
		os.Exit(1)
	}

	// Initialize root command
	rootCmd := NewRootCmd()
	err := rootCmd.Execute()

	// Always close plugins before exit (os.Exit skips defers)
	loader.Close()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// checkAndMigrateVersion checks the current version and applies migrations if needed.
func checkAndMigrateVersion(homeDir string) error {
	// Use default logger for migration
	logger := output.DefaultLogger

	// Create infrastructure factory
	factory := di.NewInfrastructureFactory(homeDir, logger)

	// Create migration service
	migrationSvc := factory.CreateMigrationService()

	// Register all migrations
	migrationSvc.RegisterMigration(migrations.NewCacheKeyMigration())

	// Check and migrate to current version
	ctx := context.Background()
	_, err := migrationSvc.CheckAndMigrate(ctx, homeDir, internal.Version)
	if err != nil {
		return fmt.Errorf("failed to migrate to version %s: %w", internal.Version, err)
	}

	return nil
}
