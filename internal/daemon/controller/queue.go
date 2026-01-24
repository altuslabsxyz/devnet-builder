// Package controller provides the controller manager and work queue for the daemon.
package controller

import (
	"sync"
)

// WorkQueue is a rate-limited deduplicating work queue.
// Items added while an item is being processed will be re-queued
// when Done is called. This is inspired by the Kubernetes workqueue.
type WorkQueue struct {
	// queue is the ordered list of items to process
	queue []interface{}

	// dirty tracks items that need processing
	dirty map[interface{}]struct{}

	// processing tracks items currently being processed
	processing map[interface{}]struct{}

	// cond is used to signal when items are added
	cond *sync.Cond

	// shuttingDown indicates the queue is shutting down
	shuttingDown bool

	mu sync.Mutex
}

// NewWorkQueue creates a new work queue.
func NewWorkQueue() *WorkQueue {
	q := &WorkQueue{
		queue:      make([]interface{}, 0),
		dirty:      make(map[interface{}]struct{}),
		processing: make(map[interface{}]struct{}),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Add marks an item as needing processing. If the item is already
// in the queue or being processed, it will be re-queued when Done
// is called for that item.
func (q *WorkQueue) Add(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.shuttingDown {
		return
	}

	// Mark as dirty
	if _, exists := q.dirty[item]; exists {
		// Already dirty, nothing to do
		return
	}
	q.dirty[item] = struct{}{}

	// If being processed, it will be re-added when Done is called
	if _, exists := q.processing[item]; exists {
		return
	}

	// Add to queue
	q.queue = append(q.queue, item)
	q.cond.Signal()
}

// Get blocks until an item is ready, returning the item and whether
// the queue is shutting down.
func (q *WorkQueue) Get() (interface{}, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.queue) == 0 && !q.shuttingDown {
		q.cond.Wait()
	}

	if q.shuttingDown {
		return nil, true
	}

	// Pop from front
	item := q.queue[0]
	q.queue = q.queue[1:]

	// Move from dirty to processing
	delete(q.dirty, item)
	q.processing[item] = struct{}{}

	return item, false
}

// Done marks an item as done processing. If the item was re-added
// while being processed, it will be re-queued.
func (q *WorkQueue) Done(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.processing, item)

	// If item was marked dirty while processing, re-add to queue
	if _, exists := q.dirty[item]; exists {
		q.queue = append(q.queue, item)
		q.cond.Signal()
	}
}

// Requeue adds the item back to the queue after processing fails.
// This is equivalent to calling Done() then Add().
func (q *WorkQueue) Requeue(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.processing, item)

	if q.shuttingDown {
		return
	}

	// Add back to dirty set and queue
	q.dirty[item] = struct{}{}
	q.queue = append(q.queue, item)
	q.cond.Signal()
}

// Len returns the number of items in the queue.
func (q *WorkQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// ShutDown signals the queue to stop processing.
func (q *WorkQueue) ShutDown() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.shuttingDown = true
	q.cond.Broadcast()
}

// ShuttingDown returns true if the queue is shutting down.
func (q *WorkQueue) ShuttingDown() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.shuttingDown
}
