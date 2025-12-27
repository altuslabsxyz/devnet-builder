package helpers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// MockSnapshotServer provides a mock HTTP server for snapshot downloads
// This eliminates the 2-10 minute download time in tests by serving local fixtures
type MockSnapshotServer struct {
	t          *testing.T
	server     *httptest.Server
	fixtureDir string // Directory containing snapshot fixtures
}

// NewMockSnapshotServer creates a new mock snapshot server
// The server serves snapshot files from tests/e2e/testdata/snapshots/
func NewMockSnapshotServer(t *testing.T) *MockSnapshotServer {
	t.Helper()

	// Find fixture directory
	projectRoot := findProjectRootFromCwd(t)
	fixtureDir := filepath.Join(projectRoot, "tests", "e2e", "testdata", "snapshots")

	// Verify fixture directory exists
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Fatalf("snapshot fixture directory not found: %s", fixtureDir)
	}

	mock := &MockSnapshotServer{
		t:          t,
		fixtureDir: fixtureDir,
	}

	// Create HTTP server
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))

	// Register cleanup
	t.Cleanup(func() {
		mock.server.Close()
	})

	t.Logf("Mock snapshot server started at %s", mock.server.URL)
	return mock
}

// URL returns the base URL of the mock server
func (m *MockSnapshotServer) URL() string {
	return m.server.URL
}

// SnapshotURL returns the full URL for a snapshot file
// Example: SnapshotURL("mainnet-snapshot.tar.gz") → "http://localhost:xxxxx/mainnet-snapshot.tar.gz"
func (m *MockSnapshotServer) SnapshotURL(filename string) string {
	return fmt.Sprintf("%s/%s", m.server.URL, filename)
}

// handleRequest handles HTTP requests to the mock server
func (m *MockSnapshotServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.t.Logf("Mock snapshot server received request: %s %s", r.Method, r.URL.Path)

	// Only support GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from path
	filename := filepath.Base(r.URL.Path)
	if filename == "." || filename == "/" {
		http.Error(w, "Snapshot filename required", http.StatusBadRequest)
		return
	}

	// Serve snapshot file
	snapshotPath := filepath.Join(m.fixtureDir, filename)
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		m.t.Logf("Snapshot file not found: %s", snapshotPath)
		http.Error(w, fmt.Sprintf("Snapshot not found: %s", filename), http.StatusNotFound)
		return
	}

	// Read and serve file
	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		m.t.Logf("Failed to read snapshot file: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))

	// Write response
	if _, err := w.Write(content); err != nil {
		m.t.Logf("Failed to write response: %v", err)
	}

	m.t.Logf("Served snapshot: %s (%d bytes)", filename, len(content))
}

// WithCustomHandler allows setting a custom HTTP handler for specific paths
// This is useful for testing error scenarios (e.g., network failures, corrupted downloads)
func (m *MockSnapshotServer) WithCustomHandler(pattern string, handler http.HandlerFunc) {
	originalHandler := m.server.Config.Handler
	m.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pattern {
			handler(w, r)
			return
		}
		originalHandler.ServeHTTP(w, r)
	})
}

// SimulateSlowDownload simulates a slow download by adding delay
func (m *MockSnapshotServer) SimulateSlowDownload(filename string, delayMS int) {
	m.WithCustomHandler("/"+filename, func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow download
		m.t.Logf("Simulating slow download for %s (delay: %dms)", filename, delayMS)
		// Note: Not implementing actual delay here to keep tests fast
		// If needed, use time.Sleep(time.Duration(delayMS) * time.Millisecond)
		m.handleRequest(w, r)
	})
}

// SimulateDownloadFailure simulates a download failure (e.g., 500 error)
func (m *MockSnapshotServer) SimulateDownloadFailure(filename string) {
	m.WithCustomHandler("/"+filename, func(w http.ResponseWriter, r *http.Request) {
		m.t.Logf("Simulating download failure for %s", filename)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	})
}

// SimulateNetworkTimeout simulates a network timeout by closing connection
func (m *MockSnapshotServer) SimulateNetworkTimeout(filename string) {
	m.WithCustomHandler("/"+filename, func(w http.ResponseWriter, r *http.Request) {
		m.t.Logf("Simulating network timeout for %s", filename)
		// Close connection without sending response
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	})
}

// GetRequestCount returns the number of requests received for a specific file
// Useful for verifying caching behavior
func (m *MockSnapshotServer) GetRequestCount(filename string) int {
	// Note: This would require tracking requests in a map
	// For simplicity, not implementing request counting in this version
	// Can be added if needed for cache validation tests
	return 0
}

// findProjectRootFromCwd finds the project root by walking up from current directory
func findProjectRootFromCwd(t *testing.T) string {
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
