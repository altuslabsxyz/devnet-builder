package ctxhelper_test

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ctxhelper"
	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
)

func TestHomeDir(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		fallback string
		want     string
	}{
		{
			name:     "nil context returns fallback",
			ctx:      nil,
			fallback: "/fallback/path",
			want:     "/fallback/path",
		},
		{
			name:     "empty context returns fallback",
			ctx:      context.Background(),
			fallback: "/fallback/path",
			want:     "/fallback/path",
		},
		{
			name: "context with empty homeDir returns fallback",
			ctx: ctxconfig.WithConfig(context.Background(), ctxconfig.New(
				ctxconfig.WithHomeDir(""),
			)),
			fallback: "/fallback/path",
			want:     "/fallback/path",
		},
		{
			name: "context with homeDir returns context value",
			ctx: ctxconfig.WithConfig(context.Background(), ctxconfig.New(
				ctxconfig.WithHomeDir("/context/path"),
			)),
			fallback: "/fallback/path",
			want:     "/context/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctxhelper.HomeDir(tt.ctx, tt.fallback)
			if got != tt.want {
				t.Errorf("HomeDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHomeDirStrict(t *testing.T) {
	t.Run("returns empty for no context config", func(t *testing.T) {
		got := ctxhelper.HomeDirStrict(context.Background())
		if got != "" {
			t.Errorf("HomeDirStrict() = %q, want empty string", got)
		}
	})

	t.Run("returns value from context", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithHomeDir("/strict/path"),
		))
		got := ctxhelper.HomeDirStrict(ctx)
		if got != "/strict/path" {
			t.Errorf("HomeDirStrict() = %q, want /strict/path", got)
		}
	})
}

func TestExecutionMode(t *testing.T) {
	t.Run("returns empty for no context config", func(t *testing.T) {
		got := ctxhelper.ExecutionMode(context.Background())
		if got != "" {
			t.Errorf("ExecutionMode() = %q, want empty", got)
		}
	})

	t.Run("returns value from context", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithExecutionMode(types.ExecutionModeDocker),
		))
		got := ctxhelper.ExecutionMode(ctx)
		if got != types.ExecutionModeDocker {
			t.Errorf("ExecutionMode() = %q, want %q", got, types.ExecutionModeDocker)
		}
	})
}

func TestExecutionModeWithFallback(t *testing.T) {
	t.Run("returns fallback when not in context", func(t *testing.T) {
		got := ctxhelper.ExecutionModeWithFallback(context.Background(), types.ExecutionModeLocal)
		if got != types.ExecutionModeLocal {
			t.Errorf("ExecutionModeWithFallback() = %q, want %q", got, types.ExecutionModeLocal)
		}
	})

	t.Run("returns context value when present", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithExecutionMode(types.ExecutionModeDocker),
		))
		got := ctxhelper.ExecutionModeWithFallback(ctx, types.ExecutionModeLocal)
		if got != types.ExecutionModeDocker {
			t.Errorf("ExecutionModeWithFallback() = %q, want %q", got, types.ExecutionModeDocker)
		}
	})
}

func TestChainIDWithFallback(t *testing.T) {
	t.Run("returns fallback when not in context", func(t *testing.T) {
		got := ctxhelper.ChainIDWithFallback(context.Background(), "fallback-chain")
		if got != "fallback-chain" {
			t.Errorf("ChainIDWithFallback() = %q, want fallback-chain", got)
		}
	})

	t.Run("returns context value when present", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithChainID("context-chain"),
		))
		got := ctxhelper.ChainIDWithFallback(ctx, "fallback-chain")
		if got != "context-chain" {
			t.Errorf("ChainIDWithFallback() = %q, want context-chain", got)
		}
	})
}

func TestVerbose(t *testing.T) {
	t.Run("returns false for no context config", func(t *testing.T) {
		got := ctxhelper.Verbose(context.Background())
		if got != false {
			t.Errorf("Verbose() = %v, want false", got)
		}
	})

	t.Run("returns value from context", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithVerbose(true),
		))
		got := ctxhelper.Verbose(ctx)
		if got != true {
			t.Errorf("Verbose() = %v, want true", got)
		}
	})
}

