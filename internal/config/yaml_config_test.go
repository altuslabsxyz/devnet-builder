// internal/config/yaml_config_test.go
package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestYAMLDevnet_Unmarshal(t *testing.T) {
	yamlContent := `
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test-devnet
  labels:
    team: core
spec:
  network: stable
  networkType: mainnet
  validators: 4
  mode: docker
`
	var devnet YAMLDevnet
	err := yaml.Unmarshal([]byte(yamlContent), &devnet)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if devnet.APIVersion != "devnet.lagos/v1" {
		t.Errorf("expected apiVersion devnet.lagos/v1, got %s", devnet.APIVersion)
	}
	if devnet.Kind != "Devnet" {
		t.Errorf("expected kind Devnet, got %s", devnet.Kind)
	}
	if devnet.Metadata.Name != "test-devnet" {
		t.Errorf("expected name test-devnet, got %s", devnet.Metadata.Name)
	}
	if devnet.Spec.Validators != 4 {
		t.Errorf("expected 4 validators, got %d", devnet.Spec.Validators)
	}
}

func TestYAMLDevnet_Validate_Valid(t *testing.T) {
	devnet := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	err := devnet.Validate()
	if err != nil {
		t.Errorf("Validate() failed for valid devnet: %v", err)
	}
}

func TestYAMLDevnet_Validate_MissingName(t *testing.T) {
	devnet := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata:   YAMLMetadata{Name: ""},
		Spec:       YAMLDevnetSpec{Network: "stable"},
	}

	err := devnet.Validate()
	if err == nil {
		t.Error("Validate() should fail for missing name")
	}
}

func TestYAMLDevnet_Validate_InvalidAPIVersion(t *testing.T) {
	devnet := YAMLDevnet{
		APIVersion: "invalid/v1",
		Kind:       "Devnet",
		Metadata:   YAMLMetadata{Name: "test"},
		Spec:       YAMLDevnetSpec{Network: "stable"},
	}

	err := devnet.Validate()
	if err == nil {
		t.Error("Validate() should fail for invalid apiVersion")
	}
}

func TestYAMLDevnet_Validate_InvalidValidators(t *testing.T) {
	devnet := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 0,
		},
	}

	err := devnet.Validate()
	if err == nil {
		t.Error("Validate() should fail for zero validators")
	}
}

func TestYAMLDevnetNamespace(t *testing.T) {
	yamlContent := `
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test
  namespace: production
spec:
  network: stable
  validators: 4
`
	var devnet YAMLDevnet
	err := yaml.Unmarshal([]byte(yamlContent), &devnet)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if devnet.Metadata.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", devnet.Metadata.Namespace)
	}
}

func TestYAMLDevnetNamespace_DefaultValue(t *testing.T) {
	yamlContent := `
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test
spec:
  network: stable
  validators: 4
`
	var devnet YAMLDevnet
	err := yaml.Unmarshal([]byte(yamlContent), &devnet)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	// When namespace is not specified, it should be empty string (caller handles default)
	if devnet.Metadata.Namespace != "" {
		t.Errorf("expected empty namespace when not specified, got %s", devnet.Metadata.Namespace)
	}
}

func TestYAMLDevnet_Validate_WithNamespace(t *testing.T) {
	devnet := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata:   YAMLMetadata{Name: "test", Namespace: "production"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	err := devnet.Validate()
	if err != nil {
		t.Errorf("Validate() failed for devnet with namespace: %v", err)
	}
}

func TestYAMLDevnet_Validate_WithoutNamespace(t *testing.T) {
	devnet := YAMLDevnet{
		APIVersion: "devnet.lagos/v1",
		Kind:       "Devnet",
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	err := devnet.Validate()
	if err != nil {
		t.Errorf("Validate() failed for devnet without namespace: %v", err)
	}
}
