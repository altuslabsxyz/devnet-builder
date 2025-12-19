package stable

import (
	"fmt"
	"strings"

	"github.com/stablelabs/stable-devnet/internal/network"
)

const (
	// GitHubOwner is the GitHub organization for Stable.
	GitHubOwner = "stablelabs"

	// GitHubRepo is the GitHub repository for Stable.
	GitHubRepo = "stable"

	// DefaultRepo is the default stable repository URL.
	DefaultRepo = "https://github.com/stablelabs/stable.git"

	// EVMChainIDMainnet is the EVM chain ID for mainnet fork.
	EVMChainIDMainnet = 988

	// EVMChainIDTestnet is the EVM chain ID for testnet fork.
	EVMChainIDTestnet = 2201

	// EVMChainIDDevnet is the EVM chain ID for devnet.
	EVMChainIDDevnet = 2200
)

// BinarySourceConfig returns the binary source configuration for Stable.
func BinarySourceConfig() network.BinarySource {
	return network.BinarySource{
		Type:          network.BinarySourceGitHub,
		Owner:         GitHubOwner,
		Repo:          GitHubRepo,
		MaxRetries:    3,
		AssetNameFunc: AssetNameFunc,
	}
}

// AssetNameFunc generates the download asset filename for a Stable release.
// This matches the naming convention used by Stable GitHub releases.
func AssetNameFunc(version, goos, goarch string) string {
	// Stable uses format: stabled_v1.0.0_linux_amd64.tar.gz
	v := version
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return fmt.Sprintf("stabled_%s_%s_%s.tar.gz", v, goos, goarch)
}