func TestJSONMode(t *testing.T) {
	t.Run("returns false for no context config", func(t *testing.T) {
		got := ctxhelper.JSONMode(context.Background())
		if got != false {
			t.Errorf("JSONMode() = %v, want false", got)
		}
	})

	t.Run("returns value from context", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithJSONMode(true),
		))
		got := ctxhelper.JSONMode(ctx)
		if got != true {
			t.Errorf("JSONMode() = %v, want true", got)
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("returns nil for no context config", func(t *testing.T) {
		got := ctxhelper.Config(context.Background())
		if got != nil {
			t.Errorf("Config() = %v, want nil", got)
		}
	})

	t.Run("returns config from context", func(t *testing.T) {
		cfg := ctxconfig.New(ctxconfig.WithHomeDir("/test"))
		ctx := ctxconfig.WithConfig(context.Background(), cfg)
		got := ctxhelper.Config(ctx)
		if got != cfg {
			t.Errorf("Config() returned different config instance")
		}
	})
}

func TestMustConfig(t *testing.T) {
	t.Run("panics for no context config", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("MustConfig() should have panicked")
			}
		}()
		ctxhelper.MustConfig(context.Background())
	})

	t.Run("returns config from context", func(t *testing.T) {
		cfg := ctxconfig.New(ctxconfig.WithHomeDir("/test"))
		ctx := ctxconfig.WithConfig(context.Background(), cfg)
		got := ctxhelper.MustConfig(ctx)
		if got != cfg {
			t.Errorf("MustConfig() returned different config instance")
		}
	})
}

func TestHasConfig(t *testing.T) {
	t.Run("returns false for no context config", func(t *testing.T) {
		got := ctxhelper.HasConfig(context.Background())
		if got != false {
			t.Errorf("HasConfig() = %v, want false", got)
		}
	})

	t.Run("returns true when config present", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New())
		got := ctxhelper.HasConfig(ctx)
		if got != true {
			t.Errorf("HasConfig() = %v, want true", got)
		}
	})
}

func TestNetworkVersion(t *testing.T) {
	t.Run("returns empty for no context config", func(t *testing.T) {
		got := ctxhelper.NetworkVersion(context.Background())
		if got != "" {
			t.Errorf("NetworkVersion() = %q, want empty", got)
		}
	})

	t.Run("returns value from context", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithNetworkVersion("v1.2.3"),
		))
		got := ctxhelper.NetworkVersion(ctx)
		if got != "v1.2.3" {
			t.Errorf("NetworkVersion() = %q, want v1.2.3", got)
		}
	})
}

func TestBlockchainNetwork(t *testing.T) {
	t.Run("returns empty for no context config", func(t *testing.T) {
		got := ctxhelper.BlockchainNetwork(context.Background())
		if got != "" {
			t.Errorf("BlockchainNetwork() = %q, want empty", got)
		}
	})

	t.Run("returns value from context", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithBlockchainNetwork("stable"),
		))
		got := ctxhelper.BlockchainNetwork(ctx)
		if got != "stable" {
			t.Errorf("BlockchainNetwork() = %q, want stable", got)
		}
	})
}

func TestNumValidators(t *testing.T) {
	t.Run("returns 0 for no context config", func(t *testing.T) {
		got := ctxhelper.NumValidators(context.Background())
		if got != 0 {
			t.Errorf("NumValidators() = %d, want 0", got)
		}
	})

	t.Run("returns value from context", func(t *testing.T) {
		ctx := ctxconfig.WithConfig(context.Background(), ctxconfig.New(
			ctxconfig.WithNumValidators(4),
		))
		got := ctxhelper.NumValidators(ctx)
		if got != 4 {
			t.Errorf("NumValidators() = %d, want 4", got)
		}
	})
}
