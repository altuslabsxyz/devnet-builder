// internal/daemon/store/bolt.go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names
var (
	bucketDevnets      = []byte("devnets")
	bucketNodes        = []byte("nodes")
	bucketUpgrades     = []byte("upgrades")
	bucketTransactions = []byte("transactions")
	bucketMeta         = []byte("meta")
)

// BoltStore implements Store using BoltDB.
type BoltStore struct {
	db       *bolt.DB
	watchers map[string][]WatchHandler
	mu       sync.RWMutex
}

// NewBoltStore creates a new BoltDB-backed store.
func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketDevnets,
			bucketNodes,
			bucketUpgrades,
			bucketTransactions,
			bucketMeta,
		}
		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{
		db:       db,
		watchers: make(map[string][]WatchHandler),
	}, nil
}

// Close closes the database.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// notify sends events to registered watchers.
func (s *BoltStore) notify(resourceType, eventType string, resource interface{}) {
	s.mu.RLock()
	handlers := s.watchers[resourceType]
	s.mu.RUnlock()

	for _, h := range handlers {
		go h(eventType, resource)
	}
}

// Watch registers a handler for resource changes.
func (s *BoltStore) Watch(ctx context.Context, resourceType string, handler WatchHandler) error {
	s.mu.Lock()
	s.watchers[resourceType] = append(s.watchers[resourceType], handler)
	s.mu.Unlock()

	// Send initial list as ADDED events
	switch resourceType {
	case "devnets":
		devnets, err := s.ListDevnets(ctx)
		if err != nil {
			return err
		}
		for _, d := range devnets {
			handler("ADDED", d)
		}
	}

	// Block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// encode marshals a value to JSON.
func encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// decode unmarshals JSON to a value.
func decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
