package ports

import (
	"context"
	"time"
)

// Command represents a command to be executed.
type Command struct {
	Binary  string
	Args    []string
	WorkDir string
	Env     []string
	LogPath string
	PIDPath string
}

// ProcessHandle represents a running process.
type ProcessHandle interface {
	// PID returns the process ID.
	PID() int

	// IsRunning checks if the process is still running.
	IsRunning() bool

	// Wait blocks until the process exits.
	Wait() error

	// Kill terminates the process.
	Kill() error
}

// ProcessExecutor defines operations for executing processes.
// This abstraction allows switching between local processes and Docker containers.
type ProcessExecutor interface {
	// Start launches a new process.
	Start(ctx context.Context, cmd Command) (ProcessHandle, error)

	// Stop gracefully stops a process with timeout.
	Stop(ctx context.Context, handle ProcessHandle, timeout time.Duration) error

	// Kill forcefully terminates a process.
	Kill(handle ProcessHandle) error

	// IsRunning checks if a process is running.
	IsRunning(handle ProcessHandle) bool

	// Logs retrieves the last N lines from the process log.
	Logs(handle ProcessHandle, lines int) ([]string, error)
}

// DockerHandle represents a running Docker container.
type DockerHandle interface {
	ProcessHandle

	// ContainerID returns the Docker container ID.
	ContainerID() string

	// ContainerName returns the container name.
	ContainerName() string
}

// DockerExecutor extends ProcessExecutor with Docker-specific operations.
type DockerExecutor interface {
	ProcessExecutor

	// RunContainer starts a Docker container.
	RunContainer(ctx context.Context, config ContainerConfig) (DockerHandle, error)

	// StopContainer stops a Docker container.
	StopContainer(ctx context.Context, containerID string, timeout time.Duration) error

	// RemoveContainer removes a Docker container.
	RemoveContainer(ctx context.Context, containerID string, force bool) error

	// ContainerLogs retrieves container logs.
	ContainerLogs(ctx context.Context, containerID string, lines int) ([]string, error)

	// ExecInContainer executes a command inside a running container.
	ExecInContainer(ctx context.Context, containerID string, cmd []string) ([]byte, error)
}

// ContainerConfig holds Docker container configuration.
type ContainerConfig struct {
	Image       string
	Name        string
	Cmd         []string
	Env         []string
	Volumes     []VolumeMount
	Ports       []PortMapping
	NetworkMode string
	AutoRemove  bool
	Detach      bool
}

// VolumeMount represents a Docker volume mount.
type VolumeMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// PortMapping represents a Docker port mapping.
type PortMapping struct {
	Host      int
	Container int
	Protocol  string // "tcp" or "udp"
}

// NodeManager manages the lifecycle of a single node.
type NodeManager interface {
	// Start starts the node.
	Start(ctx context.Context) error

	// Stop stops the node.
	Stop(ctx context.Context, timeout time.Duration) error

	// IsRunning checks if the node is running.
	IsRunning() bool

	// GetPID returns the process ID (for local mode).
	GetPID() *int

	// GetContainerID returns the container ID (for docker mode).
	GetContainerID() string

	// Logs retrieves the last N lines of logs.
	Logs(lines int) ([]string, error)

	// LogPath returns the path to the log file.
	LogPath() string
}
