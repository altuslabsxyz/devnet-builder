//go:build linux

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	systemdUnitPrefix = "devnet-"
	systemdUnitSuffix = ".service"
	systemdUserDir    = ".config/systemd/user"
)

func newPlatformServiceBackend() (ServiceBackend, error) {
	return newSystemdBackend()
}

// systemdBackend implements ServiceBackend using Linux systemd user services.
type systemdBackend struct {
	unitDir string
	logger  *slog.Logger
}

func newSystemdBackend() (*systemdBackend, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	unitDir := filepath.Join(home, systemdUserDir)
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	return &systemdBackend{
		unitDir: unitDir,
		logger:  slog.Default(),
	}, nil
}

func (b *systemdBackend) ServiceID(nodeID string) string {
	return systemdUnitPrefix + nodeID + systemdUnitSuffix
}

func (b *systemdBackend) unitPath(serviceID string) string {
	return filepath.Join(b.unitDir, serviceID)
}

// InstallService writes a systemd unit file and reloads the daemon.
func (b *systemdBackend) InstallService(ctx context.Context, def *ServiceDefinition) error {
	unit, err := b.renderUnit(def)
	if err != nil {
		return fmt.Errorf("failed to render unit file: %w", err)
	}

	path := b.unitPath(def.ID)
	if err := os.WriteFile(path, unit, 0644); err != nil {
		return fmt.Errorf("failed to write unit file: %w", err)
	}

	if err := b.runSystemctl(ctx, "daemon-reload"); err != nil {
		os.Remove(path)
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

// UninstallService stops, disables, and removes the unit file.
func (b *systemdBackend) UninstallService(ctx context.Context, serviceID string) error {
	// Stop the service (ignore errors if not running)
	_ = b.runSystemctl(ctx, "stop", serviceID)

	// Disable the service (ignore errors if not enabled)
	_ = b.runSystemctl(ctx, "disable", serviceID)

	// Remove unit file
	path := b.unitPath(serviceID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}

	// Reload daemon config
	_ = b.runSystemctl(ctx, "daemon-reload")

	return nil
}

// StartService starts a systemd user service.
func (b *systemdBackend) StartService(ctx context.Context, serviceID string) error {
	return b.runSystemctl(ctx, "start", serviceID)
}

// StopService stops a running systemd user service.
func (b *systemdBackend) StopService(ctx context.Context, serviceID string, force bool) error {
	if force {
		return b.runSystemctl(ctx, "kill", "--signal=SIGKILL", serviceID)
	}
	return b.runSystemctl(ctx, "stop", serviceID)
}

// RestartService restarts a systemd user service.
func (b *systemdBackend) RestartService(ctx context.Context, serviceID string) error {
	return b.runSystemctl(ctx, "restart", serviceID)
}

// GetServiceStatus queries systemd for the service status.
func (b *systemdBackend) GetServiceStatus(ctx context.Context, serviceID string) (*ServiceStatus, error) {
	output, err := b.runSystemctlOutput(ctx, "show",
		"--property=ActiveState,MainPID,ExecMainStatus,ExecMainStartTimestamp",
		serviceID)
	if err != nil {
		return &ServiceStatus{Running: false}, nil
	}
	return parseSystemctlShow(output), nil
}

// IsInstalled checks if the unit file exists.
func (b *systemdBackend) IsInstalled(_ context.Context, serviceID string) (bool, error) {
	_, err := os.Stat(b.unitPath(serviceID))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// runSystemctl executes systemctl --user with the given arguments.
func (b *systemdBackend) runSystemctl(ctx context.Context, args ...string) error {
	fullArgs := append([]string{"--user"}, args...)
	cmd := exec.CommandContext(ctx, "systemctl", fullArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user %s: %w (output: %s)",
			strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

// runSystemctlOutput executes systemctl --user and returns stdout.
func (b *systemdBackend) runSystemctlOutput(ctx context.Context, args ...string) (string, error) {
	fullArgs := append([]string{"--user"}, args...)
	cmd := exec.CommandContext(ctx, "systemctl", fullArgs...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("systemctl --user %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

// parseSystemctlShow parses the output of systemctl show --property=... into ServiceStatus.
// Output format is Key=Value per line.
func parseSystemctlShow(output string) *ServiceStatus {
	status := &ServiceStatus{}
	props := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.IndexByte(line, '='); idx > 0 {
			props[line[:idx]] = line[idx+1:]
		}
	}

	// ActiveState: "active", "inactive", "failed", "activating", "deactivating"
	if state, ok := props["ActiveState"]; ok {
		status.Running = state == "active"
	}

	// MainPID
	if pidStr, ok := props["MainPID"]; ok {
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
			status.PID = pid
		}
	}

	// ExecMainStatus (exit code)
	if codeStr, ok := props["ExecMainStatus"]; ok {
		if code, err := strconv.Atoi(codeStr); err == nil {
			status.ExitCode = code
		}
	}

	// ExecMainStartTimestamp (e.g., "Thu 2025-01-01 12:00:00 UTC")
	if tsStr, ok := props["ExecMainStartTimestamp"]; ok && tsStr != "" {
		if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", tsStr); err == nil {
			status.StartedAt = t
		}
	}

	return status
}

var unitTemplate = template.Must(template.New("unit").Parse(`[Unit]
Description=Devnet node {{ .NodeID }}

[Service]
Type=simple
ExecStart={{ .ExecStart }}
WorkingDirectory={{ .WorkingDirectory }}
{{- range .EnvLines }}
Environment={{ . }}
{{- end }}
StandardOutput=append:{{ .StdoutPath }}
StandardError=append:{{ .StderrPath }}
{{- if .RestartOnFailure }}
Restart=on-failure
RestartSec=5
{{- end }}
TimeoutStopSec={{ .TimeoutStopSec }}
KillSignal=SIGTERM
`))

type unitData struct {
	*ServiceDefinition
	ExecStart      string
	EnvLines       []string
	TimeoutStopSec int
}

// renderUnit generates a systemd unit file from a service definition.
func (b *systemdBackend) renderUnit(def *ServiceDefinition) ([]byte, error) {
	// Build ExecStart: space-separated command
	execStart := strings.Join(def.Command, " ")

	// Build Environment lines: each as "KEY=VALUE" (quoted)
	var envLines []string
	for k, v := range def.Environment {
		envLines = append(envLines, fmt.Sprintf("%q", k+"="+v))
	}

	timeout := int(def.GracePeriod.Seconds())
	if timeout <= 0 {
		timeout = 30
	}

	data := unitData{
		ServiceDefinition: def,
		ExecStart:         execStart,
		EnvLines:          envLines,
		TimeoutStopSec:    timeout,
	}

	var buf bytes.Buffer
	if err := unitTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}
	return buf.Bytes(), nil
}
