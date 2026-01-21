package ctxconfig

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/types"
)

func TestNew(t *testing.T) {
	cfg := New(
		WithHomeDir("/home/test"),
		WithChainID("test-chain-1"),
		WithVerbose(true),
	)

	if cfg.HomeDir() != "/home/test" {
		t.Errorf("expected HomeDir /home/test, got %s", cfg.HomeDir())
	}
	if cfg.ChainID() != "test-chain-1" {
		t.Errorf("expected ChainID test-chain-1, got %s", cfg.ChainID())
	}
	if !cfg.Verbose() {
		t.Error("expected Verbose true, got false")
	}
}

func TestConfigClone(t *testing.T) {
	original := New(
		WithHomeDir("/original"),
		WithChainID("original-chain"),
	)

	clone := original.Clone(WithChainID("cloned-chain"))

	if original.ChainID() != "original-chain" {
		t.Errorf("original ChainID changed: %s", original.ChainID())
	}
	if clone.ChainID() != "cloned-chain" {
		t.Errorf("expected clone ChainID cloned-chain, got %s", clone.ChainID())
	}
	if clone.HomeDir() != "/original" {
		t.Errorf("expected clone HomeDir /original, got %s", clone.HomeDir())
	}
}

func TestNilConfigClone(t *testing.T) {
	var nilCfg *Config
	clone := nilCfg.Clone(WithHomeDir("/new"))

	if clone == nil {
		t.Fatal("Clone of nil should return non-nil config")
	}
	if clone.HomeDir() != "/new" {
		t.Errorf("expected HomeDir /new, got %s", clone.HomeDir())
	}
}

func TestWithConfig(t *testing.T) {
	ctx := context.Background()
	cfg := New(WithChainID("ctx-chain"))

	newCtx := WithConfig(ctx, cfg)

	retrieved := FromContext(newCtx)
	if retrieved == nil {
		t.Fatal("expected config in context, got nil")
	}
	if retrieved.ChainID() != "ctx-chain" {
		t.Errorf("expected ChainID ctx-chain, got %s", retrieved.ChainID())
	}
}

func TestFromContextNil(t *testing.T) {
	ctx := context.Background()
	cfg := FromContext(ctx)

	if cfg != nil {
		t.Errorf("expected nil config from empty context, got %v", cfg)
	}
}

func TestFromContextNilContext(t *testing.T) {
	cfg := FromContext(nil)

	if cfg != nil {
		t.Errorf("expected nil config from nil context, got %v", cfg)
	}
}

func TestMustFromContextPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustFromContext to panic on empty context")
		}
	}()

	ctx := context.Background()
	MustFromContext(ctx)
}

func TestMustFromContextSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := New(WithChainID("must-chain"))
	ctx = WithConfig(ctx, cfg)

	retrieved := MustFromContext(ctx)
	if retrieved.ChainID() != "must-chain" {
		t.Errorf("expected ChainID must-chain, got %s", retrieved.ChainID())
	}
}

func TestNilConfigAccessors(t *testing.T) {
	var cfg *Config

	if cfg.HomeDir() != "" {
		t.Error("expected empty HomeDir from nil config")
	}
	if cfg.ConfigPath() != "" {
		t.Error("expected empty ConfigPath from nil config")
	}
	if cfg.JSONMode() {
		t.Error("expected false JSONMode from nil config")
	}
	if cfg.NoColor() {
		t.Error("expected false NoColor from nil config")
	}
	if cfg.Verbose() {
		t.Error("expected false Verbose from nil config")
	}
	if cfg.ChainID() != "" {
		t.Error("expected empty ChainID from nil config")
	}
	if cfg.ExecutionMode() != "" {
		t.Error("expected empty ExecutionMode from nil config")
	}
}

func TestAllOptions(t *testing.T) {
	cfg := New(
		WithHomeDir("/test/home"),
		WithConfigPath("/test/config.toml"),
		WithJSONMode(true),
		WithNoColor(true),
		WithVerbose(true),
		WithChainID("all-opts-chain"),
		WithNetworkVersion("v1.0.0"),
		WithBlockchainNetwork("stable"),
		WithNetworkName("mainnet"),
		WithExecutionMode(types.ExecutionModeLocal),
		WithNumValidators(4),
		WithNumAccounts(10),
		WithNoCache(true),
		WithCacheTTL("1h"),
		WithGitHubToken("ghp_test"),
		WithDockerImage("myimage:latest"),
	)

	if cfg.HomeDir() != "/test/home" {
		t.Errorf("HomeDir mismatch")
	}
	if cfg.ConfigPath() != "/test/config.toml" {
		t.Errorf("ConfigPath mismatch")
	}
	if !cfg.JSONMode() {
		t.Errorf("JSONMode mismatch")
	}
	if !cfg.NoColor() {
		t.Errorf("NoColor mismatch")
	}
	if !cfg.Verbose() {
		t.Errorf("Verbose mismatch")
	}
	if cfg.ChainID() != "all-opts-chain" {
		t.Errorf("ChainID mismatch")
	}
	if cfg.NetworkVersion() != "v1.0.0" {
		t.Errorf("NetworkVersion mismatch")
	}
	if cfg.BlockchainNetwork() != "stable" {
		t.Errorf("BlockchainNetwork mismatch")
	}
	if cfg.NetworkName() != "mainnet" {
		t.Errorf("NetworkName mismatch")
	}
	if cfg.ExecutionMode() != types.ExecutionModeLocal {
		t.Errorf("ExecutionMode mismatch")
	}
	if cfg.NumValidators() != 4 {
		t.Errorf("NumValidators mismatch")
	}
	if cfg.NumAccounts() != 10 {
		t.Errorf("NumAccounts mismatch")
	}
	if !cfg.NoCache() {
		t.Errorf("NoCache mismatch")
	}
	if cfg.CacheTTL() != "1h" {
		t.Errorf("CacheTTL mismatch")
	}
	if cfg.GitHubToken() != "ghp_test" {
		t.Errorf("GitHubToken mismatch")
	}
	if cfg.DockerImage() != "myimage:latest" {
		t.Errorf("DockerImage mismatch")
	}
}

