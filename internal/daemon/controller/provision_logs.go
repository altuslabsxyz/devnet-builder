package controller

import (
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// ProvisionLogEntry represents a single log entry from the provisioner.
// Field naming matches the proto definition and client package for consistency.
type ProvisionLogEntry struct {
	Timestamp time.Time // Matches proto field and client.ProvisionLogEntry
	Level     string    // "info", "warn", "error"
	Message   string
	Phase     string
	// Progress fields for detailed step tracking
	StepName        string  // "Downloading snapshot", "Extracting...", etc.
	StepStatus      string  // "running", "completed", "failed"
	ProgressCurrent int64   // Bytes downloaded, etc. (0 if indeterminate)
	ProgressTotal   int64   // Total bytes (0 if unknown)
	ProgressUnit    string  // "bytes", "files", "" for indeterminate
	StepDetail      string  // "from cache", etc.
	Speed           float64 // bytes per second (for download progress)
}

// logSubscriberBufferSize is the buffer size for log subscriber channels.
// This allows some tolerance for slow consumers before messages are dropped.
const logSubscriberBufferSize = 100

// logSubscriber wraps a channel with a done signal for safe concurrent access.
type logSubscriber struct {
	ch   chan *ProvisionLogEntry
	done chan struct{}
}

// initLogSubscribers initializes the log subscribers map if needed.
// This is safe to call multiple times.
func (c *DevnetController) initLogSubscribers() {
	c.logMu.Lock()
	defer c.logMu.Unlock()

	if c.logSubscribers == nil {
		c.logSubscribers = make(map[string][]*logSubscriber)
	}
}

// SubscribeProvisionLogs subscribes to provisioner log entries for a devnet.
// Returns a receive-only channel that will receive log entries.
// The caller must call UnsubscribeProvisionLogs when done to avoid leaks.
func (c *DevnetController) SubscribeProvisionLogs(namespace, name string) <-chan *ProvisionLogEntry {
	c.initLogSubscribers()

	sub := &logSubscriber{
		ch:   make(chan *ProvisionLogEntry, logSubscriberBufferSize),
		done: make(chan struct{}),
	}

	key := logSubscriberKey(namespace, name)
	c.logMu.Lock()
	c.logSubscribers[key] = append(c.logSubscribers[key], sub)
	c.logMu.Unlock()

	return sub.ch
}

// logSubscriberKey creates a composite key for log subscriptions.
// Uses types.DefaultNamespace when namespace is empty for consistency.
func logSubscriberKey(namespace, name string) string {
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return namespace + "/" + name
}

// UnsubscribeProvisionLogs unsubscribes from provisioner log entries.
// The done channel is closed to signal the subscriber should stop reading.
// The log channel is NOT closed here to avoid races with concurrent broadcasts.
// Consumers should select on both the log channel and their context for clean shutdown.
func (c *DevnetController) UnsubscribeProvisionLogs(namespace, name string, ch <-chan *ProvisionLogEntry) {
	c.logMu.Lock()
	defer c.logMu.Unlock()

	key := logSubscriberKey(namespace, name)
	subscribers := c.logSubscribers[key]
	for i, sub := range subscribers {
		if sub.ch == ch {
			// Signal done to stop broadcasts to this subscriber
			close(sub.done)
			// Remove from slice - broadcasts will no longer target this subscriber
			c.logSubscribers[key] = append(subscribers[:i], subscribers[i+1:]...)
			// Note: we intentionally do NOT close sub.ch here to avoid races
			// with concurrent broadcasts. The channel will be garbage collected.
			break
		}
	}
}

// broadcastLog sends a log entry to all subscribers for a devnet.
// This method is non-blocking: if a subscriber's buffer is full, the message is dropped.
func (c *DevnetController) broadcastLog(namespace, name string, entry *ProvisionLogEntry) {
	key := logSubscriberKey(namespace, name)
	c.logMu.RLock()
	subscribers := c.logSubscribers[key]
	// Make a copy of the slice to avoid holding the lock during sends
	subscribersCopy := make([]*logSubscriber, len(subscribers))
	copy(subscribersCopy, subscribers)
	c.logMu.RUnlock()

	for _, sub := range subscribersCopy {
		// Check if subscriber is still active before sending
		select {
		case <-sub.done:
			// Subscriber has been unsubscribed, skip
			continue
		default:
		}

		// Non-blocking send - drop message if buffer is full or subscriber done
		select {
		case sub.ch <- entry:
		case <-sub.done:
			// Subscriber unsubscribed while we were trying to send
		default:
			// Channel buffer full, log the dropped message
			if c.logger != nil {
				c.logger.Warn("provision log dropped due to slow consumer",
					"namespace", namespace,
					"devnet", name,
					"message", entry.Message)
			}
		}
	}
}

// BroadcastProvisionLog broadcasts a log entry to all subscribers for a devnet.
// This is the public API for components to emit provision logs.
func (c *DevnetController) BroadcastProvisionLog(namespace, name string, entry *ProvisionLogEntry) {
	c.broadcastLog(namespace, name, entry)
}
