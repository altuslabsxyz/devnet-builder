// cmd/dvb/provision_test.go
package main

import (
	"bytes"
	"strings"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

func TestFormatProvisionYAML(t *testing.T) {
	spec := &v1.DevnetSpec{
		Plugin:      "stable",
		NetworkType: "mainnet",
		Validators:  4,
		FullNodes:   1,
		Mode:        "docker",
		SdkVersion:  "v1.0.0",
		ForkNetwork: "mainnet",
		ChainId:     "mainnet-1",
	}

	var buf bytes.Buffer
	if err := formatProvisionYAML(&buf, "production", "test-devnet", spec); err != nil {
		t.Fatalf("formatProvisionYAML failed: %v", err)
	}

	output := buf.String()

	expectedFields := []string{
		"apiVersion: devnet.lagos/v1",
		"kind: Devnet",
		"name: test-devnet",
		"namespace: production",
		"network: stable",
		"networkType: mainnet",
		"networkVersion: v1.0.0",
		"validators: 4",
		"fullNodes: 1",
		"mode: docker",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("missing '%s' in output:\n%s", field, output)
		}
	}
}

func TestFormatProvisionYAML_OmitsEmptyFields(t *testing.T) {
	spec := &v1.DevnetSpec{
		Plugin:     "stable",
		Validators: 2,
		Mode:       "docker",
	}

	var buf bytes.Buffer
	if err := formatProvisionYAML(&buf, "", "minimal-devnet", spec); err != nil {
		t.Fatalf("formatProvisionYAML failed: %v", err)
	}

	output := buf.String()

	unexpectedFields := []string{
		"networkType:",
		"networkVersion:",
		"fullNodes:",
		"chainId:",
		"namespace:",
		"forkNetwork:",
	}

	for _, field := range unexpectedFields {
		if strings.Contains(output, field) {
			t.Errorf("should omit '%s' when empty, got:\n%s", field, output)
		}
	}
}

func TestDetectProvisionMode(t *testing.T) {
	tests := []struct {
		name     string
		opts     *provisionOptions
		expected ProvisionMode
	}{
		{
			name:     "file mode when file flag is set",
			opts:     &provisionOptions{file: "devnet.yaml"},
			expected: FileMode,
		},
		{
			name:     "flag mode when name is set",
			opts:     &provisionOptions{name: "my-devnet"},
			expected: FlagMode,
		},
		{
			name:     "interactive mode when no flags",
			opts:     &provisionOptions{},
			expected: InteractiveMode,
		},
		{
			name:     "file mode takes priority over name",
			opts:     &provisionOptions{file: "devnet.yaml", name: "my-devnet"},
			expected: FileMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectProvisionMode(tt.opts)
			if got != tt.expected {
				t.Errorf("detectProvisionMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}
