package main

import (
	"bytes"
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFormatDescribeOutput(t *testing.T) {
	devnet := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "test-devnet",
			CreatedAt: timestamppb.New(time.Now().Add(-1 * time.Hour)),
		},
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Mode:       "docker",
			Validators: 4,
		},
		Status: &v1.DevnetStatus{
			Phase:      "Provisioning",
			Nodes:      4,
			ReadyNodes: 2,
			Message:    "Creating nodes",
			Conditions: []*v1.Condition{
				{
					Type:               "Ready",
					Status:             "False",
					Reason:             "NodesNotReady",
					Message:            "2/4 nodes ready",
					LastTransitionTime: timestamppb.Now(),
				},
				{
					Type:               "Progressing",
					Status:             "True",
					Reason:             "CreatingNodes",
					Message:            "Creating validator nodes",
					LastTransitionTime: timestamppb.Now(),
				},
			},
			Events: []*v1.Event{
				{
					Timestamp: timestamppb.Now(),
					Type:      "Normal",
					Reason:    "Provisioning",
					Message:   "Started provisioning",
					Component: "devnet-controller",
				},
			},
		},
	}

	var buf bytes.Buffer
	// Pass plugin available=true and empty networks list to avoid troubleshooting output
	formatDescribeOutput(&buf, devnet, nil, true, nil)
	output := buf.String()

	// Check key sections exist
	if !bytes.Contains([]byte(output), []byte("Name:")) {
		t.Error("missing Name field")
	}
	if !bytes.Contains([]byte(output), []byte("Conditions:")) {
		t.Error("missing Conditions section")
	}
	if !bytes.Contains([]byte(output), []byte("Events:")) {
		t.Error("missing Events section")
	}
	if !bytes.Contains([]byte(output), []byte("NodesNotReady")) {
		t.Error("missing condition reason")
	}
}

func TestFormatDescribeOutputWithLongEventMessage(t *testing.T) {
	// Create a very long multi-line error message similar to real stack traces
	longMessage := `Provisioning failed: orchestrator execution failed: forking phase failed: genesis fork failed: failed to fetch genesis: failed to export genesis from snapshot: state export export error: export failed: exit status 2
Output: 10:48AM INF Upgrading IAVL storage for faster queries + execution on live state. This may take a while commit=436F6D6D697449447B5B5D3A307D module=server store_key="KVStoreKey{0xc0047f3c10, precisebank}" version=1
10:48AM INF Upgrading IAVL storage for faster queries + execution on live state. This may take a while commit=436F6D6D697449447B5B5D3A307D module=server store_key="KVStoreKey{0xc0047f3b70, staking}" version=1
panic: collections: not found: key 'no_key' of type github.com/cosmos/gogoproto/cosmos.distribution.v1beta1.FeePool`

	devnet := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "test-devnet",
			CreatedAt: timestamppb.New(time.Now().Add(-1 * time.Hour)),
		},
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Mode:       "local",
			Validators: 1,
		},
		Status: &v1.DevnetStatus{
			Phase:      "Degraded",
			Nodes:      1,
			ReadyNodes: 0,
			Message:    "No nodes healthy",
			Events: []*v1.Event{
				{
					Timestamp: timestamppb.Now(),
					Type:      "Warning",
					Reason:    "ProvisioningFailed",
					Message:   longMessage,
					Component: "devnet-controller",
				},
			},
		},
	}

	var buf bytes.Buffer
	formatDescribeOutput(&buf, devnet, nil, true, nil)
	output := buf.String()

	// Verify the message is truncated
	lines := bytes.Split([]byte(output), []byte("\n"))
	var eventLine string
	for _, line := range lines {
		if bytes.Contains(line, []byte("ProvisioningFailed")) {
			eventLine = string(line)
			break
		}
	}

	if eventLine == "" {
		t.Fatal("event line not found in output")
	}

	// The message should be truncated to ~120 chars
	if len(eventLine) > 300 {
		t.Errorf("event line too long: %d chars, expected < 300", len(eventLine))
	}

	// Should contain truncation indicator
	if !bytes.Contains([]byte(eventLine), []byte("...")) {
		t.Error("expected truncation indicator '...' in long message")
	}

	// Should not contain newlines in the message portion
	msgStart := bytes.Index([]byte(eventLine), []byte("ProvisioningFailed"))
	if msgStart >= 0 {
		msgPortion := eventLine[msgStart:]
		// The original message has literal "\n" which should not appear in output
		if bytes.Contains([]byte(msgPortion), []byte("Output:")) && bytes.Count([]byte(msgPortion), []byte("\n")) > 0 {
			t.Error("event message should not contain literal newlines")
		}
	}
}
