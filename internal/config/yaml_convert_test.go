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

func TestYAMLDevnet_ToProto_WithNamespace(t *testing.T) {
	yaml := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLMetadata{
			Name:      "test-devnet",
			Namespace: "production",
		},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	proto := yaml.ToProto()

	if proto.Metadata.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", proto.Metadata.Namespace)
	}
}

func TestYAMLDevnet_ToProto_DefaultNamespace(t *testing.T) {
	yaml := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLMetadata{
			Name: "test-devnet",
			// Namespace not specified
		},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	proto := yaml.ToProto()

	if proto.Metadata.Namespace != "default" {
		t.Errorf("expected namespace default when not specified, got %s", proto.Metadata.Namespace)
	}
}

func TestYAMLDevnet_FromProto_WithNamespace(t *testing.T) {
	proto := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name:      "proto-devnet",
			Namespace: "staging",
		},
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
		},
	}

	yaml := YAMLDevnetFromProto(proto)

	if yaml.Metadata.Namespace != "staging" {
		t.Errorf("expected namespace staging, got %s", yaml.Metadata.Namespace)
	}
}

func TestYAMLDevnet_ToCreateRequest_WithNamespace(t *testing.T) {
	yaml := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLMetadata{
			Name:      "test-devnet",
			Namespace: "production",
		},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
		},
	}

	req := yaml.ToCreateRequest()

	if req.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", req.Namespace)
	}
}

func TestYAMLDevnet_ToCreateRequest_DefaultNamespace(t *testing.T) {
	yaml := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata: YAMLMetadata{
			Name: "test-devnet",
		},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
		},
	}

	req := yaml.ToCreateRequest()

	if req.Namespace != "default" {
		t.Errorf("expected namespace default when not specified, got %s", req.Namespace)
	}
}
