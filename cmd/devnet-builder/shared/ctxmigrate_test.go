package shared

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
)

func TestConfigFromGlobals(t *testing.T) {
	// Set up globals
	SetHomeDir("/test/home")
	SetConfigPath("/test/config.toml")
	SetJSONMode(true)
	SetNoColor(true)
	SetVerbose(true)
	SetLoadedFileConfig(nil)

	cfg := ConfigFromGlobals()

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
	SetHomeDir("")
	SetConfigPath("")
	SetJSONMode(false)
	SetNoColor(false)
	SetVerbose(false)
}

func TestSyncConfigToGlobals(t *testing.T) {
	// Reset globals first
	SetHomeDir("")
	SetConfigPath("")
	SetJSONMode(false)
	SetNoColor(false)
	SetVerbose(false)

	cfg := ctxconfig.New(
		ctxconfig.WithHomeDir("/sync/home"),
		ctxconfig.WithConfigPath("/sync/config.toml"),
		ctxconfig.WithJSONMode(true),
		ctxconfig.WithNoColor(true),
		ctxconfig.WithVerbose(true),
	)

	SyncConfigToGlobals(cfg)

	if GetHomeDir() != "/sync/home" {
		t.Errorf("expected HomeDir /sync/home, got %s", GetHomeDir())
	}
	if GetConfigPath() != "/sync/config.toml" {
		t.Errorf("expected ConfigPath /sync/config.toml, got %s", GetConfigPath())
	}
	if !GetJSONMode() {
		t.Error("expected JSONMode true")
	}
	if !GetNoColor() {
		t.Error("expected NoColor true")
	}
	if !GetVerbose() {
		t.Error("expected Verbose true")
	}

	// Clean up
	SetHomeDir("")
	SetConfigPath("")
	SetJSONMode(false)
	SetNoColor(false)
	SetVerbose(false)
}

func TestSyncConfigToGlobalsNil(t *testing.T) {
	// Should not panic
	SyncConfigToGlobals(nil)
}

func TestEnsureConfigInContext(t *testing.T) {
	// Test with existing config in context
	existingCfg := ctxconfig.New(ctxconfig.WithChainID("existing-chain"))
	ctx := ctxconfig.WithConfig(context.Background(), existingCfg)

	newCtx, cfg := EnsureConfigInContext(ctx)
	if cfg.ChainID() != "existing-chain" {
		t.Errorf("expected existing ChainID, got %s", cfg.ChainID())
	}
	if newCtx != ctx {
		t.Error("expected same context when config exists")
	}

	// Test without config in context (creates from globals)
	SetHomeDir("/ensure/home")
	SetLoadedFileConfig(nil)

	emptyCtx := context.Background()
	newCtx, cfg = EnsureConfigInContext(emptyCtx)

	if cfg.HomeDir() != "/ensure/home" {
		t.Errorf("expected HomeDir from globals, got %s", cfg.HomeDir())
	}
	if newCtx == emptyCtx {
		t.Error("expected new context when config created from globals")
	}

	// Clean up
	SetHomeDir("")
}

func TestWithChainIDInContext(t *testing.T) {
	// Test with existing config
	existingCfg := ctxconfig.New(
		ctxconfig.WithHomeDir("/existing"),
		ctxconfig.WithChainID("old-chain"),
	)
	ctx := ctxconfig.WithConfig(context.Background(), existingCfg)

	newCtx := WithChainIDInContext(ctx, "new-chain")
	newCfg := ctxconfig.FromContext(newCtx)

	if newCfg.ChainID() != "new-chain" {
		t.Errorf("expected ChainID new-chain, got %s", newCfg.ChainID())
	}
	if newCfg.HomeDir() != "/existing" {
		t.Errorf("expected HomeDir preserved, got %s", newCfg.HomeDir())
	}

	// Original should be unchanged
	originalCfg := ctxconfig.FromContext(ctx)
	if originalCfg.ChainID() != "old-chain" {
		t.Errorf("original config modified: %s", originalCfg.ChainID())
	}
}

func TestWithChainIDInContextNoExisting(t *testing.T) {
	SetHomeDir("/globals/home")
	SetLoadedFileConfig(nil)

	ctx := context.Background()
	newCtx := WithChainIDInContext(ctx, "from-globals-chain")
	newCfg := ctxconfig.FromContext(newCtx)

	if newCfg.ChainID() != "from-globals-chain" {
		t.Errorf("expected ChainID from-globals-chain, got %s", newCfg.ChainID())
	}
	if newCfg.HomeDir() != "/globals/home" {
		t.Errorf("expected HomeDir from globals, got %s", newCfg.HomeDir())
	}

	// Clean up
	SetHomeDir("")
}

func TestWithNetworkVersionInContext(t *testing.T) {
	existingCfg := ctxconfig.New(ctxconfig.WithNetworkVersion("v1.0.0"))
	ctx := ctxconfig.WithConfig(context.Background(), existingCfg)

	newCtx := WithNetworkVersionInContext(ctx, "v2.0.0")
	newCfg := ctxconfig.FromContext(newCtx)

	if newCfg.NetworkVersion() != "v2.0.0" {
		t.Errorf("expected NetworkVersion v2.0.0, got %s", newCfg.NetworkVersion())
	}
}

func TestWithBlockchainNetworkInContext(t *testing.T) {
	existingCfg := ctxconfig.New(ctxconfig.WithBlockchainNetwork("stable"))
	ctx := ctxconfig.WithConfig(context.Background(), existingCfg)

	newCtx := WithBlockchainNetworkInContext(ctx, "ault")
	newCfg := ctxconfig.FromContext(newCtx)

	if newCfg.BlockchainNetwork() != "ault" {
		t.Errorf("expected BlockchainNetwork ault, got %s", newCfg.BlockchainNetwork())
	}
}

func TestUpdateConfigInContext(t *testing.T) {
	existingCfg := ctxconfig.New(
		ctxconfig.WithHomeDir("/original"),
		ctxconfig.WithChainID("original-chain"),
	)
	ctx := ctxconfig.WithConfig(context.Background(), existingCfg)

	newCtx := UpdateConfigInContext(ctx,
		ctxconfig.WithChainID("updated-chain"),
		ctxconfig.WithVerbose(true),
	)
	newCfg := ctxconfig.FromContext(newCtx)

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

func TestUpdateConfigInContextNoExisting(t *testing.T) {
	SetHomeDir("/update/home")
	SetLoadedFileConfig(nil)

	ctx := context.Background()
	newCtx := UpdateConfigInContext(ctx, ctxconfig.WithChainID("new-chain"))
	newCfg := ctxconfig.FromContext(newCtx)

	if newCfg.ChainID() != "new-chain" {
		t.Errorf("expected ChainID new-chain, got %s", newCfg.ChainID())
	}
	if newCfg.HomeDir() != "/update/home" {
		t.Errorf("expected HomeDir from globals, got %s", newCfg.HomeDir())
	}

	// Clean up
	SetHomeDir("")
}
