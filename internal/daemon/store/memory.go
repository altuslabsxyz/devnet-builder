// internal/daemon/store/memory.go
package store

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	devnets      map[string]*types.Devnet      // key: "namespace/name"
	nodes        map[string]*types.Node        // key: "namespace/devnetName/index"
	upgrades     map[string]*types.Upgrade     // key: "namespace/name"
	transactions map[string]*types.Transaction // key: global unique name
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

// memoryDevnetKey creates a key for devnet storage: namespace/name
func memoryDevnetKey(namespace, name string) string {
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return namespace + "/" + name
}

// memoryNodeKey creates a key for node storage: namespace/devnetName/index
func memoryNodeKey(namespace, devnetName string, index int) string {
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return fmt.Sprintf("%s/%s/%d", namespace, devnetName, index)
}

// memoryUpgradeKey creates a key for upgrade storage: namespace/name
func memoryUpgradeKey(namespace, name string) string {
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return namespace + "/" + name
}

// CreateDevnet creates a new devnet.
func (m *MemoryStore) CreateDevnet(ctx context.Context, devnet *types.Devnet) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure namespace is set
	devnet.Metadata.EnsureNamespace()

	key := memoryDevnetKey(devnet.Metadata.Namespace, devnet.Metadata.Name)
	if _, exists := m.devnets[key]; exists {
		return ErrAlreadyExists
	}

	// Deep copy to avoid mutation
	copy := *devnet
	m.devnets[key] = &copy
	return nil
}

// GetDevnet retrieves a devnet by namespace and name.
func (m *MemoryStore) GetDevnet(ctx context.Context, namespace, name string) (*types.Devnet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	key := memoryDevnetKey(namespace, name)
	devnet, exists := m.devnets[key]
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

	// Ensure namespace is set
	devnet.Metadata.EnsureNamespace()

	key := memoryDevnetKey(devnet.Metadata.Namespace, devnet.Metadata.Name)
	if _, exists := m.devnets[key]; !exists {
		return ErrNotFound
	}

	copy := *devnet
	m.devnets[key] = &copy
	return nil
}

// DeleteDevnet deletes a devnet by namespace and name.
func (m *MemoryStore) DeleteDevnet(ctx context.Context, namespace, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	key := memoryDevnetKey(namespace, name)
	if _, exists := m.devnets[key]; !exists {
		return ErrNotFound
	}

	delete(m.devnets, key)
	return nil
}

// ListDevnets lists devnets, optionally filtered by namespace.
// If namespace is empty, returns all devnets across all namespaces.
func (m *MemoryStore) ListDevnets(ctx context.Context, namespace string) ([]*types.Devnet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*types.Devnet, 0, len(m.devnets))
	for _, d := range m.devnets {
		// Filter by namespace if specified
		if namespace != "" {
			ns := d.Metadata.Namespace
			if ns == "" {
				ns = types.DefaultNamespace
			}
			if ns != namespace {
				continue
			}
		}
		copy := *d
		result = append(result, &copy)
	}
	return result, nil
}

// ListNamespaces returns a sorted list of all unique namespaces.
func (m *MemoryStore) ListNamespaces(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	namespaceSet := make(map[string]struct{})
	for _, d := range m.devnets {
		ns := d.Metadata.Namespace
		if ns == "" {
			ns = types.DefaultNamespace
		}
		namespaceSet[ns] = struct{}{}
	}

	// Convert to sorted slice
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	return namespaces, nil
}

// CreateNode creates a new node.
func (m *MemoryStore) CreateNode(ctx context.Context, node *types.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure namespace is set
	node.Metadata.EnsureNamespace()

	key := memoryNodeKey(node.Metadata.Namespace, node.Spec.DevnetRef, node.Spec.Index)
	if _, exists := m.nodes[key]; exists {
		return ErrAlreadyExists
	}

	copy := *node
	m.nodes[key] = &copy
	return nil
}

// GetNode retrieves a node by namespace, devnet name, and index.
func (m *MemoryStore) GetNode(ctx context.Context, namespace, devnetName string, index int) (*types.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	key := memoryNodeKey(namespace, devnetName, index)
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

	// Ensure namespace is set
	node.Metadata.EnsureNamespace()

	key := memoryNodeKey(node.Metadata.Namespace, node.Spec.DevnetRef, node.Spec.Index)
	if _, exists := m.nodes[key]; !exists {
		return ErrNotFound
	}

	copy := *node
	m.nodes[key] = &copy
	return nil
}

// DeleteNode deletes a node by namespace, devnet name, and index.
func (m *MemoryStore) DeleteNode(ctx context.Context, namespace, devnetName string, index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	key := memoryNodeKey(namespace, devnetName, index)
	if _, exists := m.nodes[key]; !exists {
		return ErrNotFound
	}

	delete(m.nodes, key)
	return nil
}

