// internal/tui/components/spinner_test.go
package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestSpinnerModel_Init(t *testing.T) {
	m := NewSpinnerModel("Loading...")
	cmd := m.Init()
	assert.NotNil(t, cmd, "spinner should return tick command on init")
}

func TestSpinnerModel_View(t *testing.T) {
	m := NewSpinnerModel("Loading...")
	view := m.View()
	assert.Contains(t, view, "Loading...")
}

func TestSpinnerModel_Update_TickMsg(t *testing.T) {
	m := NewSpinnerModel("Loading...")
	// Simulate a spinner tick
	newModel, _ := m.Update(m.spinner.Tick())
	sm := newModel.(SpinnerModel)
	assert.NotNil(t, sm)
}

// Ensure tea.Model interface is satisfied
var _ tea.Model = SpinnerModel{}
