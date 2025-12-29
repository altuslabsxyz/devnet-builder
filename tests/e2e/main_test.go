package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/b-harvest/devnet-builder/tests/e2e/helpers"
)

// Global test fixtures shared across all tests
var (
	// snapshotServer provides mock snapshot downloads
	snapshotServer *helpers.MockSnapshotServer

	// githubAPI provides mock GitHub API responses
	githubAPI *helpers.MockGitHubAPI

	// testdataDir points to the testdata directory with fixtures
	testdataDir string

	// sharedBlockchainBinary is the pre-built blockchain binary path
	// Built once in TestMain and reused across all tests
	sharedBlockchainBinary string
)

// TestMain sets up the test suite and runs all tests
func TestMain(m *testing.M) {
	// Setup phase
	code := 0
	defer func() {
		os.Exit(code)
	}()

	// Find testdata directory
	projectRoot := findProjectRoot()
	testdataDir = filepath.Join(projectRoot, "tests", "e2e", "testdata")

	// Verify testdata directory exists
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		panic("testdata directory not found: " + testdataDir)
	}

	// Verify devnet-builder binary exists or can be built
	binaryPath := filepath.Join(projectRoot, "devnet-builder")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Binary doesn't exist - this is expected, tests will fail with helpful message
		// We don't auto-build here to keep TestMain simple
		println("WARNING: devnet-builder binary not found at", binaryPath)
		println("Run 'go build' in project root before running E2E tests")
	}

	// Verify blockchain binary exists (built once, reused across all tests)
	// This avoids rebuilding from source for every test (60+ seconds each)
	userHome, err := os.UserHomeDir()
	if err != nil {
		println("WARNING: Cannot determine user home directory:", err.Error())
	} else {
		// For stable network, binary should be at ~/.devnet-builder/bin/stabled
		sharedBlockchainBinary = filepath.Join(userHome, ".devnet-builder", "bin", "stabled")
		if _, err := os.Stat(sharedBlockchainBinary); os.IsNotExist(err) {
			println("WARNING: Blockchain binary not found at", sharedBlockchainBinary)
			println("Deploy tests will build from source (slow). Pre-build binary for faster tests:")
			println("  devnet-builder deploy --mode local --validators 1")
			sharedBlockchainBinary = "" // Clear if not found
		} else {
			println("✓ Using pre-built blockchain binary:", sharedBlockchainBinary)
		}
	}

	// Run tests
	code = m.Run()

	// Teardown phase (if needed)
	// Note: Individual test cleanup is handled by t.Cleanup()
}

// findProjectRoot finds the project root directory by looking for go.mod
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("failed to get working directory: " + err.Error())
	}

	// Walk up directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}

// setupTest is a common test setup function
// Returns a fully configured test context with cleanup registered
func setupTest(t *testing.T) (*helpers.TestContext, *helpers.CommandRunner, *helpers.StateValidator, *helpers.CleanupManager) {
	t.Helper()

	// GITHUB_TOKEN is required for E2E tests
	skipIfGitHubTokenNotSet(t)

	// Create test context
	ctx := helpers.NewTestContext(t)

	// Create mock servers
	snapshotServer := helpers.NewMockSnapshotServer(t)
	githubAPI := helpers.NewMockGitHubAPI(t)

	// Configure environment to use mock servers and skip real downloads
	// Use GITHUB_TOKEN from environment (already validated above)
	githubToken := os.Getenv("GITHUB_TOKEN")
	ctx.WithEnv("GITHUB_TOKEN", githubToken)
	ctx.WithEnv("SNAPSHOT_URL", snapshotServer.URL())
	ctx.WithEnv("GITHUB_API_URL", githubAPI.URL())

	// Create default config file to avoid "missing required configuration" errors
	createDefaultConfig(t, ctx)

	// Copy pre-built blockchain binary to test environment (if available)
	// This avoids rebuilding from source for every test
	setupTestBinaries(t, ctx)

	// Create test helpers
	runner := helpers.NewCommandRunner(t, ctx)
	validator := helpers.NewStateValidator(t, ctx)
	cleanup := helpers.NewCleanupManager(t, ctx)

	return ctx, runner, validator, cleanup
}

