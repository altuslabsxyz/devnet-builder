package node

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/stablelabs/stable-devnet/internal/output"
)

const (
	// DefaultDockerImage is the default Docker image for running nodes.
	DefaultDockerImage = "stablelabs/stabled:latest"

	// DockerStopTimeout is the timeout for gracefully stopping a container.
	DockerStopTimeout = 30 * time.Second
)

// DockerManager manages nodes running in Docker containers.
type DockerManager struct {
	Image  string
	Logger *output.Logger
}

// NewDockerManager creates a new DockerManager.
func NewDockerManager(image string, logger *output.Logger) *DockerManager {
	if image == "" {
		image = DefaultDockerImage
	}
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &DockerManager{
		Image:  image,
		Logger: logger,
	}
}

// Start starts a node in a Docker container.
func (m *DockerManager) Start(ctx context.Context, node *Node, genesisPath string) error {
	containerName := ContainerNameForIndex(node.Index)

	// Check if container already exists
	if m.IsRunning(ctx, node) {
		return fmt.Errorf("container %s is already running", containerName)
	}

	// Remove existing container if it exists (stopped)
	m.removeContainer(ctx, containerName)

	// Build docker run command
	args := []string{
		"run", "-d",
		"--name", containerName,
		"--network", "host", // Use host networking for simplicity
		"-v", fmt.Sprintf("%s:/root/.stabled", node.HomeDir),
		"-v", fmt.Sprintf("%s:/root/.stabled/config/genesis.json:ro", genesisPath),
		m.Image,
		"stabled", "start",
		"--home", "/root/.stabled",
		fmt.Sprintf("--rpc.laddr=tcp://0.0.0.0:%d", node.Ports.RPC),
		fmt.Sprintf("--p2p.laddr=tcp://0.0.0.0:%d", node.Ports.P2P),
		fmt.Sprintf("--grpc.address=0.0.0.0:%d", node.Ports.GRPC),
		fmt.Sprintf("--api.enabled-unsafe-cors=true"),
		fmt.Sprintf("--api.enable=true"),
		fmt.Sprintf("--json-rpc.address=0.0.0.0:%d", node.Ports.EVMRPC),
		fmt.Sprintf("--json-rpc.ws-address=0.0.0.0:%d", node.Ports.EVMWS),
	}

	m.Logger.Debug("Starting container: docker %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start container: %w\nOutput: %s", err, string(output))
	}

	// Get container ID
	containerID := strings.TrimSpace(string(output))
	node.ContainerID = containerID
	node.ContainerName = containerName
	node.Status = NodeStatusStarting

	return nil
}

// Stop stops a running Docker container.
func (m *DockerManager) Stop(ctx context.Context, node *Node, timeout time.Duration) error {
	if node.ContainerID == "" && node.ContainerName == "" {
		return nil // Nothing to stop
	}

	containerRef := node.ContainerID
	if containerRef == "" {
		containerRef = node.ContainerName
	}

	// Graceful stop with timeout
	timeoutSec := int(timeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = int(DockerStopTimeout.Seconds())
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", "-t", fmt.Sprintf("%d", timeoutSec), containerRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if container doesn't exist
		if strings.Contains(string(output), "No such container") {
			node.SetStopped()
			return nil
		}
		return fmt.Errorf("failed to stop container: %w\nOutput: %s", err, string(output))
	}

	node.SetStopped()
	return nil
}

// IsRunning checks if a Docker container is running.
func (m *DockerManager) IsRunning(ctx context.Context, node *Node) bool {
	containerRef := node.ContainerID
	if containerRef == "" {
		containerRef = ContainerNameForIndex(node.Index)
	}

	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", containerRef)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}

// removeContainer removes a stopped container.
func (m *DockerManager) removeContainer(ctx context.Context, name string) {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
	cmd.Run() // Ignore errors
}

// GetLogs retrieves logs from a Docker container.
func (m *DockerManager) GetLogs(ctx context.Context, node *Node, tail int, since string) (string, error) {
	containerRef := node.ContainerID
	if containerRef == "" {
		containerRef = node.ContainerName
	}

	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	if since != "" {
		args = append(args, "--since", since)
	}
	args = append(args, containerRef)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}

	return string(output), nil
}

// FollowLogs streams logs from a Docker container.
func (m *DockerManager) FollowLogs(ctx context.Context, node *Node, tail int) (*exec.Cmd, error) {
	containerRef := node.ContainerID
	if containerRef == "" {
		containerRef = node.ContainerName
	}

	args := []string{"logs", "-f"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, containerRef)

	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd, nil
}

// PullImage pulls the Docker image if not present.
func (m *DockerManager) PullImage(ctx context.Context) error {
	m.Logger.Debug("Pulling Docker image: %s", m.Image)

	cmd := exec.CommandContext(ctx, "docker", "pull", m.Image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull image: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// IsDockerAvailable checks if Docker is available and running.
func IsDockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}
