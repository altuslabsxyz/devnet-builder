package controller

import (
	"sync"
	"testing"
	"time"
)

func TestDevnetController_SubscribeProvisionLogs_ReturnsChannel(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	ch := c.SubscribeProvisionLogs("default", "test-devnet")
	if ch == nil {
		t.Fatal("expected non-nil channel from SubscribeProvisionLogs")
	}

	// Verify we can receive from the channel (it's a receive-only type)
	// Don't actually try to receive since nothing was sent
	// Just verify the channel exists and is non-nil
}

func TestDevnetController_SubscribeProvisionLogs_MultipleSubscribers(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	ch1 := c.SubscribeProvisionLogs("default", "test-devnet")
	ch2 := c.SubscribeProvisionLogs("default", "test-devnet")
	ch3 := c.SubscribeProvisionLogs("default", "other-devnet")

	if ch1 == nil || ch2 == nil || ch3 == nil {
		t.Fatal("expected non-nil channels")
	}

	// All channels should be different
	if ch1 == ch2 {
		t.Error("expected different channels for different subscribers")
	}
}

func TestDevnetController_BroadcastLog_SendsToAllSubscribers(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	ch1 := c.SubscribeProvisionLogs("default", "test-devnet")
	ch2 := c.SubscribeProvisionLogs("default", "test-devnet")

	entry := &ProvisionLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
		Phase:     "provisioning",
	}

	// Broadcast in goroutine to avoid blocking
	go c.broadcastLog("default", "test-devnet", entry)

	// Both subscribers should receive the entry
	var received1, received2 *ProvisionLogEntry

	select {
	case received1 = <-ch1:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ch1")
	}

	select {
	case received2 = <-ch2:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ch2")
	}

	if received1.Message != entry.Message {
		t.Errorf("ch1: expected message %q, got %q", entry.Message, received1.Message)
	}
	if received2.Message != entry.Message {
		t.Errorf("ch2: expected message %q, got %q", entry.Message, received2.Message)
	}
}

func TestDevnetController_BroadcastLog_OnlySendsToMatchingDevnet(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	ch1 := c.SubscribeProvisionLogs("default", "devnet-1")
	ch2 := c.SubscribeProvisionLogs("default", "devnet-2")

	entry := &ProvisionLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
		Phase:     "provisioning",
	}

	// Broadcast to devnet-1 only
	go c.broadcastLog("default", "devnet-1", entry)

	// ch1 should receive
	select {
	case received := <-ch1:
		if received.Message != entry.Message {
			t.Errorf("expected message %q, got %q", entry.Message, received.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for ch1")
	}

	// ch2 should NOT receive
	select {
	case <-ch2:
		t.Error("ch2 should not receive message for devnet-1")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message for devnet-2
	}
}

func TestDevnetController_UnsubscribeProvisionLogs_RemovesChannel(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	ch := c.SubscribeProvisionLogs("default", "test-devnet")

	// Unsubscribe
	c.UnsubscribeProvisionLogs("default", "test-devnet", ch)

	entry := &ProvisionLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
		Phase:     "provisioning",
	}

	// Broadcast should not block (no subscribers)
	done := make(chan struct{})
	go func() {
		c.broadcastLog("default", "test-devnet", entry)
		close(done)
	}()

	select {
	case <-done:
		// Expected - broadcast completed without blocking
	case <-time.After(100 * time.Millisecond):
		t.Fatal("broadcastLog blocked after unsubscribe")
	}

	// Channel should NOT receive any new messages after unsubscribe
	// (broadcasts skip unsubscribed channels via the done signal)
	select {
	case <-ch:
		t.Error("unsubscribed channel should not receive messages")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message received
	}
}

func TestDevnetController_BroadcastLog_DoesNotBlockOnSlowConsumer(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	// Create a subscriber but don't read from it
	_ = c.SubscribeProvisionLogs("default", "test-devnet")

	entry := &ProvisionLogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "test message",
		Phase:     "provisioning",
	}

	// Broadcast should not block even with slow consumer
	done := make(chan struct{})
	go func() {
		// Send multiple messages quickly
		for i := 0; i < 10; i++ {
			c.broadcastLog("default", "test-devnet", entry)
		}
		close(done)
	}()

	select {
	case <-done:
		// Expected - broadcasts completed without blocking
	case <-time.After(500 * time.Millisecond):
		t.Fatal("broadcastLog blocked on slow consumer")
	}
}

