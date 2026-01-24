// internal/daemon/store/memory.go
package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	devnets      map[string]*types.Devnet
	nodes        map[string]*types.Node // key: "devnetName/index"
	upgrades     map[string]*types.Upgrade
	transactions map[string]*types.Transaction
	mu           sync.RWMutex
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		devnets:      make(map[string]*types.Devnet),
		nodes:        make(map[string]*types.Node),
		upgrades:     make(map[string]*types.Upgrade),
		transactions: make(map[string]*types.Transaction),
	}
}

// CreateDevnet creates a new devnet.
func (m *MemoryStore) CreateDevnet(ctx context.Context, devnet *types.Devnet) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := devnet.Metadata.Name
	if _, exists := m.devnets[name]; exists {
		return ErrAlreadyExists
	}

	// Deep copy to avoid mutation
	copy := *devnet
	m.devnets[name] = &copy
	return nil
}

// GetDevnet retrieves a devnet by name.
func (m *MemoryStore) GetDevnet(ctx context.Context, name string) (*types.Devnet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devnet, exists := m.devnets[name]
	if !exists {
		return nil, ErrNotFound
	}

	// Return a copy
	copy := *devnet
	return &copy, nil
}

// UpdateDevnet updates an existing devnet.
func (m *MemoryStore) UpdateDevnet(ctx context.Context, devnet *types.Devnet) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := devnet.Metadata.Name
	if _, exists := m.devnets[name]; !exists {
		return ErrNotFound
	}

	copy := *devnet
	m.devnets[name] = &copy
	return nil
}

// DeleteDevnet deletes a devnet.
func (m *MemoryStore) DeleteDevnet(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devnets[name]; !exists {
		return ErrNotFound
	}

	delete(m.devnets, name)
	return nil
}

// ListDevnets lists all devnets.
func (m *MemoryStore) ListDevnets(ctx context.Context) ([]*types.Devnet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*types.Devnet, 0, len(m.devnets))
	for _, d := range m.devnets {
		copy := *d
		result = append(result, &copy)
	}
	return result, nil
}

// CreateNode creates a new node.
func (m *MemoryStore) CreateNode(ctx context.Context, node *types.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := nodeKey(node.Spec.DevnetRef, node.Spec.Index)
	if _, exists := m.nodes[key]; exists {
		return ErrAlreadyExists
	}

	copy := *node
	m.nodes[key] = &copy
	return nil
}

// GetNode retrieves a node.
func (m *MemoryStore) GetNode(ctx context.Context, devnetName string, index int) (*types.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := nodeKey(devnetName, index)
	node, exists := m.nodes[key]
	if !exists {
		return nil, ErrNotFound
	}

	copy := *node
	return &copy, nil
}

// UpdateNode updates a node.
func (m *MemoryStore) UpdateNode(ctx context.Context, node *types.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := nodeKey(node.Spec.DevnetRef, node.Spec.Index)
	if _, exists := m.nodes[key]; !exists {
		return ErrNotFound
	}

	copy := *node
	m.nodes[key] = &copy
	return nil
}

// DeleteNode deletes a node.
func (m *MemoryStore) DeleteNode(ctx context.Context, devnetName string, index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := nodeKey(devnetName, index)
	if _, exists := m.nodes[key]; !exists {
		return ErrNotFound
	}

	delete(m.nodes, key)
	return nil
}

// ListNodes lists nodes for a devnet.
func (m *MemoryStore) ListNodes(ctx context.Context, devnetName string) ([]*types.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*types.Node
	for _, n := range m.nodes {
		if n.Spec.DevnetRef == devnetName {
			copy := *n
			result = append(result, &copy)
		}
	}
	return result, nil
}

// DeleteNodesByDevnet deletes all nodes belonging to a devnet.
func (m *MemoryStore) DeleteNodesByDevnet(ctx context.Context, devnetName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, node := range m.nodes {
		if node.Spec.DevnetRef == devnetName {
			delete(m.nodes, key)
		}
	}
	return nil
}

// CreateUpgrade creates a new upgrade.
func (m *MemoryStore) CreateUpgrade(ctx context.Context, upgrade *types.Upgrade) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := upgrade.Metadata.Name
	if _, exists := m.upgrades[name]; exists {
		return ErrAlreadyExists
	}

	copy := *upgrade
	m.upgrades[name] = &copy
	return nil
}

// GetUpgrade retrieves an upgrade.
func (m *MemoryStore) GetUpgrade(ctx context.Context, name string) (*types.Upgrade, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upgrade, exists := m.upgrades[name]
	if !exists {
		return nil, ErrNotFound
	}

	copy := *upgrade
	return &copy, nil
}

// UpdateUpgrade updates an upgrade.
func (m *MemoryStore) UpdateUpgrade(ctx context.Context, upgrade *types.Upgrade) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := upgrade.Metadata.Name
	if _, exists := m.upgrades[name]; !exists {
		return ErrNotFound
	}

	copy := *upgrade
	m.upgrades[name] = &copy
	return nil
}

// ListUpgrades lists upgrades for a devnet.
func (m *MemoryStore) ListUpgrades(ctx context.Context, devnetName string) ([]*types.Upgrade, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*types.Upgrade
	for _, u := range m.upgrades {
		if u.Spec.DevnetRef == devnetName {
			copy := *u
			result = append(result, &copy)
		}
	}
	return result, nil
}

// CreateTransaction creates a new transaction.
func (m *MemoryStore) CreateTransaction(ctx context.Context, tx *types.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := tx.Metadata.Name
	if _, exists := m.transactions[id]; exists {
		return ErrAlreadyExists
	}

	copy := *tx
	m.transactions[id] = &copy
	return nil
}

// GetTransaction retrieves a transaction.
func (m *MemoryStore) GetTransaction(ctx context.Context, id string) (*types.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tx, exists := m.transactions[id]
	if !exists {
		return nil, ErrNotFound
	}

	copy := *tx
	return &copy, nil
}

// UpdateTransaction updates a transaction.
func (m *MemoryStore) UpdateTransaction(ctx context.Context, tx *types.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := tx.Metadata.Name
	if _, exists := m.transactions[id]; !exists {
		return ErrNotFound
	}

	copy := *tx
	m.transactions[id] = &copy
	return nil
}

// ListTransactions lists transactions for a devnet.
func (m *MemoryStore) ListTransactions(ctx context.Context, devnetName string, opts ListTxOptions) ([]*types.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*types.Transaction
	for _, tx := range m.transactions {
		if tx.Spec.DevnetRef == devnetName {
			if opts.TxType != "" && tx.Spec.TxType != opts.TxType {
				continue
			}
			if opts.Phase != "" && tx.Status.Phase != opts.Phase {
				continue
			}
			copy := *tx
			result = append(result, &copy)
			if opts.Limit > 0 && len(result) >= opts.Limit {
				break
			}
		}
	}
	return result, nil
}

// Watch watches for resource changes (not implemented for memory store).
func (m *MemoryStore) Watch(ctx context.Context, resourceType string, handler WatchHandler) error {
	// Not implemented for memory store
	<-ctx.Done()
	return ctx.Err()
}

// Close closes the store.
func (m *MemoryStore) Close() error {
	return nil
}

func nodeKey(devnetName string, index int) string {
	return fmt.Sprintf("%s/%d", devnetName, index)
}

// Ensure MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)
