package core

import "testing"

func TestMatchesLevel(t *testing.T) {
	tests := []struct {
		name        string
		entryLevel  string
		targetLevel string
		want        bool
	}{
		// Exact matches
		{"debug matches debug", "debug", "debug", true},
		{"info matches info", "info", "info", true},
		{"warn matches warn", "warn", "warn", true},
		{"error matches error", "error", "error", true},

		// Hierarchy: debug < info < warn < error
		{"debug does not match info target", "debug", "info", false},
		{"info matches debug target", "info", "debug", true},
		{"warn matches info target", "warn", "info", true},
		{"error matches warn target", "error", "warn", true},
		{"info does not match error target", "info", "error", false},

		// Abbreviations
		{"inf matches info", "inf", "info", true},
		{"wrn matches warn", "wrn", "warn", true},
		{"err matches error", "err", "error", true},
		{"dbg matches debug", "dbg", "debug", true},

		// Case insensitivity
		{"INFO matches info", "INFO", "info", true},
		{"Error matches error", "Error", "error", true},

		// Unknown levels are included
		{"unknown is included", "unknown", "info", true},
		{"custom level is included", "custom", "error", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesLevel(tt.entryLevel, tt.targetLevel)
			if got != tt.want {
				t.Errorf("matchesLevel(%q, %q) = %v, want %v",
					tt.entryLevel, tt.targetLevel, got, tt.want)
			}
		})
	}
}
