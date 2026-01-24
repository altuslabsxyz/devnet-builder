package controller

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
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
