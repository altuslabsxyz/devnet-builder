package controller

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
)

// mockController is a test controller that tracks reconcile calls
type mockController struct {
	reconcileCalls []string
	reconcileErr   error
	mu             sync.Mutex
}

func (m *mockController) Reconcile(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reconcileCalls = append(m.reconcileCalls, key)
	return m.reconcileErr
}

func (m *mockController) getCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.reconcileCalls))
	copy(result, m.reconcileCalls)
	return result
}

func TestManager_Register(t *testing.T) {
	m := NewManager()

	ctrl := &mockController{}
	m.Register("devnets", ctrl)

	if !m.HasController("devnets") {
		t.Error("expected controller to be registered")
	}

	if m.HasController("nodes") {
		t.Error("expected nodes controller to not be registered")
	}
}

func TestManager_Enqueue(t *testing.T) {
	m := NewManager()
	ctrl := &mockController{}
	m.Register("devnets", ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start manager with 1 worker
	go m.Start(ctx, 1)

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Enqueue items
	m.Enqueue("devnets", "my-devnet")
	m.Enqueue("devnets", "other-devnet")

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	calls := ctrl.getCalls()
	if len(calls) != 2 {
		t.Errorf("expected 2 reconcile calls, got %d", len(calls))
	}
}

func TestManager_EnqueueUnknownType(t *testing.T) {
	m := NewManager()

	// Should not panic when enqueueing unknown type
	m.Enqueue("unknown", "key")
}

func TestManager_RequeueOnError(t *testing.T) {
	m := NewManager()

	ctrl := &mockController{}
	ctrl.reconcileErr = errors.New("temporary error")

	m.Register("devnets", ctrl)

	ctx, cancel := context.WithCancel(context.Background())

	// Start manager
	go m.Start(ctx, 1)
	time.Sleep(10 * time.Millisecond)

	// Enqueue one item
	m.Enqueue("devnets", "failing-devnet")

	// Wait for multiple retries
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Should have been called multiple times due to requeue
	calls := ctrl.getCalls()
	if len(calls) < 2 {
		t.Errorf("expected multiple reconcile calls due to requeue, got %d", len(calls))
	}
}

func TestManager_MultipleControllers(t *testing.T) {
	m := NewManager()

	devnetCtrl := &mockController{}
	nodeCtrl := &mockController{}

	m.Register("devnets", devnetCtrl)
	m.Register("nodes", nodeCtrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.Start(ctx, 2)
	time.Sleep(10 * time.Millisecond)

	m.Enqueue("devnets", "devnet-1")
	m.Enqueue("nodes", "node-1")
	m.Enqueue("devnets", "devnet-2")

	time.Sleep(50 * time.Millisecond)

	devnetCalls := devnetCtrl.getCalls()
	nodeCalls := nodeCtrl.getCalls()

	if len(devnetCalls) != 2 {
		t.Errorf("expected 2 devnet reconcile calls, got %d", len(devnetCalls))
	}
	if len(nodeCalls) != 1 {
		t.Errorf("expected 1 node reconcile call, got %d", len(nodeCalls))
	}
}

func TestManager_GracefulShutdown(t *testing.T) {
	m := NewManager()
	ctrl := &mockController{}
	m.Register("devnets", ctrl)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.Start(ctx, 1)
	}()

	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Should complete without hanging
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("manager did not shut down gracefully")
	}
}

