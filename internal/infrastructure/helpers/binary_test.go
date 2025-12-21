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
		binaryName string
		want       string
	}{
		{
			name:       "custom path set",
			customPath: "/custom/path/to/binary",
			homeDir:    "/home/user/.devnet",
			binaryName: "stabled",
			want:       "/custom/path/to/binary",
		},
		{
			name:       "custom path empty - use default with stabled",
			customPath: "",
			homeDir:    "/home/user/.devnet",
			binaryName: "stabled",
			want:       filepath.Join("/home/user/.devnet", "bin", "stabled"),
		},
		{
			name:       "custom path empty - use default with aultd",
			customPath: "",
			homeDir:    "/home/user/.devnet",
			binaryName: "aultd",
			want:       filepath.Join("/home/user/.devnet", "bin", "aultd"),
		},
		{
			name:       "custom path with spaces",
			customPath: "/path/with spaces/binary",
			homeDir:    "/home/user/.devnet",
			binaryName: "stabled",
			want:       "/path/with spaces/binary",
		},
		{
			name:       "relative custom path",
			customPath: "./relative/binary",
			homeDir:    "/home/user/.devnet",
			binaryName: "stabled",
			want:       "./relative/binary",
		},
		{
			name:       "empty home dir with no custom",
			customPath: "",
			homeDir:    "",
			binaryName: "stabled",
			want:       filepath.Join("", "bin", "stabled"),
		},
		{
			name:       "empty binary name uses fallback",
			customPath: "",
			homeDir:    "/home/user/.devnet",
			binaryName: "",
			want:       filepath.Join("/home/user/.devnet", "bin", "binary"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBinaryPath(tt.customPath, tt.homeDir, tt.binaryName)
			if got != tt.want {
				t.Errorf("ResolveBinaryPath(%q, %q, %q) = %q, want %q",
					tt.customPath, tt.homeDir, tt.binaryName, got, tt.want)
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
