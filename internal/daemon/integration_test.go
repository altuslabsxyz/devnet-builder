//go:build integration

// internal/daemon/integration_test.go
package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevnetLifecycle(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "devnetd-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "devnetd.sock")

	// Create and start server
	cfg := &server.Config{
		SocketPath: socketPath,
		DataDir:    tmpDir,
		Foreground: true,
		Workers:    1,
		LogLevel:   "error", // Quiet logs for tests
	}

	srv, err := server.New(cfg)
	require.NoError(t, err)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()

	// Wait for server to be ready
	require.Eventually(t, func() bool {
		return client.IsDaemonRunningAt(socketPath)
	}, 5*time.Second, 100*time.Millisecond, "server should be ready")

	// Connect client
	c, err := client.NewWithSocket(socketPath)
	require.NoError(t, err)
	defer c.Close()

	// Test: Create devnet
	t.Run("CreateDevnet", func(t *testing.T) {
		spec := &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			FullNodes:  1,
			Mode:       "docker",
		}

		devnet, err := c.CreateDevnet(ctx, "test-devnet", spec, map[string]string{
			"env": "test",
		})
		require.NoError(t, err)
		assert.Equal(t, "test-devnet", devnet.Metadata.Name)
		assert.Equal(t, "stable", devnet.Spec.Plugin)
		assert.Equal(t, int32(4), devnet.Spec.Validators)
		assert.Equal(t, int32(1), devnet.Spec.FullNodes)
		assert.Equal(t, "docker", devnet.Spec.Mode)
		assert.Equal(t, "Pending", devnet.Status.Phase)
		assert.Equal(t, "test", devnet.Metadata.Labels["env"])
	})

	// Test: Get devnet
	t.Run("GetDevnet", func(t *testing.T) {
		devnet, err := c.GetDevnet(ctx, "test-devnet")
		require.NoError(t, err)
		assert.Equal(t, "test-devnet", devnet.Metadata.Name)
	})

	// Test: Get non-existent devnet
	t.Run("GetDevnetNotFound", func(t *testing.T) {
		_, err := c.GetDevnet(ctx, "non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	// Test: Create duplicate devnet
	t.Run("CreateDevnetAlreadyExists", func(t *testing.T) {
		spec := &v1.DevnetSpec{
			Plugin: "stable",
		}
		_, err := c.CreateDevnet(ctx, "test-devnet", spec, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	// Test: List devnets
	t.Run("ListDevnets", func(t *testing.T) {
		// Create another devnet
		spec := &v1.DevnetSpec{
			Plugin:     "nightly",
			Validators: 2,
		}
		_, err := c.CreateDevnet(ctx, "test-devnet-2", spec, nil)
		require.NoError(t, err)

		devnets, err := c.ListDevnets(ctx)
		require.NoError(t, err)
		assert.Len(t, devnets, 2)

		// Verify names are present
		names := make(map[string]bool)
		for _, d := range devnets {
			names[d.Metadata.Name] = true
		}
		assert.True(t, names["test-devnet"])
		assert.True(t, names["test-devnet-2"])
	})

	// Test: Start devnet
	t.Run("StartDevnet", func(t *testing.T) {
		devnet, err := c.StartDevnet(ctx, "test-devnet")
		require.NoError(t, err)
		assert.Equal(t, "test-devnet", devnet.Metadata.Name)

		// Wait for controller to stabilize the devnet phase
		// The controller may reconcile and update phase, causing generation changes
		time.Sleep(200 * time.Millisecond)
	})

	// Test: Stop devnet
	t.Run("StopDevnet", func(t *testing.T) {
		// Retry a few times in case of generation conflict with controller
		var lastErr error
		for i := 0; i < 3; i++ {
			devnet, err := c.StopDevnet(ctx, "test-devnet")
			if err == nil {
				assert.Equal(t, "test-devnet", devnet.Metadata.Name)
				return
			}
			lastErr = err
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("StopDevnet failed after retries: %v", lastErr)
	})

	// Test: Delete devnet
	t.Run("DeleteDevnet", func(t *testing.T) {
		err := c.DeleteDevnet(ctx, "test-devnet")
		require.NoError(t, err)

		// Verify it's gone
		_, err = c.GetDevnet(ctx, "test-devnet")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	// Test: List after delete
	t.Run("ListAfterDelete", func(t *testing.T) {
		devnets, err := c.ListDevnets(ctx)
		require.NoError(t, err)
		assert.Len(t, devnets, 1)
		assert.Equal(t, "test-devnet-2", devnets[0].Metadata.Name)
	})

	// Shutdown server
	cancel()

	// Wait for server to stop (or timeout)
	select {
	case <-errCh:
		// Server stopped
	case <-time.After(5 * time.Second):
		t.Log("Server shutdown timed out")
	}
}
