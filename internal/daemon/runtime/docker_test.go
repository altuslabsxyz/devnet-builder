package runtime

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dockertypes "github.com/docker/docker/api/types"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDockerClient implements dockerClient for testing
type mockDockerClient struct {
	createFn  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	startFn   func(ctx context.Context, containerID string, opts container.StartOptions) error
	stopFn    func(ctx context.Context, containerID string, opts container.StopOptions) error
	restartFn func(ctx context.Context, containerID string, opts container.StopOptions) error
	removeFn  func(ctx context.Context, containerID string, opts container.RemoveOptions) error
	inspectFn func(ctx context.Context, containerID string) (dockertypes.ContainerJSON, error)
	logsFn    func(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error)

	createCalls  []createCall
	startCalls   []string
	stopCalls    []string
	restartCalls []string
	removeCalls  []string
}

type createCall struct {
	config     *container.Config
	hostConfig *container.HostConfig
	name       string
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	m.createCalls = append(m.createCalls, createCall{config: config, hostConfig: hostConfig, name: containerName})
	if m.createFn != nil {
		return m.createFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "test-container-id"}, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, opts container.StartOptions) error {
	m.startCalls = append(m.startCalls, containerID)
	if m.startFn != nil {
		return m.startFn(ctx, containerID, opts)
	}
	return nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, opts container.StopOptions) error {
	m.stopCalls = append(m.stopCalls, containerID)
	if m.stopFn != nil {
		return m.stopFn(ctx, containerID, opts)
	}
	return nil
}

func (m *mockDockerClient) ContainerRestart(ctx context.Context, containerID string, opts container.StopOptions) error {
	m.restartCalls = append(m.restartCalls, containerID)
	if m.restartFn != nil {
		return m.restartFn(ctx, containerID, opts)
	}
	return nil
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, opts container.RemoveOptions) error {
	m.removeCalls = append(m.removeCalls, containerID)
	if m.removeFn != nil {
		return m.removeFn(ctx, containerID, opts)
	}
	return nil
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (dockertypes.ContainerJSON, error) {
	if m.inspectFn != nil {
		return m.inspectFn(ctx, containerID)
	}
	return dockertypes.ContainerJSON{
		ContainerJSONBase: &dockertypes.ContainerJSONBase{
			State: &dockertypes.ContainerState{
				Running:   true,
				StartedAt: time.Now().Format(time.RFC3339),
			},
		},
	}, nil
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
	if m.logsFn != nil {
		return m.logsFn(ctx, containerID, opts)
	}
	return io.NopCloser(nil), nil
}

func (m *mockDockerClient) Close() error {
	return nil
}

// TestDockerRuntimeImplementsInterface verifies DockerRuntime implements NodeRuntime.
func TestDockerRuntimeImplementsInterface(t *testing.T) {
	// This is a compile-time check - if DockerRuntime doesn't implement
	// NodeRuntime, this won't compile.
	var _ controller.NodeRuntime = (*DockerRuntime)(nil)
}

func TestContainerName(t *testing.T) {
	tests := []struct {
		name     string
		node     *types.Node
		expected string
	}{
		{
			name: "basic node",
			node: &types.Node{
				Spec: types.NodeSpec{
					DevnetRef: "mydevnet",
					Index:     0,
				},
			},
			expected: "dvb-mydevnet-node-0",
		},
		{
			name: "multi-digit index",
			node: &types.Node{
				Spec: types.NodeSpec{
					DevnetRef: "testnet",
					Index:     42,
				},
			},
			expected: "dvb-testnet-node-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerName(tt.node)
			if got != tt.expected {
				t.Errorf("containerName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDockerRuntime_StartNode(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client:       mock,
		logger:       testLogger(),
		defaultImage: "stablelabs/stabled:latest",
		containers:   make(map[string]*containerState),
	}

	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "test-devnet-validator-0",
		},
		Spec: types.NodeSpec{
			DevnetRef:  "test-devnet",
			Index:      0,
			Role:       "validator",
			HomeDir:    "/tmp/node-home",
			BinaryPath: "/usr/bin/stabled",
		},
	}

	err := rt.StartNode(context.Background(), node, StartOptions{})
	require.NoError(t, err)

	// Verify container was created
	require.Len(t, mock.createCalls, 1)
	assert.Equal(t, "dvb-test-devnet-validator-0", mock.createCalls[0].name)

	// Verify container was started
	require.Len(t, mock.startCalls, 1)

	// Verify state tracking
	state, exists := rt.containers["test-devnet-validator-0"]
	require.True(t, exists)
	assert.Equal(t, "test-container-id", state.containerID)
}

func TestDockerRuntime_StopNode_Graceful(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client:       mock,
		logger:       testLogger(),
		defaultImage: "stablelabs/stabled:latest",
		containers: map[string]*containerState{
			"test-node": {
				containerID: "container-123",
				nodeID:      "test-node",
				stopCh:      make(chan struct{}),
				stoppedCh:   make(chan struct{}),
			},
		},
	}

	err := rt.StopNode(context.Background(), "test-node", true)
	require.NoError(t, err)

	// Verify container was stopped
	require.Len(t, mock.stopCalls, 1)
	assert.Equal(t, "container-123", mock.stopCalls[0])

	// Verify container was removed
	require.Len(t, mock.removeCalls, 1)
	assert.Equal(t, "container-123", mock.removeCalls[0])

	// Verify state removed
	_, exists := rt.containers["test-node"]
	assert.False(t, exists)
}

func TestDockerRuntime_StopNode_Force(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client:       mock,
		logger:       testLogger(),
		defaultImage: "stablelabs/stabled:latest",
		containers: map[string]*containerState{
			"test-node": {
				containerID: "container-456",
				nodeID:      "test-node",
				stopCh:      make(chan struct{}),
				stoppedCh:   make(chan struct{}),
			},
		},
	}

	err := rt.StopNode(context.Background(), "test-node", false)
	require.NoError(t, err)

	// Force stop should skip graceful stop and go straight to remove
	require.Len(t, mock.stopCalls, 0)
	require.Len(t, mock.removeCalls, 1)
	assert.Equal(t, "container-456", mock.removeCalls[0])

	// Verify state removed
	_, exists := rt.containers["test-node"]
	assert.False(t, exists)
}

