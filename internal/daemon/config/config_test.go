// internal/daemon/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Server defaults
	if cfg.Server.LogLevel != "info" {
		t.Errorf("expected log_level 'info', got %q", cfg.Server.LogLevel)
	}
	if cfg.Server.Workers != 2 {
		t.Errorf("expected workers 2, got %d", cfg.Server.Workers)
	}
	if cfg.Server.Foreground != true {
		t.Errorf("expected foreground true, got %v", cfg.Server.Foreground)
	}

	// Docker defaults
	if cfg.Docker.Enabled != false {
		t.Errorf("expected docker.enabled false, got %v", cfg.Docker.Enabled)
	}
	if cfg.Docker.Image != "stablelabs/stabled:latest" {
		t.Errorf("expected docker.image 'stablelabs/stabled:latest', got %q", cfg.Docker.Image)
	}

	// Timeout defaults
	if cfg.Timeouts.Shutdown != 30*time.Second {
		t.Errorf("expected shutdown timeout 30s, got %v", cfg.Timeouts.Shutdown)
	}
	if cfg.Timeouts.HealthCheck != 5*time.Second {
		t.Errorf("expected health_check timeout 5s, got %v", cfg.Timeouts.HealthCheck)
	}

	// Network defaults
	if cfg.Network.PortOffset != 100 {
		t.Errorf("expected port_offset 100, got %d", cfg.Network.PortOffset)
	}
}

func TestFileConfigIsEmpty(t *testing.T) {
	fc := &FileConfig{}
	if !fc.IsEmpty() {
		t.Error("expected empty FileConfig to return IsEmpty() == true")
	}

	// Set one field
	val := "test"
	fc.Server.LogLevel = &val
	if fc.IsEmpty() {
		t.Error("expected non-empty FileConfig to return IsEmpty() == false")
	}
}

func TestLoaderLoadFromFile(t *testing.T) {
	// Create temp directory with config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "devnetd.toml")
	configContent := `
[server]
log_level = "debug"
workers = 4

[docker]
enabled = true
image = "custom/image:v1"

[github]
token = "ghp_test123"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(tmpDir, configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Check values from file override defaults
	if cfg.Server.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug', got %q", cfg.Server.LogLevel)
	}
	if cfg.Server.Workers != 4 {
		t.Errorf("expected workers 4, got %d", cfg.Server.Workers)
	}
	if cfg.Docker.Enabled != true {
		t.Errorf("expected docker.enabled true, got %v", cfg.Docker.Enabled)
	}
	if cfg.Docker.Image != "custom/image:v1" {
		t.Errorf("expected docker.image 'custom/image:v1', got %q", cfg.Docker.Image)
	}
	if cfg.GitHub.Token != "ghp_test123" {
		t.Errorf("expected github.token 'ghp_test123', got %q", cfg.GitHub.Token)
	}

	// Check defaults are preserved for unset values
	if cfg.Server.Foreground != true {
		t.Errorf("expected foreground true (default), got %v", cfg.Server.Foreground)
	}
}

func TestLoaderEnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "devnetd.toml")
	configContent := `
[server]
log_level = "debug"

[github]
token = "file_token"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set env var
	t.Setenv("DEVNETD_GITHUB_TOKEN", "env_token")
	t.Setenv("DEVNETD_LOG_LEVEL", "warn")

	loader := NewLoader(tmpDir, configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Env should override file
	if cfg.GitHub.Token != "env_token" {
		t.Errorf("expected github.token 'env_token' from env, got %q", cfg.GitHub.Token)
	}
	if cfg.Server.LogLevel != "warn" {
		t.Errorf("expected log_level 'warn' from env, got %q", cfg.Server.LogLevel)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.Server.LogLevel = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid workers",
			modify: func(c *Config) {
				c.Server.Workers = 0
			},
			wantErr: true,
		},
		{
			name: "negative port offset",
			modify: func(c *Config) {
				c.Network.PortOffset = -1
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := Validate(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
