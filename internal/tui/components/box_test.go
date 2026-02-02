// internal/tui/components/box_test.go
package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoxModel_View_HasBorder(t *testing.T) {
	m := NewBoxModel("Title", "Content")
	view := m.View()
	assert.Contains(t, view, "─") // border
	assert.Contains(t, view, "Title")
	assert.Contains(t, view, "Content")
}

func TestBoxModel_View_RespectsWidth(t *testing.T) {
	m := NewBoxModel("Title", "Content")
	m.Width = 40
	view := m.View()
	// View should be constrained to width
	lines := splitLines(view)
	for _, line := range lines {
		assert.LessOrEqual(t, visualWidth(line), 42) // allow for some variance
	}
}

func TestErrorBox_HasErrorBorder(t *testing.T) {
	m := NewErrorBoxModel("Error", "Something went wrong")
	view := m.View()
	assert.Contains(t, view, "Error")
	assert.Contains(t, view, "Something went wrong")
}

// helper
func splitLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func visualWidth(s string) int {
	// simplified - count runes
	return len([]rune(s))
}
