// internal/daemon/config/file.go
package config

// FileConfig represents the raw devnetd.toml file contents.
// All fields are pointers to distinguish "not set" from "set to zero/false".
type FileConfig struct {
	Server   FileServerConfig   `toml:"server"`
	Auth     FileAuthConfig     `toml:"auth"`
	Docker   FileDockerConfig   `toml:"docker"`
	GitHub   FileGitHubConfig   `toml:"github"`
	Timeouts FileTimeoutConfig  `toml:"timeouts"`
	Snapshot FileSnapshotConfig `toml:"snapshot"`
	Network  FileNetworkConfig  `toml:"network"`
}

// FileServerConfig is the TOML representation of ServerConfig.
type FileServerConfig struct {
	Socket     *string `toml:"socket"`
	DataDir    *string `toml:"data_dir"`
	LogLevel   *string `toml:"log_level"`
	Workers    *int    `toml:"workers"`
	Foreground *bool   `toml:"foreground"`

	// Remote listener settings
	Listen  *string `toml:"listen"`
	TLSCert *string `toml:"tls_cert"`
	TLSKey  *string `toml:"tls_key"`
}

// FileAuthConfig is the TOML representation of AuthConfig.
type FileAuthConfig struct {
	Enabled  *bool   `toml:"enabled"`
	KeysFile *string `toml:"keys_file"`
}

// FileDockerConfig is the TOML representation of DockerConfig.
type FileDockerConfig struct {
	Enabled *bool   `toml:"enabled"`
	Image   *string `toml:"image"`
}

// FileGitHubConfig is the TOML representation of GitHubConfig.
type FileGitHubConfig struct {
	Token *string `toml:"token"`
}

// FileTimeoutConfig is the TOML representation of TimeoutConfig.
// Uses strings for duration values since TOML cannot decode directly to time.Duration.
type FileTimeoutConfig struct {
	Shutdown         *string `toml:"shutdown"`
	HealthCheck      *string `toml:"health_check"`
	SnapshotDownload *string `toml:"snapshot_download"`
}

// FileSnapshotConfig is the TOML representation of SnapshotConfig.
// Uses strings for duration values since TOML cannot decode directly to time.Duration.
type FileSnapshotConfig struct {
	CacheTTL   *string `toml:"cache_ttl"`
	MaxRetries *int    `toml:"max_retries"`
	RetryDelay *string `toml:"retry_delay"`
}

// FileNetworkConfig is the TOML representation of NetworkConfig.
type FileNetworkConfig struct {
	PortOffset   *int `toml:"port_offset"`
	BaseRPCPort  *int `toml:"base_rpc_port"`
	BaseP2PPort  *int `toml:"base_p2p_port"`
	BaseRESTPort *int `toml:"base_rest_port"`
	BaseGRPCPort *int `toml:"base_grpc_port"`
}

// IsEmpty returns true if no configuration values are set.
func (f *FileConfig) IsEmpty() bool {
	return f.Server.Socket == nil &&
		f.Server.DataDir == nil &&
		f.Server.LogLevel == nil &&
		f.Server.Workers == nil &&
		f.Server.Foreground == nil &&
		f.Auth.Enabled == nil &&
		f.Auth.KeysFile == nil &&
		f.Docker.Enabled == nil &&
		f.Docker.Image == nil &&
		f.GitHub.Token == nil &&
		f.Timeouts.Shutdown == nil &&
		f.Timeouts.HealthCheck == nil &&
		f.Timeouts.SnapshotDownload == nil &&
		f.Snapshot.CacheTTL == nil &&
		f.Snapshot.MaxRetries == nil &&
		f.Snapshot.RetryDelay == nil &&
		f.Network.PortOffset == nil &&
		f.Network.BaseRPCPort == nil &&
		f.Network.BaseP2PPort == nil &&
		f.Network.BaseRESTPort == nil &&
		f.Network.BaseGRPCPort == nil
}