// setupTestWithMocks sets up test with mock servers
func setupTestWithMocks(t *testing.T) (*helpers.TestContext, *helpers.CommandRunner, *helpers.StateValidator, *helpers.CleanupManager, *helpers.MockSnapshotServer, *helpers.MockGitHubAPI) {
	t.Helper()

	// Setup base test context
	ctx, runner, validator, cleanup := setupTest(t)

	// Create mock servers
	snapshotSrv := helpers.NewMockSnapshotServer(t)
	githubSrv := helpers.NewMockGitHubAPI(t)

	// Configure environment to use mock servers
	ctx.WithEnv("SNAPSHOT_URL", snapshotSrv.URL())
	ctx.WithEnv("GITHUB_API_URL", githubSrv.URL())

	return ctx, runner, validator, cleanup, snapshotSrv, githubSrv
}

// createDefaultConfig creates a minimal config file with required settings
func createDefaultConfig(t *testing.T, ctx *helpers.TestContext) {
	t.Helper()

	// Create .devnet-builder directory
	configDir := filepath.Join(ctx.HomeDir, ".devnet-builder")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config directory: %v", err)
	}

	// Copy stable plugin to test environment
	// The binary loads plugins from ~/.devnet-builder/plugins/
	setupTestPlugins(t, ctx)

	// Create minimal config file with required blockchain_network setting
	// Use GITHUB_TOKEN from environment (required for E2E tests)
	githubToken := os.Getenv("GITHUB_TOKEN")
	configContent := `# Auto-generated test configuration
blockchain_network = "stable"
github_token = "` + githubToken + `"
`

	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	ctx.WithConfig(configPath)
}

// setupTestPlugins copies required plugins to the test environment
func setupTestPlugins(t *testing.T, ctx *helpers.TestContext) {
	t.Helper()

	// Create plugins directory in test environment
	pluginsDir := filepath.Join(ctx.HomeDir, ".devnet-builder", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("failed to create plugins directory: %v", err)
	}

	// Copy stable plugin from user's home directory
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get user home directory: %v", err)
	}

	sourcePlugin := filepath.Join(userHome, ".devnet-builder", "plugins", "stable-plugin")
	destPlugin := filepath.Join(pluginsDir, "stable-plugin")

	// Check if source plugin exists
	if _, err := os.Stat(sourcePlugin); os.IsNotExist(err) {
		t.Logf("WARNING: stable-plugin not found at %s, tests may fail", sourcePlugin)
		return
	}

	// Copy plugin binary
	input, err := os.ReadFile(sourcePlugin)
	if err != nil {
		t.Fatalf("failed to read plugin: %v", err)
	}

	if err := os.WriteFile(destPlugin, input, 0755); err != nil {
		t.Fatalf("failed to write plugin: %v", err)
	}

	t.Logf("Copied stable-plugin to test environment: %s", destPlugin)
}

// setupTestBinaries copies pre-built blockchain binary to test environment
// This avoids rebuilding from source for every test (60+ seconds each)
func setupTestBinaries(t *testing.T, ctx *helpers.TestContext) {
	t.Helper()

	// Skip if no shared binary available
	if sharedBlockchainBinary == "" {
		t.Logf("No pre-built blockchain binary available, deploy will build from source")
		return
	}

	// Read blockchain binary once
	input, err := os.ReadFile(sharedBlockchainBinary)
	if err != nil {
		t.Logf("WARNING: failed to read blockchain binary: %v", err)
		return
	}

	binaryName := filepath.Base(sharedBlockchainBinary)

	// Copy to ~/.devnet-builder/bin/ (standard location)
	devnetBuilderBinDir := filepath.Join(ctx.HomeDir, ".devnet-builder", "bin")
	if err := os.MkdirAll(devnetBuilderBinDir, 0755); err != nil {
		t.Fatalf("failed to create .devnet-builder/bin directory: %v", err)
	}
	devnetBuilderBinary := filepath.Join(devnetBuilderBinDir, binaryName)
	if err := os.WriteFile(devnetBuilderBinary, input, 0755); err != nil {
		t.Fatalf("failed to write blockchain binary to .devnet-builder/bin: %v", err)
	}

	// Also copy to ~/bin/ (used by provision/key creation)
	homeBinDir := filepath.Join(ctx.HomeDir, "bin")
	if err := os.MkdirAll(homeBinDir, 0755); err != nil {
		t.Fatalf("failed to create bin directory: %v", err)
	}
	homeBinary := filepath.Join(homeBinDir, binaryName)
	if err := os.WriteFile(homeBinary, input, 0755); err != nil {
		t.Fatalf("failed to write blockchain binary to bin: %v", err)
	}

	t.Logf("Using pre-built blockchain binary: %s", sharedBlockchainBinary)
}

