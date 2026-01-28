// cmd/dvb/provision_test.go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatProvisionYAML(t *testing.T) {
	opts := &provisionOptions{
		name:        "test-devnet",
		namespace:   "production",
		network:     "stable",
		networkType: "mainnet",
		validators:  4,
		fullNodes:   1,
		mode:        "docker",
		chainID:     "mainnet-1",
		sdkVersion:  "v1.0.0",
	}

	var buf bytes.Buffer
	if err := formatProvisionYAML(&buf, opts); err != nil {
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
		"chainId: mainnet-1",
		"forkNetwork: mainnet",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("missing '%s' in output:\n%s", field, output)
		}
	}
}

func TestFormatProvisionYAML_OmitsEmptyFields(t *testing.T) {
	opts := &provisionOptions{
		name:       "minimal-devnet",
		network:    "stable",
		validators: 2,
		mode:       "docker",
	}

	var buf bytes.Buffer
	if err := formatProvisionYAML(&buf, opts); err != nil {
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
