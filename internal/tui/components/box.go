// internal/tui/components/box.go
package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altuslabsxyz/devnet-builder/internal/tui"
)

// BoxModel represents a bordered box with title and content
type BoxModel struct {
	Title   string
	Content string
	Width   int
	style   lipgloss.Style
}

// NewBoxModel creates a standard info box
func NewBoxModel(title, content string) BoxModel {
	return BoxModel{
		Title:   title,
		Content: content,
		Width:   60,
		style:   tui.BoxStyle,
	}
}

// NewErrorBoxModel creates an error-styled box
func NewErrorBoxModel(title, content string) BoxModel {
	return BoxModel{
		Title:   title,
		Content: content,
		Width:   60,
		style:   tui.ErrorBoxStyle,
	}
}

// NewSuccessBoxModel creates a success-styled box
func NewSuccessBoxModel(title, content string) BoxModel {
	return BoxModel{
		Title:   title,
		Content: content,
		Width:   60,
		style:   tui.SuccessBoxStyle,
	}
}

// Init implements tea.Model
func (m BoxModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m BoxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.Width = wsm.Width - 4
		if m.Width < 40 {
			m.Width = 40
		}
	}
	return m, nil
}

// View implements tea.Model
func (m BoxModel) View() string {
	// Build title line
	titleLine := ""
	if m.Title != "" {
		titleStyle := lipgloss.NewStyle().Bold(true)
		titleLine = titleStyle.Render(m.Title)
	}

	// Apply box style with width
	boxStyle := m.style.Width(m.Width)

	if titleLine != "" {
		// Render box with title at top
		return boxStyle.Render(titleLine + "\n" + m.Content)
	}
	return boxStyle.Render(m.Content)
}

// SetContent updates the box content
func (m *BoxModel) SetContent(content string) {
	m.Content = content
}

// SetTitle updates the box title
func (m *BoxModel) SetTitle(title string) {
	m.Title = title
}