// getFixturePath returns the absolute path to a fixture file
func getFixturePath(subpath string) string {
	return filepath.Join(testdataDir, subpath)
}

// loadGenesisFixture returns the path to a genesis fixture file
func loadGenesisFixture(network string) string {
	filename := network + "-minimal.json"
	return getFixturePath(filepath.Join("genesis", filename))
}

// loadConfigFixture returns the path to a config fixture file
func loadConfigFixture(name string) string {
	filename := name + ".toml"
	return getFixturePath(filepath.Join("configs", filename))
}

// loadSnapshotFixture returns the path to a snapshot fixture file
func loadSnapshotFixture(network string) string {
	filename := network + "-snapshot.tar.gz"
	return getFixturePath(filepath.Join("snapshots", filename))
}

// loadGoldenFile returns the path to a golden file
func loadGoldenFile(name string) string {
	return getFixturePath(filepath.Join("golden", name))
}

// skipIfDockerNotAvailable skips the test if Docker is not available
func skipIfDockerNotAvailable(t *testing.T) {
	t.Helper()
	// Check if docker command is available
	if _, err := os.Stat("/usr/bin/docker"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/docker"); os.IsNotExist(err) {
			t.Skip("Docker not available, skipping test")
		}
	}
}

// skipIfBinaryNotBuilt skips the test if devnet-builder binary is not built
func skipIfBinaryNotBuilt(t *testing.T) {
	t.Helper()
	projectRoot := findProjectRoot()
	binaryPath := filepath.Join(projectRoot, "devnet-builder")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("devnet-builder binary not built. Run 'go build' first.")
	}
}

// requireDocker fails the test if Docker is not available
func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/usr/bin/docker"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/docker"); os.IsNotExist(err) {
			t.Fatal("Docker is required for this test but is not available")
		}
	}
}

// requireBinary fails the test if devnet-builder binary is not built
func requireBinary(t *testing.T) {
	t.Helper()
	projectRoot := findProjectRoot()
	binaryPath := filepath.Join(projectRoot, "devnet-builder")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatal("devnet-builder binary not built. Run 'go build' first.")
	}
}

// skipIfBlockchainBinaryNotAvailable skips the test if blockchain binary is not available
// Deploy tests require pre-built blockchain binaries at ~/.devnet-builder/bin/stabled
func skipIfBlockchainBinaryNotAvailable(t *testing.T) {
	t.Helper()

	// Check user's home directory for blockchain binary
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine user home directory")
		return
	}

	// For stable network, binary should be at ~/.devnet-builder/bin/stabled
	binaryPath := filepath.Join(userHome, ".devnet-builder", "bin", "stabled")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skip("Blockchain binary not found. Deploy tests require pre-built binary at ~/.devnet-builder/bin/stabled")
		return
	}

	t.Logf("Using blockchain binary: %s", binaryPath)
}

// skipIfGitHubTokenNotSet skips the test if GITHUB_TOKEN environment variable is not set
// E2E tests require a valid GitHub token for API access
func skipIfGitHubTokenNotSet(t *testing.T) {
	t.Helper()

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		t.Skip("GITHUB_TOKEN environment variable not set. E2E tests require a valid GitHub token.")
	}
}

// requireBlockchainBinary fails the test if blockchain binary is not available
func requireBlockchainBinary(t *testing.T) {
	t.Helper()

	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal("Cannot determine user home directory")
	}

	binaryPath := filepath.Join(userHome, ".devnet-builder", "bin", "stabled")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Blockchain binary not found. Deploy tests require pre-built binary at: %s\n"+
			"See tests/e2e/README.md for setup instructions.", binaryPath)
	}
}
