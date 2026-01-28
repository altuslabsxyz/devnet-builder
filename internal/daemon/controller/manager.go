package controller

import (
	"context"
	"log/slog"
	"sync"
)

// Controller defines the interface for resource controllers.
type Controller interface {
	// Reconcile processes a single item by key.
	// Return an error to requeue the item.
	Reconcile(ctx context.Context, key string) error
}

// Manager manages multiple controllers and their work queues.
type Manager struct {
	controllers map[string]Controller
	queues      map[string]*WorkQueue
	mu          sync.RWMutex
	logger      *slog.Logger

	// Shutdown coordination
	stopOnce sync.Once
	stopped  chan struct{} // Closed when all workers have stopped
}

// NewManager creates a new controller manager.
func NewManager() *Manager {
	return &Manager{
		controllers: make(map[string]Controller),
		queues:      make(map[string]*WorkQueue),
		logger:      slog.Default(),
		stopped:     make(chan struct{}),
	}
}

// SetLogger sets the logger for the manager.
func (m *Manager) SetLogger(logger *slog.Logger) {
	m.logger = logger
}

// Register adds a controller for a resource type.
func (m *Manager) Register(resourceType string, ctrl Controller) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.controllers[resourceType] = ctrl
	m.queues[resourceType] = NewWorkQueue()
}

// HasController returns true if a controller is registered for the type.
func (m *Manager) HasController(resourceType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.controllers[resourceType]
	return exists
}

// Enqueue adds a key to the work queue for a resource type.
func (m *Manager) Enqueue(resourceType, key string) {
	m.mu.RLock()
	queue, exists := m.queues[resourceType]
	m.mu.RUnlock()

	if !exists {
		m.logger.Warn("enqueue for unknown resource type",
			"resourceType", resourceType,
			"key", key)
		return
	}

	queue.Add(key)
}

// Start begins processing all registered controllers.
// It blocks until the context is cancelled and all workers have stopped.
func (m *Manager) Start(ctx context.Context, workersPerController int) {
	m.mu.RLock()
	resourceTypes := make([]string, 0, len(m.controllers))
	for rt := range m.controllers {
		resourceTypes = append(resourceTypes, rt)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup

	// Start workers for each controller
	for _, rt := range resourceTypes {
		for i := 0; i < workersPerController; i++ {
			wg.Add(1)
			go func(resourceType string, workerID int) {
				defer wg.Done()
				m.runWorker(ctx, resourceType, workerID)
			}(rt, i)
		}
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown all queues to unblock workers waiting on Get()
	m.mu.RLock()
	for _, queue := range m.queues {
		queue.ShutDown()
	}
	m.mu.RUnlock()

	// Wait for all workers to finish
	wg.Wait()

	// Signal that all workers have stopped
	m.stopOnce.Do(func() {
		close(m.stopped)
	})
}

// Stop signals the manager to shutdown and waits for all workers to complete.
// This should be called during graceful shutdown to ensure workers finish
// before resources (like the database) are closed.
func (m *Manager) Stop() {
	// Shutdown all queues to unblock any waiting workers
	m.mu.RLock()
	for _, queue := range m.queues {
		queue.ShutDown()
	}
	m.mu.RUnlock()

	// Wait for Start() to complete (all workers stopped)
	<-m.stopped
}

// runWorker processes items from the queue for a resource type.
func (m *Manager) runWorker(ctx context.Context, resourceType string, workerID int) {
	m.mu.RLock()
	queue := m.queues[resourceType]
	ctrl := m.controllers[resourceType]
	m.mu.RUnlock()

	m.logger.Debug("worker started",
		"resourceType", resourceType,
		"workerID", workerID)

	for {
		item, shutdown := queue.Get()
		if shutdown {
			m.logger.Debug("worker shutting down",
				"resourceType", resourceType,
				"workerID", workerID)
			return
		}

		key := item.(string)
		m.processItem(ctx, resourceType, ctrl, queue, key)
	}
}

// processItem handles a single work item.
func (m *Manager) processItem(ctx context.Context, resourceType string, ctrl Controller, queue *WorkQueue, key string) {
	defer queue.Done(key)

	m.logger.Debug("reconciling",
		"resourceType", resourceType,
		"key", key)

	err := ctrl.Reconcile(ctx, key)
	if err != nil {
		m.logger.Error("reconcile failed, requeuing",
			"resourceType", resourceType,
			"key", key,
			"error", err)
		queue.Requeue(key)
		return
	}

	m.logger.Debug("reconcile complete",
		"resourceType", resourceType,
		"key", key)
}

// GetQueue returns the work queue for a resource type.
// This is useful for testing or advanced use cases.
func (m *Manager) GetQueue(resourceType string) *WorkQueue {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.queues[resourceType]
}