func TestDevnetController_BroadcastLog_ConcurrentSafe(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Spawn goroutines that subscribe, broadcast, and unsubscribe concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ch := c.SubscribeProvisionLogs("default", "test-devnet")
			entry := &ProvisionLogEntry{
				Timestamp: time.Now(),
				Level:     "info",
				Message:   "concurrent test",
				Phase:     "provisioning",
			}
			c.broadcastLog("default", "test-devnet", entry)
			c.UnsubscribeProvisionLogs("default", "test-devnet", ch)
		}(i)
	}

	// This should complete without race conditions or deadlocks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for concurrent operations")
	}
}

func TestProvisionLogEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := &ProvisionLogEntry{
		Timestamp: now,
		Level:     "warn",
		Message:   "test warning",
		Phase:     "configuration",
	}

	if entry.Timestamp != now {
		t.Errorf("expected timestamp %v, got %v", now, entry.Timestamp)
	}
	if entry.Level != "warn" {
		t.Errorf("expected level %q, got %q", "warn", entry.Level)
	}
	if entry.Message != "test warning" {
		t.Errorf("expected message %q, got %q", "test warning", entry.Message)
	}
	if entry.Phase != "configuration" {
		t.Errorf("expected phase %q, got %q", "configuration", entry.Phase)
	}
}

func TestProvisionLogEntry_ProgressFields(t *testing.T) {
	entry := &ProvisionLogEntry{
		StepName:        "Downloading snapshot",
		StepStatus:      "running",
		ProgressCurrent: 500 * 1024 * 1024,
		ProgressTotal:   1000 * 1024 * 1024,
		ProgressUnit:    "bytes",
		Speed:           50 * 1024 * 1024,
	}

	if entry.StepName != "Downloading snapshot" {
		t.Errorf("expected step name %q, got %q", "Downloading snapshot", entry.StepName)
	}
	if entry.StepStatus != "running" {
		t.Errorf("expected step status %q, got %q", "running", entry.StepStatus)
	}
	if entry.ProgressCurrent != 500*1024*1024 {
		t.Errorf("expected progress current %d, got %d", 500*1024*1024, entry.ProgressCurrent)
	}
	if entry.ProgressTotal != 1000*1024*1024 {
		t.Errorf("expected progress total %d, got %d", 1000*1024*1024, entry.ProgressTotal)
	}
	if entry.ProgressUnit != "bytes" {
		t.Errorf("expected progress unit %q, got %q", "bytes", entry.ProgressUnit)
	}
	if entry.Speed != 50*1024*1024 {
		t.Errorf("expected speed %f, got %f", float64(50*1024*1024), entry.Speed)
	}
}

func TestDevnetController_BroadcastLog_PreservesSpeedField(t *testing.T) {
	c := &DevnetController{}
	c.initLogSubscribers()

	ch := c.SubscribeProvisionLogs("default", "test-devnet")

	entry := &ProvisionLogEntry{
		Timestamp:       time.Now(),
		Level:           "info",
		Message:         "Downloading snapshot",
		Phase:           "genesis-fork",
		StepName:        "Downloading snapshot",
		StepStatus:      "running",
		ProgressCurrent: 100 * 1024 * 1024,
		ProgressTotal:   500 * 1024 * 1024,
		ProgressUnit:    "bytes",
		Speed:           25.5 * 1024 * 1024, // 25.5 MB/s
	}

	go c.broadcastLog("default", "test-devnet", entry)

	select {
	case received := <-ch:
		if received.Speed != entry.Speed {
			t.Errorf("expected speed %f, got %f", entry.Speed, received.Speed)
		}
		if received.ProgressCurrent != entry.ProgressCurrent {
			t.Errorf("expected progress current %d, got %d", entry.ProgressCurrent, received.ProgressCurrent)
		}
		if received.ProgressTotal != entry.ProgressTotal {
			t.Errorf("expected progress total %d, got %d", entry.ProgressTotal, received.ProgressTotal)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast")
	}
}
