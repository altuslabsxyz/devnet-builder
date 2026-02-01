// internal/tui/components/steplist.go
package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altuslabsxyz/devnet-builder/internal/tui"
)

// StepStatus represents the status of a step
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepCompleted
	StepFailed
)

// Step represents a single step in a process
type Step struct {
	Name     string
	Status   StepStatus
	Detail   string
	Progress *ProgressModel // optional embedded progress
}

// StepListModel manages a list of steps
type StepListModel struct {
	Steps   []Step
	spinner SpinnerModel
}

// NewStepListModel creates an empty step list
func NewStepListModel() StepListModel {
	return StepListModel{
		Steps:   []Step{},
		spinner: NewSpinnerModel(""),
	}
}

// AddStep adds a new pending step
func (m *StepListModel) AddStep(name string) {
	m.Steps = append(m.Steps, Step{
		Name:   name,
		Status: StepPending,
	})
}

// SetStatus updates the status of a step by index
func (m *StepListModel) SetStatus(index int, status StepStatus) {
	if index >= 0 && index < len(m.Steps) {
		m.Steps[index].Status = status
	}
}

// SetDetail updates the detail of a step by index
func (m *StepListModel) SetDetail(index int, detail string) {
	if index >= 0 && index < len(m.Steps) {
		m.Steps[index].Detail = detail
	}
}

// SetProgress attaches a progress model to a step
func (m *StepListModel) SetProgress(index int, progress *ProgressModel) {
	if index >= 0 && index < len(m.Steps) {
		m.Steps[index].Progress = progress
	}
}

// FindStepByName returns the index of a step by name, or -1 if not found
func (m *StepListModel) FindStepByName(name string) int {
	for i, step := range m.Steps {
		if step.Name == name {
			return i
		}
	}
	return -1
}

// Init implements tea.Model
func (m StepListModel) Init() tea.Cmd {
	return m.spinner.Init()
}

// Update implements tea.Model
func (m StepListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	spinnerModel, cmd := m.spinner.Update(msg)
	m.spinner = spinnerModel.(SpinnerModel)
	return m, cmd
}

// View implements tea.Model
func (m StepListModel) View() string {
	var b strings.Builder

	for _, step := range m.Steps {
		b.WriteString(m.renderStep(step))
		b.WriteString("\n")
	}

	return strings.TrimSuffix(b.String(), "\n")
}

func (m StepListModel) renderStep(step Step) string {
	var prefix string
	var style lipgloss.Style

	switch step.Status {
	case StepPending:
		prefix = "  "
		style = tui.MutedStyle
	case StepRunning:
		prefix = tui.RunningStyle.Render("→")
		style = lipgloss.NewStyle().Foreground(tui.ColorInfo)
	case StepCompleted:
		prefix = tui.SuccessStyle.Render("✓")
		style = lipgloss.NewStyle().Foreground(tui.ColorSuccess)
	case StepFailed:
		prefix = tui.ErrorStyle.Render("✗")
		style = lipgloss.NewStyle().Foreground(tui.ColorError)
	}

	name := style.Render(step.Name)

	// Add detail if present
	detail := ""
	if step.Detail != "" {
		detail = tui.MutedStyle.Render(fmt.Sprintf(" (%s)", step.Detail))
	}

	// Add progress if present and running
	progress := ""
	if step.Status == StepRunning && step.Progress != nil {
		progress = "\n    " + step.Progress.View()
	}

	return prefix + name + detail + progress
}
