// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

// Color palette - matches current fatih/color semantics
var (
	ColorSuccess = lipgloss.Color("#22c55e") // Green
	ColorError   = lipgloss.Color("#ef4444") // Red
	ColorWarning = lipgloss.Color("#eab308") // Yellow
	ColorInfo    = lipgloss.Color("#06b6d4") // Cyan
	ColorMuted   = lipgloss.Color("#6b7280") // Gray
)

// Text styles
var (
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			SetString("✓ ")

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			SetString("✗ ")

	RunningStyle = lipgloss.NewStyle().
			Foreground(ColorInfo).
			SetString("→ ")

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BoldStyle = lipgloss.NewStyle().
			Bold(true)
)

// Box styles
var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorInfo).
			Padding(0, 1)

	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorError).
			Padding(0, 1)

	SuccessBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Padding(0, 1)
)

// TitleStyle creates a styled title for boxes
func TitleStyle(title string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorInfo).
		Bold(true).
		SetString(title)
}
