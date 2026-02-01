// internal/daemon/config/loader.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ConfigFileName is the default config file name.
const ConfigFileName = "devnetd.toml"

// Environment variable names
const (
	EnvGitHubToken        = "DEVNETD_GITHUB_TOKEN" //nolint:gosec // This is an env var name, not a credential
	EnvLogLevel           = "DEVNETD_LOG_LEVEL"
	EnvSocket             = "DEVNETD_SOCKET"
	EnvDataDir            = "DEVNETD_DATA_DIR"
	EnvWorkers            = "DEVNETD_WORKERS"
	EnvForeground         = "DEVNETD_FOREGROUND"
	EnvDockerEnabled      = "DEVNETD_DOCKER_ENABLED"
	EnvDockerImage        = "DEVNETD_DOCKER_IMAGE"
	EnvShutdownTimeout    = "DEVNETD_SHUTDOWN_TIMEOUT"
	EnvHealthCheckTimeout = "DEVNETD_HEALTH_CHECK_TIMEOUT"

	// Remote listener environment variables
	EnvListen  = "DEVNETD_LISTEN"
	EnvTLSCert = "DEVNETD_TLS_CERT"
	EnvTLSKey  = "DEVNETD_TLS_KEY"

	// Authentication environment variables
	EnvAuthEnabled  = "DEVNETD_AUTH_ENABLED"
	EnvAuthKeysFile = "DEVNETD_AUTH_KEYS_FILE"
)

// Loader loads configuration from file, environment, and applies defaults.
type Loader struct {
	dataDir    string
	configPath string // explicit config path (empty = use default)
}

// NewLoader creates a new config loader.
// dataDir is the base data directory (for finding devnetd.toml).
// configPath is an explicit config file path (empty = use dataDir/devnetd.toml).
func NewLoader(dataDir, configPath string) *Loader {
	return &Loader{
		dataDir:    dataDir,
		configPath: configPath,
	}
}

// Load loads configuration with priority: defaults < file < env.
// Returns fully populated Config ready for use.
func (l *Loader) Load() (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Override dataDir if provided
	if l.dataDir != "" {
		cfg.Server.DataDir = l.dataDir
		cfg.Server.Socket = filepath.Join(l.dataDir, "devnetd.sock")
	}

	// Load from file
	fileCfg, err := l.loadFile()
	if err != nil {
		return nil, err
	}

	// Merge file config into defaults
	if fileCfg != nil {
		mergeFileConfig(cfg, fileCfg)
	}

	// Apply environment variables (highest priority before flags)
	applyEnvVars(cfg)

	return cfg, nil
}

// loadFile loads and parses the config file.
// Returns nil if no config file exists (not an error).
func (l *Loader) loadFile() (*FileConfig, error) {
	configPath := l.configPath
	if configPath == "" {
		configPath = filepath.Join(l.dataDir, ConfigFileName)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config file is OK
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var fileCfg FileConfig
	if err := toml.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("invalid TOML in %s: %w", configPath, err)
	}

	return &fileCfg, nil
}

