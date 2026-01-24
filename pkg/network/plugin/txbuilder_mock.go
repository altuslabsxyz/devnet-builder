// pkg/network/plugin/txbuilder_mock.go
package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// MockTxBuilder implements network.TxBuilder for testing purposes.
type MockTxBuilder struct {
	mu           sync.Mutex
	txCounter    int
	broadcasts   []*network.SignedTx
	supportTypes []network.TxType

	// Configurable behaviors for testing
	BuildErr     error
	SignErr      error
	BroadcastErr error
}

// NewMockTxBuilder creates a new mock TxBuilder.
func NewMockTxBuilder() *MockTxBuilder {
	return &MockTxBuilder{
		supportTypes: []network.TxType{
			network.TxTypeGovProposal,
			network.TxTypeGovVote,
			network.TxTypeBankSend,
		},
	}
}

// BuildTx constructs a mock unsigned transaction.
func (m *MockTxBuilder) BuildTx(ctx context.Context, req *network.TxBuildRequest) (*network.UnsignedTx, error) {
	if m.BuildErr != nil {
		return nil, m.BuildErr
	}

	m.mu.Lock()
	m.txCounter++
	seq := uint64(m.txCounter)
	m.mu.Unlock()

	// Create deterministic tx bytes from request
	txBytes := []byte(fmt.Sprintf("tx:%s:%s:%d", req.TxType, req.Sender, seq))
	signDoc := []byte(fmt.Sprintf("signdoc:%s", string(txBytes)))

	return &network.UnsignedTx{
		TxBytes:       txBytes,
		SignDoc:       signDoc,
		AccountNumber: 1,
		Sequence:      seq,
	}, nil
}

// SignTx signs a mock transaction.
func (m *MockTxBuilder) SignTx(ctx context.Context, tx *network.UnsignedTx, key *network.SigningKey) (*network.SignedTx, error) {
	if m.SignErr != nil {
		return nil, m.SignErr
	}

	// Create mock signature from sign doc
	hash := sha256.Sum256(tx.SignDoc)
	signature := hash[:]

	return &network.SignedTx{
		TxBytes:   tx.TxBytes,
		Signature: signature,
		PubKey:    []byte(key.Address),
	}, nil
}

// BroadcastTx broadcasts a mock transaction.
func (m *MockTxBuilder) BroadcastTx(ctx context.Context, tx *network.SignedTx) (*network.TxBroadcastResult, error) {
	if m.BroadcastErr != nil {
		return nil, m.BroadcastErr
	}

	m.mu.Lock()
	m.broadcasts = append(m.broadcasts, tx)
	m.mu.Unlock()

	// Generate tx hash from tx bytes
	hash := sha256.Sum256(tx.TxBytes)
	txHash := hex.EncodeToString(hash[:])

	return &network.TxBroadcastResult{
		TxHash: txHash,
		Code:   0,
		Height: 100,
	}, nil
}

// SupportedTxTypes returns supported transaction types.
func (m *MockTxBuilder) SupportedTxTypes() []network.TxType {
	return m.supportTypes
}

// Close is a no-op for the mock builder.
func (m *MockTxBuilder) Close() {}

// GetBroadcasts returns all broadcast transactions (for test assertions).
func (m *MockTxBuilder) GetBroadcasts() []*network.SignedTx {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.broadcasts
}

// Ensure MockTxBuilder implements network.TxBuilder.
var _ network.TxBuilder = (*MockTxBuilder)(nil)
