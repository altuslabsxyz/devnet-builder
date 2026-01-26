package store

import (
	"context"
	"testing"
)

func TestStoreInterfaceHasNamespace(t *testing.T) {
	// This test ensures the interface has namespace parameters.
	// It will fail to compile if the interface is wrong.
	var _ Store = (*mockNamespaceStore)(nil)
}

// mockNamespaceStore implements Store with namespace-aware signatures.
// This serves as a compile-time check that the interface is correctly defined.
type mockNamespaceStore struct{}

// Devnet operations - namespace-scoped
func (m *mockNamespaceStore) CreateDevnet(ctx context.Context, devnet *Devnet) error {
	return nil
}

func (m *mockNamespaceStore) GetDevnet(ctx context.Context, namespace, name string) (*Devnet, error) {
	return nil, nil
}

func (m *mockNamespaceStore) UpdateDevnet(ctx context.Context, devnet *Devnet) error {
	return nil
}

func (m *mockNamespaceStore) DeleteDevnet(ctx context.Context, namespace, name string) error {
	return nil
}

func (m *mockNamespaceStore) ListDevnets(ctx context.Context, namespace string) ([]*Devnet, error) {
	return nil, nil
}

func (m *mockNamespaceStore) ListNamespaces(ctx context.Context) ([]string, error) {
	return nil, nil
}

// Node operations - namespace-scoped
func (m *mockNamespaceStore) CreateNode(ctx context.Context, node *Node) error {
	return nil
}

func (m *mockNamespaceStore) GetNode(ctx context.Context, namespace, devnetName string, index int) (*Node, error) {
	return nil, nil
}

func (m *mockNamespaceStore) UpdateNode(ctx context.Context, node *Node) error {
	return nil
}

func (m *mockNamespaceStore) DeleteNode(ctx context.Context, namespace, devnetName string, index int) error {
	return nil
}

func (m *mockNamespaceStore) ListNodes(ctx context.Context, namespace, devnetName string) ([]*Node, error) {
	return nil, nil
}

func (m *mockNamespaceStore) DeleteNodesByDevnet(ctx context.Context, namespace, devnetName string) error {
	return nil
}

// Upgrade operations - namespace-scoped
func (m *mockNamespaceStore) CreateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return nil
}

func (m *mockNamespaceStore) GetUpgrade(ctx context.Context, namespace, name string) (*Upgrade, error) {
	return nil, nil
}

func (m *mockNamespaceStore) UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return nil
}

func (m *mockNamespaceStore) DeleteUpgrade(ctx context.Context, namespace, name string) error {
	return nil
}

func (m *mockNamespaceStore) ListUpgrades(ctx context.Context, namespace, devnetName string) ([]*Upgrade, error) {
	return nil, nil
}

func (m *mockNamespaceStore) DeleteUpgradesByDevnet(ctx context.Context, namespace, devnetName string) error {
	return nil
}

// Transaction operations - namespace-scoped
func (m *mockNamespaceStore) CreateTransaction(ctx context.Context, tx *Transaction) error {
	return nil
}

func (m *mockNamespaceStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	return nil, nil
}

func (m *mockNamespaceStore) UpdateTransaction(ctx context.Context, tx *Transaction) error {
	return nil
}

func (m *mockNamespaceStore) ListTransactions(ctx context.Context, namespace, devnetName string, opts ListTxOptions) ([]*Transaction, error) {
	return nil, nil
}

// Watch and Close
func (m *mockNamespaceStore) Watch(ctx context.Context, resourceType string, handler WatchHandler) error {
	return nil
}

func (m *mockNamespaceStore) Close() error {
	return nil
}
