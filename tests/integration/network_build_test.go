package integration_test

import (
	"testing"
	"time"

	"github.com/b-harvest/devnet-builder/pkg/network"
)

// TestMultiNetworkBuild verifies that mainnet and testnet builds produce
// different cache keys and hashes (T017).
func TestMultiNetworkBuild(t *testing.T) {
	commitHash := "abc123def456789012345678901234567890abcd"

	// Create mainnet build config (EVM Chain ID 988)
	mainnetConfig := &network.BuildConfig{
		LDFlags: []string{"-X github.com/stablelabs/stable/app.EVMChainID=988"},
	}

	// Create testnet build config (EVM Chain ID 2201)
	testnetConfig := &network.BuildConfig{
		LDFlags: []string{"-X github.com/stablelabs/stable/app.EVMChainID=2201"},
	}

	// Verify configs produce different hashes
	mainnetHash := mainnetConfig.Hash()
	testnetHash := testnetConfig.Hash()
	if mainnetHash == testnetHash {
		t.Errorf("Mainnet and testnet configs should produce different hashes, both got: %s", mainnetHash)
	}

	t.Logf("Mainnet config hash: %s", mainnetHash)
	t.Logf("Testnet config hash: %s", testnetHash)

	// Verify cache keys would be different
	// Cache key format from spec: {networkType}/{commitHash}-{configHash}
	mainnetCacheKey := "mainnet/" + commitHash + "-" + mainnetHash[:8]
	testnetCacheKey := "testnet/" + commitHash + "-" + testnetHash[:8]

	if mainnetCacheKey == testnetCacheKey {
		t.Error("Mainnet and testnet should have different cache keys")
	}

	// Verify cache key format includes network type
	if mainnetCacheKey[:8] != "mainnet/" {
		t.Errorf("Mainnet cache key should start with 'mainnet/', got: %s", mainnetCacheKey)
	}
	if testnetCacheKey[:8] != "testnet/" {
		t.Errorf("Testnet cache key should start with 'testnet/', got: %s", testnetCacheKey)
	}

	t.Logf("Mainnet cache key: %s", mainnetCacheKey)
	t.Logf("Testnet cache key: %s", testnetCacheKey)
}

// TestCacheHitPerformance verifies that hash computation is fast enough for
// cache key generation - target <1ms (T019).
func TestCacheKeyComputationPerformance(t *testing.T) {
	// Create a realistic BuildConfig
	buildConfig := &network.BuildConfig{
		Tags: []string{"netgo", "ledger", "osusergo", "no_dynamic_precompiles"},
		LDFlags: []string{
			"-X github.com/stablelabs/stable/app.EVMChainID=2201",
			"-X github.com/stablelabs/stable/app.Version=v1.0.0",
			"-X github.com/stablelabs/stable/app.Commit=abc123",
			"-w",
			"-s",
		},
		Env: map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        "linux",
			"GOARCH":      "amd64",
		},
		ExtraArgs: []string{"--skip-validate", "--clean"},
	}

	// Measure hash computation performance (1000 iterations for accuracy)
	iterations := 1000
	start := time.Now()
	var lastHash string
	for i := 0; i < iterations; i++ {
		lastHash = buildConfig.Hash()
		if lastHash == "" {
			t.Fatal("Hash computation returned empty string")
		}
	}
	elapsed := time.Since(start)

	// Calculate average computation time
	avgComputation := elapsed / time.Duration(iterations)

	// Verify average computation is under 1ms (performance goal from plan.md)
	maxComputationTime := 1 * time.Millisecond
	if avgComputation > maxComputationTime {
		t.Errorf("Average hash computation time %v exceeds maximum %v", avgComputation, maxComputationTime)
	}

	t.Logf("Average hash computation time: %v (max allowed: %v)", avgComputation, maxComputationTime)
	t.Logf("Hash result: %s", lastHash)

	// Also verify the hash is deterministic across iterations
	hash1 := buildConfig.Hash()
	hash2 := buildConfig.Hash()
	if hash1 != hash2 {
		t.Error("Hash computation is not deterministic")
	}
}

