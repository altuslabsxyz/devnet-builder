package core

import "testing"

func TestParseNodeArg(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		numNodes  int
		wantIndex int
		wantErr   bool
	}{
		{"numeric 0", "0", 4, 0, false},
		{"numeric 1", "1", 4, 1, false},
		{"numeric 3", "3", 4, 3, false},
		{"node0 format", "node0", 4, 0, false},
		{"node1 format", "node1", 4, 1, false},
		{"node3 format", "node3", 4, 3, false},
		{"out of range numeric", "5", 4, 0, true},
		{"out of range node", "node5", 4, 0, true},
		{"negative", "-1", 4, 0, true},
		{"invalid string", "invalid", 4, 0, true},
		{"empty", "", 4, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, err := parseNodeArg(tt.input, tt.numNodes)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseNodeArg(%q, %d) expected error, got nil", tt.input, tt.numNodes)
				}
				return
			}
			if err != nil {
				t.Errorf("parseNodeArg(%q, %d) unexpected error: %v", tt.input, tt.numNodes, err)
				return
			}
			if index != tt.wantIndex {
				t.Errorf("parseNodeArg(%q, %d) = %d, want %d", tt.input, tt.numNodes, index, tt.wantIndex)
			}
		})
	}
}
