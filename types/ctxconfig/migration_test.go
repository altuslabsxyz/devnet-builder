package ctxconfig

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
)

func TestFromGlobals(t *testing.T) {
	// Set up globals
	shared.SetHomeDir("/test/home")
	shared.SetConfigPath("/test/config.toml")
	shared.SetJSONMode(true)
	shared.SetNoColor(true)
	shared.SetVerbose(true)
	shared.SetLoadedFileConfig(nil)

	cfg := FromGlobals()

	if cfg.HomeDir() != "/test/home" {
		t.Errorf("expected HomeDir /test/home, got %s", cfg.HomeDir())
	}
	if cfg.ConfigPath() != "/test/config.toml" {
		t.Errorf("expected ConfigPath /test/config.toml, got %s", cfg.ConfigPath())
	}
	if !cfg.JSONMode() {
		t.Error("expected JSONMode true")
	}
	if !cfg.NoColor() {
		t.Error("expected NoColor true")
	}
	if !cfg.Verbose() {
		t.Error("expected Verbose true")
	}

	// Clean up
	shared.SetHomeDir("")
	shared.SetConfigPath("")
	shared.SetJSONMode(false)
	shared.SetNoColor(false)
	shared.SetVerbose(false)
}

func TestSyncToGlobals(t *testing.T) {
	// Reset globals first
	shared.SetHomeDir("")
	shared.SetConfigPath("")
	shared.SetJSONMode(false)
	shared.SetNoColor(false)
	shared.SetVerbose(false)

	cfg := New(
		WithHomeDir("/sync/home"),
		WithConfigPath("/sync/config.toml"),
		WithJSONMode(true),
		WithNoColor(true),
		WithVerbose(true),
	)

	SyncToGlobals(cfg)

	if shared.GetHomeDir() != "/sync/home" {
		t.Errorf("expected HomeDir /sync/home, got %s", shared.GetHomeDir())
	}
	if shared.GetConfigPath() != "/sync/config.toml" {
		t.Errorf("expected ConfigPath /sync/config.toml, got %s", shared.GetConfigPath())
	}
	if !shared.GetJSONMode() {
		t.Error("expected JSONMode true")
	}
	if !shared.GetNoColor() {
		t.Error("expected NoColor true")
	}
	if !shared.GetVerbose() {
		t.Error("expected Verbose true")
	}

	// Clean up
	shared.SetHomeDir("")
	shared.SetConfigPath("")
	shared.SetJSONMode(false)
	shared.SetNoColor(false)
	shared.SetVerbose(false)
}

func TestSyncToGlobalsNil(t *testing.T) {
	// Should not panic
	SyncToGlobals(nil)
}

func TestEnsureInContext(t *testing.T) {
	// Test with existing config in context
	existingCfg := New(WithChainID("existing-chain"))
	ctx := WithConfig(context.Background(), existingCfg)

	newCtx, cfg := EnsureInContext(ctx)
	if cfg.ChainID() != "existing-chain" {
		t.Errorf("expected existing ChainID, got %s", cfg.ChainID())
	}
	if newCtx != ctx {
		t.Error("expected same context when config exists")
	}

	// Test without config in context (creates from globals)
	shared.SetHomeDir("/ensure/home")
	shared.SetLoadedFileConfig(nil)

	emptyCtx := context.Background()
	newCtx, cfg = EnsureInContext(emptyCtx)

	if cfg.HomeDir() != "/ensure/home" {
		t.Errorf("expected HomeDir from globals, got %s", cfg.HomeDir())
	}
	if newCtx == emptyCtx {
		t.Error("expected new context when config created from globals")
	}

	// Clean up
	shared.SetHomeDir("")
}

func TestWithChainIDInContext(t *testing.T) {
	// Test with existing config
	existingCfg := New(
		WithHomeDir("/existing"),
		WithChainID("old-chain"),
	)
	ctx := WithConfig(context.Background(), existingCfg)

	newCtx := WithChainIDInContext(ctx, "new-chain")
	newCfg := FromContext(newCtx)

	if newCfg.ChainID() != "new-chain" {
		t.Errorf("expected ChainID new-chain, got %s", newCfg.ChainID())
	}
	if newCfg.HomeDir() != "/existing" {
		t.Errorf("expected HomeDir preserved, got %s", newCfg.HomeDir())
	}

	// Original should be unchanged
	originalCfg := FromContext(ctx)
	if originalCfg.ChainID() != "old-chain" {
		t.Errorf("original config modified: %s", originalCfg.ChainID())
	}
}

func TestWithChainIDInContextNoExisting(t *testing.T) {
	shared.SetHomeDir("/globals/home")
	shared.SetLoadedFileConfig(nil)

	ctx := context.Background()
	newCtx := WithChainIDInContext(ctx, "from-globals-chain")
	newCfg := FromContext(newCtx)

	if newCfg.ChainID() != "from-globals-chain" {
		t.Errorf("expected ChainID from-globals-chain, got %s", newCfg.ChainID())
	}
	if newCfg.HomeDir() != "/globals/home" {
		t.Errorf("expected HomeDir from globals, got %s", newCfg.HomeDir())
	}

	// Clean up
	shared.SetHomeDir("")
}

func TestWithNetworkVersionInContext(t *testing.T) {
	existingCfg := New(WithNetworkVersion("v1.0.0"))
	ctx := WithConfig(context.Background(), existingCfg)

	newCtx := WithNetworkVersionInContext(ctx, "v2.0.0")
	newCfg := FromContext(newCtx)

	if newCfg.NetworkVersion() != "v2.0.0" {
		t.Errorf("expected NetworkVersion v2.0.0, got %s", newCfg.NetworkVersion())
	}
}

func TestWithBlockchainNetworkInContext(t *testing.T) {
	existingCfg := New(WithBlockchainNetwork("stable"))
	ctx := WithConfig(context.Background(), existingCfg)

	newCtx := WithBlockchainNetworkInContext(ctx, "ault")
	newCfg := FromContext(newCtx)

	if newCfg.BlockchainNetwork() != "ault" {
		t.Errorf("expected BlockchainNetwork ault, got %s", newCfg.BlockchainNetwork())
	}
}

func TestUpdateInContext(t *testing.T) {
	existingCfg := New(
		WithHomeDir("/original"),
		WithChainID("original-chain"),
	)
	ctx := WithConfig(context.Background(), existingCfg)

	newCtx := UpdateInContext(ctx,
		WithChainID("updated-chain"),
		WithVerbose(true),
	)
	newCfg := FromContext(newCtx)

	if newCfg.ChainID() != "updated-chain" {
		t.Errorf("expected ChainID updated-chain, got %s", newCfg.ChainID())
	}
	if newCfg.HomeDir() != "/original" {
		t.Errorf("expected HomeDir preserved, got %s", newCfg.HomeDir())
	}
	if !newCfg.Verbose() {
		t.Error("expected Verbose true")
	}
}

func TestUpdateInContextNoExisting(t *testing.T) {
	shared.SetHomeDir("/update/home")
	shared.SetLoadedFileConfig(nil)

	ctx := context.Background()
	newCtx := UpdateInContext(ctx, WithChainID("new-chain"))
	newCfg := FromContext(newCtx)

	if newCfg.ChainID() != "new-chain" {
		t.Errorf("expected ChainID new-chain, got %s", newCfg.ChainID())
	}
	if newCfg.HomeDir() != "/update/home" {
		t.Errorf("expected HomeDir from globals, got %s", newCfg.HomeDir())
	}

	// Clean up
	shared.SetHomeDir("")
}
