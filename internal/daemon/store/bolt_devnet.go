// internal/daemon/store/bolt_devnet.go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	bolt "go.etcd.io/bbolt"
)

// CreateDevnet creates a new devnet.
func (s *BoltStore) CreateDevnet(ctx context.Context, devnet *Devnet) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		// Check if already exists
		if b.Get([]byte(devnet.Metadata.Name)) != nil {
			return &AlreadyExistsError{Resource: "devnet", Name: devnet.Metadata.Name}
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

		if err := b.Put([]byte(devnet.Metadata.Name), data); err != nil {
			return fmt.Errorf("failed to store devnet: %w", err)
		}

		s.notify("devnets", "ADDED", devnet)
		return nil
	})
}

// GetDevnet retrieves a devnet by name.
func (s *BoltStore) GetDevnet(ctx context.Context, name string) (*Devnet, error) {
	var devnet Devnet

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)
		data := b.Get([]byte(name))
		if data == nil {
			return &NotFoundError{Resource: "devnet", Name: name}
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

		// Get existing for conflict detection
		existing := b.Get([]byte(devnet.Metadata.Name))
		if existing == nil {
			return &NotFoundError{Resource: "devnet", Name: devnet.Metadata.Name}
		}

		var old types.Devnet
		if err := decode(existing, &old); err != nil {
			return fmt.Errorf("failed to decode existing devnet: %w", err)
		}

		// Optimistic concurrency check
		if old.Metadata.Generation != devnet.Metadata.Generation {
			return &ConflictError{
				Resource: "devnet",
				Name:     devnet.Metadata.Name,
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

		if err := b.Put([]byte(devnet.Metadata.Name), data); err != nil {
			return fmt.Errorf("failed to store devnet: %w", err)
		}

		s.notify("devnets", "MODIFIED", devnet)
		return nil
	})
}

// DeleteDevnet deletes a devnet.
func (s *BoltStore) DeleteDevnet(ctx context.Context, name string) error {
	var devnet *Devnet

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		data := b.Get([]byte(name))
		if data == nil {
			return &NotFoundError{Resource: "devnet", Name: name}
		}

		devnet = &Devnet{}
		if err := decode(data, devnet); err != nil {
			return err
		}

		return b.Delete([]byte(name))
	})
	if err != nil {
		return err
	}

	s.notify("devnets", "DELETED", devnet)
	return nil
}

// ListDevnets returns all devnets.
func (s *BoltStore) ListDevnets(ctx context.Context) ([]*Devnet, error) {
	var devnets []*Devnet

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)
		return b.ForEach(func(k, v []byte) error {
			var devnet Devnet
			if err := decode(v, &devnet); err != nil {
				return err
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

// Upgrade operations (stub implementations for now)

func (s *BoltStore) CreateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) GetUpgrade(ctx context.Context, name string) (*Upgrade, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *BoltStore) UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) ListUpgrades(ctx context.Context, devnetName string) ([]*Upgrade, error) {
	return nil, fmt.Errorf("not implemented")
}

// Transaction operations (stub implementations for now)

func (s *BoltStore) CreateTransaction(ctx context.Context, tx *Transaction) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *BoltStore) UpdateTransaction(ctx context.Context, tx *Transaction) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) ListTransactions(ctx context.Context, devnetName string, opts ListTxOptions) ([]*Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}
