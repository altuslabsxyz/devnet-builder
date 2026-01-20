package node

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// getCurrentUserID returns the current user's UID:GID for docker --user flag.
// This ensures files created by containers are owned by the current user.
func getCurrentUserID() string {
	uid := os.Getuid()
	gid := os.Getgid()
	return fmt.Sprintf("%d:%d", uid, gid)
}

const (
	// DefaultDockerImage is the default Docker image for running nodes.
	DefaultDockerImage = "stablelabs/stabled:latest"

	// DockerStopTimeout is the timeout for gracefully stopping a container.
	DockerStopTimeout = 30 * time.Second
)

// ResourceLimits defines container resource constraints
type ResourceLimits struct {
	Memory string // Memory limit (e.g., "2g", "512m")
	CPUs   string // CPU limit (e.g., "2.0", "0.5")
}

// DefaultResourceLimits returns default resource limits for containers
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		Memory: "2g",
		CPUs:   "2.0",
	}
}

// HealthCheckConfig defines container health check parameters
type HealthCheckConfig struct {
	Interval time.Duration // Interval between health checks
	Timeout  time.Duration // Timeout for each health check
	Retries  int           // Number of retries before marking unhealthy
}

// DefaultHealthCheckConfig returns default health check configuration
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Interval: 30 * time.Second,
		Timeout:  10 * time.Second,
		Retries:  3,
	}
}

// ContainerConfig holds configuration for starting a Docker container
type ContainerConfig struct {
	Node           *Node              // Node to start
	GenesisPath    string             // Path to genesis file
	NetworkID      string             // Docker network ID (empty = host networking)
	ResourceLimits *ResourceLimits    // Resource constraints (nil = no limits)
	HealthCheck    *HealthCheckConfig // Health check config (nil = no health check)
}

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

// Start starts a node in a Docker container (backward compatible).
func (m *DockerManager) Start(ctx context.Context, node *Node, genesisPath string) error {
	config := &ContainerConfig{
		Node:        node,
		GenesisPath: genesisPath,
		NetworkID:   "", // Use host networking
	}
	return m.StartWithConfig(ctx, config)
}

