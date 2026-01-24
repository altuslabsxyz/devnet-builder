// pkg/network/cosmos/version.go
package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// abciInfoResponse represents the ABCI info response from a Cosmos node.
type abciInfoResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Response struct {
			Version         string `json:"version"`
			AppVersion      string `json:"app_version"`
			LastBlockHeight string `json:"last_block_height"`
			Data            string `json:"data"`
		} `json:"response"`
	} `json:"result"`
}

// DetectSDKVersion queries the ABCI info endpoint to detect the SDK version.
// Returns an SDKVersion struct with the detected framework, version, and features.
func DetectSDKVersion(ctx context.Context, rpcEndpoint string) (*network.SDKVersion, error) {
	if rpcEndpoint == "" {
		return nil, fmt.Errorf("RPC endpoint is required")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Query ABCI info endpoint
	url := strings.TrimRight(rpcEndpoint, "/") + "/abci_info"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query ABCI info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ABCI info request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var abciInfo abciInfoResponse
	if err := json.Unmarshal(body, &abciInfo); err != nil {
		return nil, fmt.Errorf("failed to parse ABCI info response: %w", err)
	}

	version := abciInfo.Result.Response.Version
	if version == "" {
		return nil, fmt.Errorf("empty version in ABCI info response")
	}

	// Detect features based on version
	features := detectFeatures(version)

	return &network.SDKVersion{
		Framework: network.FrameworkCosmosSDK,
		Version:   version,
		Features:  features,
	}, nil
}

// parseSDKVersion parses a semantic version string into major, minor, patch components.
// Supports versions with or without 'v' prefix and pre-release suffixes.
func parseSDKVersion(version string) (major, minor, patch int, err error) {
	if version == "" {
		return 0, 0, 0, fmt.Errorf("empty version string")
	}

	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Remove pre-release suffix (everything after -)
	if idx := strings.Index(version, "-"); idx != -1 {
		version = version[:idx]
	}

	// Split by dots
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, 0, fmt.Errorf("invalid version format: %s", version)
	}

	// Parse major version
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %w", err)
	}

	// Parse minor version
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %w", err)
	}

	// Parse patch version (optional)
	if len(parts) >= 3 {
		// Handle patch versions that might have additional suffixes
		patchStr := parts[2]
		re := regexp.MustCompile(`^(\d+)`)
		matches := re.FindStringSubmatch(patchStr)
		if len(matches) > 1 {
			patch, err = strconv.Atoi(matches[1])
			if err != nil {
				patch = 0
			}
		}
	}

	return major, minor, patch, nil
}

// versionAtLeast returns true if the given version is at least the specified minimum.
func versionAtLeast(version string, minMajor, minMinor, minPatch int) bool {
	major, minor, patch, err := parseSDKVersion(version)
	if err != nil {
		return false
	}

	// Compare major version
	if major > minMajor {
		return true
	}
	if major < minMajor {
		return false
	}

	// Major versions are equal, compare minor
	if minor > minMinor {
		return true
	}
	if minor < minMinor {
		return false
	}

	// Minor versions are equal, compare patch
	return patch >= minPatch
}

// detectFeatures determines the SDK features available based on version.
// This provides a reasonable default set of features based on SDK version history.
func detectFeatures(version string) []string {
	var features []string

	// Authz module was added in SDK v0.43
	if versionAtLeast(version, 0, 43, 0) {
		features = append(features, network.FeatureAuthz)
	}

	// Feegrant module was added in SDK v0.43
	if versionAtLeast(version, 0, 43, 0) {
		features = append(features, network.FeatureFeegrant)
	}

	// Gov v1 was added in SDK v0.46
	if versionAtLeast(version, 0, 46, 0) {
		features = append(features, network.FeatureGovV1)
	}

	// Group module was added in SDK v0.46, but became more stable in v0.47
	if versionAtLeast(version, 0, 47, 0) {
		features = append(features, network.FeatureGroup)
	}

	return features
}
