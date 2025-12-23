package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// DockerExecutorImpl implements DockerExecutor for Docker container execution.
type DockerExecutorImpl struct {
	*LocalExecutor // Embed for local process fallback
}

// NewDockerExecutor creates a new DockerExecutorImpl.
func NewDockerExecutor() *DockerExecutorImpl {
	return &DockerExecutorImpl{
		LocalExecutor: NewLocalExecutor(),
	}
}

// dockerHandle implements DockerHandle for Docker containers.
type dockerHandle struct {
	containerID   string
	containerName string
	logPath       string
}

// PID returns 0 for Docker containers (no direct PID).
func (h *dockerHandle) PID() int {
	return 0
}

// IsRunning checks if the container is running.
func (h *dockerHandle) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", h.containerID)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "true"
}

// Wait blocks until the container exits.
func (h *dockerHandle) Wait() error {
	cmd := exec.Command("docker", "wait", h.containerID)
	return cmd.Run()
}

// Kill terminates the container.
func (h *dockerHandle) Kill() error {
	cmd := exec.Command("docker", "kill", h.containerID)
	return cmd.Run()
}

// ContainerID returns the Docker container ID.
func (h *dockerHandle) ContainerID() string {
	return h.containerID
}

// ContainerName returns the container name.
func (h *dockerHandle) ContainerName() string {
	return h.containerName
}

// RunContainer starts a Docker container.
func (e *DockerExecutorImpl) RunContainer(ctx context.Context, config ports.ContainerConfig) (ports.DockerHandle, error) {
	args := []string{"run", "-d"}

	// Container name
	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}

	// Environment variables
	for _, env := range config.Env {
		args = append(args, "-e", env)
	}

	// Volume mounts
	for _, vol := range config.Volumes {
		mountSpec := fmt.Sprintf("%s:%s", vol.Source, vol.Target)
		if vol.ReadOnly {
			mountSpec += ":ro"
		}
		args = append(args, "-v", mountSpec)
	}

	// Port mappings
	for _, port := range config.Ports {
		proto := port.Protocol
		if proto == "" {
			proto = "tcp"
		}
		args = append(args, "-p", fmt.Sprintf("%d:%d/%s", port.Host, port.Container, proto))
	}

	// Network mode
	if config.NetworkMode != "" {
		args = append(args, "--network", config.NetworkMode)
	}

	// Auto remove
	if config.AutoRemove {
		args = append(args, "--rm")
	}

	// Run as current user
	uid := os.Getuid()
	gid := os.Getgid()
	args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid))

	// Image
	args = append(args, config.Image)

	// Command
	args = append(args, config.Cmd...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &ContainerError{
			ContainerID: config.Name,
			Operation:   "run",
			Message:     fmt.Sprintf("%v: %s", err, string(output)),
		}
	}

	containerID := strings.TrimSpace(string(output))

	return &dockerHandle{
		containerID:   containerID,
		containerName: config.Name,
	}, nil
}

// StopContainer stops a Docker container.
func (e *DockerExecutorImpl) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	timeoutSec := int(timeout.Seconds())
	if timeoutSec < 1 {
		timeoutSec = 30
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", "-t", fmt.Sprintf("%d", timeoutSec), containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "No such container") {
			return nil // Already stopped/removed
		}
		return &ContainerError{
			ContainerID: containerID,
			Operation:   "stop",
			Message:     fmt.Sprintf("%v: %s", err, outputStr),
		}
	}

	return nil
}

// RemoveContainer removes a Docker container.
func (e *DockerExecutorImpl) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "No such container") {
			return nil // Already removed
		}
		return &ContainerError{
			ContainerID: containerID,
			Operation:   "remove",
			Message:     fmt.Sprintf("%v: %s", err, outputStr),
		}
	}

	return nil
}

// ContainerLogs retrieves container logs.
func (e *DockerExecutorImpl) ContainerLogs(ctx context.Context, containerID string, lines int) ([]string, error) {
	args := []string{"logs"}
	if lines > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", lines))
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &ContainerError{
			ContainerID: containerID,
			Operation:   "logs",
			Message:     fmt.Sprintf("%v: %s", err, string(output)),
		}
	}

	logLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return logLines, nil
}

// ExecInContainer executes a command inside a running container.
func (e *DockerExecutorImpl) ExecInContainer(ctx context.Context, containerID string, command []string) ([]byte, error) {
	args := append([]string{"exec", containerID}, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &ContainerError{
			ContainerID: containerID,
			Operation:   "exec",
			Message:     fmt.Sprintf("%v: %s", err, string(output)),
		}
	}

	return output, nil
}

// Start launches a process (delegates to local executor for non-Docker commands).
func (e *DockerExecutorImpl) Start(ctx context.Context, cmd ports.Command) (ports.ProcessHandle, error) {
	return e.LocalExecutor.Start(ctx, cmd)
}

// Stop gracefully stops a process.
func (e *DockerExecutorImpl) Stop(ctx context.Context, handle ports.ProcessHandle, timeout time.Duration) error {
	// Check if it's a Docker handle
	if dh, ok := handle.(*dockerHandle); ok {
		return e.StopContainer(ctx, dh.containerID, timeout)
	}
	return e.LocalExecutor.Stop(ctx, handle, timeout)
}

// Kill forcefully terminates a process.
func (e *DockerExecutorImpl) Kill(handle ports.ProcessHandle) error {
	if dh, ok := handle.(*dockerHandle); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return e.RemoveContainer(ctx, dh.containerID, true)
	}
	return e.LocalExecutor.Kill(handle)
}

// IsRunning checks if a process is running.
func (e *DockerExecutorImpl) IsRunning(handle ports.ProcessHandle) bool {
	return handle.IsRunning()
}

// Logs retrieves logs from a process.
func (e *DockerExecutorImpl) Logs(handle ports.ProcessHandle, lines int) ([]string, error) {
	if dh, ok := handle.(*dockerHandle); ok {
		return e.ContainerLogs(context.Background(), dh.containerID, lines)
	}
	return e.LocalExecutor.Logs(handle, lines)
}

// PullImage pulls a Docker image.
func (e *DockerExecutorImpl) PullImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ImageError{
			Image:   image,
			Message: fmt.Sprintf("pull failed: %v: %s", err, string(output)),
		}
	}
	return nil
}

// ImageExists checks if a Docker image exists locally.
func (e *DockerExecutorImpl) ImageExists(ctx context.Context, image string) bool {
	cmd := exec.CommandContext(ctx, "docker", "images", "-q", image)
	output, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(output))) > 0
}

// IsDockerAvailable checks if Docker is available and running.
func IsDockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}

// FindContainerByName finds a container by name and returns a handle.
func (e *DockerExecutorImpl) FindContainerByName(ctx context.Context, name string) (ports.DockerHandle, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.Id}}", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, &ContainerError{
			ContainerID: name,
			Operation:   "find",
			Message:     "container not found",
		}
	}

	containerID := strings.TrimSpace(string(output))
	return &dockerHandle{
		containerID:   containerID,
		containerName: name,
	}, nil
}

// Ensure DockerExecutorImpl implements DockerExecutor.
var _ ports.DockerExecutor = (*DockerExecutorImpl)(nil)
