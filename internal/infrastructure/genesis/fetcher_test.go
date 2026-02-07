package genesis

import (
	"strings"
	"testing"
)

func TestExtractGenesisJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantField string // field that should be present in result
	}{
		{
			name:      "clean genesis output",
			input:     `{"chain_id":"test-1","app_state":{}}`,
			wantErr:   false,
			wantField: "chain_id",
		},
		{
			name:      "genesis with preceding log lines",
			input:     "WARNING: some log message\nINFO: starting export\n" + `{"chain_id":"test-1","app_state":{}}`,
			wantErr:   false,
			wantField: "chain_id",
		},
		{
			name:      "genesis with preceding JSON log line",
			input:     `{"level":"info","msg":"starting export"}` + "\n" + `{"chain_id":"test-1","app_state":{}}`,
			wantErr:   false,
			wantField: "chain_id",
		},
		{
			name:    "only non-genesis JSON",
			input:   `{"level":"info","msg":"export complete"}`,
			wantErr: true,
		},
		{
			name:    "no JSON at all",
			input:   "Some random output\nwith no JSON\n",
			wantErr: true,
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: true,
		},
		{
			name:      "genesis with only app_state (no chain_id at top level)",
			input:     `{"app_state":{"bank":{},"staking":{}}}`,
			wantErr:   false,
			wantField: "app_state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractGenesisJSON([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil with result: %s", string(result))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantField != "" && !strings.Contains(string(result), tt.wantField) {
				t.Errorf("result should contain %q, got: %s", tt.wantField, string(result))
			}
		})
	}
}
