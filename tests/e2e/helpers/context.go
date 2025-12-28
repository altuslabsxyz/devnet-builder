package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestContext provides an isolated test environment with automatic cleanup
type TestContext struct {
	t          *testing.T
	HomeDir    string            // Isolated home directory for this test
	BinaryPath string            // Path to devnet-builder binary
	ConfigPath string            // Path to test config file (optional)
	Env        map[string]string // Environment variables for test
	cleanups   []func()          // Cleanup functions to run on test completion
}

// NewTestContext creates a new isolated test environment
// - Creates a temporary home directory (automatically cleaned up via t.TempDir())
// - Sets up environment variables
// - Registers cleanup handlers
func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	// Create isolated temporary directory
	homeDir := t.TempDir()

	// Find devnet-builder binary (assumes it's built in project root)
	projectRoot := findProjectRoot(t)
	binaryPath := filepath.Join(projectRoot, "devnet-builder")

	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("devnet-builder binary not found at %s. Run 'go build' first.", binaryPath)
	}

	ctx := &TestContext{
		t:          t,
		HomeDir:    homeDir,
		BinaryPath: binaryPath,
		Env: map[string]string{
			"DEVNET_HOME": homeDir,
			"HOME":        homeDir, // Override HOME to prevent pollution
		},
		cleanups: make([]func(), 0),
	}

	// Register cleanup to run all cleanup functions
	t.Cleanup(func() {
		ctx.RunCleanups()
	})

	return ctx
}

// WithConfig sets a test configuration file path
func (ctx *TestContext) WithConfig(configPath string) *TestContext {
	ctx.ConfigPath = configPath
	return ctx
}

// WithEnv adds or overrides an environment variable
func (ctx *TestContext) WithEnv(key, value string) *TestContext {
	ctx.Env[key] = value
	return ctx
}

// AddCleanup registers a cleanup function to run when test completes
// Cleanup functions run in reverse order (LIFO)
func (ctx *TestContext) AddCleanup(cleanup func()) {
	ctx.cleanups = append([]func(){cleanup}, ctx.cleanups...)
}

// RunCleanups executes all registered cleanup functions
// Called automatically via t.Cleanup(), but can be called manually if needed
func (ctx *TestContext) RunCleanups() {
	for _, cleanup := range ctx.cleanups {
		cleanup()
	}
	ctx.cleanups = nil
}

// GetEnv returns all environment variables as a slice
// Test-specific environment variables (ctx.Env) override system environment
func (ctx *TestContext) GetEnv() []string {
	// Start with system environment
	baseEnv := os.Environ()

	// Create map to track which keys to override
	overrideKeys := make(map[string]bool)
	for key := range ctx.Env {
		overrideKeys[key] = true
	}

	// Filter out system env vars that will be overridden
	env := make([]string, 0, len(baseEnv)+len(ctx.Env))
	for _, envVar := range baseEnv {
		// Split on first '=' to get key
		idx := -1
		for i, c := range envVar {
			if c == '=' {
				idx = i
				break
			}
		}
		if idx == -1 {
			continue
		}
		key := envVar[:idx]

		// Skip if this key will be overridden by ctx.Env
		if !overrideKeys[key] {
			env = append(env, envVar)
		}
	}

	// Add test-specific environment variables (these take precedence)
	for key, value := range ctx.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

// CreateSubDir creates a subdirectory within the test home directory
func (ctx *TestContext) CreateSubDir(subpath string) string {
	ctx.t.Helper()
	dir := filepath.Join(ctx.HomeDir, subpath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		ctx.t.Fatalf("failed to create subdirectory %s: %v", subpath, err)
	}
	return dir
}

// WriteFile writes content to a file within the test home directory
func (ctx *TestContext) WriteFile(subpath string, content []byte) string {
	ctx.t.Helper()
	path := filepath.Join(ctx.HomeDir, subpath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		ctx.t.Fatalf("failed to create directory for %s: %v", subpath, err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		ctx.t.Fatalf("failed to write file %s: %v", subpath, err)
	}
	return path
}

// ReadFile reads a file from the test home directory
func (ctx *TestContext) ReadFile(subpath string) []byte {
	ctx.t.Helper()
	path := filepath.Join(ctx.HomeDir, subpath)
	content, err := os.ReadFile(path)
	if err != nil {
		ctx.t.Fatalf("failed to read file %s: %v", subpath, err)
	}
	return content
}

// FileExists checks if a file exists within the test home directory
func (ctx *TestContext) FileExists(subpath string) bool {
	path := filepath.Join(ctx.HomeDir, subpath)
	_, err := os.Stat(path)
	return err == nil
}

// findProjectRoot finds the project root directory by looking for go.mod
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Walk up directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}
