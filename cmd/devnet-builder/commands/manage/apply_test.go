package manage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCmd_DryRun(t *testing.T) {
	// Create temp YAML file
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

	cmd := NewApplyCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"-f", yamlPath, "--dry-run"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Apply command failed: %v", err)
	}

	output := buf.String()
	if !contains(output, "test-devnet") {
		t.Errorf("output should contain devnet name, got: %s", output)
	}
	if !contains(output, "dry-run") || !contains(output, "Plan:") {
		t.Errorf("output should indicate dry-run mode, got: %s", output)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
