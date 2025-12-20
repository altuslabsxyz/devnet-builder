package main

import (
	"fmt"
	"strings"
)

// NodeResult represents a node in the JSON output.
type NodeResult struct {
	Index  int    `json:"index"`
	RPC    string `json:"rpc"`
	EVMRPC string `json:"evm_rpc"`
	Status string `json:"status"`
}

// FailedNodeJSON represents a failed node in JSON output.
type FailedNodeJSON struct {
	Index   int      `json:"index"`
	Error   string   `json:"error"`
	LogPath string   `json:"log_path"`
	LogTail []string `json:"log_tail,omitempty"`
}

// DockerImageSelectionResult holds the result of docker image selection.
type DockerImageSelectionResult struct {
	ImageTag  string // Selected image tag or full custom URL
	IsCustom  bool   // True if user entered a custom image URL
	FromCache bool   // True if versions were loaded from cache
}

// DefaultGHCRImage is the default GHCR image for stable.
const DefaultGHCRImage = "ghcr.io/stablelabs/stable"

// DefaultDockerPackage is the default container package name for docker images.
const DefaultDockerPackage = "stable"

// normalizeImageURL converts a tag-only input to a full GHCR URL.
// If the input already contains a registry (contains "/" or ":"), it returns as-is.
// Otherwise, it constructs: ghcr.io/stablelabs/stable:{tag}
func normalizeImageURL(image string) string {
	if image == "" {
		return ""
	}
	// If it looks like a full URL (contains "/" indicating a registry path), return as-is
	if strings.Contains(image, "/") {
		return image
	}
	// Otherwise, treat it as a tag and construct GHCR URL
	return fmt.Sprintf("%s:%s", DefaultGHCRImage, image)
}

// getErrorCode returns an error code string based on the error message.
func getErrorCode(err error) string {
	errStr := err.Error()
	switch {
	case contains(errStr, "prerequisite"):
		return "PREREQUISITE_MISSING"
	case contains(errStr, "already exists"):
		return "DEVNET_ALREADY_RUNNING"
	case contains(errStr, "snapshot"):
		return "SNAPSHOT_DOWNLOAD_FAILED"
	case contains(errStr, "start"):
		return "NODE_START_FAILED"
	case contains(errStr, "port"):
		return "PORT_CONFLICT"
	default:
		return "GENERAL_ERROR"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
