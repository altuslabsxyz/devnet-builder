// Package evm provides EVM RPC client implementations.
package evm

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Client implements ports.EVMClient using go-ethereum.
type Client struct {
	rpcURL  string
	client  *ethclient.Client
	chainID *big.Int
}

// NewClient creates a new EVM client.
func NewClient(rpcURL string) *Client {
	return &Client{
		rpcURL: rpcURL,
	}
}

// Connect establishes a connection to the EVM RPC endpoint.
func (c *Client) Connect(ctx context.Context) error {
	client, err := ethclient.DialContext(ctx, c.rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to EVM RPC: %w", err)
	}
	c.client = client

	// Cache chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to get chain ID: %w", err)
	}
	c.chainID = chainID

	return nil
}

// SendTransaction sends a signed transaction (RLP encoded).
func (c *Client) SendTransaction(ctx context.Context, signedTx []byte) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("client not connected")
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(signedTx); err != nil {
		return "", fmt.Errorf("failed to decode transaction: %w", err)
	}

	if err := c.client.SendTransaction(ctx, tx); err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return tx.Hash().Hex(), nil
}

// SendRawTransaction sends a pre-built transaction (not yet used - placeholder).
func (c *Client) SendRawTransaction(ctx context.Context, tx *ports.EVMTransaction) (string, error) {
	// This method would need the private key to sign
	// For now, use SendTransaction with pre-signed tx
	return "", fmt.Errorf("SendRawTransaction requires signing - use SendTransaction with pre-signed tx")
}

// GetBalance retrieves the balance of an address.
func (c *Client) GetBalance(ctx context.Context, address string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("client not connected")
	}

	addr := common.HexToAddress(address)
	balance, err := c.client.BalanceAt(ctx, addr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get balance: %w", err)
	}

	return balance.String(), nil
}

// GetNonce retrieves the pending nonce for an address.
func (c *Client) GetNonce(ctx context.Context, address string) (uint64, error) {
	if c.client == nil {
		return 0, fmt.Errorf("client not connected")
	}

	addr := common.HexToAddress(address)
	nonce, err := c.client.PendingNonceAt(ctx, addr)
	if err != nil {
		return 0, fmt.Errorf("failed to get nonce: %w", err)
	}

	return nonce, nil
}

// GetChainID returns the chain ID.
func (c *Client) GetChainID(ctx context.Context) (int64, error) {
	if c.chainID != nil {
		return c.chainID.Int64(), nil
	}

	if c.client == nil {
		return 0, fmt.Errorf("client not connected")
	}

	chainID, err := c.client.ChainID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get chain ID: %w", err)
	}

	c.chainID = chainID
	return chainID.Int64(), nil
}

// SuggestGasPrice returns a suggested gas price.
func (c *Client) SuggestGasPrice(ctx context.Context) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("client not connected")
	}

	gasPrice, err := c.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to suggest gas price: %w", err)
	}

	return gasPrice.String(), nil
}

// EstimateGas estimates gas for a transaction.
func (c *Client) EstimateGas(ctx context.Context, msg *ports.EVMCallMsg) (uint64, error) {
	if c.client == nil {
		return 0, fmt.Errorf("client not connected")
	}

	toAddr := common.HexToAddress(msg.To)
	gasPrice := new(big.Int)
	gasPrice.SetString(msg.GasPrice, 10)

	callMsg := ethereum.CallMsg{
		From:     common.HexToAddress(msg.From),
		To:       &toAddr,
		GasPrice: gasPrice,
		Data:     msg.Data,
	}

	gas, err := c.client.EstimateGas(ctx, callMsg)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate gas: %w", err)
	}

	return gas, nil
}

// WaitForTransaction waits for a transaction to be mined.
func (c *Client) WaitForTransaction(ctx context.Context, txHash string, timeout time.Duration) (*ports.TxReceipt, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	hash := common.HexToHash(txHash)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash)
		case <-ticker.C:
			receipt, err := c.client.TransactionReceipt(ctx, hash)
			if err != nil {
				// Transaction not yet mined, continue waiting
				continue
			}

			// Convert logs
			logs := make([]ports.TxLog, len(receipt.Logs))
			for i, log := range receipt.Logs {
				topics := make([]string, len(log.Topics))
				for j, topic := range log.Topics {
					topics[j] = topic.Hex()
				}
				logs[i] = ports.TxLog{
					Address: log.Address.Hex(),
					Topics:  topics,
					Data:    log.Data,
				}
			}

			return &ports.TxReceipt{
				TxHash:      receipt.TxHash.Hex(),
				BlockNumber: receipt.BlockNumber.Int64(),
				Status:      receipt.Status == types.ReceiptStatusSuccessful,
				GasUsed:     receipt.GasUsed,
				Logs:        logs,
			}, nil
		}
	}
}

// Close closes the client connection.
func (c *Client) Close() error {
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
	return nil
}

// GetEthClient returns the underlying ethclient for advanced operations.
// This is useful for signing transactions.
func (c *Client) GetEthClient() *ethclient.Client {
	return c.client
}

// GetCachedChainID returns the cached chain ID as big.Int.
func (c *Client) GetCachedChainID() *big.Int {
	return c.chainID
}

// Ensure Client implements ports.EVMClient.
var _ ports.EVMClient = (*Client)(nil)
