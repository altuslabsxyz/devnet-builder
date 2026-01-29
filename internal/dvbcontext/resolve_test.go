package dvbcontext

import (
	"errors"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name              string
		explicitDevnet    string
		explicitNamespace string
		context           *Context
		wantNamespace     string
		wantDevnet        string
		wantErr           bool
	}{
		{
			name:              "explicit devnet and namespace",
			explicitDevnet:    "my-chain",
			explicitNamespace: "production",
			context:           nil,
			wantNamespace:     "production",
			wantDevnet:        "my-chain",
			wantErr:           false,
		},
		{
			name:              "explicit devnet only - default namespace",
			explicitDevnet:    "my-chain",
			explicitNamespace: "",
			context:           nil,
			wantNamespace:     "default",
			wantDevnet:        "my-chain",
			wantErr:           false,
		},
		{
			name:              "context provides devnet",
			explicitDevnet:    "",
			explicitNamespace: "",
			context:           &Context{Namespace: "staging", Devnet: "ctx-chain"},
			wantNamespace:     "staging",
			wantDevnet:        "ctx-chain",
			wantErr:           false,
		},
		{
			name:              "explicit overrides context devnet",
			explicitDevnet:    "explicit-chain",
			explicitNamespace: "",
			context:           &Context{Namespace: "staging", Devnet: "ctx-chain"},
			wantNamespace:     "staging",
			wantDevnet:        "explicit-chain",
			wantErr:           false,
		},
		{
			name:              "explicit overrides context namespace",
			explicitDevnet:    "",
			explicitNamespace: "production",
			context:           &Context{Namespace: "staging", Devnet: "ctx-chain"},
			wantNamespace:     "production",
			wantDevnet:        "ctx-chain",
			wantErr:           false,
		},
		{
			name:              "explicit overrides both",
			explicitDevnet:    "explicit-chain",
			explicitNamespace: "production",
			context:           &Context{Namespace: "staging", Devnet: "ctx-chain"},
			wantNamespace:     "production",
			wantDevnet:        "explicit-chain",
			wantErr:           false,
		},
		{
			name:              "no devnet anywhere - error",
			explicitDevnet:    "",
			explicitNamespace: "",
			context:           nil,
			wantNamespace:     "",
			wantDevnet:        "",
			wantErr:           true,
		},
		{
			name:              "namespace from context, no devnet - error",
			explicitDevnet:    "",
			explicitNamespace: "production",
			context:           nil,
			wantNamespace:     "",
			wantDevnet:        "",
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNs, gotDevnet, err := Resolve(tt.explicitDevnet, tt.explicitNamespace, tt.context)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Resolve() expected error, got nil")
				}
				if !errors.Is(err, ErrNoDevnet) {
					t.Errorf("Resolve() error = %v, want ErrNoDevnet", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Resolve() unexpected error = %v", err)
				return
			}

			if gotNs != tt.wantNamespace {
				t.Errorf("Resolve() namespace = %q, want %q", gotNs, tt.wantNamespace)
			}
			if gotDevnet != tt.wantDevnet {
				t.Errorf("Resolve() devnet = %q, want %q", gotDevnet, tt.wantDevnet)
			}
		})
	}
}

func TestNoDevnetError(t *testing.T) {
	t.Run("error message without suggestion", func(t *testing.T) {
		err := NewNoDevnetError("")
		msg := err.Error()
		expected := "no devnet specified and no context set. Run 'dvb use <devnet>' to set context"
		if msg != expected {
			t.Errorf("NoDevnetError.Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("error message with suggestion", func(t *testing.T) {
		suggestion := "Found 1 devnet: my-devnet\n\nRun: dvb use my-devnet"
		err := NewNoDevnetError(suggestion)
		msg := err.Error()
		expected := "no devnet specified and no context set\n\n" + suggestion
		if msg != expected {
			t.Errorf("NoDevnetError.Error() = %q, want %q", msg, expected)
		}
	})

	t.Run("errors.Is compatibility with ErrNoDevnet", func(t *testing.T) {
		err := NewNoDevnetError("some suggestion")
		if !errors.Is(err, ErrNoDevnet) {
			t.Errorf("errors.Is(NoDevnetError, ErrNoDevnet) = false, want true")
		}
	})

	t.Run("errors.Is with nil target returns false", func(t *testing.T) {
		err := NewNoDevnetError("some suggestion")
		if errors.Is(err, nil) {
			t.Errorf("errors.Is(NoDevnetError, nil) = true, want false")
		}
	})
}
