// internal/plugin/cosmos/builder_test.go
package cosmos

import (
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// Compile-time check that CosmosBuilder implements PluginBuilder
var _ types.PluginBuilder = (*CosmosBuilder)(nil)

func TestDefaultBuildFlags(t *testing.T) {
	builder := NewCosmosBuilder("stabled", "github.com/cosmosphere-labs/stable")

	flags := builder.DefaultBuildFlags()

	if _, ok := flags["ldflags"]; !ok {
		t.Error("Expected ldflags in default build flags")
	}

	if _, ok := flags["tags"]; !ok {
		t.Error("Expected tags in default build flags")
	}
}

func TestBinaryName(t *testing.T) {
	builder := NewCosmosBuilder("stabled", "github.com/cosmosphere-labs/stable")

	if builder.BinaryName() != "stabled" {
		t.Errorf("Expected binary name 'stabled', got '%s'", builder.BinaryName())
	}
}

func TestDefaultGitRepo(t *testing.T) {
	builder := NewCosmosBuilder("stabled", "github.com/cosmosphere-labs/stable")

	if builder.DefaultGitRepo() != "github.com/cosmosphere-labs/stable" {
		t.Errorf("Unexpected default git repo: %s", builder.DefaultGitRepo())
	}
}
