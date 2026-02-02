// internal/tui/views/provision.go
package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/altuslabsxyz/devnet-builder/internal/tui"
	"github.com/altuslabsxyz/devnet-builder/internal/tui/components"
)

// Message types for provision view
type StepProgressMsg struct {
	StepName   string
	StepStatus string // "running", "completed", "failed"
	Current    int64
	Total      int64
	Unit       string
	Detail     string
	Speed      float64
}

type ProvisionCompleteMsg struct {
	Error error
}

type ProvisionErrorMsg struct {
	Error error
}

// ProvisionModel is the TUI model for provision command
type ProvisionModel struct {
	DevnetName string
	Network    string
	Steps      components.StepListModel
	Box        components.BoxModel
	Done       bool
	Error      error
	width      int
	height     int
}

// Standard provision steps
var defaultSteps = []string{
	"Building binary",
	"Downloading snapshot",
	"Extracting archive",
	"Exporting state",
	"Initializing nodes",
	"Starting nodes",
}

// NewProvisionModel creates a new provision TUI model
func NewProvisionModel(devnetName, network string) ProvisionModel {
	steps := components.NewStepListModel()
	for _, name := range defaultSteps {
		steps.AddStep(name)
	}

	box := components.NewBoxModel(
		fmt.Sprintf("Provision: %s", devnetName),
		fmt.Sprintf("Network: %s\nStatus: Initializing...", network),
	)

	return ProvisionModel{
		DevnetName: devnetName,
		Network:    network,
		Steps:      steps,
		Box:        box,
		width:      80,
		height:     24,
	}
}

// Init implements tea.Model
func (m ProvisionModel) Init() tea.Cmd {
	return tea.Batch(
		m.Steps.Init(),
	)
}

// Update implements tea.Model
func (m ProvisionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.Done = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.Box.Width = msg.Width - 4

	case StepProgressMsg:
		m.handleStepProgress(msg)

	case ProvisionCompleteMsg:
		m.Done = true
		if msg.Error != nil {
			m.Error = msg.Error
		}
		return m, tea.Quit

	case ProvisionErrorMsg:
		m.Error = msg.Error
		m.Done = true
		return m, tea.Quit
	}

	// Update child components
	stepsModel, stepsCmd := m.Steps.Update(msg)
	m.Steps = stepsModel.(components.StepListModel)
	cmds = append(cmds, stepsCmd)

	return m, tea.Batch(cmds...)
}

func (m *ProvisionModel) handleStepProgress(msg StepProgressMsg) {
	idx := m.Steps.FindStepByName(msg.StepName)
	if idx == -1 {
		// Step not found, add it dynamically
		m.Steps.AddStep(msg.StepName)
		idx = len(m.Steps.Steps) - 1
	}

	// Update step status
	switch msg.StepStatus {
	case "running":
		m.Steps.SetStatus(idx, components.StepRunning)
		// Add progress if bytes-based
		if msg.Total > 0 {
			progress := components.NewProgressModel(msg.StepName, msg.Total, msg.Current)
			progress.ShowSpeed = true
			progress.ShowETA = true
			progress.Speed = msg.Speed
			m.Steps.SetProgress(idx, &progress)
		}
	case "completed":
		m.Steps.SetStatus(idx, components.StepCompleted)
		m.Steps.SetProgress(idx, nil) // clear progress
	case "failed":
		m.Steps.SetStatus(idx, components.StepFailed)
		m.Steps.SetProgress(idx, nil)
	}

	if msg.Detail != "" {
		m.Steps.SetDetail(idx, msg.Detail)
	}
}

// View implements tea.Model
func (m ProvisionModel) View() string {
	var b strings.Builder

	// Header box
	m.Box.SetContent(fmt.Sprintf("Network: %s\nStatus: %s",
		m.Network,
		m.statusText(),
	))
	b.WriteString(m.Box.View())
	b.WriteString("\n\n")

	// Steps
	b.WriteString(m.Steps.View())
	b.WriteString("\n")

	// Error message if present
	if m.Error != nil {
		b.WriteString("\n")
		errorBox := components.NewErrorBoxModel("Error", m.Error.Error())
		b.WriteString(errorBox.View())
	}

	// Done message
	if m.Done && m.Error == nil {
		b.WriteString("\n")
		successBox := components.NewSuccessBoxModel("Complete",
			fmt.Sprintf("Devnet %s provisioned successfully", m.DevnetName))
		b.WriteString(successBox.View())
	}

	return b.String()
}

func (m ProvisionModel) statusText() string {
	if m.Error != nil {
		return tui.ErrorStyle.Render("Failed")
	}
	if m.Done {
		return tui.SuccessStyle.Render("Complete")
	}
	return "Provisioning..."
}

// GetError returns any error that occurred
func (m ProvisionModel) GetError() error {
	return m.Error
}

// IsDone returns whether provisioning is complete
func (m ProvisionModel) IsDone() bool {
	return m.Done
}
