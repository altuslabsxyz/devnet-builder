// internal/daemon/controller/transaction.go
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// TxController reconciles Transaction resources.
type TxController struct {
	store   store.Store
	runtime TxRuntime
	logger  *slog.Logger

	// Cache for in-flight transactions (unsigned tx bytes)
	unsignedTxCache map[string]*network.UnsignedTx
}

// NewTxController creates a new TxController.
func NewTxController(s store.Store, r TxRuntime) *TxController {
	return &TxController{
		store:           s,
		runtime:         r,
		logger:          slog.Default(),
		unsignedTxCache: make(map[string]*network.UnsignedTx),
	}
}

// SetLogger sets the logger.
func (c *TxController) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

// Reconcile processes a single transaction by name.
func (c *TxController) Reconcile(ctx context.Context, key string) error {
	c.logger.Debug("reconciling transaction", "key", key)

	tx, err := c.store.GetTransaction(ctx, key)
	if err != nil {
		if store.IsNotFound(err) {
			c.logger.Debug("transaction not found (deleted?)", "key", key)
			return nil
		}
		return err
	}

	switch tx.Status.Phase {
	case "", types.TxPhasePending:
		return c.reconcilePending(ctx, tx)
	case types.TxPhaseBuilding:
		return c.reconcileBuilding(ctx, tx)
	case types.TxPhaseSigning:
		return c.reconcileSigning(ctx, tx)
	case types.TxPhaseSubmitted:
		return c.reconcileSubmitted(ctx, tx)
	case types.TxPhaseConfirmed, types.TxPhaseFailed:
		return nil // Terminal states
	default:
		c.logger.Warn("unknown transaction phase", "key", key, "phase", tx.Status.Phase)
		return nil
	}
}

func (c *TxController) reconcilePending(ctx context.Context, tx *types.Transaction) error {
	c.logger.Info("transaction pending, moving to building",
		"name", tx.Metadata.Name,
		"devnet", tx.Spec.DevnetRef)

	tx.Status.Phase = types.TxPhaseBuilding
	tx.Status.Message = "Building transaction"
	tx.Metadata.UpdatedAt = time.Now()

	return c.store.UpdateTransaction(ctx, tx)
}

func (c *TxController) reconcileBuilding(ctx context.Context, tx *types.Transaction) error {
	c.logger.Debug("building transaction", "name", tx.Metadata.Name)

	builder, err := c.runtime.GetTxBuilder(ctx, tx.Spec.DevnetRef)
	if err != nil {
		return c.setFailed(ctx, tx, fmt.Sprintf("failed to get TxBuilder: %v", err))
	}

	unsignedTx, err := builder.BuildTx(ctx, &network.TxBuildRequest{
		TxType:   network.TxType(tx.Spec.TxType),
		Sender:   tx.Spec.Signer,
		Payload:  tx.Spec.Payload,
		GasLimit: 200000, // TODO: make configurable
	})
	if err != nil {
		return c.setFailed(ctx, tx, fmt.Sprintf("failed to build tx: %v", err))
	}

	// Cache unsigned tx for signing phase
	c.unsignedTxCache[tx.Metadata.Name] = unsignedTx

	tx.Status.Phase = types.TxPhaseSigning
	tx.Status.Message = "Signing transaction"
	tx.Metadata.UpdatedAt = time.Now()

	return c.store.UpdateTransaction(ctx, tx)
}

func (c *TxController) reconcileSigning(ctx context.Context, tx *types.Transaction) error {
	c.logger.Debug("signing transaction", "name", tx.Metadata.Name)

	// Get cached unsigned tx
	unsignedTx, ok := c.unsignedTxCache[tx.Metadata.Name]
	if !ok {
		// If not in cache, rebuild it
		builder, err := c.runtime.GetTxBuilder(ctx, tx.Spec.DevnetRef)
		if err != nil {
			return c.setFailed(ctx, tx, fmt.Sprintf("failed to get TxBuilder: %v", err))
		}
		unsignedTx, err = builder.BuildTx(ctx, &network.TxBuildRequest{
			TxType:  network.TxType(tx.Spec.TxType),
			Sender:  tx.Spec.Signer,
			Payload: tx.Spec.Payload,
		})
		if err != nil {
			return c.setFailed(ctx, tx, fmt.Sprintf("failed to rebuild tx: %v", err))
		}
	}

	// Get signing key
	key, err := c.runtime.GetSigningKey(ctx, tx.Spec.DevnetRef, tx.Spec.Signer)
	if err != nil {
		return c.setFailed(ctx, tx, fmt.Sprintf("failed to get signing key: %v", err))
	}

	// Sign
	builder, _ := c.runtime.GetTxBuilder(ctx, tx.Spec.DevnetRef)
	signedTx, err := builder.SignTx(ctx, unsignedTx, key)
	if err != nil {
		return c.setFailed(ctx, tx, fmt.Sprintf("failed to sign tx: %v", err))
	}

	// Broadcast
	result, err := builder.BroadcastTx(ctx, signedTx)
	if err != nil {
		return c.setFailed(ctx, tx, fmt.Sprintf("failed to broadcast tx: %v", err))
	}

	// Clean up cache
	delete(c.unsignedTxCache, tx.Metadata.Name)

	tx.Status.Phase = types.TxPhaseSubmitted
	tx.Status.TxHash = result.TxHash
	tx.Status.Message = fmt.Sprintf("Submitted: %s", result.TxHash)
	tx.Metadata.UpdatedAt = time.Now()

	return c.store.UpdateTransaction(ctx, tx)
}

func (c *TxController) reconcileSubmitted(ctx context.Context, tx *types.Transaction) error {
	c.logger.Debug("waiting for confirmation", "name", tx.Metadata.Name, "txHash", tx.Status.TxHash)

	receipt, err := c.runtime.WaitForConfirmation(ctx, tx.Spec.DevnetRef, tx.Status.TxHash)
	if err != nil {
		return c.setFailed(ctx, tx, fmt.Sprintf("confirmation failed: %v", err))
	}

	if !receipt.Success {
		return c.setFailed(ctx, tx, fmt.Sprintf("tx failed: %s", receipt.Log))
	}

	tx.Status.Phase = types.TxPhaseConfirmed
	tx.Status.Height = receipt.Height
	tx.Status.GasUsed = receipt.GasUsed
	tx.Status.Message = fmt.Sprintf("Confirmed at height %d", receipt.Height)
	tx.Metadata.UpdatedAt = time.Now()

	c.logger.Info("transaction confirmed",
		"name", tx.Metadata.Name,
		"txHash", tx.Status.TxHash,
		"height", tx.Status.Height)

	return c.store.UpdateTransaction(ctx, tx)
}

func (c *TxController) setFailed(ctx context.Context, tx *types.Transaction, errMsg string) error {
	c.logger.Error("transaction failed", "name", tx.Metadata.Name, "error", errMsg)

	tx.Status.Phase = types.TxPhaseFailed
	tx.Status.Error = errMsg
	tx.Status.Message = "Transaction failed"
	tx.Metadata.UpdatedAt = time.Now()

	// Clean up cache
	delete(c.unsignedTxCache, tx.Metadata.Name)

	return c.store.UpdateTransaction(ctx, tx)
}
