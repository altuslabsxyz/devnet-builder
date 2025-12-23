package helpers

import (
	"errors"
	"testing"
)

// Mock types for testing
type mockMetadata struct {
	Name string
}

type mockDevnet struct {
	Nodes int
}

func TestDevnetLoader_LoadOrFail(t *testing.T) {
	tests := []struct {
		name           string
		loader         *DevnetLoader
		wantErr        bool
		wantErrContain string
		wantMetadata   bool
		wantDevnet     bool
	}{
		{
			name: "successful load",
			loader: &DevnetLoader{
				HomeDir: "/test/home",
				ExistsCheck: func(path string) bool {
					return true
				},
				MetadataLoader: func(path string) (interface{}, error) {
					return &mockMetadata{Name: "test"}, nil
				},
				NodesLoader: func(path string, meta interface{}) (interface{}, error) {
					return &mockDevnet{Nodes: 4}, nil
				},
			},
			wantMetadata: true,
			wantDevnet:   true,
		},
		{
			name: "devnet does not exist",
			loader: &DevnetLoader{
				HomeDir: "/nonexistent",
				ExistsCheck: func(path string) bool {
					return false
				},
			},
			wantErr:        true,
			wantErrContain: "no devnet found at",
		},
		{
			name: "metadata load error",
			loader: &DevnetLoader{
				HomeDir: "/test/home",
				ExistsCheck: func(path string) bool {
					return true
				},
				MetadataLoader: func(path string) (interface{}, error) {
					return nil, errors.New("metadata corrupted")
				},
			},
			wantErr:        true,
			wantErrContain: "failed to load devnet metadata",
		},
		{
			name: "nodes load error",
			loader: &DevnetLoader{
				HomeDir: "/test/home",
				ExistsCheck: func(path string) bool {
					return true
				},
				MetadataLoader: func(path string) (interface{}, error) {
					return &mockMetadata{Name: "test"}, nil
				},
				NodesLoader: func(path string, meta interface{}) (interface{}, error) {
					return nil, errors.New("node config invalid")
				},
			},
			wantErr:        true,
			wantErrContain: "failed to load devnet",
		},
		{
			name: "nil callbacks - no ops",
			loader: &DevnetLoader{
				HomeDir: "/test/home",
			},
			wantMetadata: false,
			wantDevnet:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.loader.LoadOrFail()

			if tt.wantErr {
				if err == nil {
					t.Error("LoadOrFail() error = nil, wantErr = true")
					return
				}
				if tt.wantErrContain != "" && !containsString(err.Error(), tt.wantErrContain) {
					t.Errorf("LoadOrFail() error = %v, wantErrContain %q", err, tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadOrFail() unexpected error = %v", err)
				return
			}

			if tt.wantMetadata && result.Metadata == nil {
				t.Error("LoadOrFail() Metadata = nil, want non-nil")
			}
			if tt.wantDevnet && result.Devnet == nil {
				t.Error("LoadOrFail() Devnet = nil, want non-nil")
			}
		})
	}
}

func TestDevnetLoader_LoadMetadataOrFail(t *testing.T) {
	tests := []struct {
		name           string
		loader         *DevnetLoader
		wantErr        bool
		wantErrContain string
		wantMetadata   bool
	}{
		{
			name: "successful load",
			loader: &DevnetLoader{
				HomeDir: "/test/home",
				ExistsCheck: func(path string) bool {
					return true
				},
				MetadataLoader: func(path string) (interface{}, error) {
					return &mockMetadata{Name: "test"}, nil
				},
			},
			wantMetadata: true,
		},
		{
			name: "devnet does not exist",
			loader: &DevnetLoader{
				HomeDir: "/nonexistent",
				ExistsCheck: func(path string) bool {
					return false
				},
			},
			wantErr:        true,
			wantErrContain: "no devnet found at",
		},
		{
			name: "metadata load error",
			loader: &DevnetLoader{
				HomeDir: "/test/home",
				ExistsCheck: func(path string) bool {
					return true
				},
				MetadataLoader: func(path string) (interface{}, error) {
					return nil, errors.New("metadata corrupted")
				},
			},
			wantErr:        true,
			wantErrContain: "failed to load devnet metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := tt.loader.LoadMetadataOrFail()

			if tt.wantErr {
				if err == nil {
					t.Error("LoadMetadataOrFail() error = nil, wantErr = true")
					return
				}
				if tt.wantErrContain != "" && !containsString(err.Error(), tt.wantErrContain) {
					t.Errorf("LoadMetadataOrFail() error = %v, wantErrContain %q", err, tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadMetadataOrFail() unexpected error = %v", err)
				return
			}

			if tt.wantMetadata && metadata == nil {
				t.Error("LoadMetadataOrFail() = nil, want non-nil")
			}
		})
	}
}

func TestDevnetLoadError(t *testing.T) {
	tests := []struct {
		name    string
		err     *DevnetLoadError
		wantMsg string
	}{
		{
			name: "exists stage",
			err: &DevnetLoadError{
				HomeDir: "/test/home",
				Stage:   "exists",
			},
			wantMsg: "no devnet found at /test/home",
		},
		{
			name: "metadata stage",
			err: &DevnetLoadError{
				HomeDir: "/test/home",
				Stage:   "metadata",
				Wrapped: errors.New("corrupted file"),
			},
			wantMsg: "failed to load devnet metadata: corrupted file",
		},
		{
			name: "nodes stage",
			err: &DevnetLoadError{
				HomeDir: "/test/home",
				Stage:   "nodes",
				Wrapped: errors.New("invalid config"),
			},
			wantMsg: "failed to load devnet: invalid config",
		},
		{
			name: "unknown stage",
			err: &DevnetLoadError{
				HomeDir: "/test/home",
				Stage:   "unknown",
				Wrapped: errors.New("something"),
			},
			wantMsg: "devnet error at /test/home: something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("DevnetLoadError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestDevnetLoadError_Unwrap(t *testing.T) {
	wrapped := errors.New("wrapped error")
	err := &DevnetLoadError{
		HomeDir: "/test",
		Stage:   "metadata",
		Wrapped: wrapped,
	}

	if err.Unwrap() != wrapped {
		t.Error("DevnetLoadError.Unwrap() should return wrapped error")
	}

	errNoWrap := &DevnetLoadError{
		HomeDir: "/test",
		Stage:   "exists",
	}

	if errNoWrap.Unwrap() != nil {
		t.Error("DevnetLoadError.Unwrap() should return nil when no wrapped error")
	}
}