// TestSameCommitDifferentNetwork verifies that the same commit with different
// network types produces different cache keys (T017 - acceptance criteria 4).
func TestSameCommitDifferentNetwork(t *testing.T) {
	commitHash := "abc123def456789012345678901234567890abcd"

	// Create configs for testnet and mainnet
	testnetConfig := &network.BuildConfig{
		LDFlags: []string{"-X github.com/stablelabs/stable/app.EVMChainID=2201"},
	}
	mainnetConfig := &network.BuildConfig{
		LDFlags: []string{"-X github.com/stablelabs/stable/app.EVMChainID=988"},
	}

	// Compute cache keys
	testnetKey := "testnet/" + commitHash + "-" + testnetConfig.Hash()[:8]
	mainnetKey := "mainnet/" + commitHash + "-" + mainnetConfig.Hash()[:8]

	// Verify they're different (cache miss expected when switching networks)
	if testnetKey == mainnetKey {
		t.Error("Testnet and mainnet cache keys should be different for same commit")
	}

	t.Logf("Testnet key: %s", testnetKey)
	t.Logf("Mainnet key: %s", mainnetKey)

	// Verify both use the same commit hash
	if testnetKey[8:8+40] != commitHash {
		t.Error("Testnet key doesn't contain correct commit hash")
	}
	if mainnetKey[8:8+40] != commitHash {
		t.Error("Mainnet key doesn't contain correct commit hash")
	}
}

// TestNetworkTypeIsolation verifies that network type is properly isolated
// in cache keys (T017).
func TestNetworkTypeIsolation(t *testing.T) {
	commitHash := "1234567890123456789012345678901234567890"

	// Same build config for all networks
	buildConfig := &network.BuildConfig{
		Tags: []string{"netgo"},
	}
	configHash := buildConfig.Hash()[:8]

	// Generate cache keys for different network types
	networks := []string{"mainnet", "testnet", "devnet", "localnet", "custom"}
	cacheKeys := make(map[string]string)

	for _, networkType := range networks {
		cacheKey := networkType + "/" + commitHash + "-" + configHash
		cacheKeys[networkType] = cacheKey

		// Verify network type is in the key
		if cacheKey[:len(networkType)+1] != networkType+"/" {
			t.Errorf("Cache key for %s doesn't start with '%s/', got: %s",
				networkType, networkType, cacheKey)
		}
	}

	// Verify all keys are unique
	seen := make(map[string]string)
	for networkType, key := range cacheKeys {
		if existingNetwork, exists := seen[key]; exists {
			t.Errorf("Cache key collision: %s and %s both produce key: %s",
				networkType, existingNetwork, key)
		}
		seen[key] = networkType
	}

	t.Logf("Generated %d unique cache keys for different network types", len(cacheKeys))
}

// TestBuildConfigHashInCacheKey verifies that build configuration affects
// cache key generation (T018).
func TestBuildConfigHashInCacheKey(t *testing.T) {
	commitHash := "abc123def456789012345678901234567890abcd"
	networkType := "testnet"

	// Create different build configs
	config1 := &network.BuildConfig{
		LDFlags: []string{"-X main.Version=1.0"},
	}
	config2 := &network.BuildConfig{
		LDFlags: []string{"-X main.Version=2.0"},
	}
	config3 := &network.BuildConfig{
		Tags: []string{"netgo"},
	}

	// Generate cache keys
	key1 := networkType + "/" + commitHash + "-" + config1.Hash()[:8]
	key2 := networkType + "/" + commitHash + "-" + config2.Hash()[:8]
	key3 := networkType + "/" + commitHash + "-" + config3.Hash()[:8]

	// All should be different
	if key1 == key2 {
		t.Error("Different ldflags should produce different cache keys")
	}
	if key1 == key3 {
		t.Error("Different config types should produce different cache keys")
	}
	if key2 == key3 {
		t.Error("All configs should produce unique cache keys")
	}

	t.Logf("Config1 key: %s", key1)
	t.Logf("Config2 key: %s", key2)
	t.Logf("Config3 key: %s", key3)
}
