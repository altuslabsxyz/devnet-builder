package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
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
	var _ NodeRuntime = (*DockerRuntime)(nil)
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

func TestDockerRuntime_GetLogs(t *testing.T) {
	t.Run("happy path - returns log stream", func(t *testing.T) {
		expectedLogs := "log line 1\nlog line 2\n"
		var capturedOpts container.LogsOptions
		var capturedContainerID string

		mock := &mockDockerClient{
			logsFn: func(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
				capturedContainerID = containerID
				capturedOpts = opts
				return io.NopCloser(strings.NewReader(expectedLogs)), nil
			},
		}

		rt := &DockerRuntime{
			client:     mock,
			logger:     testLogger(),
			containers: map[string]*containerState{
				"test-node": {
					containerID: "container-logs-123",
					nodeID:      "test-node",
				},
			},
		}

		reader, err := rt.GetLogs(context.Background(), "test-node", LogOptions{})
		require.NoError(t, err)
		require.NotNil(t, reader)
		defer reader.Close()

		// Verify correct container ID was used
		assert.Equal(t, "container-logs-123", capturedContainerID)

		// Verify default options
		assert.True(t, capturedOpts.ShowStdout)
		assert.True(t, capturedOpts.ShowStderr)
		assert.True(t, capturedOpts.Timestamps)
		assert.False(t, capturedOpts.Follow)
		assert.Empty(t, capturedOpts.Tail)
		assert.Empty(t, capturedOpts.Since)

		// Verify log content
		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, expectedLogs, string(content))
	})
}

func TestDockerRuntime_GetLogs_NotFound(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client:     mock,
		logger:     testLogger(),
		containers: make(map[string]*containerState),
	}

	reader, err := rt.GetLogs(context.Background(), "nonexistent", LogOptions{})
	require.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "node nonexistent not found")
}

func TestDockerRuntime_GetLogs_WithOptions(t *testing.T) {
	sinceTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	var capturedOpts container.LogsOptions

	mock := &mockDockerClient{
		logsFn: func(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
			capturedOpts = opts
			return io.NopCloser(strings.NewReader("logs")), nil
		},
	}

	rt := &DockerRuntime{
		client:     mock,
		logger:     testLogger(),
		containers: map[string]*containerState{
			"test-node": {
				containerID: "container-opts-456",
				nodeID:      "test-node",
			},
		},
	}

	opts := LogOptions{
		Follow: true,
		Lines:  100,
		Since:  sinceTime,
	}

	reader, err := rt.GetLogs(context.Background(), "test-node", opts)
	require.NoError(t, err)
	require.NotNil(t, reader)
	defer reader.Close()

	// Verify options were mapped correctly
	assert.True(t, capturedOpts.ShowStdout)
	assert.True(t, capturedOpts.ShowStderr)
	assert.True(t, capturedOpts.Timestamps)
	assert.True(t, capturedOpts.Follow)
	assert.Equal(t, "100", capturedOpts.Tail)
	assert.Equal(t, sinceTime.Format(time.RFC3339), capturedOpts.Since)
}

func TestDockerRuntime_Cleanup(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client: mock,
		logger: testLogger(),
		containers: map[string]*containerState{
			"node-1": {containerID: "container-111", nodeID: "node-1"},
			"node-2": {containerID: "container-222", nodeID: "node-2"},
		},
	}

	err := rt.Cleanup(context.Background())
	require.NoError(t, err)

	// Verify all containers were removed
	assert.Len(t, mock.removeCalls, 2)
	assert.Contains(t, mock.removeCalls, "container-111")
	assert.Contains(t, mock.removeCalls, "container-222")

	// Verify map is cleared
	assert.Len(t, rt.containers, 0)
}

func TestDockerRuntime_Cleanup_PartialFailure(t *testing.T) {
	removeErr := assert.AnError

	mock := &mockDockerClient{
		removeFn: func(ctx context.Context, containerID string, opts container.RemoveOptions) error {
			// Fail on the second container
			if containerID == "container-222" {
				return removeErr
			}
			return nil
		},
	}

	rt := &DockerRuntime{
		client: mock,
		logger: testLogger(),
		containers: map[string]*containerState{
			"node-1": {containerID: "container-111", nodeID: "node-1"},
			"node-2": {containerID: "container-222", nodeID: "node-2"},
		},
	}

	err := rt.Cleanup(context.Background())

	// Should return the last error
	require.Error(t, err)
	assert.Equal(t, removeErr, err)

	// Verify removal was attempted for all containers (continues despite errors)
	assert.Len(t, mock.removeCalls, 2)
	assert.Contains(t, mock.removeCalls, "container-111")
	assert.Contains(t, mock.removeCalls, "container-222")

	// Map should still be cleared even with partial failure
	assert.Len(t, rt.containers, 0)
}

func TestDockerRuntime_Cleanup_EmptyMap(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client:     mock,
		logger:     testLogger(),
		containers: make(map[string]*containerState),
	}

	err := rt.Cleanup(context.Background())
	require.NoError(t, err)

	// Verify no container removal calls were made
	assert.Len(t, mock.removeCalls, 0)

	// Map should still be properly initialized (empty)
	assert.Len(t, rt.containers, 0)
}

