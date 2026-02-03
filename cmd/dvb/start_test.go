// cmd/dvb/start_test.go
package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// Note: mockDaemonClient is defined in provision_test.go and shared across test files.

func TestNewStartCmd_FlagRegistration(t *testing.T) {
	cmd := newStartCmd()

	// Check that all expected flags are registered
	flags := []struct {
		name      string
		shorthand string
	}{
		{"namespace", "n"},
		{"no-wait", ""},
		{"verbose", "v"},
		{"force", "f"},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("flag --%s not registered", f.name)
			continue
		}
		if f.shorthand != "" && flag.Shorthand != f.shorthand {
			t.Errorf("flag --%s shorthand = %q, want %q", f.name, flag.Shorthand, f.shorthand)
		}
	}
}

func TestNewStartCmd_DefaultFlags(t *testing.T) {
	cmd := newStartCmd()

	tests := []struct {
		name     string
		flagName string
		want     string
	}{
		{"namespace defaults to empty", "namespace", ""},
		{"no-wait defaults to false", "no-wait", "false"},
		{"verbose defaults to false", "verbose", "false"},
		{"force defaults to false", "force", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("flag --%s not found", tt.flagName)
			}
			if flag.DefValue != tt.want {
				t.Errorf("flag --%s default = %q, want %q", tt.flagName, flag.DefValue, tt.want)
			}
		})
	}
}

// TestPollStartStatus_ReturnsOnRunning tests that polling returns nil when phase becomes Running
func TestPollStartStatus_ReturnsOnRunning(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "test-devnet",
			Namespace: "default",
		},
		Spec: &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase:      types.PhaseRunning,
			Nodes:      3,
			ReadyNodes: 3,
			Message:    "All nodes running",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollStartStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err != nil {
		t.Errorf("pollStartStatus() returned error = %v, want nil", err)
	}
}

// TestPollStartStatus_ReturnsErrorOnDegraded tests that polling returns error when phase becomes Degraded
func TestPollStartStatus_ReturnsErrorOnDegraded(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "test-devnet",
			Namespace: "default",
		},
		Spec: &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase:   types.PhaseDegraded,
			Message: "Node failed to start",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollStartStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err == nil {
		t.Error("pollStartStatus() returned nil, want error for Degraded phase")
	}
	if !strings.Contains(err.Error(), "degraded") {
		t.Errorf("pollStartStatus() error = %v, want to contain 'degraded'", err)
	}
}

// TestPollStartStatus_ReturnsErrorOnStopped tests that polling returns error when phase becomes Stopped
func TestPollStartStatus_ReturnsErrorOnStopped(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "test-devnet",
			Namespace: "default",
		},
		Spec: &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase:   types.PhaseStopped,
			Message: "Devnet was stopped",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollStartStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err == nil {
		t.Error("pollStartStatus() returned nil, want error for Stopped phase")
	}
	if !strings.Contains(err.Error(), "stopped unexpectedly") {
		t.Errorf("pollStartStatus() error = %v, want to contain 'stopped unexpectedly'", err)
	}
}

// TestPollStartStatus_ContinuesOnPending tests that polling continues when phase is Pending
func TestPollStartStatus_ContinuesOnPending(t *testing.T) {
	mock := newMockDaemonClient()

	// Start with Pending, then transition to Running
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
		Spec:     &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase: types.PhasePending,
		},
	})

	// After some polls, set to Running
	go func() {
		time.Sleep(130 * time.Millisecond) // ~2.6 intervals at 50ms
		mock.SetDevnet("default", "test-devnet", &v1.Devnet{
			Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
			Spec:     &v1.DevnetSpec{},
			Status: &v1.DevnetStatus{
				Phase: types.PhaseRunning,
			},
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollStartStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err != nil {
		t.Errorf("pollStartStatus() returned error = %v, want nil", err)
	}

	// Should have polled at least 3 times (Pending -> Pending -> Running)
	if mock.GetCallCount() < 3 {
		t.Errorf("expected at least 3 GetDevnet calls, got %d", mock.GetCallCount())
	}
}

// TestPollStartStatus_HandlesGetError tests error handling when GetDevnet fails
func TestPollStartStatus_HandlesGetError(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetGetError(fmt.Errorf("connection refused"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollStartStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err == nil {
		t.Error("pollStartStatus() returned nil, want error")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("pollStartStatus() error = %v, want to contain 'connection refused'", err)
	}
}

// TestPollStartStatus_HandlesContextCancellation tests context cancellation
func TestPollStartStatus_HandlesContextCancellation(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
		Spec:     &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase: types.PhasePending, // Never transitions to Running
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pollStartStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err == nil {
		t.Error("pollStartStatus() returned nil, want context deadline error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("pollStartStatus() error = %v, want to contain 'context deadline exceeded'", err)
	}
}

// TestPollStartStatus_TransitionsFromProvisioning tests transition from Provisioning to Running
func TestPollStartStatus_TransitionsFromProvisioning(t *testing.T) {
	mock := newMockDaemonClient()

	// Start with Provisioning
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
		Spec:     &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase: types.PhaseProvisioning,
		},
	})

	// Transition to Running
	go func() {
		time.Sleep(80 * time.Millisecond)
		mock.SetDevnet("default", "test-devnet", &v1.Devnet{
			Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
			Spec:     &v1.DevnetSpec{},
			Status: &v1.DevnetStatus{
				Phase:      types.PhaseRunning,
				Nodes:      2,
				ReadyNodes: 2,
			},
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollStartStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err != nil {
		t.Errorf("pollStartStatus() returned error = %v, want nil", err)
	}
}
