// internal/daemon/config/config.go
package config

import (
	"os"
	"path/filepath"
	"time"
)

// Config is the single source of truth for devnetd configuration.
// Priority: defaults < config file < environment variables < CLI flags
type Config struct {
	Server   ServerConfig   `toml:"server"`
	Auth     AuthConfig     `toml:"auth"`
	Docker   DockerConfig   `toml:"docker"`
	GitHub   GitHubConfig   `toml:"github"`
	Timeouts TimeoutConfig  `toml:"timeouts"`
	Snapshot SnapshotConfig `toml:"snapshot"`
	Network  NetworkConfig  `toml:"network"`
}

// ServerConfig holds core server settings.
type ServerConfig struct {
	Socket     string `toml:"socket"`
	DataDir    string `toml:"data_dir"`
	LogLevel   string `toml:"log_level"`
	Workers    int    `toml:"workers"`
	Foreground bool   `toml:"foreground"`

	// Remote listener settings (optional - enables remote access)
	Listen  string `toml:"listen"`   // TCP address (e.g., "0.0.0.0:9000"), empty = local only
	TLSCert string `toml:"tls_cert"` // Path to TLS certificate file
	TLSKey  string `toml:"tls_key"`  // Path to TLS private key file
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Enabled  bool   `toml:"enabled"`   // Enable API key authentication for remote connections
	KeysFile string `toml:"keys_file"` // Path to API keys file
}

// DockerConfig holds Docker runtime settings.
type DockerConfig struct {
	Enabled bool   `toml:"enabled"`
	Image   string `toml:"image"`
}

// GitHubConfig holds GitHub API settings.
type GitHubConfig struct {
	Token string `toml:"token"`
}

// TimeoutConfig holds various timeout settings.
type TimeoutConfig struct {
	Shutdown         time.Duration `toml:"shutdown"`
	HealthCheck      time.Duration `toml:"health_check"`
	SnapshotDownload time.Duration `toml:"snapshot_download"`
}

// SnapshotConfig holds snapshot download settings.
type SnapshotConfig struct {
	CacheTTL   time.Duration `toml:"cache_ttl"`
	MaxRetries int           `toml:"max_retries"`
	RetryDelay time.Duration `toml:"retry_delay"`
}

// NetworkConfig holds network port settings.
type NetworkConfig struct {
	PortOffset   int `toml:"port_offset"`
	BaseRPCPort  int `toml:"base_rpc_port"`
	BaseP2PPort  int `toml:"base_p2p_port"`
	BaseRESTPort int `toml:"base_rest_port"`
	BaseGRPCPort int `toml:"base_grpc_port"`
}

// DefaultDataDir returns the default data directory path.
func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devnet-builder")
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *Config {
	dataDir := DefaultDataDir()
	return &Config{
		Server: ServerConfig{
			Socket:     filepath.Join(dataDir, "devnetd.sock"),
			DataDir:    dataDir,
			LogLevel:   "info",
			Workers:    2,
			Foreground: true,
		},
		Auth: AuthConfig{
			Enabled:  true, // Auth enabled by default when Listen is set
			KeysFile: filepath.Join(dataDir, "api-keys.yaml"),
		},
		Docker: DockerConfig{
			Enabled: false,
			Image:   "stablelabs/stabled:latest",
		},
		GitHub: GitHubConfig{
			Token: "",
		},
		Timeouts: TimeoutConfig{
			Shutdown:         30 * time.Second,
			HealthCheck:      5 * time.Second,
			SnapshotDownload: 30 * time.Minute,
		},
		Snapshot: SnapshotConfig{
			CacheTTL:   30 * time.Minute,
			MaxRetries: 3,
			RetryDelay: 5 * time.Second,
		},
		Network: NetworkConfig{
			PortOffset:   100,
			BaseRPCPort:  26657,
			BaseP2PPort:  26656,
			BaseRESTPort: 1317,
			BaseGRPCPort: 9090,
		},
	}
}