func TestManager_WorkersPerController(t *testing.T) {
	m := NewManager()

	// Controller that takes time to process
	slowCtrl := &mockController{}
	m.Register("slow", slowCtrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start with 3 workers
	go m.Start(ctx, 3)
	time.Sleep(10 * time.Millisecond)

	// Enqueue multiple items
	for i := 0; i < 5; i++ {
		m.Enqueue("slow", "item")
	}

	// Due to deduplication, only 1 should be processed initially
	time.Sleep(50 * time.Millisecond)

	calls := slowCtrl.getCalls()
	// With deduplication, we might get 1-2 calls depending on timing
	if len(calls) < 1 {
		t.Errorf("expected at least 1 call, got %d", len(calls))
	}
}

func TestManager_StopWithTimeout_Graceful(t *testing.T) {
	m := NewManager()
	ctrl := &mockController{}
	m.Register("devnets", ctrl)

	ctx, cancel := context.WithCancel(context.Background())

	// Start manager in background
	go m.Start(ctx, 1)
	time.Sleep(10 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// StopWithTimeout should return true (graceful) when workers stop in time
	graceful := m.StopWithTimeout(time.Second)
	if !graceful {
		t.Error("expected graceful shutdown")
	}
}

// blockingController blocks during Reconcile until cancelled
type blockingController struct {
	started chan struct{} // closed when Reconcile starts
}

func (c *blockingController) Reconcile(ctx context.Context, key string) error {
	close(c.started)
	// Block forever unless context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

func TestManager_StopWithTimeout_Timeout(t *testing.T) {
	m := NewManager()
	blocking := &blockingController{started: make(chan struct{})}
	m.Register("devnets", blocking)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start manager in background
	go m.Start(ctx, 1)
	time.Sleep(10 * time.Millisecond)

	// Enqueue an item that will block
	m.Enqueue("devnets", "blocking-item")

	// Wait for the blocking controller to start processing
	select {
	case <-blocking.started:
		// Good, controller is now blocking
	case <-time.After(time.Second):
		t.Fatal("controller did not start")
	}

	// StopWithTimeout should return false (timeout) since worker is blocked
	// and context isn't cancelled
	graceful := m.StopWithTimeout(50 * time.Millisecond)
	if graceful {
		t.Error("expected timeout, not graceful shutdown")
	}
}

func TestManager_SubscribeProvisionLogs_NoDevnetController(t *testing.T) {
	m := NewManager()

	// No devnets controller registered
	ch := m.SubscribeProvisionLogs("default", "test-devnet")
	if ch != nil {
		t.Error("expected nil channel when no devnets controller is registered")
	}
}

func TestManager_SubscribeProvisionLogs_WrongControllerType(t *testing.T) {
	m := NewManager()

	// Register a mock controller (not DevnetController)
	ctrl := &mockController{}
	m.Register("devnets", ctrl)

	ch := m.SubscribeProvisionLogs("default", "test-devnet")
	if ch != nil {
		t.Error("expected nil channel when wrong controller type is registered")
	}
}

func TestManager_SubscribeProvisionLogs_Success(t *testing.T) {
	m := NewManager()
	s := store.NewMemoryStore()
	devnetCtrl := NewDevnetController(s, nil)
	m.Register("devnets", devnetCtrl)

	ch := m.SubscribeProvisionLogs("default", "test-devnet")
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Clean up
	m.UnsubscribeProvisionLogs("default", "test-devnet", ch)
}

func TestManager_UnsubscribeProvisionLogs_NoController(t *testing.T) {
	m := NewManager()

	// Should not panic when no controller is registered
	m.UnsubscribeProvisionLogs("default", "test-devnet", nil)
}

func TestManager_UnsubscribeProvisionLogs_WrongControllerType(t *testing.T) {
	m := NewManager()

	ctrl := &mockController{}
	m.Register("devnets", ctrl)

	// Should not panic when wrong controller type
	m.UnsubscribeProvisionLogs("default", "test-devnet", nil)
}

func TestManager_SubscribeProvisionLogs_ReceivesLogs(t *testing.T) {
	m := NewManager()
	s := store.NewMemoryStore()
	devnetCtrl := NewDevnetController(s, nil)
	m.Register("devnets", devnetCtrl)

	ch := m.SubscribeProvisionLogs("default", "test-devnet")
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	defer m.UnsubscribeProvisionLogs("default", "test-devnet", ch)

	// Broadcast a log entry via the controller
	entry := &ProvisionLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
		Phase:     "testing",
	}
	devnetCtrl.broadcastLog("default", "test-devnet", entry)

	// Should receive the log entry
	select {
	case received := <-ch:
		if received.Message != "test message" {
			t.Errorf("expected message 'test message', got %q", received.Message)
		}
		if received.Phase != "testing" {
			t.Errorf("expected phase 'testing', got %q", received.Phase)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for log entry")
	}
}
