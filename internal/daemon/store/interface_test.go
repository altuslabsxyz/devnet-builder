// internal/daemon/store/interface_test.go
package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStoreInterface ensures the interface is well-defined.
func TestStoreInterface(t *testing.T) {
	// This is a compile-time check that BoltStore implements Store.
	// We'll implement BoltStore in the next task.
	var _ Store = (*mockStore)(nil)
}

// mockStore is a minimal mock for interface testing.
type mockStore struct{}

func (m *mockStore) CreateDevnet(ctx context.Context, devnet *Devnet) error { return nil }
func (m *mockStore) GetDevnet(ctx context.Context, namespace, name string) (*Devnet, error) {
	return nil, nil
}
func (m *mockStore) UpdateDevnet(ctx context.Context, devnet *Devnet) error         { return nil }
func (m *mockStore) DeleteDevnet(ctx context.Context, namespace, name string) error { return nil }
func (m *mockStore) ListDevnets(ctx context.Context, namespace string) ([]*Devnet, error) {
	return nil, nil
}

func (m *mockStore) CreateNode(ctx context.Context, node *Node) error { return nil }
func (m *mockStore) GetNode(ctx context.Context, namespace, devnetName string, index int) (*Node, error) {
	return nil, nil
}
func (m *mockStore) UpdateNode(ctx context.Context, node *Node) error { return nil }
func (m *mockStore) DeleteNode(ctx context.Context, namespace, devnetName string, index int) error {
	return nil
}
func (m *mockStore) ListNodes(ctx context.Context, namespace, devnetName string) ([]*Node, error) {
	return nil, nil
}
func (m *mockStore) DeleteNodesByDevnet(ctx context.Context, namespace, devnetName string) error {
	return nil
}

func (m *mockStore) CreateUpgrade(ctx context.Context, upgrade *Upgrade) error { return nil }
func (m *mockStore) GetUpgrade(ctx context.Context, namespace, name string) (*Upgrade, error) {
	return nil, nil
}
func (m *mockStore) UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error       { return nil }
func (m *mockStore) DeleteUpgrade(ctx context.Context, namespace, name string) error { return nil }
func (m *mockStore) ListUpgrades(ctx context.Context, namespace, devnetName string) ([]*Upgrade, error) {
	return nil, nil
}
func (m *mockStore) DeleteUpgradesByDevnet(ctx context.Context, namespace, devnetName string) error {
	return nil
}

func (m *mockStore) CreateTransaction(ctx context.Context, tx *Transaction) error { return nil }
func (m *mockStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	return nil, nil
}
func (m *mockStore) UpdateTransaction(ctx context.Context, tx *Transaction) error { return nil }
func (m *mockStore) ListTransactions(ctx context.Context, namespace, devnetName string, opts ListTxOptions) ([]*Transaction, error) {
	return nil, nil
}
func (m *mockStore) DeleteTransaction(ctx context.Context, name string) error { return nil }
func (m *mockStore) DeleteTransactionsByDevnet(ctx context.Context, devnetName string) error {
	return nil
}

func (m *mockStore) ListNamespaces(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockStore) Watch(ctx context.Context, resourceType string, handler WatchHandler) error {
	return nil
}
func (m *mockStore) Close() error { return nil }

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{Resource: "devnet", Name: "test"}
	assert.Contains(t, err.Error(), "devnet")
	assert.Contains(t, err.Error(), "test")
	assert.True(t, IsNotFound(err))
}

func TestConflictError(t *testing.T) {
	err := &ConflictError{Resource: "devnet", Name: "test", Message: "generation mismatch"}
	assert.Contains(t, err.Error(), "conflict")
	assert.True(t, IsConflict(err))
}

func TestAlreadyExistsError(t *testing.T) {
	err := &AlreadyExistsError{Resource: "devnet", Name: "test"}
	assert.Contains(t, err.Error(), "devnet")
	assert.Contains(t, err.Error(), "test")
	assert.Contains(t, err.Error(), "already exists")
	assert.True(t, IsAlreadyExists(err))
	assert.False(t, IsAlreadyExists(&NotFoundError{Resource: "devnet", Name: "test"}))
}
