package manage

import "testing"

func TestParsePortMapping(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		nodeIndex     int
		wantLocal     int
		wantContainer int
		wantErr       bool
	}{
		// Single port (same local and container)
		{"single port 26657", "26657", 0, 26657, 26657, false},
		{"single port 1317", "1317", 1, 1317, 1317, false},
		{"single port 9090", "9090", 2, 9090, 9090, false},

		// local:container format
		{"local:container 8080:26657", "8080:26657", 0, 8080, 26657, false},
		{"local:container 3000:1317", "3000:1317", 1, 3000, 1317, false},

		// Error cases
		{"invalid port string", "invalid", 0, 0, 0, true},
		{"port too high", "99999", 0, 0, 0, true},
		{"port zero", "0", 0, 0, 0, true},
		{"negative port", "-1", 0, 0, 0, true},
		{"invalid local port", "abc:26657", 0, 0, 0, true},
		{"invalid container port", "8080:abc", 0, 0, 0, true},
		{"too many colons", "8080:26657:extra", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping, err := parsePortMapping(tt.input, tt.nodeIndex)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parsePortMapping(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parsePortMapping(%q) unexpected error: %v", tt.input, err)
				return
			}
			if mapping.LocalPort != tt.wantLocal {
				t.Errorf("parsePortMapping(%q).LocalPort = %d, want %d", tt.input, mapping.LocalPort, tt.wantLocal)
			}
			if mapping.ContainerPort != tt.wantContainer {
				t.Errorf("parsePortMapping(%q).ContainerPort = %d, want %d", tt.input, mapping.ContainerPort, tt.wantContainer)
			}
			if mapping.NodeIndex != tt.nodeIndex {
				t.Errorf("parsePortMapping(%q).NodeIndex = %d, want %d", tt.input, mapping.NodeIndex, tt.nodeIndex)
			}
		})
	}
}

func TestParsePortMappings(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		nodeIndex int
		wantCount int
		wantErr   bool
	}{
		{"single port", []string{"26657"}, 0, 1, false},
		{"multiple ports", []string{"26657", "1317", "9090"}, 0, 3, false},
		{"mixed formats", []string{"26657", "8080:1317"}, 0, 2, false},
		{"empty args", []string{}, 0, 0, false},
		{"with invalid port", []string{"26657", "invalid"}, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappings, err := parsePortMappings(tt.args, tt.nodeIndex)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parsePortMappings(%v) expected error, got nil", tt.args)
				}
				return
			}
			if err != nil {
				t.Errorf("parsePortMappings(%v) unexpected error: %v", tt.args, err)
				return
			}
			if len(mappings) != tt.wantCount {
				t.Errorf("parsePortMappings(%v) returned %d mappings, want %d", tt.args, len(mappings), tt.wantCount)
			}
		})
	}
}

func TestParseNodeArgPF(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		numNodes  int
		wantIndex int
		wantErr   bool
	}{
		{"numeric 0", "0", 4, 0, false},
		{"numeric 1", "1", 4, 1, false},
		{"node0 format", "node0", 4, 0, false},
		{"node1 format", "node1", 4, 1, false},
		{"out of range", "5", 4, 0, true},
		{"invalid string", "invalid", 4, 0, true},
		{"empty", "", 4, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, err := parseNodeArgPF(tt.input, tt.numNodes)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseNodeArgPF(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseNodeArgPF(%q) unexpected error: %v", tt.input, err)
				return
			}
			if index != tt.wantIndex {
				t.Errorf("parseNodeArgPF(%q) = %d, want %d", tt.input, index, tt.wantIndex)
			}
		})
	}
}
