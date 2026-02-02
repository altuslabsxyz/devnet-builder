// internal/tui/components/spinner.go
package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altuslabsxyz/devnet-builder/internal/tui"
)

// SpinnerModel wraps bubbles spinner with a message
type SpinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
}

// NewSpinnerModel creates a new spinner with the given message
func NewSpinnerModel(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(tui.ColorInfo)
	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

// Init implements tea.Model
func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model
func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, nil
	}

	if tickMsg, ok := msg.(spinner.TickMsg); ok {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tickMsg)
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model
func (m SpinnerModel) View() string {
	if m.done {
		return ""
	}
	return m.spinner.View() + " " + m.message
}

// SetMessage updates the spinner message
func (m *SpinnerModel) SetMessage(message string) {
	m.message = message
}

// Stop marks the spinner as done
func (m *SpinnerModel) Stop() {
	m.done = true
}
