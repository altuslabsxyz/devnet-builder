package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_MissingFile(t *testing.T) {
	// Use a temp directory that doesn't exist
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Server)
	assert.Empty(t, cfg.APIKey)
	assert.Empty(t, cfg.Namespace)
}

func TestLoadConfig_ValidFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".dvb")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configContent := `server: "devnetd.example.com:9000"
api_key: "devnet_abc123"
namespace: "team-a"
`
	err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "devnetd.example.com:9000", cfg.Server)
	assert.Equal(t, "devnet_abc123", cfg.APIKey)
	assert.Equal(t, "team-a", cfg.Namespace)
}

func TestClientConfig_Save(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	cfg := &ClientConfig{
		Server:    "devnetd.example.com:9000",
		APIKey:    "devnet_xyz789",
		Namespace: "team-b",
	}

	err := cfg.Save()
	require.NoError(t, err)

	// Verify file was created
	configPath := filepath.Join(tmpDir, ".dvb", "config.yaml")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Verify file has correct permissions (0600)
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Load and verify
	loaded, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg.Server, loaded.Server)
	assert.Equal(t, cfg.APIKey, loaded.APIKey)
	assert.Equal(t, cfg.Namespace, loaded.Namespace)
}

func TestClientConfig_Get(t *testing.T) {
	cfg := &ClientConfig{
		Server:    "server.example.com:9000",
		APIKey:    "devnet_key123",
		Namespace: "my-namespace",
	}

	assert.Equal(t, "server.example.com:9000", cfg.Get("server"))
	assert.Equal(t, "devnet_key123", cfg.Get("api-key"))
	assert.Equal(t, "my-namespace", cfg.Get("namespace"))
	assert.Empty(t, cfg.Get("unknown"))
}

func TestClientConfig_Set(t *testing.T) {
	cfg := &ClientConfig{}

	err := cfg.Set("server", "new.server.com:9000")
	require.NoError(t, err)
	assert.Equal(t, "new.server.com:9000", cfg.Server)

	err = cfg.Set("api-key", "devnet_newkey")
	require.NoError(t, err)
	assert.Equal(t, "devnet_newkey", cfg.APIKey)

	err = cfg.Set("namespace", "new-namespace")
	require.NoError(t, err)
	assert.Equal(t, "new-namespace", cfg.Namespace)

	// Test unknown key
	err = cfg.Set("unknown", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config key")
}

func TestClientConfig_IsRemote(t *testing.T) {
	cfg := &ClientConfig{}
	assert.False(t, cfg.IsRemote())

	cfg.Server = "server.example.com:9000"
	assert.True(t, cfg.IsRemote())
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create config directory and file with invalid YAML
	configDir := filepath.Join(tmpDir, ".dvb")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	invalidYAML := `server: [invalid yaml`
	err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(invalidYAML), 0600)
	require.NoError(t, err)

	_, err = LoadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestClientConfig_PartialConfig(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create config with only some fields
	configDir := filepath.Join(tmpDir, ".dvb")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configContent := `server: "devnetd.example.com:9000"
`
	err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "devnetd.example.com:9000", cfg.Server)
	assert.Empty(t, cfg.APIKey)
	assert.Empty(t, cfg.Namespace)
}

// TestCheckConfigFilePermissions_SecurePermissions verifies no warning for 0600.
func TestCheckConfigFilePermissions_SecurePermissions(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".dvb")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, "config.yaml")
	err = os.WriteFile(configPath, []byte("api_key: secret"), 0600)
	require.NoError(t, err)

	warning := CheckConfigFilePermissions()
	assert.Empty(t, warning, "secure permissions should not produce a warning")
}

// TestCheckConfigFilePermissions_InsecurePermissions verifies warning for world-readable file.
func TestCheckConfigFilePermissions_InsecurePermissions(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".dvb")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, "config.yaml")
	// Write with insecure permissions (world-readable)
	err = os.WriteFile(configPath, []byte("api_key: secret"), 0644)
	require.NoError(t, err)

	warning := CheckConfigFilePermissions()
	assert.NotEmpty(t, warning, "insecure permissions should produce a warning")
	assert.Contains(t, warning, "insecure permissions")
	assert.Contains(t, warning, "644") // should mention the actual permissions
}

// TestCheckConfigFilePermissions_MissingFile verifies no warning when file doesn't exist.
func TestCheckConfigFilePermissions_MissingFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	warning := CheckConfigFilePermissions()
	assert.Empty(t, warning, "missing file should not produce a warning")
}
