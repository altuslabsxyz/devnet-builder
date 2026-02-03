// internal/daemon/store/bolt_devnet.go
package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	bolt "go.etcd.io/bbolt"
)

// CreateDevnet creates a new devnet.
func (s *BoltStore) CreateDevnet(ctx context.Context, devnet *Devnet) error {
	if devnet == nil {
		return fmt.Errorf("devnet cannot be nil")
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		// Ensure namespace is set
		devnet.Metadata.EnsureNamespace()

		key := devnetKey(devnet.Metadata.Namespace, devnet.Metadata.Name)

		// Check if already exists
		if b.Get(key) != nil {
			return &AlreadyExistsError{Resource: "devnet", Name: devnet.Metadata.FullName()}
		}

		// Set metadata
		now := time.Now()
		devnet.Metadata.Generation = 1
		devnet.Metadata.CreatedAt = now
		devnet.Metadata.UpdatedAt = now

		data, err := encode(devnet)
		if err != nil {
			return fmt.Errorf("failed to encode devnet: %w", err)
		}

		if err := b.Put(key, data); err != nil {
			return fmt.Errorf("failed to store devnet: %w", err)
		}

		s.notify("devnets", "ADDED", devnet)
		return nil
	})
}

// GetDevnet retrieves a devnet by namespace and name.
func (s *BoltStore) GetDevnet(ctx context.Context, namespace, name string) (*Devnet, error) {
	var devnet Devnet

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)
		key := devnetKey(namespace, name)
		data := b.Get(key)
		if data == nil {
			return &NotFoundError{Resource: "devnet", Name: namespace + "/" + name}
		}
		return decode(data, &devnet)
	})
	if err != nil {
		return nil, err
	}

	return &devnet, nil
}

// UpdateDevnet updates an existing devnet.
func (s *BoltStore) UpdateDevnet(ctx context.Context, devnet *Devnet) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		// Ensure namespace is set
		devnet.Metadata.EnsureNamespace()

		key := devnetKey(devnet.Metadata.Namespace, devnet.Metadata.Name)

		// Get existing for conflict detection
		existing := b.Get(key)
		if existing == nil {
			return &NotFoundError{Resource: "devnet", Name: devnet.Metadata.FullName()}
		}

		var old types.Devnet
		if err := decode(existing, &old); err != nil {
			return fmt.Errorf("failed to decode existing devnet: %w", err)
		}

		// Optimistic concurrency check
		if old.Metadata.Generation != devnet.Metadata.Generation {
			return &ConflictError{
				Resource: "devnet",
				Name:     devnet.Metadata.FullName(),
				Message:  fmt.Sprintf("generation mismatch: expected %d, got %d", old.Metadata.Generation, devnet.Metadata.Generation),
			}
		}

		// Update metadata
		devnet.Metadata.Generation++
		devnet.Metadata.UpdatedAt = time.Now()

		data, err := encode(devnet)
		if err != nil {
			return fmt.Errorf("failed to encode devnet: %w", err)
		}

		if err := b.Put(key, data); err != nil {
			return fmt.Errorf("failed to store devnet: %w", err)
		}

		s.notify("devnets", "MODIFIED", devnet)
		return nil
	})
}

// DeleteDevnet deletes a devnet by namespace and name.
func (s *BoltStore) DeleteDevnet(ctx context.Context, namespace, name string) error {
	var devnet *Devnet

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		key := devnetKey(namespace, name)
		data := b.Get(key)
		if data == nil {
			return &NotFoundError{Resource: "devnet", Name: namespace + "/" + name}
		}

		devnet = &Devnet{}
		if err := decode(data, devnet); err != nil {
			return err
		}

		return b.Delete(key)
	})
	if err != nil {
		return err
	}

	s.notify("devnets", "DELETED", devnet)
	return nil
}

// ListDevnets returns all devnets, optionally filtered by namespace.
// If namespace is empty, returns all devnets across all namespaces.
func (s *BoltStore) ListDevnets(ctx context.Context, namespace string) ([]*Devnet, error) {
	var devnets []*Devnet

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)
		return b.ForEach(func(k, v []byte) error {
			var devnet Devnet
			if err := decode(v, &devnet); err != nil {
				return err
			}

			// Filter by namespace if specified
			if namespace != "" {
				ns := devnet.Metadata.Namespace
				if ns == "" {
					ns = types.DefaultNamespace
				}
				if ns != namespace {
					return nil
				}
			}

			devnets = append(devnets, &devnet)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return devnets, nil
}

// ListNamespaces returns a sorted list of all unique namespaces.
func (s *BoltStore) ListNamespaces(ctx context.Context) ([]string, error) {
	namespaceSet := make(map[string]struct{})

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)
		return b.ForEach(func(k, v []byte) error {
			ns, _ := parseDevnetKey(k)
			namespaceSet[ns] = struct{}{}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	// Convert to sorted slice
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	return namespaces, nil
}
