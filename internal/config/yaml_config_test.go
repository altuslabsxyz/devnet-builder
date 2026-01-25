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
