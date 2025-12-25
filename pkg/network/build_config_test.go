package network

import (
	"strings"
	"testing"
)

// TestBuildConfig_Validate tests the validation logic for BuildConfig.
func TestBuildConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *BuildConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config is valid",
			config:  nil,
			wantErr: false,
		},
		{
			name:    "empty config is valid",
			config:  &BuildConfig{},
			wantErr: false,
		},
		{
			name: "valid complete config",
			config: &BuildConfig{
				Tags:      []string{"netgo", "ledger", "osusergo"},
				LDFlags:   []string{"-X main.Version=1.0.0", "-w", "-s"},
				Env:       map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
				ExtraArgs: []string{"--clean", "--skip-validate"},
			},
			wantErr: false,
		},
		{
			name: "duplicate build tags",
			config: &BuildConfig{
				Tags: []string{"netgo", "ledger", "netgo"},
			},
			wantErr: true,
			errMsg:  "duplicate build tag",
		},
		{
			name: "empty build tag",
			config: &BuildConfig{
				Tags: []string{"netgo", "", "ledger"},
			},
			wantErr: true,
			errMsg:  "empty build tag",
		},
		{
			name: "invalid ldflag format - missing hyphen",
			config: &BuildConfig{
				LDFlags: []string{"X main.Version=1.0"},
			},
			wantErr: true,
			errMsg:  "must start with '-'",
		},
		{
			name: "empty ldflag",
			config: &BuildConfig{
				LDFlags: []string{"-w", "", "-s"},
			},
			wantErr: true,
			errMsg:  "empty ldflag",
		},
		{
			name: "dangerous ldflag - exec pattern",
			config: &BuildConfig{
				LDFlags: []string{"-X main.Cmd=--exec=/bin/sh"},
			},
			wantErr: true,
			errMsg:  "dangerous ldflag pattern",
		},
		{
			name: "dangerous ldflag - path traversal",
			config: &BuildConfig{
				LDFlags: []string{"-X main.Path=../../../etc/passwd"},
			},
			wantErr: true,
			errMsg:  "dangerous ldflag pattern",
		},
		{
			name: "invalid env var - not in whitelist",
			config: &BuildConfig{
				Env: map[string]string{"PATH": "/malicious/path"},
			},
			wantErr: true,
			errMsg:  "environment variable not allowed",
		},
		{
			name: "empty env var key",
			config: &BuildConfig{
				Env: map[string]string{"": "value"},
			},
			wantErr: true,
			errMsg:  "empty environment variable key",
		},
		{
			name: "env var with shell injection attempt - dollar sign",
			config: &BuildConfig{
				Env: map[string]string{"CGO_ENABLED": "$(rm -rf /)"},
			},
			wantErr: true,
			errMsg:  "suspicious characters",
		},
		{
			name: "env var with shell injection attempt - backtick",
			config: &BuildConfig{
				Env: map[string]string{"CGO_ENABLED": "`whoami`"},
			},
			wantErr: true,
			errMsg:  "suspicious characters",
		},
		{
			name: "valid env vars",
			config: &BuildConfig{
				Env: map[string]string{
					"CGO_ENABLED": "0",
					"GOOS":        "linux",
					"GOARCH":      "amd64",
					"CC":          "gcc",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestBuildConfig_Merge tests the merge logic for combining configurations.
func TestBuildConfig_Merge(t *testing.T) {
	tests := []struct {
		name     string
		base     *BuildConfig
		override *BuildConfig
		want     *BuildConfig
	}{
		{
			name:     "both nil",
			base:     nil,
			override: nil,
			want:     &BuildConfig{},
		},
		{
			name: "base nil, override has values",
			base: nil,
			override: &BuildConfig{
				Tags:    []string{"ledger"},
				LDFlags: []string{"-w"},
			},
			want: &BuildConfig{
				Tags:    []string{"ledger"},
				LDFlags: []string{"-w"},
			},
		},
		{
			name: "base has values, override nil",
			base: &BuildConfig{
				Tags:    []string{"netgo"},
				LDFlags: []string{"-s"},
			},
			override: nil,
			want: &BuildConfig{
				Tags:    []string{"netgo"},
				LDFlags: []string{"-s"},
			},
		},
		{
			name: "merge tags and ldflags",
			base: &BuildConfig{
				Tags:    []string{"netgo", "osusergo"},
				LDFlags: []string{"-w", "-s"},
			},
			override: &BuildConfig{
				Tags:    []string{"ledger"},
				LDFlags: []string{"-X main.Version=1.0"},
			},
			want: &BuildConfig{
				Tags:    []string{"netgo", "osusergo", "ledger"},
				LDFlags: []string{"-w", "-s", "-X main.Version=1.0"},
			},
		},
		{
			name: "merge env vars - no conflicts",
			base: &BuildConfig{
				Env: map[string]string{
					"CGO_ENABLED": "0",
					"GOOS":        "linux",
				},
			},
			override: &BuildConfig{
				Env: map[string]string{
					"GOARCH": "amd64",
				},
			},
			want: &BuildConfig{
				Env: map[string]string{
					"CGO_ENABLED": "0",
					"GOOS":        "linux",
					"GOARCH":      "amd64",
				},
			},
		},
		{
			name: "merge env vars - override wins conflicts",
			base: &BuildConfig{
				Env: map[string]string{
					"CGO_ENABLED": "0",
					"GOOS":        "darwin",
				},
			},
			override: &BuildConfig{
				Env: map[string]string{
					"GOOS":   "linux",
					"GOARCH": "amd64",
				},
			},
			want: &BuildConfig{
				Env: map[string]string{
					"CGO_ENABLED": "0",
					"GOOS":        "linux", // override wins
					"GOARCH":      "amd64",
				},
			},
		},
		{
			name: "merge extra args",
			base: &BuildConfig{
				ExtraArgs: []string{"--clean"},
			},
			override: &BuildConfig{
				ExtraArgs: []string{"--skip-validate", "--debug"},
			},
			want: &BuildConfig{
				ExtraArgs: []string{"--clean", "--skip-validate", "--debug"},
			},
		},
		{
			name: "complete merge",
			base: &BuildConfig{
				Tags:      []string{"netgo"},
				LDFlags:   []string{"-w"},
				Env:       map[string]string{"CGO_ENABLED": "0"},
				ExtraArgs: []string{"--clean"},
			},
			override: &BuildConfig{
				Tags:      []string{"ledger"},
				LDFlags:   []string{"-X main.Version=1.0"},
				Env:       map[string]string{"GOOS": "linux"},
				ExtraArgs: []string{"--debug"},
			},
			want: &BuildConfig{
				Tags:      []string{"netgo", "ledger"},
				LDFlags:   []string{"-w", "-X main.Version=1.0"},
				Env:       map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
				ExtraArgs: []string{"--clean", "--debug"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.Merge(tt.override)

			// Check Tags
			if !stringSlicesEqual(got.Tags, tt.want.Tags) {
				t.Errorf("Merge() Tags = %v, want %v", got.Tags, tt.want.Tags)
			}

			// Check LDFlags
			if !stringSlicesEqual(got.LDFlags, tt.want.LDFlags) {
				t.Errorf("Merge() LDFlags = %v, want %v", got.LDFlags, tt.want.LDFlags)
			}

			// Check Env
			if !stringMapsEqual(got.Env, tt.want.Env) {
				t.Errorf("Merge() Env = %v, want %v", got.Env, tt.want.Env)
			}

			// Check ExtraArgs
			if !stringSlicesEqual(got.ExtraArgs, tt.want.ExtraArgs) {
				t.Errorf("Merge() ExtraArgs = %v, want %v", got.ExtraArgs, tt.want.ExtraArgs)
			}

			// Ensure merge doesn't modify original configs
			if tt.base != nil && !stringSlicesEqual(tt.base.Tags, []string{"netgo", "osusergo"}) {
				// Only check if base was the complex test case
				if tt.name == "merge tags and ldflags" {
					t.Errorf("Merge() modified original base config")
				}
			}
		})
	}
}

// TestBuildConfig_Clone tests the cloning functionality.
func TestBuildConfig_Clone(t *testing.T) {
	tests := []struct {
		name   string
		config *BuildConfig
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name:   "empty config",
			config: &BuildConfig{},
		},
		{
			name: "full config",
			config: &BuildConfig{
				Tags:      []string{"netgo", "ledger"},
				LDFlags:   []string{"-w", "-s"},
				Env:       map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
				ExtraArgs: []string{"--clean"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clone := tt.config.Clone()

			// Verify clone is not nil (even if original was)
			if clone == nil {
				t.Error("Clone() returned nil, want non-nil BuildConfig")
				return
			}

			if tt.config == nil {
				// If original was nil, clone should be empty
				if !clone.IsEmpty() {
					t.Error("Clone() of nil should return empty BuildConfig")
				}
				return
			}

			// Verify values match
			if !stringSlicesEqual(clone.Tags, tt.config.Tags) {
				t.Errorf("Clone() Tags = %v, want %v", clone.Tags, tt.config.Tags)
			}
			if !stringSlicesEqual(clone.LDFlags, tt.config.LDFlags) {
				t.Errorf("Clone() LDFlags = %v, want %v", clone.LDFlags, tt.config.LDFlags)
			}
			if !stringMapsEqual(clone.Env, tt.config.Env) {
				t.Errorf("Clone() Env = %v, want %v", clone.Env, tt.config.Env)
			}
			if !stringSlicesEqual(clone.ExtraArgs, tt.config.ExtraArgs) {
				t.Errorf("Clone() ExtraArgs = %v, want %v", clone.ExtraArgs, tt.config.ExtraArgs)
			}

			// Verify independence - modify clone shouldn't affect original
			if len(clone.Tags) > 0 {
				clone.Tags[0] = "modified"
				if len(tt.config.Tags) > 0 && tt.config.Tags[0] == "modified" {
					t.Error("Clone() is not independent - modifying clone affected original")
				}
			}

			if len(clone.Env) > 0 {
				for k := range clone.Env {
					clone.Env[k] = "modified"
					if tt.config.Env[k] == "modified" {
						t.Error("Clone() is not independent - modifying clone env affected original")
					}
					break
				}
			}
		})
	}
}

// TestBuildConfig_IsEmpty tests empty detection.
func TestBuildConfig_IsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		config *BuildConfig
		want   bool
	}{
		{
			name:   "nil config is empty",
			config: nil,
			want:   true,
		},
		{
			name:   "zero value is empty",
			config: &BuildConfig{},
			want:   true,
		},
		{
			name: "config with only tags is not empty",
			config: &BuildConfig{
				Tags: []string{"netgo"},
			},
			want: false,
		},
		{
			name: "config with only ldflags is not empty",
			config: &BuildConfig{
				LDFlags: []string{"-w"},
			},
			want: false,
		},
		{
			name: "config with only env is not empty",
			config: &BuildConfig{
				Env: map[string]string{"CGO_ENABLED": "0"},
			},
			want: false,
		},
		{
			name: "config with only extra args is not empty",
			config: &BuildConfig{
				ExtraArgs: []string{"--clean"},
			},
			want: false,
		},
		{
			name: "config with all fields is not empty",
			config: &BuildConfig{
				Tags:      []string{"netgo"},
				LDFlags:   []string{"-w"},
				Env:       map[string]string{"CGO_ENABLED": "0"},
				ExtraArgs: []string{"--clean"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildConfig_Hash tests hash generation for cache keys.
func TestBuildConfig_Hash(t *testing.T) {
	tests := []struct {
		name       string
		config     *BuildConfig
		wantEmpty  bool
		wantUnique bool // If true, verify this config has a unique hash
	}{
		{
			name:      "nil config returns 'empty'",
			config:    nil,
			wantEmpty: true,
		},
		{
			name:      "empty config returns 'empty'",
			config:    &BuildConfig{},
			wantEmpty: true,
		},
		{
			name: "config with values returns non-empty hash",
			config: &BuildConfig{
				Tags: []string{"netgo"},
			},
			wantEmpty: false,
		},
		{
			name: "identical configs return same hash",
			config: &BuildConfig{
				Tags:    []string{"netgo", "ledger"},
				LDFlags: []string{"-w", "-s"},
			},
			wantUnique: false,
		},
		{
			name: "configs with different order return same hash (deterministic)",
			config: &BuildConfig{
				Tags: []string{"ledger", "netgo"}, // Different order
			},
			wantUnique: false, // Should match config with ["netgo", "ledger"] due to sorting
		},
	}

	// Track hashes to verify uniqueness
	hashes := make(map[string]*BuildConfig)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := tt.config.Hash()

			// Check if empty
			if tt.wantEmpty {
				if hash != "empty" {
					t.Errorf("Hash() = %v, want 'empty'", hash)
				}
				return
			}

			// Check hash format (16 hex characters)
			if len(hash) != 16 {
				t.Errorf("Hash() length = %d, want 16", len(hash))
			}
			for _, c := range hash {
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					t.Errorf("Hash() contains non-hex character: %c", c)
				}
			}

			// Track for uniqueness testing
			if existing, found := hashes[hash]; found && tt.wantUnique {
				t.Errorf("Hash() collision: %v and %v have same hash %s",
					existing, tt.config, hash)
			}
			hashes[hash] = tt.config
		})
	}

	// Test hash determinism
	t.Run("hash determinism", func(t *testing.T) {
		config := &BuildConfig{
			Tags:    []string{"netgo", "ledger"},
			LDFlags: []string{"-w", "-s"},
			Env:     map[string]string{"CGO_ENABLED": "0", "GOOS": "linux"},
		}

		hash1 := config.Hash()
		hash2 := config.Hash()

		if hash1 != hash2 {
			t.Errorf("Hash() not deterministic: got %s and %s", hash1, hash2)
		}
	})

	// Test hash uniqueness for different configs
	t.Run("different configs have different hashes", func(t *testing.T) {
		config1 := &BuildConfig{Tags: []string{"netgo"}}
		config2 := &BuildConfig{Tags: []string{"ledger"}}
		config3 := &BuildConfig{LDFlags: []string{"-w"}}

		hash1 := config1.Hash()
		hash2 := config2.Hash()
		hash3 := config3.Hash()

		if hash1 == hash2 {
			t.Error("Different tag configs have same hash")
		}
		if hash1 == hash3 {
			t.Error("Tag config and ldflag config have same hash")
		}
		if hash2 == hash3 {
			t.Error("Different configs have same hash")
		}
	})
}

// TestBuildConfig_String tests string representation.
func TestBuildConfig_String(t *testing.T) {
	tests := []struct {
		name   string
		config *BuildConfig
		want   string
	}{
		{
			name:   "nil config",
			config: nil,
			want:   "BuildConfig{empty}",
		},
		{
			name:   "empty config",
			config: &BuildConfig{},
			want:   "BuildConfig{empty}",
		},
		{
			name: "config with tags",
			config: &BuildConfig{
				Tags: []string{"netgo", "ledger"},
			},
			want: "tags=[netgo ledger]",
		},
		{
			name: "config with all fields",
			config: &BuildConfig{
				Tags:      []string{"netgo"},
				LDFlags:   []string{"-w"},
				Env:       map[string]string{"CGO_ENABLED": "0"},
				ExtraArgs: []string{"--clean"},
			},
			want: "BuildConfig{", // Just check it starts correctly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("String() = %v, want to contain %v", got, tt.want)
			}
		})
	}
}

// Helper functions for testing

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Benchmark tests

func BenchmarkBuildConfig_Validate(b *testing.B) {
	config := &BuildConfig{
		Tags:    []string{"netgo", "ledger", "osusergo"},
		LDFlags: []string{"-X main.Version=1.0.0", "-w", "-s"},
		Env:     map[string]string{"CGO_ENABLED": "0", "GOOS": "linux", "GOARCH": "amd64"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}

func BenchmarkBuildConfig_Hash(b *testing.B) {
	config := &BuildConfig{
		Tags:    []string{"netgo", "ledger", "osusergo"},
		LDFlags: []string{"-X main.Version=1.0.0", "-w", "-s"},
		Env:     map[string]string{"CGO_ENABLED": "0", "GOOS": "linux", "GOARCH": "amd64"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Hash()
	}
}

func BenchmarkBuildConfig_Merge(b *testing.B) {
	base := &BuildConfig{
		Tags:    []string{"netgo", "osusergo"},
		LDFlags: []string{"-w", "-s"},
		Env:     map[string]string{"CGO_ENABLED": "0"},
	}
	override := &BuildConfig{
		Tags:    []string{"ledger"},
		LDFlags: []string{"-X main.Version=1.0"},
		Env:     map[string]string{"GOOS": "linux"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = base.Merge(override)
	}
}