// StartWithConfig starts a node in a Docker container with advanced configuration.
func (m *DockerManager) StartWithConfig(ctx context.Context, config *ContainerConfig) error {
	node := config.Node
	containerName := ContainerNameForIndex(node.Index)

	// Check if container already exists
	if m.IsRunning(ctx, node) {
		return fmt.Errorf("container %s is already running", containerName)
	}

	// Remove existing container if it exists (stopped)
	m.removeContainer(ctx, containerName)

	// Build docker run command
	args := []string{"run", "-d", "--name", containerName}

	// Network configuration
	if config.NetworkID != "" {
		// Use custom bridge network with port mappings
		args = append(args, "--network", config.NetworkID,
			"-p", fmt.Sprintf("%d:%d", node.Ports.RPC, node.Ports.RPC),
			"-p", fmt.Sprintf("%d:%d", node.Ports.P2P, node.Ports.P2P),
			"-p", fmt.Sprintf("%d:%d", node.Ports.GRPC, node.Ports.GRPC),
			"-p", fmt.Sprintf("%d:%d", node.Ports.EVMRPC, node.Ports.EVMRPC),
			"-p", fmt.Sprintf("%d:%d", node.Ports.EVMWS, node.Ports.EVMWS),
		)
	} else {
		// Use host networking (backward compatible)
		args = append(args, "--network", "host")
	}

	// Resource limits
	if config.ResourceLimits != nil {
		if config.ResourceLimits.Memory != "" {
			args = append(args, "--memory", config.ResourceLimits.Memory)
		}
		if config.ResourceLimits.CPUs != "" {
			args = append(args, "--cpus", config.ResourceLimits.CPUs)
		}
	}

	// Health check configuration
	if config.HealthCheck != nil {
		args = append(args,
			"--health-cmd", fmt.Sprintf("curl -f http://localhost:%d/status || exit 1", node.Ports.RPC),
			"--health-interval", config.HealthCheck.Interval.String(),
			"--health-timeout", config.HealthCheck.Timeout.String(),
			"--health-retries", fmt.Sprintf("%d", config.HealthCheck.Retries),
		)
	}

	// User and environment
	args = append(args,
		"--user", getCurrentUserID(),
		"-e", "HOME=/data",
		"-v", fmt.Sprintf("%s:/data", node.HomeDir),
		"-v", fmt.Sprintf("%s:/data/config/genesis.json:ro", config.GenesisPath),
		m.Image,
	)

	// GHCR images have stabled as entrypoint, others need explicit command
	if !m.isGHCRImage() {
		args = append(args, "stabled")
	}

	// Chain start command
	args = append(args, "start",
		"--home", "/data",
		fmt.Sprintf("--rpc.laddr=tcp://0.0.0.0:%d", node.Ports.RPC),
		fmt.Sprintf("--p2p.laddr=tcp://0.0.0.0:%d", node.Ports.P2P),
		fmt.Sprintf("--grpc.address=0.0.0.0:%d", node.Ports.GRPC),
		"--api.enabled-unsafe-cors=true",
		"--api.enable=true",
		fmt.Sprintf("--json-rpc.address=0.0.0.0:%d", node.Ports.EVMRPC),
		fmt.Sprintf("--json-rpc.ws-address=0.0.0.0:%d", node.Ports.EVMWS),
	)

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
	containerRef := node.ContainerID
	if containerRef == "" {
		containerRef = node.ContainerName
	}
	if containerRef == "" {
		containerRef = ContainerNameForIndex(node.Index)
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
	if containerRef == "" {
		containerRef = ContainerNameForIndex(node.Index)
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
	if containerRef == "" {
		containerRef = ContainerNameForIndex(node.Index)
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

// ImagePullError represents an error when pulling a docker image fails.
type ImagePullError struct {
	Image   string
	Message string
	Output  string
}

func (e *ImagePullError) Error() string {
	return fmt.Sprintf("failed to pull docker image '%s': %s", e.Image, e.Message)
}

// ValidateImage validates that a docker image exists locally or can be pulled.
// First checks if the image exists locally to avoid unnecessary pull attempts.
// Returns a clear error message if the image cannot be found or pulled.
func (m *DockerManager) ValidateImage(ctx context.Context) error {
	m.Logger.Debug("Validating Docker image: %s", m.Image)

	// First, check if image exists locally
	checkCmd := exec.CommandContext(ctx, "docker", "images", "-q", m.Image)
	checkOutput, err := checkCmd.Output()
	if err == nil && len(strings.TrimSpace(string(checkOutput))) > 0 {
		m.Logger.Debug("Docker image found locally: %s", m.Image)
		return nil
	}

	// Image not found locally, try to pull
	m.Logger.Debug("Image not found locally, attempting to pull: %s", m.Image)
	cmd := exec.CommandContext(ctx, "docker", "pull", m.Image)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	if err != nil {
		// Parse error to provide helpful message
		if strings.Contains(outputStr, "not found") ||
			strings.Contains(outputStr, "manifest unknown") ||
			strings.Contains(outputStr, "does not exist") {
			return &ImagePullError{
				Image:   m.Image,
				Message: "image not found in registry",
				Output:  outputStr,
			}
		}
		if strings.Contains(outputStr, "unauthorized") ||
			strings.Contains(outputStr, "denied") {
			return &ImagePullError{
				Image:   m.Image,
				Message: "authentication required or access denied",
				Output:  outputStr,
			}
		}
		if strings.Contains(outputStr, "timeout") ||
			strings.Contains(outputStr, "connection refused") {
			return &ImagePullError{
				Image:   m.Image,
				Message: "registry connection failed (check network)",
				Output:  outputStr,
			}
		}
		return &ImagePullError{
			Image:   m.Image,
			Message: err.Error(),
			Output:  outputStr,
		}
	}

	return nil
}

// IsDockerAvailable checks if Docker is available and running.
func IsDockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// isGHCRImage returns true if the image is from GitHub Container Registry.
// GHCR images have stabled as entrypoint, so we don't need to prefix commands.
func (m *DockerManager) isGHCRImage() bool {
	return strings.HasPrefix(m.Image, "ghcr.io/")
}

// Init runs `stabled init` for a node in Docker.
func (m *DockerManager) Init(ctx context.Context, nodeDir, moniker, chainID string) error {
	args := []string{
		"run", "--rm",
		"--user", getCurrentUserID(),
		"-e", "HOME=/home/stabled",
		"-v", fmt.Sprintf("%s:/home/stabled", nodeDir),
		m.Image,
	}
	// GHCR images have stabled as entrypoint, others need explicit command
	if !m.isGHCRImage() {
		args = append(args, "stabled")
	}
	args = append(args, "init", moniker,
		"--chain-id", chainID,
		"--home", "/home/stabled",
	)

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
		"--user", getCurrentUserID(),
		"-e", "HOME=/home/stabled",
		"-v", fmt.Sprintf("%s:/home/stabled", nodeDir),
		m.Image,
	}
	// GHCR images have stabled as entrypoint, others need explicit command
	if !m.isGHCRImage() {
		args = append(args, "stabled")
	}
	args = append(args, "comet", "show-node-id",
		"--home", "/home/stabled",
	)

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
		"--user", getCurrentUserID(),
		"-e", "HOME=/home/stabled",
		"-v", fmt.Sprintf("%s:/home/stabled", nodeDir),
		m.Image,
	}
	// GHCR images have stabled as entrypoint, others need explicit command
	if !m.isGHCRImage() {
		args = append(args, "stabled")
	}
	args = append(args, "export",
		"--home", "/home/stabled",
	)

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
