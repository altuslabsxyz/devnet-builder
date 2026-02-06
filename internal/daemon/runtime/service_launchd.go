//go:build darwin

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

const (
	launchdLabelPrefix = "com.altuslabs.devnet."
	launchdPlistDir    = "Library/LaunchAgents"
)

func newPlatformServiceBackend() (ServiceBackend, error) {
	return newLaunchdBackend()
}

// launchdBackend implements ServiceBackend using macOS launchd.
type launchdBackend struct {
	plistDir string
	uid      int
	logger   *slog.Logger
}

func newLaunchdBackend() (*launchdBackend, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	plistDir := filepath.Join(home, launchdPlistDir)
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plist directory: %w", err)
	}

	return &launchdBackend{
		plistDir: plistDir,
		uid:      os.Getuid(),
		logger:   slog.Default(),
	}, nil
}

func (b *launchdBackend) ServiceID(nodeID string) string {
	return launchdLabelPrefix + nodeID
}

func (b *launchdBackend) plistPath(serviceID string) string {
	return filepath.Join(b.plistDir, serviceID+".plist")
}

func (b *launchdBackend) domainTarget() string {
	return fmt.Sprintf("gui/%d", b.uid)
}

func (b *launchdBackend) serviceTarget(serviceID string) string {
	return fmt.Sprintf("gui/%d/%s", b.uid, serviceID)
}

// InstallService generates a plist file and loads it into launchd.
func (b *launchdBackend) InstallService(ctx context.Context, def *ServiceDefinition) error {
	plist, err := b.renderPlist(def)
	if err != nil {
		return fmt.Errorf("failed to render plist: %w", err)
	}

	path := b.plistPath(def.ID)
	if err := os.WriteFile(path, plist, 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Bootstrap the service into launchd (modern API, macOS 10.10+)
	if err := b.runLaunchctl(ctx, "bootstrap", b.domainTarget(), path); err != nil {
		// Fallback to legacy load for older macOS
		if loadErr := b.runLaunchctl(ctx, "load", path); loadErr != nil {
			os.Remove(path)
			return fmt.Errorf("failed to load service: %w (bootstrap: %v)", loadErr, err)
		}
	}

	return nil
}

// UninstallService unloads the service and removes the plist file.
func (b *launchdBackend) UninstallService(ctx context.Context, serviceID string) error {
	// Bootout from launchd (stops if running and unloads)
	_ = b.runLaunchctl(ctx, "bootout", b.serviceTarget(serviceID))

	// Remove plist file
	path := b.plistPath(serviceID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

// StartService starts an installed service via launchctl kickstart.
func (b *launchdBackend) StartService(ctx context.Context, serviceID string) error {
	return b.runLaunchctl(ctx, "kickstart", b.serviceTarget(serviceID))
}

// StopService stops a running service.
func (b *launchdBackend) StopService(ctx context.Context, serviceID string, force bool) error {
	sig := "SIGTERM"
	if force {
		sig = "SIGKILL"
	}
	return b.runLaunchctl(ctx, "kill", sig, b.serviceTarget(serviceID))
}

// RestartService restarts a service via kickstart -k (kill + restart).
func (b *launchdBackend) RestartService(ctx context.Context, serviceID string) error {
	return b.runLaunchctl(ctx, "kickstart", "-k", b.serviceTarget(serviceID))
}

// GetServiceStatus queries launchd for the service status.
func (b *launchdBackend) GetServiceStatus(ctx context.Context, serviceID string) (*ServiceStatus, error) {
	output, err := b.runLaunchctlOutput(ctx, "print", b.serviceTarget(serviceID))
	if err != nil {
		// Service not loaded = not running
		return &ServiceStatus{Running: false}, nil
	}
	return parseLaunchctlPrint(output), nil
}

// IsInstalled checks if the plist file exists.
func (b *launchdBackend) IsInstalled(_ context.Context, serviceID string) (bool, error) {
	_, err := os.Stat(b.plistPath(serviceID))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// runLaunchctl executes a launchctl command.
func (b *launchdBackend) runLaunchctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "launchctl", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl %s: %w (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

// runLaunchctlOutput executes a launchctl command and returns stdout.
func (b *launchdBackend) runLaunchctlOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "launchctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("launchctl %s: %w", strings.Join(args, " "), err)
	}
	return string(output), nil
}

// parseLaunchctlPrint parses the output of `launchctl print gui/<uid>/<label>`.
func parseLaunchctlPrint(output string) *ServiceStatus {
	status := &ServiceStatus{}

	// Parse PID: "pid = 12345" or "pid = 0"
	pidRe := regexp.MustCompile(`(?m)^\s*pid\s*=\s*(\d+)`)
	if m := pidRe.FindStringSubmatch(output); len(m) > 1 {
		if pid, err := strconv.Atoi(m[1]); err == nil && pid > 0 {
			status.PID = pid
			status.Running = true
		}
	}

	// Parse state: "state = running" or "state = waiting"
	stateRe := regexp.MustCompile(`(?m)^\s*state\s*=\s*(\S+)`)
	if m := stateRe.FindStringSubmatch(output); len(m) > 1 {
		status.Running = m[1] == "running"
	}

	// Parse last exit code: "last exit code = 0"
	exitRe := regexp.MustCompile(`(?m)^\s*last exit code\s*=\s*(-?\d+)`)
	if m := exitRe.FindStringSubmatch(output); len(m) > 1 {
		if code, err := strconv.Atoi(m[1]); err == nil {
			status.ExitCode = code
		}
	}

	return status
}

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{ .ID }}</string>
	<key>ProgramArguments</key>
	<array>
{{- range .Command }}
		<string>{{ . }}</string>
{{- end }}
	</array>
	<key>WorkingDirectory</key>
	<string>{{ .WorkingDirectory }}</string>
{{- if .Environment }}
	<key>EnvironmentVariables</key>
	<dict>
{{- range $k, $v := .Environment }}
		<key>{{ $k }}</key>
		<string>{{ $v }}</string>
{{- end }}
	</dict>
{{- end }}
	<key>StandardOutPath</key>
	<string>{{ .StdoutPath }}</string>
	<key>StandardErrorPath</key>
	<string>{{ .StderrPath }}</string>
	<key>RunAtLoad</key>
	<false/>
{{- if .RestartOnFailure }}
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
{{- end }}
	<key>ExitTimeOut</key>
	<integer>{{ .GracePeriodSeconds }}</integer>
</dict>
</plist>
`))

type plistData struct {
	*ServiceDefinition
	GracePeriodSeconds int
}

// renderPlist generates a launchd plist XML from a service definition.
func (b *launchdBackend) renderPlist(def *ServiceDefinition) ([]byte, error) {
	data := plistData{
		ServiceDefinition:  def,
		GracePeriodSeconds: int(def.GracePeriod.Seconds()),
	}
	if data.GracePeriodSeconds <= 0 {
		data.GracePeriodSeconds = 30
	}

	var buf bytes.Buffer
	if err := plistTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}
	return buf.Bytes(), nil
}
