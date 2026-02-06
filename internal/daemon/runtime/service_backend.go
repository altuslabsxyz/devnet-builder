package runtime

import (
	"context"
	"time"
)

// ServiceBackend abstracts platform-specific service manager operations.
// launchd (macOS) and systemd (Linux) each implement this interface.
type ServiceBackend interface {
	// InstallService writes the service definition file and loads it
	// into the service manager. Does not start the service.
	InstallService(ctx context.Context, def *ServiceDefinition) error

	// UninstallService stops the service (if running), unloads it from
	// the service manager, and removes the service definition file.
	UninstallService(ctx context.Context, serviceID string) error

	// StartService starts an installed service.
	StartService(ctx context.Context, serviceID string) error

	// StopService stops a running service.
	// If force is true, sends SIGKILL immediately.
	// If force is false, sends the configured stop signal and lets the
	// service manager handle escalation after the grace period.
	StopService(ctx context.Context, serviceID string, force bool) error

	// RestartService restarts a running service (stop + start atomically).
	RestartService(ctx context.Context, serviceID string) error

	// GetServiceStatus queries the service manager for current status.
	GetServiceStatus(ctx context.Context, serviceID string) (*ServiceStatus, error)

	// IsInstalled checks if a service definition file exists for the given ID.
	IsInstalled(ctx context.Context, serviceID string) (bool, error)

	// ServiceID returns the platform-specific service identifier for a node.
	// Example: "com.altuslabs.devnet.mynode" (launchd) or "devnet-mynode.service" (systemd).
	ServiceID(nodeID string) string
}

// ServiceDefinition contains all the information needed to create a
// service definition file (plist for launchd, unit file for systemd).
type ServiceDefinition struct {
	// ID is the service identifier (platform-specific label/unit name).
	ID string

	// NodeID is the internal node identifier used by devnetd.
	NodeID string

	// Command is the full command to execute (binary + args).
	Command []string

	// WorkingDirectory is the working directory for the process.
	WorkingDirectory string

	// Environment is a map of environment variables.
	Environment map[string]string

	// StdoutPath is the path for stdout log output.
	StdoutPath string

	// StderrPath is the path for stderr log output.
	StderrPath string

	// RestartOnFailure controls whether the service manager restarts
	// the process after non-zero exit.
	RestartOnFailure bool

	// GracePeriod is how long to wait after stop signal before SIGKILL.
	GracePeriod time.Duration
}

// ServiceStatus represents the status returned by the service manager.
type ServiceStatus struct {
	Running   bool
	PID       int
	ExitCode  int
	StartedAt time.Time
}

// NewServiceBackend creates a ServiceBackend for the current platform.
// Implemented in service_launchd.go (darwin) and service_systemd.go (linux).
func NewServiceBackend() (ServiceBackend, error) {
	return newPlatformServiceBackend()
}
