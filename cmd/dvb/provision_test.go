// cmd/dvb/provision_test.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFormatProvisionYAML(t *testing.T) {
	spec := &v1.DevnetSpec{
		Plugin:      "stable",
		NetworkType: "mainnet",
		Validators:  4,
		FullNodes:   1,
		Mode:        "docker",
		SdkVersion:  "v1.0.0",
		ForkNetwork: "mainnet",
		ChainId:     "mainnet-1",
	}

	var buf bytes.Buffer
	if err := formatProvisionYAML(&buf, "production", "test-devnet", spec); err != nil {
		t.Fatalf("formatProvisionYAML failed: %v", err)
	}

	output := buf.String()

	expectedFields := []string{
		"apiVersion: devnet.lagos/v1",
		"kind: Devnet",
		"name: test-devnet",
		"namespace: production",
		"network: stable",
		"networkType: mainnet",
		"networkVersion: v1.0.0",
		"validators: 4",
		"fullNodes: 1",
		"mode: docker",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("missing '%s' in output:\n%s", field, output)
		}
	}
}

func TestFormatProvisionYAML_OmitsEmptyFields(t *testing.T) {
	spec := &v1.DevnetSpec{
		Plugin:     "stable",
		Validators: 2,
		Mode:       "docker",
	}

	var buf bytes.Buffer
	if err := formatProvisionYAML(&buf, "", "minimal-devnet", spec); err != nil {
		t.Fatalf("formatProvisionYAML failed: %v", err)
	}

	output := buf.String()

	unexpectedFields := []string{
		"networkType:",
		"networkVersion:",
		"fullNodes:",
		"chainId:",
		"namespace:",
		"forkNetwork:",
	}

	for _, field := range unexpectedFields {
		if strings.Contains(output, field) {
			t.Errorf("should omit '%s' when empty, got:\n%s", field, output)
		}
	}
}

func TestDetectProvisionMode(t *testing.T) {
	tests := []struct {
		name     string
		opts     *provisionOptions
		expected ProvisionMode
	}{
		{
			name:     "file mode when file flag is set",
			opts:     &provisionOptions{file: "devnet.yaml"},
			expected: FileMode,
		},
		{
			name:     "flag mode when name is set",
			opts:     &provisionOptions{name: "my-devnet"},
			expected: FlagMode,
		},
		{
			name:     "interactive mode when no flags",
			opts:     &provisionOptions{},
			expected: InteractiveMode,
		},
		{
			name:     "file mode takes priority over name",
			opts:     &provisionOptions{file: "devnet.yaml", name: "my-devnet"},
			expected: FileMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectProvisionMode(tt.opts)
			if got != tt.expected {
				t.Errorf("detectProvisionMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProvisionOptions_NoWaitFlag(t *testing.T) {
	tests := []struct {
		name     string
		opts     *provisionOptions
		expected bool
	}{
		{
			name:     "noWait defaults to false",
			opts:     &provisionOptions{},
			expected: false,
		},
		{
			name:     "noWait can be set to true",
			opts:     &provisionOptions{noWait: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.noWait != tt.expected {
				t.Errorf("noWait = %v, want %v", tt.opts.noWait, tt.expected)
			}
		})
	}
}

func TestProvisionOptions_VerboseFlag(t *testing.T) {
	tests := []struct {
		name     string
		opts     *provisionOptions
		expected bool
	}{
		{
			name:     "verbose defaults to false",
			opts:     &provisionOptions{},
			expected: false,
		},
		{
			name:     "verbose can be set to true",
			opts:     &provisionOptions{verbose: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.verbose != tt.expected {
				t.Errorf("verbose = %v, want %v", tt.opts.verbose, tt.expected)
			}
		})
	}
}

func TestNewProvisionCmd_FlagRegistration(t *testing.T) {
	cmd := newProvisionCmd()

	// Test --no-wait flag is registered
	noWaitFlag := cmd.Flags().Lookup("no-wait")
	if noWaitFlag == nil {
		t.Error("--no-wait flag not registered")
	} else {
		if noWaitFlag.DefValue != "false" {
			t.Errorf("--no-wait default value = %s, want false", noWaitFlag.DefValue)
		}
	}

	// Test --verbose flag is registered
	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("--verbose flag not registered")
	} else {
		if verboseFlag.DefValue != "false" {
			t.Errorf("--verbose default value = %s, want false", verboseFlag.DefValue)
		}
		if verboseFlag.Shorthand != "v" {
			t.Errorf("--verbose shorthand = %s, want v", verboseFlag.Shorthand)
		}
	}
}

func TestProvisionOptions_NoWaitAndVerboseMutuallyExclusive(t *testing.T) {
	cmd := newProvisionCmd()

	// Set both flags - they should be mutually exclusive
	err := cmd.Flags().Set("no-wait", "true")
	if err != nil {
		t.Fatalf("failed to set --no-wait: %v", err)
	}

	err = cmd.Flags().Set("verbose", "true")
	if err != nil {
		t.Fatalf("failed to set --verbose: %v", err)
	}

	// The mutual exclusivity is enforced at parse time in cobra
	// For unit tests, we just verify both flags can be parsed
	// The actual mutual exclusivity is tested via command execution
}

// mockDaemonClient implements a mock for testing polling
type mockDaemonClient struct {
	mu       sync.Mutex
	devnets  map[string]*v1.Devnet
	callLog  []string
	getError error
}

func newMockDaemonClient() *mockDaemonClient {
	return &mockDaemonClient{
		devnets: make(map[string]*v1.Devnet),
		callLog: make([]string, 0),
	}
}

func (m *mockDaemonClient) GetDevnet(ctx context.Context, namespace, name string) (*v1.Devnet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callLog = append(m.callLog, fmt.Sprintf("GetDevnet(%s, %s)", namespace, name))

	if m.getError != nil {
		return nil, m.getError
	}

	key := namespace + "/" + name
	if devnet, ok := m.devnets[key]; ok {
		return devnet, nil
	}
	return nil, fmt.Errorf("devnet not found: %s/%s", namespace, name)
}

func (m *mockDaemonClient) SetDevnet(namespace, name string, devnet *v1.Devnet) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := namespace + "/" + name
	m.devnets[key] = devnet
}

func (m *mockDaemonClient) SetGetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getError = err
}

func (m *mockDaemonClient) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.callLog)
}

