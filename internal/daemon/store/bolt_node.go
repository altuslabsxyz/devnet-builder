// internal/daemon/store/bolt_node.go
package store

import (
	"bytes"
	"context"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// nodeKey returns the BoltDB key for a node.
// Format: "devnetName/index" (e.g., "mydevnet/0", "mydevnet/1")
func boltNodeKey(devnetName string, index int) []byte {
	return []byte(fmt.Sprintf("%s/%d", devnetName, index))
}

// nodeKeyPrefix returns the prefix for all nodes in a devnet.
func nodeKeyPrefix(devnetName string) []byte {
	return []byte(devnetName + "/")
}

// CreateNode creates a new node in the store.
func (s *BoltStore) CreateNode(ctx context.Context, node *Node) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)

		key := boltNodeKey(node.Spec.DevnetRef, node.Spec.Index)

		// Check if already exists
		if b.Get(key) != nil {
			return &AlreadyExistsError{
				Resource: "node",
				Name:     fmt.Sprintf("%s/%d", node.Spec.DevnetRef, node.Spec.Index),
			}
		}

		// Set metadata
		now := time.Now()
		node.Metadata.Generation = 1
		node.Metadata.CreatedAt = now
		node.Metadata.UpdatedAt = now

		data, err := encode(node)
		if err != nil {
			return fmt.Errorf("failed to encode node: %w", err)
		}

		if err := b.Put(key, data); err != nil {
			return fmt.Errorf("failed to store node: %w", err)
		}

		s.notify("nodes", "ADDED", node)
		return nil
	})
}

// GetNode retrieves a node by devnet name and index.
func (s *BoltStore) GetNode(ctx context.Context, devnetName string, index int) (*Node, error) {
	var node Node

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		key := boltNodeKey(devnetName, index)
		data := b.Get(key)
		if data == nil {
			return &NotFoundError{
				Resource: "node",
				Name:     fmt.Sprintf("%s/%d", devnetName, index),
			}
		}
		return decode(data, &node)
	})
	if err != nil {
		return nil, err
	}

	return &node, nil
}

// UpdateNode updates an existing node.
func (s *BoltStore) UpdateNode(ctx context.Context, node *Node) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		key := boltNodeKey(node.Spec.DevnetRef, node.Spec.Index)

		// Get existing for conflict detection
		existing := b.Get(key)
		if existing == nil {
			return &NotFoundError{
				Resource: "node",
				Name:     fmt.Sprintf("%s/%d", node.Spec.DevnetRef, node.Spec.Index),
			}
		}

		var old Node
		if err := decode(existing, &old); err != nil {
			return fmt.Errorf("failed to decode existing node: %w", err)
		}

		// Optimistic concurrency check
		if old.Metadata.Generation != node.Metadata.Generation {
			return &ConflictError{
				Resource: "node",
				Name:     fmt.Sprintf("%s/%d", node.Spec.DevnetRef, node.Spec.Index),
				Message:  fmt.Sprintf("generation mismatch: expected %d, got %d", old.Metadata.Generation, node.Metadata.Generation),
			}
		}

		// Update metadata
		node.Metadata.Generation++
		node.Metadata.UpdatedAt = time.Now()

		data, err := encode(node)
		if err != nil {
			return fmt.Errorf("failed to encode node: %w", err)
		}

		if err := b.Put(key, data); err != nil {
			return fmt.Errorf("failed to store node: %w", err)
		}

		s.notify("nodes", "MODIFIED", node)
		return nil
	})
}

// DeleteNode deletes a node by devnet name and index.
func (s *BoltStore) DeleteNode(ctx context.Context, devnetName string, index int) error {
	var node *Node

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		key := boltNodeKey(devnetName, index)

		data := b.Get(key)
		if data == nil {
			return &NotFoundError{
				Resource: "node",
				Name:     fmt.Sprintf("%s/%d", devnetName, index),
			}
		}

		node = &Node{}
		if err := decode(data, node); err != nil {
			return err
		}

		return b.Delete(key)
	})
	if err != nil {
		return err
	}

	s.notify("nodes", "DELETED", node)
	return nil
}

// ListNodes returns all nodes for a given devnet.
func (s *BoltStore) ListNodes(ctx context.Context, devnetName string) ([]*Node, error) {
	var nodes []*Node
	prefix := nodeKeyPrefix(devnetName)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		c := b.Cursor()

		// Seek to the first key with our prefix and iterate
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var node Node
			if err := decode(v, &node); err != nil {
				return fmt.Errorf("failed to decode node %s: %w", string(k), err)
			}
			nodes = append(nodes, &node)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

// DeleteNodesByDevnet deletes all nodes belonging to a devnet (cascade delete).
func (s *BoltStore) DeleteNodesByDevnet(ctx context.Context, devnetName string) error {
	prefix := nodeKeyPrefix(devnetName)
	var deleted []*Node

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketNodes)
		c := b.Cursor()

		// Collect keys to delete (can't delete during iteration with range)
		var keysToDelete [][]byte
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			keysToDelete = append(keysToDelete, append([]byte(nil), k...))
			var node Node
			if err := decode(v, &node); err == nil {
				deleted = append(deleted, &node)
			}
		}

		// Delete collected keys
		for _, k := range keysToDelete {
			if err := b.Delete(k); err != nil {
				return fmt.Errorf("failed to delete node %s: %w", string(k), err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Notify for each deleted node
	for _, node := range deleted {
		s.notify("nodes", "DELETED", node)
	}
	return nil
}
