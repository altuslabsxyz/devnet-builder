// internal/tui/app_test.go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

type mockModel struct {
	initCalled   bool
	updateCalled bool
	viewCalled   bool
}

func (m *mockModel) Init() tea.Cmd {
	m.initCalled = true
	return nil
}

func (m *mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updateCalled = true
	return m, nil
}

func (m *mockModel) View() string {
	m.viewCalled = true
	return "test view"
}

func TestRunTUI_NonInteractive(t *testing.T) {
	// Non-interactive mode should fall back to simple output
	m := &mockModel{}
	err := RunSimple(m)
	assert.NoError(t, err)
	assert.True(t, m.viewCalled)
}

func TestIsInteractive_TTY(t *testing.T) {
	// This test just ensures the function exists
	_ = IsInteractive()
}
