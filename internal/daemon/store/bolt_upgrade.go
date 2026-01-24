// internal/daemon/store/bolt_upgrade.go
package store

import (
	"bytes"
	"context"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// upgradeKeyPrefix returns the prefix for all upgrades in a devnet.
func upgradeKeyPrefix(devnetName string) []byte {
	return []byte(devnetName + "/")
}

// CreateUpgrade creates a new upgrade in the store.
func (s *BoltStore) CreateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)

		key := []byte(upgrade.Metadata.Name)

		// Check if already exists
		if b.Get(key) != nil {
			return &AlreadyExistsError{
				Resource: "upgrade",
				Name:     upgrade.Metadata.Name,
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

// GetUpgrade retrieves an upgrade by name.
func (s *BoltStore) GetUpgrade(ctx context.Context, name string) (*Upgrade, error) {
	var upgrade Upgrade

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		key := []byte(name)
		data := b.Get(key)
		if data == nil {
			return &NotFoundError{
				Resource: "upgrade",
				Name:     name,
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
		key := []byte(upgrade.Metadata.Name)

		// Get existing for conflict detection
		existing := b.Get(key)
		if existing == nil {
			return &NotFoundError{
				Resource: "upgrade",
				Name:     upgrade.Metadata.Name,
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
				Name:     upgrade.Metadata.Name,
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

// ListUpgrades returns all upgrades for a given devnet.
// If devnetName is empty, returns all upgrades.
func (s *BoltStore) ListUpgrades(ctx context.Context, devnetName string) ([]*Upgrade, error) {
	var upgrades []*Upgrade

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		c := b.Cursor()

		if devnetName == "" {
			// Return all upgrades
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var upgrade Upgrade
				if err := decode(v, &upgrade); err != nil {
					return fmt.Errorf("failed to decode upgrade %s: %w", string(k), err)
				}
				upgrades = append(upgrades, &upgrade)
			}
		} else {
			// Filter by devnet
			prefix := upgradeKeyPrefix(devnetName)
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				var upgrade Upgrade
				if err := decode(v, &upgrade); err != nil {
					return fmt.Errorf("failed to decode upgrade %s: %w", string(k), err)
				}
				upgrades = append(upgrades, &upgrade)
			}

			// Also check upgrades by DevnetRef field
			for k, v := c.First(); k != nil; k, v = c.Next() {
				var upgrade Upgrade
				if err := decode(v, &upgrade); err != nil {
					return fmt.Errorf("failed to decode upgrade %s: %w", string(k), err)
				}
				if upgrade.Spec.DevnetRef == devnetName {
					// Check if already added (avoid duplicates)
					found := false
					for _, u := range upgrades {
						if u.Metadata.Name == upgrade.Metadata.Name {
							found = true
							break
						}
					}
					if !found {
						upgrades = append(upgrades, &upgrade)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return upgrades, nil
}

// DeleteUpgrade deletes an upgrade by name.
func (s *BoltStore) DeleteUpgrade(ctx context.Context, name string) error {
	var upgrade *Upgrade

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketUpgrades)
		key := []byte(name)

		data := b.Get(key)
		if data == nil {
			return &NotFoundError{
				Resource: "upgrade",
				Name:     name,
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
func (s *BoltStore) DeleteUpgradesByDevnet(ctx context.Context, devnetName string) error {
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
			if upgrade.Spec.DevnetRef == devnetName {
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
