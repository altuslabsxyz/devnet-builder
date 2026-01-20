package unit_test

import (
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// TestBuildConfigHashDeterminism verifies that Hash() produces the same output
// for identical BuildConfig instances (T014).
func TestBuildConfigHashDeterminism(t *testing.T) {
	tests := []struct {
		name   string
		config *network.BuildConfig
	}{
		{
			name:   "empty config",
			config: &network.BuildConfig{},
		},
		{
			name: "with tags only",
			config: &network.BuildConfig{
				Tags: []string{"netgo", "ledger"},
			},
		},
		{
			name: "with ldflags only",
			config: &network.BuildConfig{
				LDFlags: []string{"-X main.Version=1.0", "-w", "-s"},
			},
		},
		{
			name: "with env vars only",
			config: &network.BuildConfig{
				Env: map[string]string{
					"CGO_ENABLED": "0",
					"GOOS":        "linux",
				},
			},
		},
		{
			name: "full config",
			config: &network.BuildConfig{
				Tags:      []string{"netgo", "ledger"},
				LDFlags:   []string{"-X main.Version=1.0", "-w"},
				Env:       map[string]string{"CGO_ENABLED": "0"},
				ExtraArgs: []string{"--skip-validate"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Hash the same config multiple times
			hash1 := tt.config.Hash()
			hash2 := tt.config.Hash()
			hash3 := tt.config.Hash()

			// All hashes should be identical
			if hash1 != hash2 || hash1 != hash3 {
				t.Errorf("Hash() not deterministic: got %s, %s, %s", hash1, hash2, hash3)
			}

			// Hash should be non-empty (unless config is empty)
			if !tt.config.IsEmpty() && hash1 == "" {
				t.Error("Hash() returned empty string for non-empty config")
			}

			// Hash should be 16 hex characters (64 bits)
			if !tt.config.IsEmpty() && len(hash1) != 16 {
				t.Errorf("Hash() should return 16 hex characters, got %d: %s", len(hash1), hash1)
			}
		})
	}
}

// TestBuildConfigHashOrderIndependence verifies that Hash() produces the same
// output regardless of slice order (T015).
func TestBuildConfigHashOrderIndependence(t *testing.T) {
	tests := []struct {
		name        string
		config1     *network.BuildConfig
		config2     *network.BuildConfig
		shouldMatch bool
	}{
		{
			name: "tags in different order",
			config1: &network.BuildConfig{
				Tags: []string{"a", "b", "c"},
			},
			config2: &network.BuildConfig{
				Tags: []string{"c", "b", "a"},
			},
			shouldMatch: true,
		},
		{
			name: "ldflags in different order",
			config1: &network.BuildConfig{
				LDFlags: []string{"-X main.A=1", "-X main.B=2", "-w"},
			},
			config2: &network.BuildConfig{
				LDFlags: []string{"-w", "-X main.B=2", "-X main.A=1"},
			},
			shouldMatch: true,
		},
		{
			name: "extra args in different order",
			config1: &network.BuildConfig{
				ExtraArgs: []string{"--flag1", "--flag2", "--flag3"},
			},
			config2: &network.BuildConfig{
				ExtraArgs: []string{"--flag3", "--flag1", "--flag2"},
			},
			shouldMatch: true,
		},
		{
			name: "env vars (map iteration order doesn't matter)",
			config1: &network.BuildConfig{
				Env: map[string]string{
					"CGO_ENABLED": "0",
					"GOOS":        "linux",
					"GOARCH":      "amd64",
				},
			},
			config2: &network.BuildConfig{
				Env: map[string]string{
					"GOARCH":      "amd64",
					"CGO_ENABLED": "0",
					"GOOS":        "linux",
				},
			},
			shouldMatch: true,
		},
		{
			name: "different content should produce different hash",
			config1: &network.BuildConfig{
				Tags: []string{"netgo", "ledger"},
			},
			config2: &network.BuildConfig{
				Tags: []string{"netgo", "osusergo"},
			},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := tt.config1.Hash()
			hash2 := tt.config2.Hash()

			if tt.shouldMatch {
				if hash1 != hash2 {
					t.Errorf("Hash() should be order-independent:\nconfig1: %v (hash: %s)\nconfig2: %v (hash: %s)",
						tt.config1, hash1, tt.config2, hash2)
				}
			} else {
				if hash1 == hash2 {
					t.Errorf("Hash() should produce different hashes for different configs:\nconfig1: %v\nconfig2: %v\nboth got hash: %s",
						tt.config1, tt.config2, hash1)
				}
			}
		})
	}
}

// TestBuildConfigValidate verifies that Validate() catches invalid configurations (T016).
func TestBuildConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *network.BuildConfig
		wantError bool
		errorMsg  string
	}{
		{
			name:      "nil config is valid",
			config:    nil,
			wantError: false,
		},
		{
			name:      "empty config is valid",
			config:    &network.BuildConfig{},
			wantError: false,
		},
		{
			name: "valid config with all fields",
			config: &network.BuildConfig{
				Tags:      []string{"netgo", "ledger"},
				LDFlags:   []string{"-X main.Version=1.0", "-w", "-s"},
				Env:       map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
				ExtraArgs: []string{"--skip-validate"},
			},
			wantError: false,
		},
		{
			name: "invalid ldflag - missing hyphen prefix",
			config: &network.BuildConfig{
				LDFlags: []string{"X main.Version=1.0"},
			},
			wantError: true,
			errorMsg:  "must start with '-'",
		},
		{
			name: "invalid ldflag - dangerous pattern --exec",
			config: &network.BuildConfig{
				LDFlags: []string{"-X main.Version=1.0", "--exec /bin/sh"},
			},
			wantError: true,
			errorMsg:  "dangerous ldflag pattern",
		},
		{
			name: "invalid ldflag - path traversal",
			config: &network.BuildConfig{
				LDFlags: []string{"-X ../../../etc/passwd=hacked"},
			},
			wantError: true,
			errorMsg:  "dangerous ldflag pattern",
		},
		{
			name: "invalid env - not in whitelist",
			config: &network.BuildConfig{
				Env: map[string]string{"MALICIOUS_VAR": "evil"},
			},
			wantError: true,
			errorMsg:  "not allowed",
		},
		{
			name: "invalid env - shell injection attempt",
			config: &network.BuildConfig{
				Env: map[string]string{"CGO_ENABLED": "0; rm -rf /"},
			},
			wantError: true,
			errorMsg:  "suspicious characters",
		},
		{
			name: "invalid tag - empty",
			config: &network.BuildConfig{
				Tags: []string{"netgo", "", "ledger"},
			},
			wantError: true,
			errorMsg:  "empty build tag",
		},
		{
			name: "invalid tag - duplicate",
			config: &network.BuildConfig{
				Tags: []string{"netgo", "ledger", "netgo"},
			},
			wantError: true,
			errorMsg:  "duplicate build tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() should return error for invalid config: %v", tt.config)
				} else if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error message should contain %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() should not return error for valid config: %v, got error: %v", tt.config, err)
				}
			}
		})
	}
}

// containsSubstring checks if s contains substr (case-insensitive).
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 &&
		(s[:len(substr)] == substr || containsSubstring(s[1:], substr)))
}