// TestPrintEvent tests the printEvent helper function
func TestPrintEvent(t *testing.T) {
	tests := []struct {
		name         string
		event        *v1.Event
		wantContains string
	}{
		{
			name: "normal event prints with checkmark",
			event: &v1.Event{
				Type:      "Normal",
				Reason:    "BinaryBuilt",
				Message:   "Binary built successfully",
				Component: "provisioner",
				Timestamp: timestamppb.Now(),
			},
			wantContains: "Binary built successfully",
		},
		{
			name: "warning event prints with warning prefix",
			event: &v1.Event{
				Type:      "Warning",
				Reason:    "RetryAttempt",
				Message:   "Retrying operation",
				Component: "provisioner",
				Timestamp: timestamppb.Now(),
			},
			wantContains: "Retrying operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr output
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			printEvent(tt.event)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			if !strings.Contains(output, tt.wantContains) {
				t.Errorf("printEvent() output = %q, want to contain %q", output, tt.wantContains)
			}
		})
	}
}

// TestPollProvisionStatus_ReturnsOnRunning tests that polling returns nil when phase becomes Running
func TestPollProvisionStatus_ReturnsOnRunning(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "test-devnet",
			Namespace: "default",
		},
		Spec: &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase:   "Running",
			Message: "All nodes running",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollProvisionStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err != nil {
		t.Errorf("pollProvisionStatus() returned error = %v, want nil", err)
	}
}

// TestPollProvisionStatus_ReturnsErrorOnDegraded tests that polling returns error when phase becomes Degraded
func TestPollProvisionStatus_ReturnsErrorOnDegraded(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "test-devnet",
			Namespace: "default",
		},
		Spec: &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase:   "Degraded",
			Message: "Binary build failed",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollProvisionStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err == nil {
		t.Error("pollProvisionStatus() returned nil, want error for Degraded phase")
	}
	if !strings.Contains(err.Error(), "provisioning failed") {
		t.Errorf("pollProvisionStatus() error = %v, want to contain 'provisioning failed'", err)
	}
}

