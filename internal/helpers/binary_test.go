package helpers

import (
	"path/filepath"
	"testing"
)

func TestResolveBinaryPath(t *testing.T) {
	tests := []struct {
		name       string
		customPath string
		homeDir    string
		want       string
	}{
		{
			name:       "custom path set",
			customPath: "/custom/path/to/stabled",
			homeDir:    "/home/user/.devnet",
			want:       "/custom/path/to/stabled",
		},
		{
			name:       "custom path empty - use default",
			customPath: "",
			homeDir:    "/home/user/.devnet",
			want:       filepath.Join("/home/user/.devnet", "bin", "stabled"),
		},
		{
			name:       "custom path with spaces",
			customPath: "/path/with spaces/stabled",
			homeDir:    "/home/user/.devnet",
			want:       "/path/with spaces/stabled",
		},
		{
			name:       "relative custom path",
			customPath: "./relative/stabled",
			homeDir:    "/home/user/.devnet",
			want:       "./relative/stabled",
		},
		{
			name:       "empty home dir with no custom",
			customPath: "",
			homeDir:    "",
			want:       filepath.Join("", "bin", "stabled"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBinaryPath(tt.customPath, tt.homeDir)
			if got != tt.want {
				t.Errorf("ResolveBinaryPath(%q, %q) = %q, want %q",
					tt.customPath, tt.homeDir, got, tt.want)
			}
		})
	}
}

func TestResolveDockerImage(t *testing.T) {
	tests := []struct {
		name         string
		customImage  string
		defaultImage string
		want         string
	}{
		{
			name:         "custom image set",
			customImage:  "my-registry/stabled:v1.0.0",
			defaultImage: "stablelabs/stabled:latest",
			want:         "my-registry/stabled:v1.0.0",
		},
		{
			name:         "custom image empty - use default",
			customImage:  "",
			defaultImage: "stablelabs/stabled:latest",
			want:         "stablelabs/stabled:latest",
		},
		{
			name:         "both empty",
			customImage:  "",
			defaultImage: "",
			want:         "",
		},
		{
			name:         "custom image with tag",
			customImage:  "stablelabs/stabled:1.1.3-mainnet",
			defaultImage: "stablelabs/stabled:latest",
			want:         "stablelabs/stabled:1.1.3-mainnet",
		},
		{
			name:         "custom image with digest",
			customImage:  "stablelabs/stabled@sha256:abc123",
			defaultImage: "stablelabs/stabled:latest",
			want:         "stablelabs/stabled@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDockerImage(tt.customImage, tt.defaultImage)
			if got != tt.want {
				t.Errorf("ResolveDockerImage(%q, %q) = %q, want %q",
					tt.customImage, tt.defaultImage, got, tt.want)
			}
		})
	}
}