func TestDockerRuntime_GetNodeStatus(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)

	mock := &mockDockerClient{
		inspectFn: func(ctx context.Context, containerID string) (dockertypes.ContainerJSON, error) {
			return dockertypes.ContainerJSON{
				ContainerJSONBase: &dockertypes.ContainerJSONBase{
					State: &dockertypes.ContainerState{
						Running:    true,
						StartedAt:  startedAt.Format(time.RFC3339),
						Pid:        12345,
						ExitCode:   0,
						OOMKilled:  false,
						Restarting: false,
					},
				},
			}, nil
		},
	}

	rt := &DockerRuntime{
		client:       mock,
		logger:       testLogger(),
		defaultImage: "stablelabs/stabled:latest",
		containers: map[string]*containerState{
			"test-node": {
				containerID:  "container-789",
				nodeID:       "test-node",
				restartCount: 2,
			},
		},
	}

	status, err := rt.GetNodeStatus(context.Background(), "test-node")
	require.NoError(t, err)

	assert.True(t, status.Running)
	assert.Equal(t, 12345, status.PID)
	assert.Equal(t, 2, status.Restarts)
}

func TestDockerRuntime_GetNodeStatus_NotFound(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client:     mock,
		logger:     testLogger(),
		containers: make(map[string]*containerState),
	}

	status, err := rt.GetNodeStatus(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.False(t, status.Running)
}

func TestDockerRuntime_RestartNode(t *testing.T) {
	t.Run("happy path - node exists and restart succeeds", func(t *testing.T) {
		mock := &mockDockerClient{}

		rt := &DockerRuntime{
			client:     mock,
			logger:     testLogger(),
			containers: map[string]*containerState{
				"test-node": {
					containerID:  "container-abc123",
					nodeID:       "test-node",
					restartCount: 0,
				},
			},
		}

		err := rt.RestartNode(context.Background(), "test-node")
		require.NoError(t, err)

		// Verify container restart was called
		require.Len(t, mock.restartCalls, 1)
		assert.Equal(t, "container-abc123", mock.restartCalls[0])

		// Verify restart counter was incremented
		state := rt.containers["test-node"]
		assert.Equal(t, 1, state.restartCount)
	})

	t.Run("node not found", func(t *testing.T) {
		mock := &mockDockerClient{}

		rt := &DockerRuntime{
			client:     mock,
			logger:     testLogger(),
			containers: make(map[string]*containerState),
		}

		err := rt.RestartNode(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "node nonexistent not found")

		// Verify no restart was attempted
		assert.Len(t, mock.restartCalls, 0)
	})

	t.Run("container restart fails", func(t *testing.T) {
		mock := &mockDockerClient{
			restartFn: func(ctx context.Context, containerID string, opts container.StopOptions) error {
				return assert.AnError
			},
		}

		rt := &DockerRuntime{
			client:     mock,
			logger:     testLogger(),
			containers: map[string]*containerState{
				"test-node": {
					containerID:  "container-xyz789",
					nodeID:       "test-node",
					restartCount: 2,
				},
			},
		}

		err := rt.RestartNode(context.Background(), "test-node")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to restart container")

		// Verify restart was attempted
		require.Len(t, mock.restartCalls, 1)

		// Restart counter should still have been incremented (before the call)
		state := rt.containers["test-node"]
		assert.Equal(t, 3, state.restartCount)
	})
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
