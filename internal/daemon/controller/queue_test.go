package controller

import (
	"sync"
	"testing"
	"time"
)

func TestWorkQueue_AddAndGet(t *testing.T) {
	q := NewWorkQueue()
	defer q.ShutDown()

	q.Add("item1")
	q.Add("item2")

	item1, shutdown := q.Get()
	if shutdown {
		t.Fatal("unexpected shutdown")
	}
	if item1 != "item1" {
		t.Errorf("expected item1, got %v", item1)
	}

	item2, shutdown := q.Get()
	if shutdown {
		t.Fatal("unexpected shutdown")
	}
	if item2 != "item2" {
		t.Errorf("expected item2, got %v", item2)
	}
}

func TestWorkQueue_Deduplication(t *testing.T) {
	q := NewWorkQueue()
	defer q.ShutDown()

	// Add same item multiple times
	q.Add("item1")
	q.Add("item1")
	q.Add("item1")

	// Should only get it once
	item, _ := q.Get()
	if item != "item1" {
		t.Errorf("expected item1, got %v", item)
	}
	q.Done(item)

	// Queue should now be empty, add another item to verify
	q.Add("item2")
	item2, _ := q.Get()
	if item2 != "item2" {
		t.Errorf("expected item2, got %v", item2)
	}
}

func TestWorkQueue_DeduplicationDuringProcessing(t *testing.T) {
	q := NewWorkQueue()
	defer q.ShutDown()

	q.Add("item1")
	item, _ := q.Get()
	if item != "item1" {
		t.Errorf("expected item1, got %v", item)
	}

	// While processing, add same item - should be queued
	q.Add("item1")

	// Mark done - item should be re-queued
	q.Done(item)

	// Should get item1 again
	item2, _ := q.Get()
	if item2 != "item1" {
		t.Errorf("expected item1 again, got %v", item2)
	}
}

func TestWorkQueue_Requeue(t *testing.T) {
	q := NewWorkQueue()
	defer q.ShutDown()

	q.Add("item1")
	item, _ := q.Get()

	// Requeue explicitly
	q.Requeue(item)

	// Should get same item again
	item2, _ := q.Get()
	if item2 != "item1" {
		t.Errorf("expected item1 after requeue, got %v", item2)
	}
}

func TestWorkQueue_Shutdown(t *testing.T) {
	q := NewWorkQueue()

	// Add item and get it
	q.Add("item1")
	_, _ = q.Get()

	// Shutdown in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		q.ShutDown()
	}()

	// Get should return shutdown
	_, shutdown := q.Get()
	if !shutdown {
		t.Error("expected shutdown signal")
	}
}

func TestWorkQueue_ShuttingDown(t *testing.T) {
	q := NewWorkQueue()

	if q.ShuttingDown() {
		t.Error("should not be shutting down initially")
	}

	q.ShutDown()

	if !q.ShuttingDown() {
		t.Error("should be shutting down after ShutDown()")
	}
}

func TestWorkQueue_Len(t *testing.T) {
	q := NewWorkQueue()
	defer q.ShutDown()

	if q.Len() != 0 {
		t.Errorf("expected empty queue, got len %d", q.Len())
	}

	q.Add("item1")
	q.Add("item2")

	if q.Len() != 2 {
		t.Errorf("expected len 2, got %d", q.Len())
	}

	_, _ = q.Get()

	if q.Len() != 1 {
		t.Errorf("expected len 1 after get, got %d", q.Len())
	}
}

func TestWorkQueue_ConcurrentAddGet(t *testing.T) {
	q := NewWorkQueue()
	defer q.ShutDown()

	const numItems = 100
	var wg sync.WaitGroup

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numItems; i++ {
			q.Add(i)
		}
	}()

	// Consumer
	received := make(map[interface{}]bool)
	var mu sync.Mutex
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numItems; i++ {
			item, shutdown := q.Get()
			if shutdown {
				return
			}
			mu.Lock()
			received[item] = true
			mu.Unlock()
			q.Done(item)
		}
	}()

	wg.Wait()

	// Should have received all items
	if len(received) != numItems {
		t.Errorf("expected %d unique items, got %d", numItems, len(received))
	}
}
