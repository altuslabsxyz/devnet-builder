// internal/tui/views/provision_test.go
package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestProvisionModel_Init(t *testing.T) {
	m := NewProvisionModel("my-devnet", "cosmos")
	cmd := m.Init()
	assert.NotNil(t, cmd, "should return initial command")
}

func TestProvisionModel_View_ShowsDevnetName(t *testing.T) {
	m := NewProvisionModel("my-devnet", "cosmos")
	view := m.View()
	assert.Contains(t, view, "my-devnet")
}

func TestProvisionModel_View_ShowsSteps(t *testing.T) {
	m := NewProvisionModel("my-devnet", "cosmos")
	view := m.View()
	assert.Contains(t, view, "Building binary")
}

func TestProvisionModel_Update_StepProgress(t *testing.T) {
	m := NewProvisionModel("my-devnet", "cosmos")

	// Simulate receiving a step progress message
	msg := StepProgressMsg{
		StepName:   "Downloading snapshot",
		StepStatus: "running",
		Current:    50,
		Total:      100,
	}

	newModel, _ := m.Update(msg)
	pm := newModel.(ProvisionModel)

	view := pm.View()
	assert.Contains(t, view, "Downloading snapshot")
}

func TestProvisionModel_Update_Completed(t *testing.T) {
	m := NewProvisionModel("my-devnet", "cosmos")

	msg := ProvisionCompleteMsg{}
	newModel, cmd := m.Update(msg)
	pm := newModel.(ProvisionModel)

	assert.True(t, pm.Done)
	// tea.Quit() returns a tea.Cmd that produces tea.QuitMsg when executed
	assert.NotNil(t, cmd, "should return quit command")
	// Execute the command to verify it produces QuitMsg
	quitMsg := cmd()
	_, ok := quitMsg.(tea.QuitMsg)
	assert.True(t, ok, "command should produce QuitMsg")
}
