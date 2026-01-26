// internal/daemon/store/bolt_transaction.go
package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	bolt "go.etcd.io/bbolt"
)

// CreateTransaction creates a new transaction.
func (s *BoltStore) CreateTransaction(ctx context.Context, tx *types.Transaction) error {
	return s.db.Update(func(btx *bolt.Tx) error {
		b := btx.Bucket(bucketTransactions)
		if b == nil {
			return fmt.Errorf("transactions bucket not found")
		}

		// Ensure namespace is set
		tx.Metadata.EnsureNamespace()

		key := []byte(tx.Metadata.Name)
		if b.Get(key) != nil {
			return ErrAlreadyExists
		}

		data, err := json.Marshal(tx)
		if err != nil {
			return err
		}

		if err := b.Put(key, data); err != nil {
			return err
		}

		s.notify("transactions", "ADDED", tx)
		return nil
	})
}

// GetTransaction retrieves a transaction by name.
func (s *BoltStore) GetTransaction(ctx context.Context, name string) (*types.Transaction, error) {
	var tx types.Transaction
	err := s.db.View(func(btx *bolt.Tx) error {
		b := btx.Bucket(bucketTransactions)
		if b == nil {
			return ErrNotFound
		}

		data := b.Get([]byte(name))
		if data == nil {
			return ErrNotFound
		}

		return json.Unmarshal(data, &tx)
	})
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

// UpdateTransaction updates a transaction.
func (s *BoltStore) UpdateTransaction(ctx context.Context, tx *types.Transaction) error {
	return s.db.Update(func(btx *bolt.Tx) error {
		b := btx.Bucket(bucketTransactions)
		if b == nil {
			return ErrNotFound
		}

		// Ensure namespace is set
		tx.Metadata.EnsureNamespace()

		key := []byte(tx.Metadata.Name)
		if b.Get(key) == nil {
			return ErrNotFound
		}

		data, err := json.Marshal(tx)
		if err != nil {
			return err
		}

		if err := b.Put(key, data); err != nil {
			return err
		}

		s.notify("transactions", "MODIFIED", tx)
		return nil
	})
}

// ListTransactions lists transactions for a devnet with optional filtering.
// If namespace is empty and devnetName is empty, returns all transactions.
func (s *BoltStore) ListTransactions(ctx context.Context, namespace, devnetName string, opts ListTxOptions) ([]*types.Transaction, error) {
	var txs []*types.Transaction

	err := s.db.View(func(btx *bolt.Tx) error {
		b := btx.Bucket(bucketTransactions)
		if b == nil {
			return nil // No bucket = no transactions
		}

		return b.ForEach(func(k, v []byte) error {
			// Check limit before processing more entries
			if opts.Limit > 0 && len(txs) >= opts.Limit {
				return nil
			}

			var tx types.Transaction
			if err := json.Unmarshal(v, &tx); err != nil {
				return err
			}

			// Filter by namespace if specified
			if namespace != "" {
				ns := tx.Metadata.Namespace
				if ns == "" {
					ns = types.DefaultNamespace
				}
				if ns != namespace {
					return nil
				}
			}

			// Filter by devnet if specified
			if devnetName != "" && tx.Spec.DevnetRef != devnetName {
				return nil
			}

			// Filter by type if specified
			if opts.TxType != "" && tx.Spec.TxType != opts.TxType {
				return nil
			}

			// Filter by phase if specified
			if opts.Phase != "" && tx.Status.Phase != opts.Phase {
				return nil
			}

			txs = append(txs, &tx)
			return nil
		})
	})

	return txs, err
}

// DeleteTransaction deletes a transaction by name.
func (s *BoltStore) DeleteTransaction(ctx context.Context, name string) error {
	return s.db.Update(func(btx *bolt.Tx) error {
		b := btx.Bucket(bucketTransactions)
		if b == nil {
			return ErrNotFound
		}

		key := []byte(name)
		data := b.Get(key)
		if data == nil {
			return ErrNotFound
		}

		// Parse for notification
		var tx types.Transaction
		if err := json.Unmarshal(data, &tx); err != nil {
			return err
		}

		if err := b.Delete(key); err != nil {
			return err
		}

		s.notify("transactions", "DELETED", &tx)
		return nil
	})
}

// DeleteTransactionsByDevnet deletes all transactions for a devnet.
func (s *BoltStore) DeleteTransactionsByDevnet(ctx context.Context, devnetName string) error {
	return s.db.Update(func(btx *bolt.Tx) error {
		b := btx.Bucket(bucketTransactions)
		if b == nil {
			return nil
		}

		// Collect keys to delete
		var keysToDelete [][]byte
		err := b.ForEach(func(k, v []byte) error {
			var tx types.Transaction
			if err := json.Unmarshal(v, &tx); err != nil {
				return nil // Skip invalid entries
			}
			if tx.Spec.DevnetRef == devnetName {
				keysToDelete = append(keysToDelete, k)
			}
			return nil
		})
		if err != nil {
			return err
		}

		// Delete collected keys
		for _, k := range keysToDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}

		return nil
	})
}