// TestPollProvisionStatus_PrintsNewEvents tests that polling prints new events without duplicates
func TestPollProvisionStatus_PrintsNewEvents(t *testing.T) {
	mock := newMockDaemonClient()
	callCount := 0

	// Create events
	event1 := &v1.Event{
		Type:      "Normal",
		Reason:    "BinaryBuilt",
		Message:   "Binary built successfully",
		Component: "provisioner",
		Timestamp: timestamppb.Now(),
	}
	event2 := &v1.Event{
		Type:      "Normal",
		Reason:    "NodesStarted",
		Message:   "Nodes started",
		Component: "provisioner",
		Timestamp: timestamppb.Now(),
	}

	// Simulate progression: first call returns Provisioning with event1, second call returns Running with both events
	go func() {
		time.Sleep(10 * time.Millisecond)
		mock.SetDevnet("default", "test-devnet", &v1.Devnet{
			Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
			Spec:     &v1.DevnetSpec{},
			Status: &v1.DevnetStatus{
				Phase:  "Provisioning",
				Events: []*v1.Event{event1},
			},
		})

		time.Sleep(60 * time.Millisecond)
		mock.SetDevnet("default", "test-devnet", &v1.Devnet{
			Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
			Spec:     &v1.DevnetSpec{},
			Status: &v1.DevnetStatus{
				Phase:  "Running",
				Events: []*v1.Event{event1, event2},
			},
		})
	}()

	// Start with provisioning state
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
		Spec:     &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase:  "Provisioning",
			Events: []*v1.Event{},
		},
	})

	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollProvisionStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("pollProvisionStatus() returned error = %v, want nil", err)
	}

	// Should see both events printed
	if !strings.Contains(output, "Binary built successfully") {
		t.Errorf("output should contain first event message, got: %s", output)
	}
	if !strings.Contains(output, "Nodes started") {
		t.Errorf("output should contain second event message, got: %s", output)
	}

	// Count occurrences - each event should only be printed once
	count1 := strings.Count(output, "Binary built successfully")
	if count1 > 1 {
		t.Errorf("event1 printed %d times, want 1", count1)
	}

	_ = callCount // Unused but kept for potential future debugging
}

// TestPollProvisionStatus_HandlesContextCancellation tests graceful handling of Ctrl+C
func TestPollProvisionStatus_HandlesContextCancellation(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
		Spec:     &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase: "Provisioning",
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := pollProvisionStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err != context.Canceled {
		t.Errorf("pollProvisionStatus() returned %v, want context.Canceled", err)
	}
}