func TestFromFileConfig(t *testing.T) {
	home := "/file/home"
	noColor := true
	verbose := true
	jsonMode := true
	network := "testnet"
	blockchainNetwork := "stable"
	validators := 4
	execMode := types.ExecutionModeDocker
	networkVersion := "v1.0.0"
	noCache := true
	accounts := 10
	githubToken := "ghp_file"
	cacheTTL := "2h"

	fc := &config.FileConfig{
		Home:              &home,
		NoColor:           &noColor,
		Verbose:           &verbose,
		JSON:              &jsonMode,
		Network:           &network,
		BlockchainNetwork: &blockchainNetwork,
		Validators:        &validators,
		ExecutionMode:     &execMode,
		NetworkVersion:    &networkVersion,
		NoCache:           &noCache,
		Accounts:          &accounts,
		GitHubToken:       &githubToken,
		CacheTTL:          &cacheTTL,
	}

	cfg := New(FromFileConfig(fc))

	if cfg.HomeDir() != home {
		t.Errorf("HomeDir mismatch: got %s", cfg.HomeDir())
	}
	if cfg.NoColor() != noColor {
		t.Errorf("NoColor mismatch")
	}
	if cfg.Verbose() != verbose {
		t.Errorf("Verbose mismatch")
	}
	if cfg.JSONMode() != jsonMode {
		t.Errorf("JSONMode mismatch")
	}
	if cfg.NetworkName() != network {
		t.Errorf("NetworkName mismatch: got %s", cfg.NetworkName())
	}
	if cfg.BlockchainNetwork() != blockchainNetwork {
		t.Errorf("BlockchainNetwork mismatch")
	}
	if cfg.NumValidators() != validators {
		t.Errorf("NumValidators mismatch")
	}
	if cfg.ExecutionMode() != execMode {
		t.Errorf("ExecutionMode mismatch")
	}
	if cfg.NetworkVersion() != networkVersion {
		t.Errorf("NetworkVersion mismatch")
	}
	if cfg.NoCache() != noCache {
		t.Errorf("NoCache mismatch")
	}
	if cfg.NumAccounts() != accounts {
		t.Errorf("NumAccounts mismatch")
	}
	if cfg.GitHubToken() != githubToken {
		t.Errorf("GitHubToken mismatch")
	}
	if cfg.CacheTTL() != cacheTTL {
		t.Errorf("CacheTTL mismatch")
	}
}

func TestFromFileConfigNil(t *testing.T) {
	cfg := New(
		FromFileConfig(nil),
		WithHomeDir("/override"),
	)

	if cfg.HomeDir() != "/override" {
		t.Errorf("expected HomeDir /override, got %s", cfg.HomeDir())
	}
}

func TestFromFileConfigOverride(t *testing.T) {
	home := "/file/home"
	fc := &config.FileConfig{
		Home: &home,
	}

	cfg := New(
		FromFileConfig(fc),
		WithHomeDir("/override/home"),
	)

	if cfg.HomeDir() != "/override/home" {
		t.Errorf("expected HomeDir /override/home, got %s", cfg.HomeDir())
	}
}

// Test context helpers with fallback
func TestHomeDirHelper(t *testing.T) {
	// No config in context - use fallback
	ctx := context.Background()
	if HomeDir(ctx, "/fallback") != "/fallback" {
		t.Error("expected fallback when no config")
	}

	// Empty homeDir in config - use fallback
	ctx = WithConfig(ctx, New())
	if HomeDir(ctx, "/fallback") != "/fallback" {
		t.Error("expected fallback when homeDir empty")
	}

	// HomeDir set in config
	ctx = WithConfig(context.Background(), New(WithHomeDir("/context")))
	if HomeDir(ctx, "/fallback") != "/context" {
		t.Error("expected context value")
	}
}

func TestVerboseHelper(t *testing.T) {
	ctx := context.Background()
	if Verbose(ctx) {
		t.Error("expected false when no config")
	}

	ctx = WithConfig(ctx, New(WithVerbose(true)))
	if !Verbose(ctx) {
		t.Error("expected true from config")
	}
}

func TestJSONModeHelper(t *testing.T) {
	ctx := context.Background()
	if JSONMode(ctx) {
		t.Error("expected false when no config")
	}

	ctx = WithConfig(ctx, New(WithJSONMode(true)))
	if !JSONMode(ctx) {
		t.Error("expected true from config")
	}
}

func TestExecutionModeHelper(t *testing.T) {
	ctx := context.Background()
	if ExecutionMode(ctx) != "" {
		t.Error("expected empty when no config")
	}

	ctx = WithConfig(ctx, New(WithExecutionMode(types.ExecutionModeDocker)))
	if ExecutionMode(ctx) != types.ExecutionModeDocker {
		t.Error("expected docker from config")
	}
}
