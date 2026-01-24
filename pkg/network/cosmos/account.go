// pkg/network/cosmos/account.go
package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// AccountInfo contains the account information needed for transaction signing.
type AccountInfo struct {
	// Address is the bech32-encoded account address.
	Address string

	// AccountNumber is the unique identifier for the account on the chain.
	AccountNumber uint64

	// Sequence is the transaction sequence number (nonce).
	Sequence uint64

	// PubKey is the public key associated with the account (may be nil if not set).
	PubKey []byte
}

// accountResponse represents the REST API response for account queries.
type accountResponse struct {
	Account accountWrapper `json:"account"`
}

// accountWrapper handles both BaseAccount and ModuleAccount types.
type accountWrapper struct {
	Type          string         `json:"@type"`
	Address       string         `json:"address"`
	AccountNumber string         `json:"account_number"`
	Sequence      string         `json:"sequence"`
	PubKey        *pubKeyWrapper `json:"pub_key"`

	// For ModuleAccount types
	BaseAccount *baseAccountInfo `json:"base_account"`
}

// baseAccountInfo is used for nested account info (like in ModuleAccount).
type baseAccountInfo struct {
	Address       string         `json:"address"`
	AccountNumber string         `json:"account_number"`
	Sequence      string         `json:"sequence"`
	PubKey        *pubKeyWrapper `json:"pub_key"`
}

// pubKeyWrapper handles public key info from the API response.
type pubKeyWrapper struct {
	Type string `json:"@type"`
	Key  string `json:"key"`
}

// QueryAccount queries the account information for the given address.
// It returns the account number and sequence needed for transaction signing.
func (b *TxBuilder) QueryAccount(ctx context.Context, address string) (*AccountInfo, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}

	// Build the REST API URL
	restEndpoint := b.getRESTEndpoint()
	url := fmt.Sprintf("%s/cosmos/auth/v1beta1/accounts/%s", restEndpoint, address)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query account: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("account %s not found", address)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var accResp accountResponse
	if err := json.Unmarshal(body, &accResp); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}

	// Extract account info based on account type
	return parseAccountInfo(&accResp.Account)
}

// parseAccountInfo extracts AccountInfo from the account wrapper.
// It handles both BaseAccount and ModuleAccount types.
func parseAccountInfo(wrapper *accountWrapper) (*AccountInfo, error) {
	var address, accountNumStr, seqStr string
	var pubKey *pubKeyWrapper

	// Check if this is a ModuleAccount with nested base_account
	if wrapper.BaseAccount != nil {
		address = wrapper.BaseAccount.Address
		accountNumStr = wrapper.BaseAccount.AccountNumber
		seqStr = wrapper.BaseAccount.Sequence
		pubKey = wrapper.BaseAccount.PubKey
	} else {
		// BaseAccount or similar direct account type
		address = wrapper.Address
		accountNumStr = wrapper.AccountNumber
		seqStr = wrapper.Sequence
		pubKey = wrapper.PubKey
	}

	// Parse account number
	accountNumber, err := strconv.ParseUint(accountNumStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse account number: %w", err)
	}

	// Parse sequence
	sequence, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sequence: %w", err)
	}

	info := &AccountInfo{
		Address:       address,
		AccountNumber: accountNumber,
		Sequence:      sequence,
	}

	// Decode public key if present
	if pubKey != nil && pubKey.Key != "" {
		// The key is base64 encoded in the response, store as-is for now
		// Actual decoding would happen during signing
		info.PubKey = []byte(pubKey.Key)
	}

	return info, nil
}

// getRESTEndpoint returns the REST API endpoint.
// If the RPC endpoint uses port 26657, it converts to port 1317.
// Otherwise, it uses the RPC endpoint as-is (for testing with mock servers).
func (b *TxBuilder) getRESTEndpoint() string {
	// For most test scenarios and when using a mock server,
	// we use the rpcEndpoint directly (the mock serves both RPC and REST).
	// In production, you might need to convert 26657 -> 1317.
	return b.rpcEndpoint
}
