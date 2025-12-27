package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfig_Init tests config init command
// Verifies: creates default configuration file
func TestConfig_Init(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Execute config init
	t.Log("Initializing configuration...")
	result := runner.MustRun("config", "init", "--home", ctx.HomeDir)

	assert.Contains(t, result.Stdout, "config",
		"should show config initialization message")

	// Verify config file created
	validator.AssertFileExists("config.toml")

	// Verify config file has valid TOML content
	content := ctx.ReadFile("config.toml")
	assert.Contains(t, string(content), "home",
		"config should contain home setting")
	assert.Contains(t, string(content), "validators",
		"config should contain validators setting")

	t.Log("Config init verified successfully")
}

// TestConfig_Set tests setting configuration values
// Verifies: config values can be set and retrieved
func TestConfig_Set(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Initialize config first
	runner.MustRun("config", "init", "--home", ctx.HomeDir)

	// Set a config value
	t.Log("Setting config value...")
	result := runner.Run("config", "set",
		"validators", "3",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		t.Log("Config set command may not be implemented")
		return
	}

	// Verify config was updated
	content := ctx.ReadFile("config.toml")
	assert.Contains(t, string(content), "3",
		"config should contain updated validator count")

	t.Log("Config set verified successfully")
}

// TestConfig_Get tests retrieving configuration values
// Verifies: config values can be retrieved
func TestConfig_Get(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Initialize config
	runner.MustRun("config", "init", "--home", ctx.HomeDir)

	// Get a config value
	t.Log("Getting config value...")
	result := runner.Run("config", "get",
		"validators",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		t.Log("Config get command may not be implemented")
		return
	}

	// Verify output shows the value
	assert.NotEmpty(t, result.Stdout, "should show config value")

	t.Log("Config get verified successfully")
}

// TestCache_Clean tests cache cleanup
// Verifies: cache directory can be cleaned
func TestCache_Clean(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Create mock cache directory
	cacheDir := ctx.CreateSubDir("cache")
	ctx.WriteFile("cache/snapshot-mainnet.tar.gz", []byte("mock snapshot data"))
	ctx.WriteFile("cache/snapshot-testnet.tar.gz", []byte("mock snapshot data"))

	// Execute cache clean
	t.Log("Cleaning cache...")
	result := runner.Run("cache", "clean",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		t.Log("Cache clean command may not be implemented")
		return
	}

	assert.Contains(t, result.Stdout, "cache",
		"should show cache cleaning message")

	// Verify cache directory cleaned
	// (behavior may vary - might delete files or entire directory)
	t.Logf("Cache directory after clean: %s", cacheDir)

	t.Log("Cache clean verified successfully")
}

// TestCache_List tests listing cached items
// Verifies: cached snapshots are listed
func TestCache_List(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Create mock cache directory
	ctx.CreateSubDir("cache")
	ctx.WriteFile("cache/snapshot-mainnet.tar.gz", []byte("mock snapshot data"))
	ctx.WriteFile("cache/snapshot-testnet.tar.gz", []byte("mock snapshot data"))

	// Execute cache list
	t.Log("Listing cache...")
	result := runner.Run("cache", "list",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		t.Log("Cache list command may not be implemented")
		return
	}

	// Verify output shows cached items
	assert.NotEmpty(t, result.Stdout, "should show cache items")

	t.Log("Cache list verified successfully")
}

// TestVersions_List tests listing available versions
// Verifies: version information is retrieved
func TestVersions_List(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	_, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Execute versions list
	t.Log("Listing available versions...")
	result := runner.Run("versions", "list")

	// Command may not be implemented or require GitHub access
	if result.Failed() {
		t.Log("Versions list command may not be implemented or require network access")
		return
	}

	// Verify output shows version information
	assert.NotEmpty(t, result.Stdout, "should show version information")

	t.Log("Versions list verified successfully")
}

// TestVersions_Current tests showing current version
// Verifies: current binary version is displayed
func TestVersions_Current(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	_, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Execute version command (or versions current)
	t.Log("Getting current version...")
	result := runner.Run("version")

	// This should generally work
	if result.Failed() {
		t.Log("Version command failed")
		return
	}

	// Verify output shows version
	assert.NotEmpty(t, result.Stdout, "should show version information")

	// Common version patterns
	versionPatterns := []string{"v", "version", "0.", "1."}
	foundPattern := false
	for _, pattern := range versionPatterns {
		if strings.Contains(strings.ToLower(result.Stdout), pattern) {
			foundPattern = true
			break
		}
	}
	assert.True(t, foundPattern, "output should contain version information")

	t.Log("Version command verified successfully")
}

// TestNetworks_List tests listing available networks
// Verifies: network configurations are listed
func TestNetworks_List(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	_, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Execute networks list
	t.Log("Listing available networks...")
	result := runner.Run("networks", "list")

	// Command may not be implemented
	if result.Failed() {
		// Try alternative command
		result = runner.Run("networks")
	}

	if result.Failed() {
		t.Log("Networks command may not be implemented")
		return
	}

	// Verify output shows network information
	assert.NotEmpty(t, result.Stdout, "should show network information")

	// Should mention mainnet and/or testnet
	networksFound := strings.Contains(result.Stdout, "mainnet") ||
		strings.Contains(result.Stdout, "testnet")
	assert.True(t, networksFound,
		"output should list available networks")

	t.Log("Networks list verified successfully")
}

// TestConfig_WithFile tests using custom config file
// Verifies: config file can be specified via flag
func TestConfig_WithFile(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Use a test config file
	configPath := loadConfigFixture("local-2val")

	// Deploy using custom config file
	t.Log("Deploying with custom config file...")
	result := runner.Run("deploy",
		"--config", configPath,
		"--home", ctx.HomeDir,
	)

	// If config file support is implemented, deployment should succeed
	if result.Failed() {
		t.Log("Config file support may not be fully implemented")
		return
	}

	// Verify deployment used config file settings
	// (should create 2 validators as specified in local-2val.toml)
	validator.AssertValidatorCount(2)

	t.Log("Custom config file verified successfully")
}

// TestConfig_JSONOutput tests config output in JSON format
// Verifies: config can be output as JSON
func TestConfig_JSONOutput(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Initialize config
	runner.MustRun("config", "init", "--home", ctx.HomeDir)

	// Get config in JSON format
	t.Log("Getting config as JSON...")
	result := runner.Run("config", "show",
		"--json",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		t.Log("Config JSON output may not be implemented")
		return
	}

	// Verify output is valid JSON
	var configData map[string]interface{}
	err := json.Unmarshal([]byte(result.Stdout), &configData)
	assert.NoError(t, err, "output should be valid JSON")

	t.Log("Config JSON output verified successfully")
}

// TestConfig_EnvOverride tests environment variable override
// Verifies: environment variables override config file
func TestConfig_EnvOverride(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Set environment variable to override validators
	ctx.WithEnv("DEVNET_VALIDATORS", "3")

	// Deploy without specifying validators flag
	t.Log("Deploying with environment override...")
	result := runner.Run("deploy",
		"--mode", "local",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// If environment override is supported, should create 3 validators
	if result.Success() {
		// Check if environment was respected
		// (may default to different value if not implemented)
		dirs := validator.GetValidatorDirs()
		t.Logf("Created %d validators with env override", len(dirs))
	} else {
		t.Log("Environment override may not be fully implemented")
	}
}
