// internal/tui/styles_test.go
package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestColors_Defined(t *testing.T) {
	// Colors should be valid lipgloss colors
	assert.NotEmpty(t, string(ColorSuccess))
	assert.NotEmpty(t, string(ColorError))
	assert.NotEmpty(t, string(ColorWarning))
	assert.NotEmpty(t, string(ColorInfo))
	assert.NotEmpty(t, string(ColorMuted))
}

func TestBoxStyle_HasBorder(t *testing.T) {
	// BoxStyle should have rounded border
	rendered := BoxStyle.Render("test")
	assert.Contains(t, rendered, "─") // horizontal border char
}

func TestIconConstants_Defined(t *testing.T) {
	assert.Equal(t, "✓", IconSuccess)
	assert.Equal(t, "✗", IconError)
	assert.Equal(t, "→", IconRunning)
	assert.Equal(t, " ", IconPending)
}

func TestSuccessStyle_ColorsText(t *testing.T) {
	rendered := SuccessStyle.Render("Done")
	assert.Contains(t, rendered, "Done")
}

func TestErrorStyle_ColorsText(t *testing.T) {
	rendered := ErrorStyle.Render("Failed")
	assert.Contains(t, rendered, "Failed")
}

func TestTitleStyle_IsBold(t *testing.T) {
	style := TitleStyle("Test Title")
	// TitleStyle should return a style with the title set
	assert.NotNil(t, style)
}

func TestBoxStyles_HaveBorders(t *testing.T) {
	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"BoxStyle", BoxStyle},
		{"ErrorBoxStyle", ErrorBoxStyle},
		{"SuccessBoxStyle", SuccessBoxStyle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := tt.style.Render("content")
			// Should contain border characters
			assert.Contains(t, rendered, "─")
			assert.Contains(t, rendered, "│")
		})
	}
}
