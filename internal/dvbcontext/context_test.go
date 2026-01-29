package dvbcontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantNamespace string
		wantDevnet    string
	}{
		{
			name:          "devnet only",
			input:         "stable-testnet",
			wantNamespace: "default",
			wantDevnet:    "stable-testnet",
		},
		{
			name:          "namespace/devnet",
			input:         "staging/my-devnet",
			wantNamespace: "staging",
			wantDevnet:    "my-devnet",
		},
		{
			name:          "default namespace explicit",
			input:         "default/stable-testnet",
			wantNamespace: "default",
			wantDevnet:    "stable-testnet",
		},
		{
			name:          "with whitespace",
			input:         "  production/chain  ",
			wantNamespace: "production",
			wantDevnet:    "chain",
		},
		{
			name:          "multiple slashes",
			input:         "ns/devnet/extra",
			wantNamespace: "ns",
			wantDevnet:    "devnet/extra",
		},
		{
			name:          "empty string",
			input:         "",
			wantNamespace: "default",
			wantDevnet:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNs, gotDevnet := ParseRef(tt.input)
			if gotNs != tt.wantNamespace {
				t.Errorf("ParseRef() namespace = %q, want %q", gotNs, tt.wantNamespace)
			}
			if gotDevnet != tt.wantDevnet {
				t.Errorf("ParseRef() devnet = %q, want %q", gotDevnet, tt.wantDevnet)
			}
		})
	}
}

func TestContext_String(t *testing.T) {
	ctx := &Context{Namespace: "staging", Devnet: "my-chain"}
	want := "staging/my-chain"
	if got := ctx.String(); got != want {
		t.Errorf("Context.String() = %q, want %q", got, want)
	}
}

func TestSaveLoadClear(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create the .devnet-builder directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".devnet-builder"), 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	// Initially, no context should be set
	ctx, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ctx != nil {
		t.Errorf("Load() expected nil context initially, got %v", ctx)
	}

	// Save a context
	if err := Save("staging", "my-devnet"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load should return the saved context
	ctx, err = Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ctx == nil {
		t.Fatal("Load() expected non-nil context after save")
	}
	if ctx.Namespace != "staging" || ctx.Devnet != "my-devnet" {
		t.Errorf("Load() = %v, want staging/my-devnet", ctx)
	}

	// Clear the context
	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// Load should return nil again
	ctx, err = Load()
	if err != nil {
		t.Fatalf("Load() after clear error = %v", err)
	}
	if ctx != nil {
		t.Errorf("Load() after clear expected nil, got %v", ctx)
	}

	// Clear again should not error
	if err := Clear(); err != nil {
		t.Fatalf("Clear() second time error = %v", err)
	}
}