// ListNodes lists nodes for a namespace and devnet.
func (m *MemoryStore) ListNodes(ctx context.Context, namespace, devnetName string) ([]*types.Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	var result []*types.Node
	for _, n := range m.nodes {
		ns := n.Metadata.Namespace
		if ns == "" {
			ns = types.DefaultNamespace
		}
		if ns == namespace && n.Spec.DevnetRef == devnetName {
			copy := *n
			result = append(result, &copy)
		}
	}
	return result, nil
}

// DeleteNodesByDevnet deletes all nodes belonging to a devnet.
func (m *MemoryStore) DeleteNodesByDevnet(ctx context.Context, namespace, devnetName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	for key, node := range m.nodes {
		ns := node.Metadata.Namespace
		if ns == "" {
			ns = types.DefaultNamespace
		}
		if ns == namespace && node.Spec.DevnetRef == devnetName {
			delete(m.nodes, key)
		}
	}
	return nil
}

// CreateUpgrade creates a new upgrade.
func (m *MemoryStore) CreateUpgrade(ctx context.Context, upgrade *types.Upgrade) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure namespace is set
	upgrade.Metadata.EnsureNamespace()

	key := memoryUpgradeKey(upgrade.Metadata.Namespace, upgrade.Metadata.Name)
	if _, exists := m.upgrades[key]; exists {
		return ErrAlreadyExists
	}

	copy := *upgrade
	m.upgrades[key] = &copy
	return nil
}

// GetUpgrade retrieves an upgrade by namespace and name.
func (m *MemoryStore) GetUpgrade(ctx context.Context, namespace, name string) (*types.Upgrade, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	key := memoryUpgradeKey(namespace, name)
	upgrade, exists := m.upgrades[key]
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

	// Ensure namespace is set
	upgrade.Metadata.EnsureNamespace()

	key := memoryUpgradeKey(upgrade.Metadata.Namespace, upgrade.Metadata.Name)
	if _, exists := m.upgrades[key]; !exists {
		return ErrNotFound
	}

	copy := *upgrade
	m.upgrades[key] = &copy
	return nil
}

// DeleteUpgrade deletes an upgrade by namespace and name.
func (m *MemoryStore) DeleteUpgrade(ctx context.Context, namespace, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	key := memoryUpgradeKey(namespace, name)
	if _, exists := m.upgrades[key]; !exists {
		return ErrNotFound
	}

	delete(m.upgrades, key)
	return nil
}

// ListUpgrades lists upgrades, filtered by namespace and optionally by devnet.
// If namespace is empty, returns all upgrades. If devnetName is empty, returns all in the namespace.
func (m *MemoryStore) ListUpgrades(ctx context.Context, namespace, devnetName string) ([]*types.Upgrade, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*types.Upgrade
	for _, u := range m.upgrades {
		// Filter by namespace if specified
		if namespace != "" {
			ns := u.Metadata.Namespace
			if ns == "" {
				ns = types.DefaultNamespace
			}
			if ns != namespace {
				continue
			}
		}

		// Filter by devnet if specified
		if devnetName != "" && u.Spec.DevnetRef != devnetName {
			continue
		}

		copy := *u
		result = append(result, &copy)
	}
	return result, nil
}

// DeleteUpgradesByDevnet deletes all upgrades belonging to a devnet.
func (m *MemoryStore) DeleteUpgradesByDevnet(ctx context.Context, namespace, devnetName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	for name, upgrade := range m.upgrades {
		ns := upgrade.Metadata.Namespace
		if ns == "" {
			ns = types.DefaultNamespace
		}
		if ns == namespace && upgrade.Spec.DevnetRef == devnetName {
			delete(m.upgrades, name)
		}
	}
	return nil
}

// CreateTransaction creates a new transaction.
func (m *MemoryStore) CreateTransaction(ctx context.Context, tx *types.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure namespace is set
	tx.Metadata.EnsureNamespace()

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

	// Ensure namespace is set
	tx.Metadata.EnsureNamespace()

	id := tx.Metadata.Name
	if _, exists := m.transactions[id]; !exists {
		return ErrNotFound
	}

	copy := *tx
	m.transactions[id] = &copy
	return nil
}

// ListTransactions lists transactions, filtered by namespace and optionally by devnet.
// If namespace is empty and devnetName is empty, returns all transactions.
func (m *MemoryStore) ListTransactions(ctx context.Context, namespace, devnetName string, opts ListTxOptions) ([]*types.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*types.Transaction
	for _, tx := range m.transactions {
		// Filter by namespace if specified
		if namespace != "" {
			ns := tx.Metadata.Namespace
			if ns == "" {
				ns = types.DefaultNamespace
			}
			if ns != namespace {
				continue
			}
		}

		// Filter by devnet if specified
		if devnetName != "" && tx.Spec.DevnetRef != devnetName {
			continue
		}

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

// Ensure MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)