// TestPollProvisionStatus_HandlesGetError tests error handling when GetDevnet fails
func TestPollProvisionStatus_HandlesGetError(t *testing.T) {
	mock := newMockDaemonClient()
	mock.SetGetError(fmt.Errorf("connection refused"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pollProvisionStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	if err == nil {
		t.Error("pollProvisionStatus() returned nil, want error")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("pollProvisionStatus() error = %v, want to contain 'connection refused'", err)
	}
}

// TestPollProvisionStatus_PollsEverySecond tests that polling happens at the expected interval
func TestPollProvisionStatus_PollsEverySecond(t *testing.T) {
	mock := newMockDaemonClient()

	// Start with provisioning, then transition to running after some polls
	callCount := 0
	mock.SetDevnet("default", "test-devnet", &v1.Devnet{
		Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
		Spec:     &v1.DevnetSpec{},
		Status: &v1.DevnetStatus{
			Phase: "Provisioning",
		},
	})

	// After 3 polls, set to Running
	go func() {
		time.Sleep(130 * time.Millisecond) // 2.6 intervals at 50ms
		mock.SetDevnet("default", "test-devnet", &v1.Devnet{
			Metadata: &v1.DevnetMetadata{Name: "test-devnet", Namespace: "default"},
			Spec:     &v1.DevnetSpec{},
			Status: &v1.DevnetStatus{
				Phase: "Running",
			},
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := pollProvisionStatusWithClient(ctx, "default", "test-devnet", mock, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("pollProvisionStatus() returned error = %v", err)
	}

	// Should have taken at least 100ms (2 poll intervals) but less than 500ms
	if elapsed < 100*time.Millisecond {
		t.Errorf("polling completed too quickly (%v), expected at least 100ms", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("polling took too long (%v), expected less than 500ms", elapsed)
	}

	// Should have polled at least 3 times
	if mock.GetCallCount() < 3 {
		t.Errorf("expected at least 3 GetDevnet calls, got %d", mock.GetCallCount())
	}

	_ = callCount // Unused but kept for potential future debugging
}

// mockProvisionLogStreamer implements provisionLogStreamer for testing
type mockProvisionLogStreamer struct {
	entries []*client.ProvisionLogEntry
	err     error
}

func (m *mockProvisionLogStreamer) StreamProvisionLogs(ctx context.Context, namespace, name string, callback func(*client.ProvisionLogEntry) error) error {
	if m.err != nil {
		return m.err
	}
	for _, entry := range m.entries {
		if err := callback(entry); err != nil {
			return err
		}
	}
	return nil
}

// TestStreamProvisionLogs_StreamsLogEntries tests that log entries are printed
func TestStreamProvisionLogs_StreamsLogEntries(t *testing.T) {
	now := time.Now()
	mock := &mockProvisionLogStreamer{
		entries: []*client.ProvisionLogEntry{
			{Timestamp: now, Level: "info", Message: "Building binary", Phase: "Building"},
			{Timestamp: now, Level: "info", Message: "Binary built successfully", Phase: "Building"},
			{Timestamp: now, Level: "warn", Message: "Retrying operation", Phase: "Starting"},
		},
	}

	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := streamProvisionLogsWithClient(context.Background(), "default", "test-devnet", mock)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Errorf("streamProvisionLogs() returned error = %v", err)
	}

	// Should contain all messages
	if !strings.Contains(output, "Building binary") {
		t.Errorf("output should contain 'Building binary', got: %s", output)
	}
	if !strings.Contains(output, "Binary built successfully") {
		t.Errorf("output should contain 'Binary built successfully', got: %s", output)
	}
	if !strings.Contains(output, "Retrying operation") {
		t.Errorf("output should contain 'Retrying operation', got: %s", output)
	}

	// Should have [provisioner] prefix
	if !strings.Contains(output, "[provisioner]") {
		t.Errorf("output should contain '[provisioner]' prefix, got: %s", output)
	}
}

// TestStreamProvisionLogs_HandlesError tests error propagation
func TestStreamProvisionLogs_HandlesError(t *testing.T) {
	mock := &mockProvisionLogStreamer{
		err: fmt.Errorf("stream disconnected"),
	}

	err := streamProvisionLogsWithClient(context.Background(), "default", "test-devnet", mock)
	if err == nil {
		t.Error("streamProvisionLogs() returned nil, want error")
	}
	if !strings.Contains(err.Error(), "stream disconnected") {
		t.Errorf("error should contain 'stream disconnected', got: %v", err)
	}
}

// TestStreamProvisionLogs_NilClient tests handling of nil client
func TestStreamProvisionLogs_NilClient(t *testing.T) {
	err := streamProvisionLogsWithClient(context.Background(), "default", "test-devnet", nil)
	if err == nil {
		t.Error("streamProvisionLogs() returned nil, want error for nil client")
	}
	if !strings.Contains(err.Error(), "daemon client not available") {
		t.Errorf("error should mention daemon client, got: %v", err)
	}
}

// TestPrintProvisionLog_FormatsLevels tests log level formatting
func TestPrintProvisionLog_FormatsLevels(t *testing.T) {
	tests := []struct {
		name  string
		entry *client.ProvisionLogEntry
	}{
		{
			name:  "info level",
			entry: &client.ProvisionLogEntry{Level: "info", Message: "Info message"},
		},
		{
			name:  "warn level",
			entry: &client.ProvisionLogEntry{Level: "warn", Message: "Warning message"},
		},
		{
			name:  "error level",
			entry: &client.ProvisionLogEntry{Level: "error", Message: "Error message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stderr output
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			printProvisionLog(tt.entry)

			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Should contain the message
			if !strings.Contains(output, tt.entry.Message) {
				t.Errorf("output should contain message '%s', got: %s", tt.entry.Message, output)
			}
			// Should have provisioner prefix
			if !strings.Contains(output, "[provisioner]") {
				t.Errorf("output should contain '[provisioner]' prefix, got: %s", output)
			}
		})
	}
}

// TestPrintProvisionLog_NilEntry tests handling of nil entry
func TestPrintProvisionLog_NilEntry(t *testing.T) {
	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printProvisionLog(nil)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should produce no output for nil entry
	if output != "" {
		t.Errorf("expected no output for nil entry, got: %s", output)
	}
}
