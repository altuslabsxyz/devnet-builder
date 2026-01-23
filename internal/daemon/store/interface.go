// internal/daemon/store/interface.go
package store

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// Re-export types for convenience.
type (
	Devnet      = types.Devnet
	Node        = types.Node
	Upgrade     = types.Upgrade
	Transaction = types.Transaction
)

// WatchHandler is called when a resource changes.
type WatchHandler func(eventType string, resource interface{})

// ListTxOptions configures transaction listing.
type ListTxOptions struct {
	// TxType filters by transaction type.
	TxType string
	// Phase filters by phase.
	Phase string
	// Limit is the maximum number of results.
	Limit int
}

// Store defines the interface for resource persistence.
type Store interface {
	// Devnet operations
	CreateDevnet(ctx context.Context, devnet *Devnet) error
	GetDevnet(ctx context.Context, name string) (*Devnet, error)
	UpdateDevnet(ctx context.Context, devnet *Devnet) error
	DeleteDevnet(ctx context.Context, name string) error
	ListDevnets(ctx context.Context) ([]*Devnet, error)

	// Node operations
	CreateNode(ctx context.Context, node *Node) error
	GetNode(ctx context.Context, devnetName string, index int) (*Node, error)
	UpdateNode(ctx context.Context, node *Node) error
	DeleteNode(ctx context.Context, devnetName string, index int) error
	ListNodes(ctx context.Context, devnetName string) ([]*Node, error)

	// Upgrade operations
	CreateUpgrade(ctx context.Context, upgrade *Upgrade) error
	GetUpgrade(ctx context.Context, name string) (*Upgrade, error)
	UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error
	ListUpgrades(ctx context.Context, devnetName string) ([]*Upgrade, error)

	// Transaction operations
	CreateTransaction(ctx context.Context, tx *Transaction) error
	GetTransaction(ctx context.Context, id string) (*Transaction, error)
	UpdateTransaction(ctx context.Context, tx *Transaction) error
	ListTransactions(ctx context.Context, devnetName string, opts ListTxOptions) ([]*Transaction, error)

	// Watch watches for resource changes.
	Watch(ctx context.Context, resourceType string, handler WatchHandler) error

	// Close closes the store.
	Close() error
}
