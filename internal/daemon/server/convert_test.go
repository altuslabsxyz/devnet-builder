package server

import (
	"testing"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestDevnetToProto(t *testing.T) {
	now := time.Now()

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{
			Name:       "test-devnet",
			Generation: 3,
			CreatedAt:  now,
			UpdatedAt:  now.Add(time.Hour),
			Labels:     map[string]string{"env": "test"},
			Annotations: map[string]string{"note": "example"},
		},
		Spec: types.DevnetSpec{
			Plugin:      "stable",
			NetworkType: "cosmos",
			Validators:  4,
			FullNodes:   2,
			Mode:        "docker",
		},
		Status: types.DevnetStatus{
			Phase:           types.PhaseRunning,
			Nodes:           6,
			ReadyNodes:      6,
			CurrentHeight:   12345,
			SDKVersion:      "v0.50.0",
			LastHealthCheck: now,
			Message:         "Running smoothly",
		},
	}

	pb := DevnetToProto(devnet)

	// Check metadata
	if pb.Metadata.Name != "test-devnet" {
		t.Errorf("expected name test-devnet, got %s", pb.Metadata.Name)
	}
	if pb.Metadata.Generation != 3 {
		t.Errorf("expected generation 3, got %d", pb.Metadata.Generation)
	}
	if pb.Metadata.Labels["env"] != "test" {
		t.Errorf("expected label env=test, got %v", pb.Metadata.Labels)
	}
	if pb.Metadata.Annotations["note"] != "example" {
		t.Errorf("expected annotation note=example, got %v", pb.Metadata.Annotations)
	}

	// Check spec
	if pb.Spec.Plugin != "stable" {
		t.Errorf("expected plugin stable, got %s", pb.Spec.Plugin)
	}
	if pb.Spec.NetworkType != "cosmos" {
		t.Errorf("expected networkType cosmos, got %s", pb.Spec.NetworkType)
	}
	if pb.Spec.Validators != 4 {
		t.Errorf("expected validators 4, got %d", pb.Spec.Validators)
	}
	if pb.Spec.FullNodes != 2 {
		t.Errorf("expected fullNodes 2, got %d", pb.Spec.FullNodes)
	}
	if pb.Spec.Mode != "docker" {
		t.Errorf("expected mode docker, got %s", pb.Spec.Mode)
	}

	// Check status
	if pb.Status.Phase != "Running" {
		t.Errorf("expected phase Running, got %s", pb.Status.Phase)
	}
	if pb.Status.Nodes != 6 {
		t.Errorf("expected nodes 6, got %d", pb.Status.Nodes)
	}
	if pb.Status.ReadyNodes != 6 {
		t.Errorf("expected readyNodes 6, got %d", pb.Status.ReadyNodes)
	}
	if pb.Status.CurrentHeight != 12345 {
		t.Errorf("expected currentHeight 12345, got %d", pb.Status.CurrentHeight)
	}
	if pb.Status.SdkVersion != "v0.50.0" {
		t.Errorf("expected sdkVersion v0.50.0, got %s", pb.Status.SdkVersion)
	}
	if pb.Status.Message != "Running smoothly" {
		t.Errorf("expected message 'Running smoothly', got %s", pb.Status.Message)
	}
}

func TestDevnetFromProto(t *testing.T) {
	pb := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:        "proto-devnet",
			Generation:  5,
			Labels:      map[string]string{"team": "infra"},
			Annotations: map[string]string{"desc": "test"},
		},
		Spec: &v1.DevnetSpec{
			Plugin:      "osmosis",
			NetworkType: "cosmos",
			Validators:  3,
			FullNodes:   1,
			Mode:        "local",
			SdkVersion:  "v0.47.0",
			GenesisPath: "/path/to/genesis.json",
			SnapshotUrl: "https://example.com/snapshot.tar.gz",
		},
		Status: &v1.DevnetStatus{
			Phase:         "Provisioning",
			Nodes:         4,
			ReadyNodes:    2,
			CurrentHeight: 999,
			SdkVersion:    "v0.47.0",
			Message:       "Provisioning in progress",
		},
	}

	devnet := DevnetFromProto(pb)

	// Check metadata
	if devnet.Metadata.Name != "proto-devnet" {
		t.Errorf("expected name proto-devnet, got %s", devnet.Metadata.Name)
	}
	if devnet.Metadata.Generation != 5 {
		t.Errorf("expected generation 5, got %d", devnet.Metadata.Generation)
	}
	if devnet.Metadata.Labels["team"] != "infra" {
		t.Errorf("expected label team=infra, got %v", devnet.Metadata.Labels)
	}

	// Check spec
	if devnet.Spec.Plugin != "osmosis" {
		t.Errorf("expected plugin osmosis, got %s", devnet.Spec.Plugin)
	}
	if devnet.Spec.Validators != 3 {
		t.Errorf("expected validators 3, got %d", devnet.Spec.Validators)
	}
	if devnet.Spec.GenesisPath != "/path/to/genesis.json" {
		t.Errorf("expected genesisPath, got %s", devnet.Spec.GenesisPath)
	}
	if devnet.Spec.SnapshotURL != "https://example.com/snapshot.tar.gz" {
		t.Errorf("expected snapshotUrl, got %s", devnet.Spec.SnapshotURL)
	}

	// Check status
	if devnet.Status.Phase != "Provisioning" {
		t.Errorf("expected phase Provisioning, got %s", devnet.Status.Phase)
	}
	if devnet.Status.Nodes != 4 {
		t.Errorf("expected nodes 4, got %d", devnet.Status.Nodes)
	}
	if devnet.Status.Message != "Provisioning in progress" {
		t.Errorf("expected message, got %s", devnet.Status.Message)
	}
}

func TestDevnetFromProto_NilFields(t *testing.T) {
	// Test with minimal proto (nil nested fields)
	pb := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name: "minimal",
		},
	}

	devnet := DevnetFromProto(pb)

	if devnet.Metadata.Name != "minimal" {
		t.Errorf("expected name minimal, got %s", devnet.Metadata.Name)
	}

	// Should not panic with nil spec/status
	if devnet.Spec.Plugin != "" {
		t.Errorf("expected empty plugin, got %s", devnet.Spec.Plugin)
	}
}

func TestCreateRequestToDevnet(t *testing.T) {
	req := &v1.CreateDevnetRequest{
		Name: "new-devnet",
		Spec: &v1.DevnetSpec{
			Plugin:      "stable",
			NetworkType: "cosmos",
			Validators:  4,
			Mode:        "docker",
		},
		Labels: map[string]string{"env": "dev"},
	}

	devnet := CreateRequestToDevnet(req)

	if devnet.Metadata.Name != "new-devnet" {
		t.Errorf("expected name new-devnet, got %s", devnet.Metadata.Name)
	}
	if devnet.Metadata.Labels["env"] != "dev" {
		t.Errorf("expected label env=dev, got %v", devnet.Metadata.Labels)
	}
	if devnet.Spec.Plugin != "stable" {
		t.Errorf("expected plugin stable, got %s", devnet.Spec.Plugin)
	}
	if devnet.Status.Phase != types.PhasePending {
		t.Errorf("expected phase Pending, got %s", devnet.Status.Phase)
	}
}
