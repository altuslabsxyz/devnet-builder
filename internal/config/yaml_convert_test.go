package config

import (
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

func TestYAMLDevnet_ToProto(t *testing.T) {
	yaml := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLMetadata{
			Name: "test-devnet",
			Labels: map[string]string{
				"team": "core",
			},
		},
		Spec: YAMLDevnetSpec{
			Network:        "stable",
			NetworkType:    "mainnet",
			NetworkVersion: "v1.2.3",
			Validators:     4,
			Mode:           "docker",
			Accounts:       2,
		},
	}

	proto := yaml.ToProto()

	if proto.Metadata.Name != "test-devnet" {
		t.Errorf("expected name test-devnet, got %s", proto.Metadata.Name)
	}
	if proto.Metadata.Labels["team"] != "core" {
		t.Errorf("expected label team=core, got %v", proto.Metadata.Labels)
	}
	if proto.Spec.Plugin != "stable" {
		t.Errorf("expected plugin stable, got %s", proto.Spec.Plugin)
	}
	if proto.Spec.Validators != 4 {
		t.Errorf("expected 4 validators, got %d", proto.Spec.Validators)
	}
	if proto.Spec.Mode != "docker" {
		t.Errorf("expected mode docker, got %s", proto.Spec.Mode)
	}
}

func TestYAMLDevnet_FromProto(t *testing.T) {
	proto := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name: "proto-devnet",
			Labels: map[string]string{
				"env": "test",
			},
		},
		Spec: &v1.DevnetSpec{
			Plugin:      "stable",
			NetworkType: "mainnet",
			Validators:  2,
			Mode:        "local",
			SdkVersion:  "v1.0.0",
		},
		Status: &v1.DevnetStatus{
			Phase: "Running",
			Nodes: 2,
		},
	}

	yaml := YAMLDevnetFromProto(proto)

	if yaml.Metadata.Name != "proto-devnet" {
		t.Errorf("expected name proto-devnet, got %s", yaml.Metadata.Name)
	}
	if yaml.Spec.Network != "stable" {
		t.Errorf("expected network stable, got %s", yaml.Spec.Network)
	}
	if yaml.Spec.Validators != 2 {
		t.Errorf("expected 2 validators, got %d", yaml.Spec.Validators)
	}
}
