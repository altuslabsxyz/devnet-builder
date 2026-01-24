// pkg/network/plugin/txbuilder_test.go
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

func TestMockTxBuilder_BuildSignBroadcast(t *testing.T) {
	builder := NewMockTxBuilder()
	ctx := context.Background()

	// Build
	req := &network.TxBuildRequest{
		TxType:  network.TxTypeGovVote,
		Sender:  "cosmos1abc",
		Payload: json.RawMessage(`{"proposal_id":1,"option":"yes"}`),
		ChainID: "test-chain",
	}

	unsigned, err := builder.BuildTx(ctx, req)
	if err != nil {
		t.Fatalf("BuildTx: %v", err)
	}
	if unsigned.Sequence != 1 {
		t.Errorf("Sequence = %d, want 1", unsigned.Sequence)
	}

	// Sign
	key := &network.SigningKey{Address: "cosmos1abc"}
	signed, err := builder.SignTx(ctx, unsigned, key)
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}
	if len(signed.Signature) == 0 {
		t.Error("Signature is empty")
	}

	// Broadcast
	result, err := builder.BroadcastTx(ctx, signed)
	if err != nil {
		t.Fatalf("BroadcastTx: %v", err)
	}
	if result.TxHash == "" {
		t.Error("TxHash is empty")
	}
	if result.Code != 0 {
		t.Errorf("Code = %d, want 0", result.Code)
	}

	// Verify broadcast was recorded
	broadcasts := builder.GetBroadcasts()
	if len(broadcasts) != 1 {
		t.Errorf("Broadcasts = %d, want 1", len(broadcasts))
	}
}

func TestMockTxBuilder_Errors(t *testing.T) {
	builder := NewMockTxBuilder()
	ctx := context.Background()

	// Test build error
	builder.BuildErr = errors.New("build failed")
	_, err := builder.BuildTx(ctx, &network.TxBuildRequest{})
	if err == nil {
		t.Error("Expected build error")
	}
	builder.BuildErr = nil

	// Test sign error
	unsigned := &network.UnsignedTx{SignDoc: []byte("test")}
	builder.SignErr = errors.New("sign failed")
	_, err = builder.SignTx(ctx, unsigned, &network.SigningKey{})
	if err == nil {
		t.Error("Expected sign error")
	}
	builder.SignErr = nil

	// Test broadcast error
	signed := &network.SignedTx{TxBytes: []byte("test")}
	builder.BroadcastErr = errors.New("broadcast failed")
	_, err = builder.BroadcastTx(ctx, signed)
	if err == nil {
		t.Error("Expected broadcast error")
	}
}

func TestMockTxBuilder_SupportedTypes(t *testing.T) {
	builder := NewMockTxBuilder()
	types := builder.SupportedTxTypes()

	if len(types) == 0 {
		t.Error("SupportedTxTypes returned empty slice")
	}

	// Check for expected types
	found := false
	for _, tt := range types {
		if tt == network.TxTypeGovVote {
			found = true
			break
		}
	}
	if !found {
		t.Error("TxTypeGovVote not in supported types")
	}
}
