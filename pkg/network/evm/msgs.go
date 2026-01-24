// pkg/network/evm/msgs.go
package evm

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// NativeTransferPayload contains the fields for an EVM native transfer transaction.
type NativeTransferPayload struct {
	// ToAddress is the recipient's hex address (with 0x prefix).
	ToAddress string `json:"to_address"`
	// Amount is the amount to send in wei (as a decimal string).
	Amount string `json:"amount"`
	// Data is optional hex-encoded data to include in the transaction (with 0x prefix).
	Data string `json:"data,omitempty"`
}

// ParseNativeTransferPayload parses and validates a native transfer payload.
// It validates that the to_address is a valid Ethereum hex address using common.IsHexAddress.
func ParseNativeTransferPayload(payload json.RawMessage) (*NativeTransferPayload, error) {
	var p NativeTransferPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal native transfer payload: %w", err)
	}

	// Validate to_address is present
	if p.ToAddress == "" {
		return nil, fmt.Errorf("to_address is required")
	}

	// Validate to_address is a valid hex address
	if !common.IsHexAddress(p.ToAddress) {
		return nil, fmt.Errorf("invalid to_address: %s (must be a valid hex address)", p.ToAddress)
	}

	// Validate amount is present
	if p.Amount == "" {
		return nil, fmt.Errorf("amount is required")
	}

	// Validate amount can be parsed
	if _, err := ParseAmount(p.Amount); err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	// Validate data if present
	if p.Data != "" {
		if _, err := parseHexData(p.Data); err != nil {
			return nil, fmt.Errorf("invalid data: %w", err)
		}
	}

	return &p, nil
}

// ParseAmount parses a wei amount string to *big.Int.
// The amount must be a non-negative integer string.
func ParseAmount(s string) (*big.Int, error) {
	if s == "" {
		return nil, fmt.Errorf("amount cannot be empty")
	}

	// Check for negative numbers
	if strings.HasPrefix(s, "-") {
		return nil, fmt.Errorf("invalid amount: %s (must be non-negative)", s)
	}

	amount, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %s (must be a valid integer)", s)
	}

	return amount, nil
}

// parseHexData parses hex-encoded data (with or without 0x prefix) to bytes.
func parseHexData(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	// Remove 0x prefix if present
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")

	return hex.DecodeString(s)
}

// GetDataBytes returns the data field as bytes, or nil if empty.
func (p *NativeTransferPayload) GetDataBytes() ([]byte, error) {
	if p.Data == "" {
		return nil, nil
	}
	return parseHexData(p.Data)
}

// GetToAddress returns the to_address as a common.Address.
func (p *NativeTransferPayload) GetToAddress() common.Address {
	return common.HexToAddress(p.ToAddress)
}

// GetAmount returns the amount as *big.Int.
func (p *NativeTransferPayload) GetAmount() (*big.Int, error) {
	return ParseAmount(p.Amount)
}
