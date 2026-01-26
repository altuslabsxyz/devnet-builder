// internal/daemon/store/bolt_upgrade.go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	bolt "go.etcd.io/bbolt"
)

// upgradeKey returns the BoltDB key for an upgrade.
// Format: "namespace/name" (e.g., "default/myupgrade")
func upgradeKey(namespace, name string) []byte {
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return []byte(namespace + "/" + name)
}

// CreateUpgrade creates a new upgrade in the store.
func (s *BoltStore) CreateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)

		// Ensure namespace is set
		upgrade.Metadata.EnsureNamespace()

		key := upgradeKey(upgrade.Metadata.Namespace, upgrade.Metadata.Name)

		// Check if already exists
		if b.Get(key) != nil {
			return &AlreadyExistsError{
				Resource: "upgrade",
				Name:     upgrade.Metadata.FullName(),
			}
		}

		// Set metadata
		now := time.Now()
		upgrade.Metadata.Generation = 1
		upgrade.Metadata.CreatedAt = now
		upgrade.Metadata.UpdatedAt = now

		data, err := encode(upgrade)
		if err != nil {
			return fmt.Errorf("failed to encode upgrade: %w", err)
		}

		if err := b.Put(key, data); err != nil {
			return fmt.Errorf("failed to store upgrade: %w", err)
		}

		s.notify("upgrades", "ADDED", upgrade)
		return nil
	})
}

// GetUpgrade retrieves an upgrade by namespace and name.
func (s *BoltStore) GetUpgrade(ctx context.Context, namespace, name string) (*Upgrade, error) {
	var upgrade Upgrade

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		key := upgradeKey(namespace, name)
		data := b.Get(key)
		if data == nil {
			return &NotFoundError{
				Resource: "upgrade",
				Name:     namespace + "/" + name,
			}
		}
		return decode(data, &upgrade)
	})
	if err != nil {
		return nil, err
	}

	return &upgrade, nil
}

// UpdateUpgrade updates an existing upgrade.
func (s *BoltStore) UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)

		// Ensure namespace is set
		upgrade.Metadata.EnsureNamespace()

		key := upgradeKey(upgrade.Metadata.Namespace, upgrade.Metadata.Name)

		// Get existing for conflict detection
		existing := b.Get(key)
		if existing == nil {
			return &NotFoundError{
				Resource: "upgrade",
				Name:     upgrade.Metadata.FullName(),
			}
		}

		var old Upgrade
		if err := decode(existing, &old); err != nil {
			return fmt.Errorf("failed to decode existing upgrade: %w", err)
		}

		// Optimistic concurrency check
		if old.Metadata.Generation != upgrade.Metadata.Generation {
			return &ConflictError{
				Resource: "upgrade",
				Name:     upgrade.Metadata.FullName(),
				Message:  fmt.Sprintf("generation mismatch: expected %d, got %d", old.Metadata.Generation, upgrade.Metadata.Generation),
			}
		}

		// Update metadata
		upgrade.Metadata.Generation++
		upgrade.Metadata.UpdatedAt = time.Now()

		data, err := encode(upgrade)
		if err != nil {
			return fmt.Errorf("failed to encode upgrade: %w", err)
		}

		if err := b.Put(key, data); err != nil {
			return fmt.Errorf("failed to store upgrade: %w", err)
		}

		s.notify("upgrades", "MODIFIED", upgrade)
		return nil
	})
}

// ListUpgrades returns all upgrades, filtered by namespace and optionally by devnet.
// If namespace is empty, returns all upgrades. If devnetName is empty, returns all in the namespace.
func (s *BoltStore) ListUpgrades(ctx context.Context, namespace, devnetName string) ([]*Upgrade, error) {
	var upgrades []*Upgrade

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var upgrade Upgrade
			if err := decode(v, &upgrade); err != nil {
				return fmt.Errorf("failed to decode upgrade %s: %w", string(k), err)
			}

			// Filter by namespace if specified
			if namespace != "" {
				ns := upgrade.Metadata.Namespace
				if ns == "" {
					ns = types.DefaultNamespace
				}
				if ns != namespace {
					continue
				}
			}

			// Filter by devnet if specified
			if devnetName != "" && upgrade.Spec.DevnetRef != devnetName {
				continue
			}

			upgrades = append(upgrades, &upgrade)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return upgrades, nil
}

// DeleteUpgrade deletes an upgrade by namespace and name.
func (s *BoltStore) DeleteUpgrade(ctx context.Context, namespace, name string) error {
	var upgrade *Upgrade

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		key := upgradeKey(namespace, name)

		data := b.Get(key)
		if data == nil {
			return &NotFoundError{
				Resource: "upgrade",
				Name:     namespace + "/" + name,
			}
		}

		upgrade = &Upgrade{}
		if err := decode(data, upgrade); err != nil {
			return err
		}

		return b.Delete(key)
	})
	if err != nil {
		return err
	}

	s.notify("upgrades", "DELETED", upgrade)
	return nil
}

// DeleteUpgradesByDevnet deletes all upgrades belonging to a devnet (cascade delete).
func (s *BoltStore) DeleteUpgradesByDevnet(ctx context.Context, namespace, devnetName string) error {
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	var deleted []*Upgrade

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		c := b.Cursor()

		// Collect keys to delete
		var keysToDelete [][]byte
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var upgrade Upgrade
			if err := decode(v, &upgrade); err != nil {
				continue
			}

			// Match by namespace and DevnetRef
			ns := upgrade.Metadata.Namespace
			if ns == "" {
				ns = types.DefaultNamespace
			}
			if ns == namespace && upgrade.Spec.DevnetRef == devnetName {
				keysToDelete = append(keysToDelete, append([]byte(nil), k...))
				deleted = append(deleted, &upgrade)
			}
		}

		// Delete collected keys
		for _, k := range keysToDelete {
			if err := b.Delete(k); err != nil {
				return fmt.Errorf("failed to delete upgrade %s: %w", string(k), err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Notify for each deleted upgrade
	for _, upgrade := range deleted {
		s.notify("upgrades", "DELETED", upgrade)
	}
	return nil
}
