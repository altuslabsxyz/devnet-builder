// internal/client/client_test.go
package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// wrapGRPCError Tests
// =============================================================================

func TestWrapGRPCError_NotFound(t *testing.T) {
	grpcErr := status.Error(codes.NotFound, "devnet not found")

	wrapped := wrapGRPCError(grpcErr)

	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "not found:")
	assert.Contains(t, wrapped.Error(), "devnet not found")
}

func TestWrapGRPCError_AlreadyExists(t *testing.T) {
	grpcErr := status.Error(codes.AlreadyExists, "devnet already exists")

	wrapped := wrapGRPCError(grpcErr)

	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "already exists:")
	assert.Contains(t, wrapped.Error(), "devnet already exists")
}

func TestWrapGRPCError_InvalidArgument(t *testing.T) {
	grpcErr := status.Error(codes.InvalidArgument, "name is required")

	wrapped := wrapGRPCError(grpcErr)

	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "invalid argument:")
	assert.Contains(t, wrapped.Error(), "name is required")
}

func TestWrapGRPCError_Unavailable(t *testing.T) {
	grpcErr := status.Error(codes.Unavailable, "connection refused")

	wrapped := wrapGRPCError(grpcErr)

	assert.Error(t, wrapped)
	assert.Contains(t, wrapped.Error(), "daemon unavailable:")
	assert.Contains(t, wrapped.Error(), "connection refused")
}

func TestWrapGRPCError_OtherCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     codes.Code
		message  string
		contains string
	}{
		{"Internal", codes.Internal, "internal error", "Internal"},
		{"PermissionDenied", codes.PermissionDenied, "access denied", "PermissionDenied"},
		{"DeadlineExceeded", codes.DeadlineExceeded, "timeout", "DeadlineExceeded"},
		{"Unimplemented", codes.Unimplemented, "not supported", "Unimplemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcErr := status.Error(tt.code, tt.message)
			wrapped := wrapGRPCError(grpcErr)

			assert.Error(t, wrapped)
			assert.Contains(t, wrapped.Error(), tt.contains)
			assert.Contains(t, wrapped.Error(), tt.message)
		})
	}
}

func TestWrapGRPCError_NonGRPCError(t *testing.T) {
	// Regular Go error (not a gRPC status)
	regularErr := assert.AnError

	wrapped := wrapGRPCError(regularErr)

	// Should return the original error unchanged
	assert.Equal(t, regularErr, wrapped)
}

func TestWrapGRPCError_NilError(t *testing.T) {
	// Note: gRPC's status.FromError(nil) returns codes.OK with empty message
	// This is expected behavior - nil error becomes "OK: " when wrapped
	wrapped := wrapGRPCError(nil)

	// The function returns an error representing OK status
	// This is gRPC semantics - nil input still produces a status
	assert.NotNil(t, wrapped)
	assert.Contains(t, wrapped.Error(), "OK")
}

// =============================================================================
// DefaultSocketPath Tests
// =============================================================================

func TestDefaultSocketPath(t *testing.T) {
	path := DefaultSocketPath()

	// Should be a non-empty path
	assert.NotEmpty(t, path)

	// Should end with the socket filename
	assert.True(t, strings.HasSuffix(path, "devnetd.sock"))

	// Should contain .devnet-builder directory
	assert.Contains(t, path, ".devnet-builder")

	// Should be an absolute path
	assert.True(t, filepath.IsAbs(path))
}

func TestDefaultSocketPath_ContainsHomeDir(t *testing.T) {
	path := DefaultSocketPath()
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	// Path should start with home directory
	assert.True(t, strings.HasPrefix(path, home))
}

// =============================================================================
// IsDaemonRunningAt Tests
// =============================================================================

func TestIsDaemonRunningAt_NonExistentSocket(t *testing.T) {
	// Non-existent socket should return false
	running := IsDaemonRunningAt("/tmp/nonexistent-socket-12345.sock")
	assert.False(t, running)
}

func TestIsDaemonRunningAt_InvalidPath(t *testing.T) {
	// Invalid path should return false
	running := IsDaemonRunningAt("")
	assert.False(t, running)
}

func TestIsDaemonRunningAt_DirectoryPath(t *testing.T) {
	// Trying to connect to a directory should fail
	running := IsDaemonRunningAt("/tmp")
	assert.False(t, running)
}

// =============================================================================
// Data Type Tests
// =============================================================================

func TestExecResult_Fields(t *testing.T) {
	result := ExecResult{
		ExitCode: 0,
		Stdout:   "output here",
		Stderr:   "error here",
	}

	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "output here", result.Stdout)
	assert.Equal(t, "error here", result.Stderr)
}

func TestExecResult_NonZeroExitCode(t *testing.T) {
	result := ExecResult{
		ExitCode: 1,
		Stdout:   "",
		Stderr:   "command failed",
	}

	assert.Equal(t, 1, result.ExitCode)
	assert.NotEmpty(t, result.Stderr)
}

func TestNodeHealth_Fields(t *testing.T) {
	now := time.Now()
	health := NodeHealth{
		Status:              "Healthy",
		Message:             "Node is running normally",
		LastCheck:           now,
		ConsecutiveFailures: 0,
	}

	assert.Equal(t, "Healthy", health.Status)
	assert.Equal(t, "Node is running normally", health.Message)
	assert.Equal(t, now, health.LastCheck)
	assert.Equal(t, 0, health.ConsecutiveFailures)
}

func TestNodeHealth_Unhealthy(t *testing.T) {
	health := NodeHealth{
		Status:              "Unhealthy",
		Message:             "RPC endpoint not responding",
		ConsecutiveFailures: 3,
	}

	assert.Equal(t, "Unhealthy", health.Status)
	assert.Equal(t, 3, health.ConsecutiveFailures)
}

func TestPortInfo_Fields(t *testing.T) {
	port := PortInfo{
		Name:          "rpc",
		ContainerPort: 26657,
		HostPort:      26757,
		Protocol:      "tcp",
	}

	assert.Equal(t, "rpc", port.Name)
	assert.Equal(t, 26657, port.ContainerPort)
	assert.Equal(t, 26757, port.HostPort)
	assert.Equal(t, "tcp", port.Protocol)
}

func TestNodePorts_Fields(t *testing.T) {
	ports := NodePorts{
		DevnetName: "my-devnet",
		Index:      0,
		Ports: []PortInfo{
			{Name: "p2p", ContainerPort: 26656, HostPort: 26656, Protocol: "tcp"},
			{Name: "rpc", ContainerPort: 26657, HostPort: 26657, Protocol: "tcp"},
		},
	}

	assert.Equal(t, "my-devnet", ports.DevnetName)
	assert.Equal(t, 0, ports.Index)
	assert.Len(t, ports.Ports, 2)
	assert.Equal(t, "p2p", ports.Ports[0].Name)
	assert.Equal(t, "rpc", ports.Ports[1].Name)
}

func TestLogEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := LogEntry{
		Timestamp: now,
		Stream:    "stdout",
		Message:   "Block 100 committed",
	}

	assert.Equal(t, now, entry.Timestamp)
	assert.Equal(t, "stdout", entry.Stream)
	assert.Equal(t, "Block 100 committed", entry.Message)
}

func TestLogEntry_Stderr(t *testing.T) {
	entry := LogEntry{
		Stream:  "stderr",
		Message: "Warning: low disk space",
	}

	assert.Equal(t, "stderr", entry.Stream)
}
