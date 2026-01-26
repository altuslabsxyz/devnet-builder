// cmd/dvb/get_test.go
package main

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"sigs.k8s.io/yaml"
)

func TestProtoDevnetToYAML_BasicConversion(t *testing.T) {
	devnet := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name: "test-devnet",
			Labels: map[string]string{
				"env": "test",
			},
		},
		Spec: &v1.DevnetSpec{
			Plugin:      "stable",
			NetworkType: "mainnet",
			Validators:  4,
			FullNodes:   2,
			Mode:        "docker",
			SdkVersion:  "v1.2.3",
		},
		Status: &v1.DevnetStatus{
			Phase:         "Running",
			Nodes:         6,
			ReadyNodes:    6,
			CurrentHeight: 1000,
			SdkVersion:    "v1.2.3",
		},
	}

	result := protoDevnetToYAML(devnet)

	// Check apiVersion and kind
	if result.APIVersion != "devnet.lagos/v1" {
		t.Errorf("expected apiVersion 'devnet.lagos/v1', got '%s'", result.APIVersion)
	}
	if result.Kind != "Devnet" {
		t.Errorf("expected kind 'Devnet', got '%s'", result.Kind)
	}

	// Check metadata
	if result.Metadata.Name != "test-devnet" {
		t.Errorf("expected metadata.name 'test-devnet', got '%s'", result.Metadata.Name)
	}
	if result.Metadata.Labels["env"] != "test" {
		t.Errorf("expected label env=test, got %v", result.Metadata.Labels)
	}

	// Check spec - note the field mappings
	if result.Spec.Network != "stable" {
		t.Errorf("expected spec.network 'stable', got '%s'", result.Spec.Network)
	}
	if result.Spec.NetworkVersion != "v1.2.3" {
		t.Errorf("expected spec.networkVersion 'v1.2.3', got '%s'", result.Spec.NetworkVersion)
	}
	if result.Spec.Validators != 4 {
		t.Errorf("expected spec.validators 4, got %d", result.Spec.Validators)
	}
	if result.Spec.Mode != "docker" {
		t.Errorf("expected spec.mode 'docker', got '%s'", result.Spec.Mode)
	}

	// Check status
	if result.Status.Phase != "Running" {
		t.Errorf("expected status.phase 'Running', got '%s'", result.Status.Phase)
	}
	if result.Status.ReadyNodes != 6 {
		t.Errorf("expected status.readyNodes 6, got %d", result.Status.ReadyNodes)
	}
}

func TestProtoDevnetToYAML_YAMLMarshaling(t *testing.T) {
	devnet := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name: "test-devnet",
		},
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: &v1.DevnetStatus{
			Phase:      "Running",
			Nodes:      4,
			ReadyNodes: 4,
		},
	}

	yamlOutput := protoDevnetToYAML(devnet)
	out, err := yaml.Marshal(yamlOutput)
	if err != nil {
		t.Fatalf("failed to marshal yaml: %v", err)
	}

	yamlStr := string(out)

	// Check that YAML contains expected fields
	expectedFields := []string{
		"apiVersion: devnet.lagos/v1",
		"kind: Devnet",
		"name: test-devnet",
		"network: stable",
		"validators: 4",
		"mode: docker",
		"phase: Running",
	}

	for _, field := range expectedFields {
		if !strings.Contains(yamlStr, field) {
			t.Errorf("expected YAML to contain '%s', got:\n%s", field, yamlStr)
		}
	}
}

func TestProtoDevnetToYAML_JSONMarshaling(t *testing.T) {
	devnet := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name: "test-devnet",
		},
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Status: &v1.DevnetStatus{
			Phase:      "Running",
			Nodes:      4,
			ReadyNodes: 4,
		},
	}

	yamlOutput := protoDevnetToYAML(devnet)
	out, err := json.MarshalIndent(yamlOutput, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal json: %v", err)
	}

	jsonStr := string(out)

	// Check that JSON contains expected fields
	expectedFields := []string{
		`"apiVersion": "devnet.lagos/v1"`,
		`"kind": "Devnet"`,
		`"name": "test-devnet"`,
		`"network": "stable"`,
		`"validators": 4`,
		`"mode": "docker"`,
		`"phase": "Running"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("expected JSON to contain '%s', got:\n%s", field, jsonStr)
		}
	}
}

func TestProtoDevnetToYAML_OmitsEmptyFields(t *testing.T) {
	// Create devnet with minimal fields
	devnet := &v1.Devnet{
		Metadata: &v1.DevnetMetadata{
			Name: "minimal-devnet",
			// No labels or annotations
		},
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 2,
			Mode:       "local",
			// No networkType, networkVersion, fullNodes
		},
		Status: &v1.DevnetStatus{
			Phase:      "Pending",
			Nodes:      2,
			ReadyNodes: 0,
			// No currentHeight, sdkVersion, message
		},
	}

	yamlOutput := protoDevnetToYAML(devnet)
	out, err := yaml.Marshal(yamlOutput)
	if err != nil {
		t.Fatalf("failed to marshal yaml: %v", err)
	}

	yamlStr := string(out)

	// These fields should NOT appear when empty (due to omitempty)
	unexpectedFields := []string{
		"labels:",
		"annotations:",
		"networkType:",
		"networkVersion:",
		"fullNodes:",
		"currentHeight:",
		"sdkVersion:",
		"message:",
	}

	for _, field := range unexpectedFields {
		if strings.Contains(yamlStr, field) {
			t.Errorf("expected YAML to omit '%s' when empty, got:\n%s", field, yamlStr)
		}
	}
}
