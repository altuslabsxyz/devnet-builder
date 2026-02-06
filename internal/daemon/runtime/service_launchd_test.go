//go:build darwin

package runtime

import (
	"strings"
	"testing"
	"time"
)

func TestRenderPlist(t *testing.T) {
	b := &launchdBackend{plistDir: "/tmp/test"}

	def := &ServiceDefinition{
		ID:               "com.altuslabs.devnet.testnode",
		NodeID:           "testnode",
		Command:          []string{"/usr/bin/stabled", "start", "--home", "/data/node0"},
		WorkingDirectory: "/data/node0",
		Environment: map[string]string{
			"HOME":   "/data/node0",
			"MYVAR":  "myval",
		},
		StdoutPath:       "/var/log/testnode.log",
		StderrPath:       "/var/log/testnode.err",
		RestartOnFailure: true,
		GracePeriod:      45 * time.Second,
	}

	plist, err := b.renderPlist(def)
	if err != nil {
		t.Fatalf("renderPlist failed: %v", err)
	}

	content := string(plist)

	// Check required elements
	checks := []string{
		"<string>com.altuslabs.devnet.testnode</string>",
		"<string>/usr/bin/stabled</string>",
		"<string>start</string>",
		"<string>--home</string>",
		"<string>/data/node0</string>",
		"<key>WorkingDirectory</key>",
		"<key>EnvironmentVariables</key>",
		"<key>StandardOutPath</key>",
		"<string>/var/log/testnode.log</string>",
		"<key>StandardErrorPath</key>",
		"<string>/var/log/testnode.err</string>",
		"<key>RunAtLoad</key>",
		"<false/>",
		"<key>KeepAlive</key>",
		"<key>SuccessfulExit</key>",
		"<key>ExitTimeOut</key>",
		"<integer>45</integer>",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("plist missing expected content: %s", check)
		}
	}
}

func TestRenderPlistNoRestart(t *testing.T) {
	b := &launchdBackend{plistDir: "/tmp/test"}

	def := &ServiceDefinition{
		ID:               "com.altuslabs.devnet.norestart",
		NodeID:           "norestart",
		Command:          []string{"sleep", "60"},
		WorkingDirectory: "/tmp",
		StdoutPath:       "/dev/null",
		StderrPath:       "/dev/null",
		RestartOnFailure: false,
		GracePeriod:      10 * time.Second,
	}

	plist, err := b.renderPlist(def)
	if err != nil {
		t.Fatalf("renderPlist failed: %v", err)
	}

	content := string(plist)

	// Should NOT contain KeepAlive when RestartOnFailure is false
	if strings.Contains(content, "KeepAlive") {
		t.Error("plist should not contain KeepAlive when RestartOnFailure is false")
	}
}

func TestRenderPlistDefaultGracePeriod(t *testing.T) {
	b := &launchdBackend{plistDir: "/tmp/test"}

	def := &ServiceDefinition{
		ID:               "com.altuslabs.devnet.default-grace",
		NodeID:           "default-grace",
		Command:          []string{"sleep", "60"},
		WorkingDirectory: "/tmp",
		StdoutPath:       "/dev/null",
		StderrPath:       "/dev/null",
		GracePeriod:      0, // Should default to 30
	}

	plist, err := b.renderPlist(def)
	if err != nil {
		t.Fatalf("renderPlist failed: %v", err)
	}

	if !strings.Contains(string(plist), "<integer>30</integer>") {
		t.Error("expected default grace period of 30 seconds")
	}
}

func TestParseLaunchctlPrint(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected ServiceStatus
	}{
		{
			name: "running",
			output: `com.altuslabs.devnet.mynode = {
	active count = 1
	path = /Users/test/Library/LaunchAgents/com.altuslabs.devnet.mynode.plist
	state = running
	pid = 54321
	last exit code = 0
}`,
			expected: ServiceStatus{Running: true, PID: 54321, ExitCode: 0},
		},
		{
			name: "waiting",
			output: `com.altuslabs.devnet.mynode = {
	active count = 0
	state = waiting
	pid = 0
	last exit code = 1
}`,
			expected: ServiceStatus{Running: false, PID: 0, ExitCode: 1},
		},
		{
			name:     "empty output",
			output:   "",
			expected: ServiceStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := parseLaunchctlPrint(tt.output)

			if status.Running != tt.expected.Running {
				t.Errorf("Running = %v, want %v", status.Running, tt.expected.Running)
			}
			if status.PID != tt.expected.PID {
				t.Errorf("PID = %d, want %d", status.PID, tt.expected.PID)
			}
			if status.ExitCode != tt.expected.ExitCode {
				t.Errorf("ExitCode = %d, want %d", status.ExitCode, tt.expected.ExitCode)
			}
		})
	}
}

func TestServiceIDFormat(t *testing.T) {
	b := &launchdBackend{}
	id := b.ServiceID("my-test-node")
	if id != "com.altuslabs.devnet.my-test-node" {
		t.Errorf("ServiceID = %q, want %q", id, "com.altuslabs.devnet.my-test-node")
	}
}
