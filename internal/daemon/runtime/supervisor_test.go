// internal/daemon/runtime/supervisor_test.go
package runtime

import (
	"context"
	"testing"
	"time"
)

func TestSupervisorBackoff(t *testing.T) {
	policy := RestartPolicy{
		Policy:         "on-failure",
		MaxRestarts:    3,
		BackoffInitial: 100 * time.Millisecond,
		BackoffMax:     1 * time.Second,
		BackoffFactor:  2.0,
	}

	s := &supervisor{policy: policy}

	// Test backoff calculation
	b1 := s.calculateBackoff(0)
	if b1 != 100*time.Millisecond {
		t.Errorf("Expected 100ms, got %v", b1)
	}

	b2 := s.calculateBackoff(1)
	if b2 != 200*time.Millisecond {
		t.Errorf("Expected 200ms, got %v", b2)
	}

	b3 := s.calculateBackoff(2)
	if b3 != 400*time.Millisecond {
		t.Errorf("Expected 400ms, got %v", b3)
	}

	// Should cap at max
	b10 := s.calculateBackoff(10)
	if b10 != 1*time.Second {
		t.Errorf("Expected 1s (max), got %v", b10)
	}
}

func TestSupervisorShouldRestart(t *testing.T) {
	tests := []struct {
		name       string
		policy     string
		exitCode   int
		restarts   int
		maxRestart int
		expected   bool
	}{
		{"never policy", "never", 1, 0, 5, false},
		{"always policy", "always", 0, 0, 5, true},
		{"on-failure clean exit", "on-failure", 0, 0, 5, false},
		{"on-failure error exit", "on-failure", 1, 0, 5, true},
		{"max restarts exceeded", "on-failure", 1, 5, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &supervisor{
				policy: RestartPolicy{
					Policy:      tt.policy,
					MaxRestarts: tt.maxRestart,
				},
				restartCount: tt.restarts,
			}

			result := s.shouldRestart(tt.exitCode)
			if result != tt.expected {
				t.Errorf("shouldRestart() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSupervisorRunSimpleProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := newSupervisor(supervisorConfig{
		command: []string{"echo", "hello"},
		workDir: t.TempDir(),
		policy:  RestartPolicy{Policy: "never"},
	})

	// Start the supervisor
	go s.run(ctx)

	// Wait for process to complete
	time.Sleep(500 * time.Millisecond)

	status := s.status()
	if status.Running {
		t.Error("Process should have exited")
	}
	if status.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", status.ExitCode)
	}
}
