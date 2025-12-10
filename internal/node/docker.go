package node

import (
	"context"
	"fmt"
	"os"
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
	Image      string
	EVMChainID string
	Logger     *output.Logger
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

// NewDockerManagerWithEVMChainID creates a new DockerManager with EVM chain ID.
func NewDockerManagerWithEVMChainID(image string, evmChainID string, logger *output.Logger) *DockerManager {
	m := NewDockerManager(image, logger)
	m.EVMChainID = evmChainID
	return m
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

	// Add EVM chain ID if set
	if m.EVMChainID != "" {
		args = append(args, fmt.Sprintf("--evm.evm-chain-id=%s", m.EVMChainID))
	}

	m.Logger.Debug("Starting container: docker %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Print command error info for debugging
		m.Logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  "docker",
			Args:     args,
			WorkDir:  node.HomeDir,
			Stderr:   string(cmdOutput),
			ExitCode: -1,
			Error:    err,
		})
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Get container ID
	containerID := strings.TrimSpace(string(cmdOutput))
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

// Init runs `stabled init` for a node in Docker.
func (m *DockerManager) Init(ctx context.Context, nodeDir, moniker, chainID string) error {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		m.Image,
		"stabled", "init", moniker,
		"--chain-id", chainID,
		"--home", "/root/.stabled",
	}

	m.Logger.Debug("Docker init: docker %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker init failed: %s: %w", string(output), err)
	}
	return nil
}

// GetNodeID retrieves the node ID using `stabled comet show-node-id`.
func (m *DockerManager) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		m.Image,
		"stabled", "comet", "show-node-id",
		"--home", "/root/.stabled",
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker show-node-id failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Export runs `stabled export` to export the current state as genesis.
func (m *DockerManager) Export(ctx context.Context, nodeDir, destPath string) error {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		m.Image,
		"stabled", "export",
		"--home", "/root/.stabled",
	}

	cmd := exec.CommandContext(ctx, "docker", args...)

	// Create output file
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("docker export failed: %w", err)
	}

	// Verify output file
	info, err := os.Stat(destPath)
	if err != nil || info.Size() == 0 {
		os.Remove(destPath)
		return fmt.Errorf("exported genesis is empty or missing")
	}

	return nil
}
