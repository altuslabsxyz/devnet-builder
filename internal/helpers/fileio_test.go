package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

type testConfig struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestLoadJSON(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		want      *testConfig
		wantErr   string
		wantErrIs bool
	}{
		{
			name: "valid json",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "config.json")
				if err := os.WriteFile(path, []byte(`{"name":"test","value":42}`), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: &testConfig{Name: "test", Value: 42},
		},
		{
			name: "file not found",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent.json")
			},
			wantErr: "file not found",
		},
		{
			name: "invalid json",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "invalid.json")
				if err := os.WriteFile(path, []byte(`{invalid json}`), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: "failed to parse JSON",
		},
		{
			name: "empty file",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "empty.json")
				if err := os.WriteFile(path, []byte(``), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			wantErr: "failed to parse JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			got, err := LoadJSON[testConfig](path)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("LoadJSON() error = nil, wantErr containing %q", tt.wantErr)
					return
				}
				if !containsString(err.Error(), tt.wantErr) {
					t.Errorf("LoadJSON() error = %v, wantErr containing %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadJSON() unexpected error = %v", err)
				return
			}

			if got.Name != tt.want.Name || got.Value != tt.want.Value {
				t.Errorf("LoadJSON() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSaveJSON(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		data    any
		perm    os.FileMode
		wantErr bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "output.json")
			},
			data: &testConfig{Name: "test", Value: 123},
			perm: 0644,
		},
		{
			name: "create parent directory",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "subdir", "nested", "output.json")
			},
			data: &testConfig{Name: "nested", Value: 456},
			perm: 0644,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			err := SaveJSON(path, tt.data, tt.perm)

			if tt.wantErr {
				if err == nil {
					t.Error("SaveJSON() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("SaveJSON() unexpected error = %v", err)
				return
			}

			// Verify file was created and content is correct
			loaded, err := LoadJSON[testConfig](path)
			if err != nil {
				t.Errorf("Failed to load saved file: %v", err)
				return
			}

			original := tt.data.(*testConfig)
			if loaded.Name != original.Name || loaded.Value != original.Value {
				t.Errorf("SaveJSON() wrote %+v, want %+v", loaded, original)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "file exists",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "exists.txt")
				if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: true,
		},
		{
			name: "file does not exist",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent.txt")
			},
			want: false,
		},
		{
			name: "directory not file",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			if got := FileExists(path); got != tt.want {
				t.Errorf("FileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "directory exists",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: true,
		},
		{
			name: "directory does not exist",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			want: false,
		},
		{
			name: "file not directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "file.txt")
				if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			if got := DirExists(path); got != tt.want {
				t.Errorf("DirExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		perm    os.FileMode
		wantErr bool
	}{
		{
			name: "create new directory",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "newdir")
			},
			perm: 0755,
		},
		{
			name: "create nested directories",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "a", "b", "c")
			},
			perm: 0755,
		},
		{
			name: "directory already exists",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			perm: 0755,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			err := EnsureDir(path, tt.perm)

			if tt.wantErr {
				if err == nil {
					t.Error("EnsureDir() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("EnsureDir() unexpected error = %v", err)
				return
			}

			if !DirExists(path) {
				t.Errorf("EnsureDir() directory was not created at %s", path)
			}
		})
	}
}

func TestJSONLoadError(t *testing.T) {
	err := &JSONLoadError{
		Path:   "/path/to/file.json",
		Reason: "file not found",
	}

	if got := err.Error(); got != "file not found: /path/to/file.json" {
		t.Errorf("JSONLoadError.Error() = %q, want %q", got, "file not found: /path/to/file.json")
	}

	if err.Unwrap() != nil {
		t.Error("JSONLoadError.Unwrap() should be nil when no wrapped error")
	}

	wrapped := &JSONLoadError{
		Path:    "/path/to/file.json",
		Reason:  "failed to read",
		Wrapped: os.ErrPermission,
	}

	if wrapped.Unwrap() != os.ErrPermission {
		t.Error("JSONLoadError.Unwrap() should return wrapped error")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
