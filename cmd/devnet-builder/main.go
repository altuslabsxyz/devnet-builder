package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/commands"
	"github.com/altuslabsxyz/devnet-builder/internal"
	"github.com/altuslabsxyz/devnet-builder/internal/di"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/version/migrations"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

// Global plugin loader - shared across the application
var globalLoader *plugin.Loader

func main() {
	// Enable color output
	color.NoColor = false

	// Load plugins from ~/.devnet-builder/plugins/
	globalLoader = plugin.NewLoader()

	// Load all discovered plugins with detailed error information
	loadResult, loadErr := globalLoader.LoadAllWithErrors()
	if loadErr != nil {
		output.DefaultLogger.Debug("Plugin loading error: %v", loadErr)
	}

	// Log which plugins were successfully loaded
	if loadResult != nil {
		loadedNames := make([]string, 0, len(loadResult.Loaded))
		for _, p := range loadResult.Loaded {
			loadedNames = append(loadedNames, p.Name())
		}
		output.DefaultLogger.Debug("Successfully loaded %d plugins: %v", len(loadResult.Loaded), loadedNames)

		// Log detailed errors for plugins that failed to load
		for _, loadErr := range loadResult.Errors {
			output.DefaultLogger.Warn("Failed to load plugin %q: %v", loadErr.PluginName, loadErr.Err)
		}
	}

	// Extract plugins from result for registration
	var plugins []*plugin.PluginClient
	if loadResult != nil {
		plugins = loadResult.Loaded
	}

	// Register loaded plugins with the network registry
	for _, p := range plugins {
		// Create an adapter to convert pkg/network.Module to internal/network.NetworkModule
		adapter := newPluginAdapter(p.Module())
		if err := network.MustRegister(adapter, false); err != nil {
			output.DefaultLogger.Warn("Failed to register plugin %q: %v", p.Name(), err)
		}
	}

	// Check and migrate version before executing commands
	homeDir := commands.DefaultHomeDir()
	if err := checkAndMigrateVersion(homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "Version migration failed: %v\n", err)
		globalLoader.Close()
		os.Exit(1)
	}

	// Initialize root command from commands package
	rootCmd := commands.NewRootCmd()

	// Enhance root command with binary passthrough commands
	// This is done after root command initialization but before execution
	// We pass a nil container here since the container will be initialized
	// lazily when commands are executed
	if err := enhanceRootWithBinaryPassthrough(rootCmd, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup binary passthrough: %v\n", err)
		os.Exit(1)
	}

	err := rootCmd.Execute()

	// Always close plugins before exit (os.Exit skips defers)
	globalLoader.Close()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// GetPluginLoader returns the global plugin loader.
// This is used by commands to access the plugin system.
func GetPluginLoader() *plugin.Loader {
	return globalLoader
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
	migrationSvc.RegisterMigration(migrations.NewNoOpMigration())
	migrationSvc.RegisterMigration(migrations.NewV001ToV100Migration())
	migrationSvc.RegisterMigration(migrations.NewV010ToV100Migration())
	migrationSvc.RegisterMigration(migrations.NewV010DevToV100Migration())

	// Check and migrate to current version
	ctx := context.Background()
	_, err := migrationSvc.CheckAndMigrate(ctx, homeDir, internal.Version)
	if err != nil {
		return fmt.Errorf("failed to migrate to version %s: %w", internal.Version, err)
	}

	return nil
}
