// store.go provides persistent storage for log entries using bbolt.
package logs

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names for log storage.
var (
	bucketLogs       = []byte("logs")
	bucketNodeIndex  = []byte("node_index")
	bucketLevelIndex = []byte("level_index")
)

// LogQuery defines query parameters for retrieving log entries.
type LogQuery struct {
	NodeIndex *int       // Filter by node index (nil = all nodes)
	Level     string     // Filter by log level (empty = all levels)
	Since     *time.Time // Filter entries after this time
	Until     *time.Time // Filter entries before this time
	Contains  string     // Filter by message substring
	Limit     int        // Maximum entries to return
}

// LogStore provides persistent storage for log entries.
type LogStore struct {
	db     *bolt.DB
	idSeq  uint64 // Atomic counter for unique IDs
	mu     sync.RWMutex
}

// NewLogStore creates a new log store at the given path.
func NewLogStore(path string) (*LogStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open log database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketLogs,
			bucketNodeIndex,
			bucketLevelIndex,
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

	return &LogStore{
		db: db,
	}, nil
}

// Close closes the log store.
func (s *LogStore) Close() error {
	return s.db.Close()
}

// Store saves a single log entry.
func (s *LogStore) Store(ctx context.Context, entry LogEntry) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.storeEntry(tx, entry)
	})
}

// StoreBatch saves multiple log entries in a single transaction.
func (s *LogStore) StoreBatch(ctx context.Context, entries []LogEntry) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, entry := range entries {
			if err := s.storeEntry(tx, entry); err != nil {
				return err
			}
		}
		return nil
	})
}

// storeEntry saves a single entry within a transaction.
func (s *LogStore) storeEntry(tx *bolt.Tx, entry LogEntry) error {
	logsBucket := tx.Bucket(bucketLogs)
	nodeIndexBucket := tx.Bucket(bucketNodeIndex)
	levelIndexBucket := tx.Bucket(bucketLevelIndex)

	// Generate unique key: timestamp (8 bytes) + sequence (8 bytes)
	id := atomic.AddUint64(&s.idSeq, 1)
	key := makeLogKey(entry.Timestamp, id)

	// Encode entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to encode log entry: %w", err)
	}

	// Store in main bucket
	if err := logsBucket.Put(key, data); err != nil {
		return err
	}

	// Update node index: node_<nodeIndex>/<key> -> key
	nodeKey := makeNodeIndexKey(entry.NodeIndex, key)
	if err := nodeIndexBucket.Put(nodeKey, key); err != nil {
		return err
	}

	// Update level index: level_<level>/<key> -> key
	levelKey := makeLevelIndexKey(entry.Level, key)
	if err := levelIndexBucket.Put(levelKey, key); err != nil {
		return err
	}

	return nil
}

// Query retrieves log entries matching the given criteria.
func (s *LogStore) Query(ctx context.Context, query LogQuery) ([]LogEntry, error) {
	var entries []LogEntry

	err := s.db.View(func(tx *bolt.Tx) error {
		logsBucket := tx.Bucket(bucketLogs)
		nodeIndexBucket := tx.Bucket(bucketNodeIndex)
		levelIndexBucket := tx.Bucket(bucketLevelIndex)

		// Determine which keys to scan
		var keysToScan [][]byte

		// If filtering by node, use node index
		if query.NodeIndex != nil {
			prefix := []byte(fmt.Sprintf("node_%d/", *query.NodeIndex))
			c := nodeIndexBucket.Cursor()
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				keysToScan = append(keysToScan, v)
			}
		} else if query.Level != "" {
			// If filtering by level, use level index
			prefix := []byte(fmt.Sprintf("level_%s/", query.Level))
			c := levelIndexBucket.Cursor()
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				keysToScan = append(keysToScan, v)
			}
		} else {
			// Full scan
			c := logsBucket.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				keyCopy := make([]byte, len(k))
				copy(keyCopy, k)
				keysToScan = append(keysToScan, keyCopy)
			}
		}

		// Fetch and filter entries
		for _, key := range keysToScan {
			if query.Limit > 0 && len(entries) >= query.Limit {
				break
			}

			data := logsBucket.Get(key)
			if data == nil {
				continue
			}

			var entry LogEntry
			if err := json.Unmarshal(data, &entry); err != nil {
				continue
			}

			// Apply additional filters
			if !s.matchesQuery(&entry, &query) {
				continue
			}

			entries = append(entries, entry)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// matchesQuery checks if an entry matches all query criteria.
func (s *LogStore) matchesQuery(entry *LogEntry, query *LogQuery) bool {
	// Check node filter
	if query.NodeIndex != nil && entry.NodeIndex != *query.NodeIndex {
		return false
	}

	// Check level filter
	if query.Level != "" && entry.Level != query.Level {
		return false
	}

	// Check time range
	if query.Since != nil && entry.Timestamp.Before(*query.Since) {
		return false
	}
	if query.Until != nil && entry.Timestamp.After(*query.Until) {
		return false
	}

	// Check message contains
	if query.Contains != "" && !strings.Contains(entry.Message, query.Contains) {
		return false
	}

	return true
}

// Prune removes log entries older than the cutoff time.
// Returns the number of entries pruned.
func (s *LogStore) Prune(ctx context.Context, cutoff time.Time) (int, error) {
	pruned := 0

	err := s.db.Update(func(tx *bolt.Tx) error {
		logsBucket := tx.Bucket(bucketLogs)
		nodeIndexBucket := tx.Bucket(bucketNodeIndex)
		levelIndexBucket := tx.Bucket(bucketLevelIndex)

		// Collect keys to delete
		var keysToDelete [][]byte
		c := logsBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry LogEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}

			if entry.Timestamp.Before(cutoff) {
				keyCopy := make([]byte, len(k))
				copy(keyCopy, k)
				keysToDelete = append(keysToDelete, keyCopy)
			}
		}

		// Delete entries and their indexes
		for _, key := range keysToDelete {
			// Get entry to find index keys
			data := logsBucket.Get(key)
			if data != nil {
				var entry LogEntry
				if json.Unmarshal(data, &entry) == nil {
					// Delete from node index
					nodeKey := makeNodeIndexKey(entry.NodeIndex, key)
					nodeIndexBucket.Delete(nodeKey)

					// Delete from level index
					levelKey := makeLevelIndexKey(entry.Level, key)
					levelIndexBucket.Delete(levelKey)
				}
			}

			// Delete from main bucket
			if err := logsBucket.Delete(key); err != nil {
				return err
			}
			pruned++
		}

		return nil
	})

	return pruned, err
}

// Count returns the total number of log entries.
func (s *LogStore) Count(ctx context.Context) (int, error) {
	var count int

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketLogs)
		count = b.Stats().KeyN
		return nil
	})

	return count, err
}

// makeLogKey creates a key from timestamp and sequence ID.
// Format: timestamp (8 bytes big-endian) + id (8 bytes big-endian)
func makeLogKey(ts time.Time, id uint64) []byte {
	key := make([]byte, 16)
	binary.BigEndian.PutUint64(key[0:8], uint64(ts.UnixNano()))
	binary.BigEndian.PutUint64(key[8:16], id)
	return key
}

// makeNodeIndexKey creates a node index key.
func makeNodeIndexKey(nodeIndex int, logKey []byte) []byte {
	prefix := []byte(fmt.Sprintf("node_%d/", nodeIndex))
	return append(prefix, logKey...)
}

// makeLevelIndexKey creates a level index key.
func makeLevelIndexKey(level string, logKey []byte) []byte {
	prefix := []byte(fmt.Sprintf("level_%s/", level))
	return append(prefix, logKey...)
}
