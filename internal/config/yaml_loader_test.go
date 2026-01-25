package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLLoader_LoadFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "devnet.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test-devnet
spec:
  network: stable
  validators: 4
  mode: docker
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := NewYAMLLoader()
	devnets, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(devnets) != 1 {
		t.Fatalf("expected 1 devnet, got %d", len(devnets))
	}

	if devnets[0].Metadata.Name != "test-devnet" {
		t.Errorf("expected name test-devnet, got %s", devnets[0].Metadata.Name)
	}
}

func TestYAMLLoader_LoadFile_MultiDocument(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "devnets.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: devnet-one
spec:
  network: stable
  validators: 4
---
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: devnet-two
spec:
  network: stable
  validators: 2
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := NewYAMLLoader()
	devnets, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if len(devnets) != 2 {
		t.Fatalf("expected 2 devnets, got %d", len(devnets))
	}

	if devnets[0].Metadata.Name != "devnet-one" {
		t.Errorf("expected first name devnet-one, got %s", devnets[0].Metadata.Name)
	}
	if devnets[1].Metadata.Name != "devnet-two" {
		t.Errorf("expected second name devnet-two, got %s", devnets[1].Metadata.Name)
	}
}

func TestYAMLLoader_LoadFile_ValidationError(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "invalid.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: ""
spec:
  network: stable
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := NewYAMLLoader()
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("LoadFile should fail for invalid YAML")
	}
}
