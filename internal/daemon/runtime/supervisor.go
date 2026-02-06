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

	// detachOnShutdown indicates the supervisor should detach (not kill process)
	// when stop() is called. Used for graceful devnetd shutdown where processes
	// should persist as orphans.
	detachOnShutdown bool
}

// supervisor manages a single process with restart logic
type supervisor struct {
	config supervisorConfig
	policy RestartPolicy

	cmd          *exec.Cmd
	pid          int
	running      bool
	startedAt    time.Time
	lastExitCode int
	lastError    string
	restartCount int

	stopCh    chan struct{}
	stoppedCh chan struct{}
	stopOnce  sync.Once
	mu        sync.RWMutex

	// detachMode when true causes stop to detach instead of kill.
	// Set dynamically via setDetachMode() before shutdown.
	detachMode bool
}

// newSupervisor creates a new process supervisor
func newSupervisor(config supervisorConfig) *supervisor {
	// Validate command - must have at least one element
	if len(config.command) == 0 {
		config.command = []string{"/bin/false"} // Will fail immediately but won't panic
	}

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
			s.maybeStopProcess()
			return
		case <-s.stopCh:
			s.maybeStopProcess()
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

		// Increment restart count under lock to avoid race condition
		s.mu.Lock()
		s.restartCount++
		restartCount := s.restartCount
		s.mu.Unlock()

		// Calculate backoff
		backoff := s.calculateBackoff(restartCount - 1)

		if s.config.logger != nil {
			s.config.logger.Info("restarting process",
				"restartCount", restartCount,
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

	// Build command - use exec.Command instead of exec.CommandContext
	// to avoid SIGKILL race condition (CommandContext sends SIGKILL on context cancellation)
	cmd := exec.Command(s.config.command[0], s.config.command[1:]...)
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

// maybeStopProcess stops the process unless detach mode is enabled.
// In detach mode, the process continues running as an orphan.
func (s *supervisor) maybeStopProcess() {
	s.mu.RLock()
	detach := s.detachMode || s.config.detachOnShutdown
	pid := s.pid // Read PID under lock to avoid race
	s.mu.RUnlock()

	if detach {
		if s.config.logger != nil {
			s.config.logger.Info("detaching from process (will continue as orphan)",
				"pid", pid)
		}
		return
	}
	s.stopProcess()
}

// stopProcess stops the running process
// Note: This function only signals the process to stop. The actual Wait() is handled
// by startAndWait() to avoid double-Wait on the same exec.Cmd (undefined behavior).
func (s *supervisor) stopProcess() {
	s.mu.Lock()
	if s.cmd == nil || s.cmd.Process == nil {
		s.mu.Unlock()
		return
	}
	process := s.cmd.Process
	gracePeriod := s.config.gracePeriod
	stopSignal := s.config.stopSignal
	s.mu.Unlock()

	// Send graceful signal
	_ = process.Signal(stopSignal)

	// Start a goroutine to force kill after grace period if process hasn't exited
	// The actual wait is handled by startAndWait(), this just ensures we escalate to SIGKILL
	go func() {
		time.Sleep(gracePeriod)
		// Process may have already exited, Signal will return error which we ignore
		_ = process.Signal(syscall.SIGKILL)
	}()
}

// stop signals the supervisor to stop gracefully
// Uses sync.Once to prevent double-close panic on stopCh
func (s *supervisor) stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	<-s.stoppedCh
}

// forceStop immediately kills the process with SIGKILL
func (s *supervisor) forceStop() {
	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Signal(syscall.SIGKILL)
	}
	s.mu.Unlock()

	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	<-s.stoppedCh
}

// setDetachMode enables or disables detach mode.
// When detach mode is enabled, stop() will detach from the process
// instead of killing it. The process continues running as an orphan.
func (s *supervisor) setDetachMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.detachMode = enabled
}

// detach marks the supervisor for detach mode.
// When the supervisor eventually stops (process exits or stop is called),
// it will NOT kill the process - the process continues running as an orphan.
// This returns immediately without waiting for the supervisor goroutine.
// Used for graceful devnetd shutdown where devnets should persist.
func (s *supervisor) detach() {
	s.setDetachMode(true)
	// Don't close stopCh - let the supervisor continue monitoring.
	// Don't wait - the process keeps running and supervisor will exit
	// when the process eventually dies on its own.
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

// newMonitoringSupervisor creates a supervisor for an already-running process.
// This supervisor only monitors the process - it doesn't manage lifecycle or restart.
// Used when reconnecting to orphaned processes after devnetd restart.
func newMonitoringSupervisor(pid int, logWriter io.Writer, logger *slog.Logger) *supervisor {
	return &supervisor{
		config: supervisorConfig{
			logWriter:        logWriter,
			logger:           logger,
			detachOnShutdown: true, // Always detach, never kill reconnected processes
		},
		pid:        pid,
		running:    true,
		startedAt:  time.Now(), // We don't know actual start time
		stopCh:     make(chan struct{}),
		stoppedCh:  make(chan struct{}),
		detachMode: true, // Always detach mode for reconnected processes
	}
}

// runMonitor monitors an already-running process without managing its lifecycle.
// Used for reconnected processes where we can't use cmd.Wait().
// Polls for process death and updates running status.
func (s *supervisor) runMonitor(ctx context.Context) {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - just exit, don't kill the process
			return
		case <-s.stopCh:
			// Stop requested - check if we should kill or detach
			s.mu.RLock()
			detach := s.detachMode || s.config.detachOnShutdown
			pid := s.pid // Read PID under lock to avoid race
			s.mu.RUnlock()

			if !detach && pid > 0 {
				// Send SIGTERM to the process
				if proc, err := os.FindProcess(pid); err == nil {
					_ = proc.Signal(syscall.SIGTERM)
				}
			}
			return
		case <-ticker.C:
			// Check if process is still alive using signal 0
			proc, err := os.FindProcess(s.pid)
			if err != nil {
				s.setProcessDead("process not found: " + err.Error())
				return
			}
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				s.setProcessDead("process exited: " + err.Error())
				return
			}
		}
	}
}

// setProcessDead marks the process as no longer running
func (s *supervisor) setProcessDead(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pid := s.pid
	s.running = false
	s.pid = 0 // Clear PID to prevent stale references
	s.lastError = reason
	if s.config.logger != nil {
		s.config.logger.Info("monitored process died",
			"pid", pid,
			"reason", reason)
	}
}
