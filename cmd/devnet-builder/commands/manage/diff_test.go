package manage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDiffCmd_NoChanges(t *testing.T) {
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

	cmd := NewDiffCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"-f", yamlPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Diff command failed: %v", err)
	}

	output := buf.String()
	if !contains(output, "test-devnet") {
		t.Errorf("output should contain devnet name, got: %s", output)
	}
}
