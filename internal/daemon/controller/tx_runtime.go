// internal/daemon/controller/tx_runtime.go
package controller

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
)

// TxRuntime abstracts plugin access for the TxController.
type TxRuntime interface {
	// GetTxBuilder returns a TxBuilder for the specified devnet.
	GetTxBuilder(ctx context.Context, devnetName string) (plugin.TxBuilder, error)

	// GetSigningKey retrieves the signing key for a validator/account.
	GetSigningKey(ctx context.Context, devnetName string, signer string) (*plugin.SigningKey, error)

	// WaitForConfirmation blocks until the transaction is confirmed or fails.
	WaitForConfirmation(ctx context.Context, devnetName string, txHash string) (*TxReceipt, error)
}

// TxReceipt contains the result of a confirmed transaction.
type TxReceipt struct {
	TxHash  string
	Height  int64
	GasUsed int64
	Success bool
	Log     string
}
