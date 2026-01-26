// internal/daemon/runtime/supervisor.go
package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// supervisorConfig contains configuration for a supervisor
type supervisorConfig struct {
	command     []string
	workDir     string
	env         map[string]string
	policy      RestartPolicy
	logWriter   io.Writer
	stopSignal  syscall.Signal
	gracePeriod time.Duration
	logger      *slog.Logger
}

// supervisor manages a single process with restart logic
type supervisor struct {
	config       supervisorConfig
	policy       RestartPolicy

	cmd          *exec.Cmd
	pid          int
	running      bool
	startedAt    time.Time
	lastExitCode int
	lastError    string
	restartCount int

	stopCh    chan struct{}
	stoppedCh chan struct{}
	mu        sync.RWMutex
}

// newSupervisor creates a new process supervisor
func newSupervisor(config supervisorConfig) *supervisor {
	if config.stopSignal == 0 {
		config.stopSignal = syscall.SIGTERM
	}
	if config.gracePeriod == 0 {
		config.gracePeriod = 10 * time.Second
	}
	if config.policy.BackoffInitial == 0 {
		config.policy = DefaultRestartPolicy()
	}

	return &supervisor{
		config:    config,
		policy:    config.policy,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// run starts the supervisor loop
func (s *supervisor) run(ctx context.Context) {
	defer close(s.stoppedCh)

	for {
		select {
		case <-ctx.Done():
			s.stopProcess()
			return
		case <-s.stopCh:
			s.stopProcess()
			return
		default:
		}

		// Start the process
		exitCode, err := s.startAndWait(ctx)

		s.mu.Lock()
		s.running = false
		s.lastExitCode = exitCode
		if err != nil {
			s.lastError = err.Error()
		}
		s.mu.Unlock()

		// Check if we should restart
		if !s.shouldRestart(exitCode) {
			return
		}

		s.restartCount++

		// Calculate backoff
		backoff := s.calculateBackoff(s.restartCount - 1)

		if s.config.logger != nil {
			s.config.logger.Info("restarting process",
				"restartCount", s.restartCount,
				"backoff", backoff,
				"exitCode", exitCode,
			)
		}

		// Wait with backoff
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-time.After(backoff):
		}
	}
}

// startAndWait starts the process and waits for it to exit
func (s *supervisor) startAndWait(ctx context.Context) (int, error) {
	s.mu.Lock()

	// Build command
	cmd := exec.CommandContext(ctx, s.config.command[0], s.config.command[1:]...)
	cmd.Dir = s.config.workDir

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.config.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set up output
	if s.config.logWriter != nil {
		cmd.Stdout = s.config.logWriter
		cmd.Stderr = s.config.logWriter
	}

	// Start process
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		return -1, fmt.Errorf("failed to start process: %w", err)
	}

	s.cmd = cmd
	s.pid = cmd.Process.Pid
	s.running = true
	s.startedAt = time.Now()
	s.mu.Unlock()

	// Wait for exit
	err := cmd.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return exitCode, err
}

// stopProcess stops the running process
func (s *supervisor) stopProcess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd == nil || s.cmd.Process == nil {
		return
	}

	// Send graceful signal
	s.cmd.Process.Signal(s.config.stopSignal)

	// Wait for graceful shutdown
	done := make(chan struct{})
	go func() {
		s.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(s.config.gracePeriod):
		// Force kill
		s.cmd.Process.Signal(syscall.SIGKILL)
	}
}

// stop signals the supervisor to stop
func (s *supervisor) stop() {
	close(s.stopCh)
	<-s.stoppedCh
}

// status returns the current status
func (s *supervisor) status() NodeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return NodeStatus{
		Running:   s.running,
		PID:       s.pid,
		StartedAt: s.startedAt,
		ExitCode:  s.lastExitCode,
		Restarts:  s.restartCount,
		LastError: s.lastError,
		UpdatedAt: time.Now(),
	}
}

// shouldRestart determines if the process should be restarted
func (s *supervisor) shouldRestart(exitCode int) bool {
	switch s.policy.Policy {
	case "never":
		return false
	case "always":
		if s.policy.MaxRestarts > 0 && s.restartCount >= s.policy.MaxRestarts {
			return false
		}
		return true
	case "on-failure":
		if exitCode == 0 {
			return false
		}
		if s.policy.MaxRestarts > 0 && s.restartCount >= s.policy.MaxRestarts {
			return false
		}
		return true
	default:
		return false
	}
}

// calculateBackoff calculates the backoff duration for a restart
func (s *supervisor) calculateBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	backoff := s.policy.BackoffInitial
	for i := 0; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * s.policy.BackoffFactor)
		if backoff > s.policy.BackoffMax {
			return s.policy.BackoffMax
		}
	}

	return backoff
}
