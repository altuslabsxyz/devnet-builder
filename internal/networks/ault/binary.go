package ault

import (
	"fmt"
	"strings"

	"github.com/stablelabs/stable-devnet/internal/network"
)

const (
	// GitHubOwner is the GitHub organization for Ault.
	GitHubOwner = "bharvest"

	// GitHubRepo is the GitHub repository for Ault.
	GitHubRepo = "ault"

	// DefaultRepo is the default Ault repository URL.
	DefaultRepo = "https://github.com/bharvest/ault.git"

	// EVMChainIDMainnet is the EVM chain ID for mainnet fork.
	EVMChainIDMainnet = 904

	// EVMChainIDTestnet is the EVM chain ID for testnet fork.
	EVMChainIDTestnet = 10904

	// EVMChainIDDevnet is the EVM chain ID for devnet.
	EVMChainIDDevnet = 900
)

// BinarySourceConfig returns the binary source configuration for Ault.
func BinarySourceConfig() network.BinarySource {
	return network.BinarySource{
		Type:          network.BinarySourceGitHub,
		Owner:         GitHubOwner,
		Repo:          GitHubRepo,
		MaxRetries:    3,
		AssetNameFunc: AssetNameFunc,
	}
}

// AssetNameFunc generates the download asset filename for an Ault release.
// This matches the naming convention used by Ault GitHub releases.
func AssetNameFunc(version, goos, goarch string) string {
	// Ault uses format: aultd_v1.0.0_linux_amd64.tar.gz
	v := version
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return fmt.Sprintf("aultd_%s_%s_%s.tar.gz", v, goos, goarch)
}
