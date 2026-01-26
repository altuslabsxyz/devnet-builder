// internal/plugin/cosmos/initializer_test.go
package cosmos

import (
	"path/filepath"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

func TestCosmosInitializerBinaryName(t *testing.T) {
	tests := []struct {
		name       string
		binaryName string
		want       string
	}{
		{"stabled", "stabled", "stabled"},
		{"gaiad", "gaiad", "gaiad"},
		{"simd", "simd", "simd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := NewCosmosInitializer(tt.binaryName)
			if got := i.BinaryName(); got != tt.want {
				t.Errorf("BinaryName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCosmosInitializerDefaultChainID(t *testing.T) {
	i := NewCosmosInitializer("stabled")

	chainID := i.DefaultChainID()
	if chainID != "devnet-1" {
		t.Errorf("DefaultChainID() = %v, want devnet-1", chainID)
	}
}

func TestCosmosInitializerDefaultMoniker(t *testing.T) {
	i := NewCosmosInitializer("stabled")

	tests := []struct {
		index int
		want  string
	}{
		{0, "validator-0"},
		{1, "validator-1"},
		{2, "validator-2"},
		{10, "validator-10"},
		{99, "validator-99"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := i.DefaultMoniker(tt.index); got != tt.want {
				t.Errorf("DefaultMoniker(%d) = %v, want %v", tt.index, got, tt.want)
			}
		})
	}
}

func TestCosmosInitializerInitCommandArgs(t *testing.T) {
	i := NewCosmosInitializer("stabled")

	homeDir := "/path/to/home"
	moniker := "my-validator"
	chainID := "test-chain-1"

	args := i.InitCommandArgs(homeDir, moniker, chainID)

	// Verify expected arguments
	expectedArgs := []string{
		"init", "my-validator",
		"--chain-id", "test-chain-1",
		"--home", "/path/to/home",
		"--overwrite",
	}

	if len(args) != len(expectedArgs) {
		t.Errorf("InitCommandArgs() returned %d args, want %d", len(args), len(expectedArgs))
	}

	for i, expected := range expectedArgs {
		if args[i] != expected {
			t.Errorf("InitCommandArgs()[%d] = %v, want %v", i, args[i], expected)
		}
	}
}

func TestCosmosInitializerInitCommandArgsContainsRequiredFlags(t *testing.T) {
	i := NewCosmosInitializer("gaiad")

	args := i.InitCommandArgs("/home/node", "validator-0", "devnet-1")

	// Check for required components
	hasInit := false
	hasChainID := false
	hasHome := false
	hasOverwrite := false
	hasMoniker := false

	for idx, arg := range args {
		switch arg {
		case "init":
			hasInit = true
			// Next arg should be moniker
			if idx+1 < len(args) && args[idx+1] == "validator-0" {
				hasMoniker = true
			}
		case "--chain-id":
			if idx+1 < len(args) && args[idx+1] == "devnet-1" {
				hasChainID = true
			}
		case "--home":
			if idx+1 < len(args) && args[idx+1] == "/home/node" {
				hasHome = true
			}
		case "--overwrite":
			hasOverwrite = true
		}
	}

	if !hasInit {
		t.Error("InitCommandArgs should include 'init' command")
	}
	if !hasMoniker {
		t.Error("InitCommandArgs should include moniker after 'init'")
	}
	if !hasChainID {
		t.Error("InitCommandArgs should include '--chain-id' with value")
	}
	if !hasHome {
		t.Error("InitCommandArgs should include '--home' with path")
	}
	if !hasOverwrite {
		t.Error("InitCommandArgs should include '--overwrite' flag")
	}
}

func TestCosmosInitializerConfigDir(t *testing.T) {
	i := NewCosmosInitializer("stabled")

	tests := []struct {
		homeDir string
		want    string
	}{
		{"/home/node", "/home/node/config"},
		{"/root/.stabled", "/root/.stabled/config"},
		{"/path/to/devnet/node0", "/path/to/devnet/node0/config"},
	}

	for _, tt := range tests {
		t.Run(tt.homeDir, func(t *testing.T) {
			got := i.ConfigDir(tt.homeDir)
			want := filepath.Join(tt.homeDir, "config")
			if got != want {
				t.Errorf("ConfigDir(%v) = %v, want %v", tt.homeDir, got, want)
			}
		})
	}
}

func TestCosmosInitializerDataDir(t *testing.T) {
	i := NewCosmosInitializer("stabled")

	tests := []struct {
		homeDir string
		want    string
	}{
		{"/home/node", "/home/node/data"},
		{"/root/.stabled", "/root/.stabled/data"},
		{"/path/to/devnet/node0", "/path/to/devnet/node0/data"},
	}

	for _, tt := range tests {
		t.Run(tt.homeDir, func(t *testing.T) {
			got := i.DataDir(tt.homeDir)
			want := filepath.Join(tt.homeDir, "data")
			if got != want {
				t.Errorf("DataDir(%v) = %v, want %v", tt.homeDir, got, want)
			}
		})
	}
}

func TestCosmosInitializerKeyringDir(t *testing.T) {
	i := NewCosmosInitializer("stabled")

	tests := []struct {
		homeDir string
		want    string
	}{
		{"/home/node", "/home/node"},
		{"/root/.stabled", "/root/.stabled"},
		{"/path/to/devnet/node0", "/path/to/devnet/node0"},
	}

	for _, tt := range tests {
		t.Run(tt.homeDir, func(t *testing.T) {
			got := i.KeyringDir(tt.homeDir)
			// For Cosmos SDK with test backend, keyring is in home dir
			if got != tt.want {
				t.Errorf("KeyringDir(%v) = %v, want %v", tt.homeDir, got, tt.want)
			}
		})
	}
}

func TestCosmosInitializerImplementsInterface(t *testing.T) {
	// This test ensures CosmosInitializer implements PluginInitializer interface
	var _ types.PluginInitializer = (*CosmosInitializer)(nil)
}

func TestNewCosmosInitializer(t *testing.T) {
	i := NewCosmosInitializer("customd")

	if i == nil {
		t.Fatal("NewCosmosInitializer returned nil")
	}

	if i.binaryName != "customd" {
		t.Errorf("Expected binaryName 'customd', got '%s'", i.binaryName)
	}
}