func TestDockerRuntime_PortMapping(t *testing.T) {
	tests := []struct {
		name          string
		nodeIndex     int
		expectedPorts map[string]string // containerPort -> hostPort
	}{
		{
			name:      "validator-0 gets base ports",
			nodeIndex: 0,
			expectedPorts: map[string]string{
				"26656/tcp": "26656",
				"26657/tcp": "26657",
				"1317/tcp":  "1317",
				"9090/tcp":  "9090",
			},
		},
		{
			name:      "validator-1 gets offset ports (+100)",
			nodeIndex: 1,
			expectedPorts: map[string]string{
				"26656/tcp": "26756",
				"26657/tcp": "26757",
				"1317/tcp":  "1417",
				"9090/tcp":  "9190",
			},
		},
		{
			name:      "validator-2 gets offset ports (+200)",
			nodeIndex: 2,
			expectedPorts: map[string]string{
				"26656/tcp": "26856",
				"26657/tcp": "26857",
				"1317/tcp":  "1517",
				"9090/tcp":  "9290",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &DockerRuntime{
				logger: testLogger(),
			}

			node := &types.Node{
				Spec: types.NodeSpec{
					DevnetRef: "test-devnet",
					Index:     tt.nodeIndex,
					Role:      "validator",
				},
			}

			portBindings, exposedPorts := rt.buildPortBindings(node)

			// Verify port bindings
			for containerPort, expectedHostPort := range tt.expectedPorts {
				bindings, exists := portBindings[nat.Port(containerPort)]
				require.True(t, exists, "port binding for %s should exist", containerPort)
				require.Len(t, bindings, 1)
				assert.Equal(t, expectedHostPort, bindings[0].HostPort,
					"host port for %s should be %s", containerPort, expectedHostPort)

				// Verify exposed port
				_, exposed := exposedPorts[nat.Port(containerPort)]
				assert.True(t, exposed, "port %s should be exposed", containerPort)
			}

			// Verify we have exactly 4 port mappings
			assert.Len(t, portBindings, 4)
			assert.Len(t, exposedPorts, 4)
		})
	}
}

func TestDockerRuntime_StartNode_WithPortBindings(t *testing.T) {
	mock := &mockDockerClient{}

	rt := &DockerRuntime{
		client:       mock,
		logger:       testLogger(),
		defaultImage: "stablelabs/stabled:latest",
		containers:   make(map[string]*containerState),
	}

	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: "test-devnet-validator-1",
		},
		Spec: types.NodeSpec{
			DevnetRef:  "test-devnet",
			Index:      1, // Should get +100 offset
			Role:       "validator",
			HomeDir:    "/tmp/node-home",
			BinaryPath: "/usr/bin/stabled",
		},
	}

	err := rt.StartNode(context.Background(), node, StartOptions{})
	require.NoError(t, err)

	// Verify container was created with port bindings
	require.Len(t, mock.createCalls, 1)
	createCall := mock.createCalls[0]

	// Verify host config has port bindings
	require.NotNil(t, createCall.hostConfig.PortBindings)
	assert.Len(t, createCall.hostConfig.PortBindings, 4)

	// Verify specific port mappings for index 1 (offset +100)
	expectedMappings := map[string]string{
		"26656/tcp": "26756", // P2P
		"26657/tcp": "26757", // RPC
		"1317/tcp":  "1417",  // REST
		"9090/tcp":  "9190",  // gRPC
	}

	for containerPort, expectedHostPort := range expectedMappings {
		bindings, exists := createCall.hostConfig.PortBindings[nat.Port(containerPort)]
		require.True(t, exists, "port binding for %s should exist", containerPort)
		require.Len(t, bindings, 1)
		assert.Equal(t, expectedHostPort, bindings[0].HostPort,
			"host port for container port %s should be %s", containerPort, expectedHostPort)
	}

	// Verify container config has exposed ports
	require.NotNil(t, createCall.config.ExposedPorts)
	assert.Len(t, createCall.config.ExposedPorts, 4)
}

func TestPortConstants(t *testing.T) {
	// Verify port constants match expected Cosmos SDK defaults
	assert.Equal(t, 26656, P2PPort, "P2P port should be 26656")
	assert.Equal(t, 26657, RPCPort, "RPC port should be 26657")
	assert.Equal(t, 1317, RESTPort, "REST port should be 1317")
	assert.Equal(t, 9090, GRPCPort, "gRPC port should be 9090")
}

func TestDockerRuntime_PortMapping_HighIndex(t *testing.T) {
	// Test that high indices still calculate ports correctly
	rt := &DockerRuntime{
		logger: testLogger(),
	}

	node := &types.Node{
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     10, // Should get +1000 offset
			Role:      "validator",
		},
	}

	portBindings, _ := rt.buildPortBindings(node)

	// Verify offset calculation (10 * 100 = 1000)
	p2pBinding := portBindings[nat.Port("26656/tcp")]
	require.Len(t, p2pBinding, 1)
	assert.Equal(t, fmt.Sprintf("%d", 26656+1000), p2pBinding[0].HostPort)

	rpcBinding := portBindings[nat.Port("26657/tcp")]
	require.Len(t, rpcBinding, 1)
	assert.Equal(t, fmt.Sprintf("%d", 26657+1000), rpcBinding[0].HostPort)
}
