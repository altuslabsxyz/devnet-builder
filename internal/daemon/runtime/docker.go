// Package runtime provides container/process runtime implementations.
package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// dockerClient abstracts Docker API operations for testability.
type dockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config,
		hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig,
		platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, opts container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, opts container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, opts container.RemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (dockertypes.ContainerJSON, error)
	ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error)
	Close() error
}

// containerState tracks a managed container's state.
type containerState struct {
	containerID   string
	nodeID        string
	node          *types.Node
	startedAt     time.Time
	restartCount  int
	lastError     string
	restartPolicy RestartPolicy

	// Supervision channels
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// DockerRuntime implements NodeRuntime using Docker containers.
type DockerRuntime struct {
	client        dockerClient
	logger        *slog.Logger
	pluginRuntime PluginRuntime
	defaultImage  string

	// Container tracking
	containers map[string]*containerState
	mu         sync.RWMutex
}

// DockerConfig configures the Docker runtime.
type DockerConfig struct {
	// DefaultImage is used when node spec doesn't specify an image.
	DefaultImage string

	// Logger for runtime operations.
	Logger *slog.Logger

	// PluginRuntime provides network-specific commands.
	PluginRuntime PluginRuntime
}

// NewDockerRuntime creates a new Docker runtime.
func NewDockerRuntime(cfg DockerConfig) (*DockerRuntime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	defaultImage := cfg.DefaultImage
	if defaultImage == "" {
		defaultImage = "stablelabs/stabled:latest"
	}

	return &DockerRuntime{
		client:        cli,
		logger:        logger,
		pluginRuntime: cfg.PluginRuntime,
		defaultImage:  defaultImage,
		containers:    make(map[string]*containerState),
	}, nil
}

// containerName generates a container name from the node spec.
func containerName(node *types.Node) string {
	return fmt.Sprintf("dvb-%s-node-%d", node.Spec.DevnetRef, node.Spec.Index)
}

// StartContainer creates and starts a container for the node.
func (r *DockerRuntime) StartContainer(ctx context.Context, node *types.Node) (string, error) {
	name := containerName(node)

	r.logger.Info("starting container",
		"name", name,
		"devnet", node.Spec.DevnetRef,
		"index", node.Spec.Index,
		"role", node.Spec.Role)

	// Determine image - use BinaryPath if it looks like a Docker image, otherwise default
	image := r.defaultImage
	if node.Spec.BinaryPath != "" {
		// If BinaryPath contains "/" it might be an image reference
		// Otherwise use the default image
		image = node.Spec.BinaryPath
	}

	// Build container config
	containerConfig := &container.Config{
		Image: image,
		Cmd:   []string{"start", "--home", "/root/.stabled"},
		Labels: map[string]string{
			"dvb.devnet": node.Spec.DevnetRef,
			"dvb.index":  fmt.Sprintf("%d", node.Spec.Index),
			"dvb.role":   node.Spec.Role,
		},
	}

	// Build host config with mounts
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyDisabled,
		},
	}

	// Mount home directory if specified
	if node.Spec.HomeDir != "" {
		hostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: node.Spec.HomeDir,
				Target: "/root/.stabled",
			},
		}
	}

	// Create container
	resp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := r.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up the created container
		_ = r.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	r.logger.Info("container started",
		"name", name,
		"containerID", resp.ID[:12])

	return resp.ID, nil
}

// StopContainer stops a running container.
func (r *DockerRuntime) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	r.logger.Info("stopping container", "containerID", containerID[:min(12, len(containerID))])

	timeoutSeconds := int(timeout.Seconds())
	if err := r.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSeconds}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

// GetContainerStatus checks if a container is running.
func (r *DockerRuntime) GetContainerStatus(ctx context.Context, containerID string) (bool, error) {
	info, err := r.client.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect container: %w", err)
	}

	return info.State.Running, nil
}

// RemoveContainer removes a container.
func (r *DockerRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	r.logger.Info("removing container", "containerID", containerID[:min(12, len(containerID))])

	if err := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	}); err != nil {
		if client.IsErrNotFound(err) {
			return nil // Already gone, that's fine
		}
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

// Close closes the Docker client.
func (r *DockerRuntime) Close() error {
	return r.client.Close()
}

// StartNode starts a node in a Docker container.
func (r *DockerRuntime) StartNode(ctx context.Context, node *types.Node, opts StartOptions) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	nodeID := node.Metadata.Name

	// Check if already running
	if state, exists := r.containers[nodeID]; exists {
		if state.containerID != "" {
			return fmt.Errorf("node %s is already running", nodeID)
		}
	}

	// Build container name
	containerName := fmt.Sprintf("dvb-%s-%s-%d", node.Spec.DevnetRef, node.Spec.Role, node.Spec.Index)

	// Determine image
	image := r.defaultImage
	if node.Spec.BinaryPath != "" && strings.Contains(node.Spec.BinaryPath, "/") && strings.Contains(node.Spec.BinaryPath, ":") {
		// Looks like a Docker image reference
		image = node.Spec.BinaryPath
	}

	// Build command
	cmd := []string{"start", "--home", "/root/.stabled"}
	if r.pluginRuntime != nil {
		cmd = r.pluginRuntime.StartCommand(node)
	}

	// Build container config
	containerConfig := &container.Config{
		Image: image,
		Cmd:   cmd,
		Tty:   true, // Simplified log handling
		Labels: map[string]string{
			"dvb.devnet": node.Spec.DevnetRef,
			"dvb.node":   nodeID,
			"dvb.index":  fmt.Sprintf("%d", node.Spec.Index),
			"dvb.role":   node.Spec.Role,
		},
	}

	// Build host config with mounts and restart policy
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyOnFailure,
		},
	}

	// Mount home directory
	if node.Spec.HomeDir != "" {
		hostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: node.Spec.HomeDir,
				Target: "/root/.stabled",
			},
		}
	}

	// Create container
	r.logger.Info("creating container",
		"name", containerName,
		"image", image,
		"nodeID", nodeID)

	resp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := r.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Clean up created container
		_ = r.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Track container state
	r.containers[nodeID] = &containerState{
		containerID:   resp.ID,
		nodeID:        nodeID,
		node:          node,
		startedAt:     time.Now(),
		restartPolicy: opts.RestartPolicy,
		stopCh:        make(chan struct{}),
		stoppedCh:     make(chan struct{}),
	}

	r.logger.Info("container started",
		"name", containerName,
		"containerID", resp.ID[:min(12, len(resp.ID))],
		"nodeID", nodeID)

	return nil
}

// StopNode stops a node's container.
func (r *DockerRuntime) StopNode(ctx context.Context, nodeID string, graceful bool) error {
	r.mu.Lock()
	state, exists := r.containers[nodeID]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("node %s not found", nodeID)
	}
	delete(r.containers, nodeID)
	r.mu.Unlock()

	// Signal supervision to stop
	close(state.stopCh)

	containerID := state.containerID

	if graceful {
		// Graceful stop with timeout
		timeout := 30
		r.logger.Info("stopping container gracefully",
			"containerID", containerID[:min(12, len(containerID))],
			"nodeID", nodeID,
			"timeout", timeout)

		if err := r.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
			r.logger.Warn("graceful stop failed, forcing removal",
				"containerID", containerID[:min(12, len(containerID))],
				"error", err)
		}
	}

	// Remove container
	r.logger.Info("removing container",
		"containerID", containerID[:min(12, len(containerID))],
		"nodeID", nodeID)

	if err := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

// Ensure DockerRuntime implements NodeRuntime.
var _ controller.NodeRuntime = (*DockerRuntime)(nil)
