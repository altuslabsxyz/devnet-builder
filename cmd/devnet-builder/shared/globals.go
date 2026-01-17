// Package shared provides shared state and utilities for devnet-builder commands.
// This package is designed to be imported by all command subpackages without
// creating import cycles.
package shared

import (
	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/di"
)

// Global configuration variables - accessed via getter/setter functions
// to enable cross-package usage without circular imports.
var (
	homeDir          string
	jsonMode         bool
	noColor          bool
	verbose          bool
	configPath       string
	loadedFileConfig *config.FileConfig
	appContainer     *di.Container
)

// GetHomeDir returns the configured home directory.
func GetHomeDir() string {
	return homeDir
}

// SetHomeDir sets the configured home directory.
func SetHomeDir(dir string) {
	homeDir = dir
}

// GetJSONMode returns whether JSON output mode is enabled.
func GetJSONMode() bool {
	return jsonMode
}

// SetJSONMode enables or disables JSON output mode.
func SetJSONMode(mode bool) {
	jsonMode = mode
}

// GetNoColor returns whether color output is disabled.
func GetNoColor() bool {
	return noColor
}

// SetNoColor enables or disables color output.
func SetNoColor(mode bool) {
	noColor = mode
}

// GetVerbose returns whether verbose output is enabled.
func GetVerbose() bool {
	return verbose
}

// SetVerbose enables or disables verbose output.
func SetVerbose(mode bool) {
	verbose = mode
}

// GetConfigPath returns the configuration file path.
func GetConfigPath() string {
	return configPath
}

// SetConfigPath sets the configuration file path.
func SetConfigPath(path string) {
	configPath = path
}

// GetLoadedFileConfig returns the loaded file configuration.
func GetLoadedFileConfig() *config.FileConfig {
	return loadedFileConfig
}

// SetLoadedFileConfig sets the loaded file configuration.
func SetLoadedFileConfig(cfg *config.FileConfig) {
	loadedFileConfig = cfg
}

// GetAppContainer returns the dependency injection container.
func GetAppContainer() *di.Container {
	return appContainer
}

// SetAppContainer sets the dependency injection container.
func SetAppContainer(container *di.Container) {
	appContainer = container
}