// mergeFileConfig merges non-nil FileConfig values into Config.
func mergeFileConfig(cfg *Config, file *FileConfig) {
	// Server
	if file.Server.Socket != nil {
		cfg.Server.Socket = *file.Server.Socket
	}
	if file.Server.DataDir != nil {
		cfg.Server.DataDir = *file.Server.DataDir
	}
	if file.Server.LogLevel != nil {
		cfg.Server.LogLevel = *file.Server.LogLevel
	}
	if file.Server.Workers != nil {
		cfg.Server.Workers = *file.Server.Workers
	}
	if file.Server.Foreground != nil {
		cfg.Server.Foreground = *file.Server.Foreground
	}
	if file.Server.Listen != nil {
		cfg.Server.Listen = *file.Server.Listen
	}
	if file.Server.TLSCert != nil {
		cfg.Server.TLSCert = *file.Server.TLSCert
	}
	if file.Server.TLSKey != nil {
		cfg.Server.TLSKey = *file.Server.TLSKey
	}

	// Auth
	if file.Auth.Enabled != nil {
		cfg.Auth.Enabled = *file.Auth.Enabled
	}
	if file.Auth.KeysFile != nil {
		cfg.Auth.KeysFile = *file.Auth.KeysFile
	}

	// Docker
	if file.Docker.Enabled != nil {
		cfg.Docker.Enabled = *file.Docker.Enabled
	}
	if file.Docker.Image != nil {
		cfg.Docker.Image = *file.Docker.Image
	}

	// GitHub
	if file.GitHub.Token != nil {
		cfg.GitHub.Token = *file.GitHub.Token
	}

	// Timeouts (parse duration strings)
	if file.Timeouts.Shutdown != nil {
		if d, err := time.ParseDuration(*file.Timeouts.Shutdown); err == nil {
			cfg.Timeouts.Shutdown = d
		}
	}
	if file.Timeouts.HealthCheck != nil {
		if d, err := time.ParseDuration(*file.Timeouts.HealthCheck); err == nil {
			cfg.Timeouts.HealthCheck = d
		}
	}
	if file.Timeouts.SnapshotDownload != nil {
		if d, err := time.ParseDuration(*file.Timeouts.SnapshotDownload); err == nil {
			cfg.Timeouts.SnapshotDownload = d
		}
	}

	// Snapshot (parse duration strings)
	if file.Snapshot.CacheTTL != nil {
		if d, err := time.ParseDuration(*file.Snapshot.CacheTTL); err == nil {
			cfg.Snapshot.CacheTTL = d
		}
	}
	if file.Snapshot.MaxRetries != nil {
		cfg.Snapshot.MaxRetries = *file.Snapshot.MaxRetries
	}
	if file.Snapshot.RetryDelay != nil {
		if d, err := time.ParseDuration(*file.Snapshot.RetryDelay); err == nil {
			cfg.Snapshot.RetryDelay = d
		}
	}

	// Network
	if file.Network.PortOffset != nil {
		cfg.Network.PortOffset = *file.Network.PortOffset
	}
	if file.Network.BaseRPCPort != nil {
		cfg.Network.BaseRPCPort = *file.Network.BaseRPCPort
	}
	if file.Network.BaseP2PPort != nil {
		cfg.Network.BaseP2PPort = *file.Network.BaseP2PPort
	}
	if file.Network.BaseRESTPort != nil {
		cfg.Network.BaseRESTPort = *file.Network.BaseRESTPort
	}
	if file.Network.BaseGRPCPort != nil {
		cfg.Network.BaseGRPCPort = *file.Network.BaseGRPCPort
	}
}

// applyEnvVars applies environment variable overrides to config.
func applyEnvVars(cfg *Config) {
	if v := os.Getenv(EnvGitHubToken); v != "" {
		cfg.GitHub.Token = v
	}
	if v := os.Getenv(EnvLogLevel); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv(EnvSocket); v != "" {
		cfg.Server.Socket = v
	}
	if v := os.Getenv(EnvDataDir); v != "" {
		cfg.Server.DataDir = v
	}
	if v := os.Getenv(EnvWorkers); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Server.Workers = i
		}
	}
	if v := os.Getenv(EnvForeground); v != "" {
		cfg.Server.Foreground = v == "true" || v == "1"
	}
	if v := os.Getenv(EnvDockerEnabled); v != "" {
		cfg.Docker.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv(EnvDockerImage); v != "" {
		cfg.Docker.Image = v
	}
	if v := os.Getenv(EnvShutdownTimeout); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeouts.Shutdown = d
		}
	}
	if v := os.Getenv(EnvHealthCheckTimeout); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeouts.HealthCheck = d
		}
	}

	// Remote listener
	if v := os.Getenv(EnvListen); v != "" {
		cfg.Server.Listen = v
	}
	if v := os.Getenv(EnvTLSCert); v != "" {
		cfg.Server.TLSCert = v
	}
	if v := os.Getenv(EnvTLSKey); v != "" {
		cfg.Server.TLSKey = v
	}

	// Authentication
	if v := os.Getenv(EnvAuthEnabled); v != "" {
		cfg.Auth.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv(EnvAuthKeysFile); v != "" {
		cfg.Auth.KeysFile = v
	}
}
