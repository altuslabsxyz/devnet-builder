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
	formatDescribeOutput(&buf, devnet, nil)
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
